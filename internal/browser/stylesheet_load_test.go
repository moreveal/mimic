package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"github.com/moreveal/mimic/internal/network"
)

func TestParserStylesheetsBlockScriptAndSurviveResponseEviction(t *testing.T) {
	var active, peak atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".css") {
			n := active.Add(1)
			defer active.Add(-1)
			for old := peak.Load(); n > old && !peak.CompareAndSwap(old, n); old = peak.Load() {
			}
			time.Sleep(40 * time.Millisecond)
			w.Header().Set("Content-Type", "text/css")
			if r.URL.Path == "/a.css" {
				fmt.Fprint(w, "#box{width:300px}")
			} else {
				fmt.Fprint(w, "#box{height:40px}")
			}
			return
		}
		if r.URL.Path != "/" {
			fmt.Fprint(w, "eviction fixture")
			return
		}
		fmt.Fprint(w, `<!doctype html><link rel="stylesheet" href="/a.css"><link rel="stylesheet" href="/b.css"><div id="box"></div><script>const box=document.getElementById('box');window.parserSize=[box.clientWidth,box.clientHeight].join(',')</script>`)
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	if got, err := p.Evaluate(ctx, `parserSize`); err != nil || got != "300,40" {
		t.Fatalf("script ran before styles: %v, %v", got, err)
	}
	if peak.Load() < 2 {
		t.Fatal("independent stylesheets fetched serially")
	}
	for i := 0; i < 140; i++ {
		u, _ := url.Parse(fmt.Sprintf("%s/evict/%d", server.URL, i))
		if _, err := p.loader.Load(ctx, network.Request{URL: u, Initiator: network.Other}); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := p.loader.CompletedURL(server.URL + "/a.css"); ok {
		t.Fatal("fixture did not evict stylesheet response")
	}
	if got, err := p.Evaluate(ctx, `[box.clientWidth,box.clientHeight].join(',')`); err != nil || got != "300,40" {
		t.Fatalf("applied stylesheet lost after CDP history eviction: %v, %v", got, err)
	}
}
