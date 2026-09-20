package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/imageresource"
)

func assertAvailableImageAccounting(t *testing.T, cache *availableImageCache) {
	t.Helper()
	bytes, entries := 0, 0
	for element := cache.order.Front(); element != nil; element = element.Next() {
		entry := element.Value.(availableImageEntry)
		if cache.entries[entry.key] != element {
			t.Fatal("FIFO/map disagree")
		}
		bytes += entry.bytes
		entries++
	}
	if bytes != cache.bytes || entries != len(cache.entries) || bytes > availableImageCacheBytes || entries > availableImageCacheEntries {
		t.Fatalf("invalid cache accounting: %d/%d bytes, %d/%d entries", bytes, cache.bytes, entries, len(cache.entries))
	}
}

func TestAvailableImageCacheBoundsAndReplacement(t *testing.T) {
	parallelBrowserTest(t)
	cache := &availableImageCache{}
	key := preloadKey{url: "image", destination: "image", mode: "cors", credentials: "include"}
	first := availableImage{resource: imageresource.New(make([]byte, 1024), ""), originClean: false}
	cache.put(key, first)
	if cache.bytes != 1024+len(key.url)+len(key.destination)+len(key.mode)+len(key.credentials)+256 {
		t.Fatal("pixel capacity/key bytes not accounted")
	}
	for i := 0; i < availableImageCacheEntries+5; i++ {
		cache.put(key, availableImage{resource: imageresource.New([]byte(fmt.Sprint(i)), ""), originClean: true})
		assertAvailableImageAccounting(t, cache)
	}
	if len(cache.entries) != 1 || cache.order.Len() != 1 {
		t.Fatal("replacement retained historical entries")
	}
	if image, ok := cache.get(key); !ok || !image.originClean || image.resource == nil {
		t.Fatal("replacement lost latest decoded state")
	}
	for i := 0; i < availableImageCacheEntries; i++ {
		cache.put(preloadKey{url: fmt.Sprint(i)}, availableImage{resource: imageresource.New(nil, "image/svg+xml")})
		assertAvailableImageAccounting(t, cache)
	}
	if _, ok := cache.get(key); ok {
		t.Fatal("oldest entry survived count limit")
	}
	if first.resource.RetainedBytes() != 1024 {
		t.Fatal("eviction mutated separately owned image")
	}
}

func TestAvailableImageCachePixelBudgetAndOversizedAdmission(t *testing.T) {
	parallelBrowserTest(t)
	cache := &availableImageCache{}
	keyA, keyB := preloadKey{url: "a"}, preloadKey{url: "b"}
	// Sharing the backing pixels must not undercount retained entries. Capacity
	// exceeds the exposed length, as can happen with a sliced decoded buffer.
	image := availableImage{resource: imageresource.New(make([]byte, availableImageCacheBytes/2), "")}
	cache.put(keyA, image)
	cache.put(keyB, image)
	assertAvailableImageAccounting(t, cache)
	if _, ok := cache.get(keyA); ok {
		t.Fatal("pixel budget did not evict oldest image")
	}
	if got, ok := cache.get(keyB); !ok || got.resource != image.resource {
		t.Fatal("new image was not retained")
	}
	oversized := availableImage{resource: imageresource.New(make([]byte, availableImageCacheBytes), "")}
	cache.put(keyB, oversized)
	assertAvailableImageAccounting(t, cache)
	if _, ok := cache.get(keyB); ok || cache.bytes != 0 {
		t.Fatal("oversized replacement retained old or new cache entry")
	}
	cache.put(keyA, availableImage{})
	if len(cache.entries) != 0 {
		t.Fatal("failed image was cached")
	}
}

func TestAvailableImageEvictionPreservesActiveImageAndLazyReload(t *testing.T) {
	serialBrowserTest(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/image" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body></body></html>")
			return
		}
		requests.Add(1)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="2" height="1"/>`)
	}))
	defer server.Close()
	p := newAsyncModulePage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `(async()=>{globalThis.activeImage=new Image;activeImage.src='/image';await activeImage.decode();return activeImage.naturalWidth})()`)
	if err != nil || value != float64(2) {
		t.Fatalf("initial image: %v %v", value, err)
	}
	retired := p.Top.Realm
	cache := retired.availableImages
	if cache == nil || len(cache.entries) != 1 {
		t.Fatal("initial successful image unavailable")
	}
	for i := 0; i < availableImageCacheEntries; i++ {
		cache.put(preloadKey{url: fmt.Sprintf("filler:%d", i)}, availableImage{resource: imageresource.New(nil, "image/svg+xml")})
	}
	assertAvailableImageAccounting(t, cache)
	value, err = p.Evaluate(ctx, `(async()=>{
	 if(!activeImage.complete||activeImage.naturalWidth!==2)return 'active lost';await activeImage.decode();
	 const lazy=new Image;lazy.loading='lazy';lazy.src='/image';await new Promise(r=>setTimeout(r,0));
	 if(lazy.complete||lazy.naturalWidth!==0)return 'detached lazy loaded';
	 document.body.append(lazy);await lazy.decode();
	 const eager=activeImage.cloneNode(true);await eager.decode();
	 return [activeImage.naturalWidth,lazy.naturalWidth,eager.naturalWidth].join(',')})()`)
	if err != nil || value != "2,2,2" {
		t.Fatalf("eviction lifecycle: %v %v", value, err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("no-store eviction/reuse: %d requests; want 2", got)
	}
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	if retired.availableImages != nil || retired.imageLoads != nil {
		t.Fatal("retired document retained image state")
	}
}
