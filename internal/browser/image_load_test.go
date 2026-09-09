package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestDetachedImageLoadCoalescesAndBlocksDocumentLoad(t *testing.T) {
	var obsolete, images atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<script>globalThis.events=[];const img=new Image;img.onload=()=>{events.push('image');document.body.className='loaded';document.body.append(img)};img.onerror=()=>events.push('error');img.src='/obsolete.png';img.src='/pixel.svg';addEventListener('load',()=>events.push('window'));</script>`)
		case "/obsolete.png":
			obsolete.Add(1)
			w.WriteHeader(404)
		case "/pixel.svg":
			images.Add(1)
			w.Header().Set("Content-Type", "image/svg+xml")
			fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`)
		default:
			w.WriteHeader(404)
		}
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `events.join(',')+':'+document.body.className`)
	if err != nil || value != "image,window:loaded" {
		t.Fatalf("load ordering: %v %v", value, err)
	}
	if obsolete.Load() != 0 || images.Load() != 1 {
		t.Fatalf("requests: obsolete=%d image=%d", obsolete.Load(), images.Load())
	}
}

func TestImageReplacementCancelsObsoleteRequest(t *testing.T) {
	started, canceled := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, "<body></body>")
		case "/old":
			close(started)
			<-r.Context().Done()
			close(canceled)
		case "/new":
			w.Header().Set("Content-Type", "image/svg+xml")
			fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`)
		default:
			w.WriteHeader(404)
		}
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Evaluate(ctx, `globalThis.img=new Image;globalThis.events=[];globalThis.loaded=new Promise(resolve=>{img.onload=()=>{events.push('load');resolve()};img.onerror=()=>events.push('error')});img.src='/old';true`); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("old image did not start")
	}
	value, err := p.Evaluate(ctx, `(async()=>{img.src='/new';await loaded;return events.join(',')})()`)
	if err != nil || value != "load" {
		t.Fatalf("replacement events: %v %v", value, err)
	}
	select {
	case <-canceled:
	case <-ctx.Done():
		t.Fatal("old image was not canceled")
	}
}
