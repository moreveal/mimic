package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestDocumentAvailableImagesChrome152(t *testing.T) {
	fixture, err := os.ReadFile("testdata/image_reuse_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := os.ReadFile("testdata/image_reuse_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Rows []struct {
			Control  string `json:"control"`
			Loading  string `json:"loading"`
			Result   any    `json:"result"`
			Requests int    `json:"requests"`
		} `json:"rows"`
	}
	if err := json.Unmarshal(evidence, &oracle); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	counts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/image" {
			mu.Lock()
			counts[req.URL.RawQuery]++
			mu.Unlock()
			w.Header().Set("Content-Type", "image/svg+xml")
			if strings.HasPrefix(req.URL.RawQuery, "nostore") {
				w.Header().Set("Cache-Control", "no-store")
			} else {
				w.Header().Set("Cache-Control", "max-age=600")
			}
			fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="2" height="1"/>`)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body></body></html>")
	}))
	defer server.Close()
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	for _, row := range oracle.Rows {
		t.Run(row.Control+"/"+row.Loading, func(t *testing.T) {
			expression := "(" + string(fixture) + ")(" + strconv.Quote(row.Control) + "," + strconv.Quote(row.Loading) + ").then(JSON.stringify)"
			value, err := p.Evaluate(ctx, expression)
			if err != nil {
				t.Fatal(err)
			}
			var actual any
			if err := json.Unmarshal([]byte(value.(string)), &actual); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(actual, row.Result) {
				t.Fatalf("observations: got %v want %v", actual, row.Result)
			}
			mu.Lock()
			count := counts[row.Control+row.Loading]
			mu.Unlock()
			if count != row.Requests {
				t.Fatalf("requests: got %d want %d", count, row.Requests)
			}
		})
	}
	// Successful no-store images belong to the old document, not the Context.
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `(async()=>{const image=new Image;image.src='/image?nostoreeager';await image.decode();return image.naturalWidth})()`)
	if err != nil || value != float64(2) {
		t.Fatalf("new document image: %v %v", value, err)
	}
	mu.Lock()
	count := counts["nostoreeager"]
	mu.Unlock()
	if count != 2 {
		t.Fatalf("no-store image leaked between documents: %d requests", count)
	}
}

func TestAvailableImageKeysAndLazyUpdates(t *testing.T) {
	var mu sync.Mutex
	counts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		mu.Lock()
		counts[req.URL.Path]++
		count := counts[req.URL.Path]
		mu.Unlock()
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "image/svg+xml")
		if req.URL.Path == "/cors" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if req.URL.Path == "/retry" && count == 1 {
			fmt.Fprint(w, "broken")
			return
		}
		fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="2" height="1"/>`)
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		mu.Lock()
		clear(counts)
		mu.Unlock()
		navigateCapabilityFixture(t, p)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		expression := `(async()=>{
		const base=` + strconv.Quote(server.URL) + `;
		const load=async(path,cors)=>{const image=new Image;if(cors!==undefined)image.crossOrigin=cors;image.src=base+path;return image.decode().then(()=>image.naturalWidth,()=> 'error')};
		const out=[];
		out.push(await load('/opaque'),await load('/opaque','anonymous'));
		out.push(await load('/cors','anonymous'),await load('/cors','use-credentials'));
		out.push(await load('/retry'),await load('/retry'));
		for(const remove of [false,true]){
		  const image=new Image;image.loading='LAZY';image.src=base+'/lazy'+remove;
		  await new Promise(resolve=>setTimeout(resolve,20));
		  out.push(image.loading,image.complete,image.naturalWidth);
		  if(remove)image.removeAttribute('loading');else image.loading='eager';
		  await image.decode();out.push(image.loading,image.naturalWidth);
		}
		return out.join(',')})()`
		value, err := p.Evaluate(ctx, expression)
		want := "2,error,2,error,error,2,lazy,false,0,eager,2,lazy,false,0,eager,2"
		if err != nil || value != want {
			t.Fatalf("image lifecycle: %v %v; want %s", value, err, want)
		}
	})
}
