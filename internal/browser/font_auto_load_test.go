package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRenderedCSSFontStartsResourceLoad(t *testing.T) {
	parallelBrowserTest(t)
	var fontRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/":
			fmt.Fprint(w, `<!doctype html><link rel="stylesheet" href="/style.css"><p>rendered text</p>`)
		case "/style.css":
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, `@font-face{font-family:Probe;src:url('/probe.woff2')}p{font-family:Probe}`)
		case "/probe.woff2":
			fontRequests.Add(1)
			w.Header().Set("Content-Type", "font/woff2")
			fmt.Fprint(w, "invalid bytes are enough to observe the request")
		default:
			http.NotFound(w, request)
		}
	}))
	defer server.Close()

	p := testPage(t)
	defer p.Close()
	if err := p.Navigate(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for fontRequests.Load() == 0 && time.Now().Before(deadline) {
		if _, err := p.Evaluate(context.Background(), `new Promise(resolve=>setTimeout(resolve,10))`); err != nil {
			t.Fatal(err)
		}
	}
	if got := fontRequests.Load(); got != 1 {
		t.Fatalf("rendered CSS font issued %d requests", got)
	}
}

func TestFontFaceSetEventHandlerProperties(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	defer p.Close()
	value, err := p.Evaluate(context.Background(), `(async()=>{
		const set=document.fonts, seen=[];
		const first=e=>seen.push(e.type+':first'), second=e=>seen.push(e.type+':second');
		set.onloading=first;
		set.onloading=second;
		set.onloadingdone=e=>seen.push(e.type);
		set.onloadingerror=e=>seen.push(e.type);
		const face=new FontFace('HandlerProbe','url(data:font/woff2;base64,AA==)');
		set.add(face);
		await face.load().catch(()=>{});
		await new Promise(resolve=>setTimeout(resolve,20));
		const descriptor=Object.getOwnPropertyDescriptor(FontFaceSet.prototype,'onloading');
		return [seen.join(','),set.onloading===second,descriptor.enumerable,descriptor.configurable];
	})()`)
	if err != nil {
		t.Fatal(err)
	}
	want := []any{"loading:second,loadingdone,loadingerror", true, true, true}
	got := value.([]any)
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("event handler surface: got %#v want %#v", got, want)
		}
	}
}
