package browser

import (
	"context"
	"fmt"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBlitzDetachedFocusedAncestorClearsNativeState(t *testing.T) {
	serialBrowserTest(t)
	t.Setenv("MIMIC_STYLE_ENGINE", "blitz")
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><style>#box{width:100px}#box:focus-within{width:200px}</style><div id="box"><input></div>`))
	}))
	defer server.Close()
	ctx := context.Background()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `(()=>{const box=document.querySelector('#box'),input=box.firstChild;input.focus();const focused=box.getBoundingClientRect().width;box.remove();input.blur();document.body.getBoundingClientRect();document.body.append(box);return JSON.stringify([focused,box.matches(':focus-within'),box.getBoundingClientRect().width])})()`)
	if err != nil || value != `[200,false,100]` {
		t.Fatalf("retired native focus state: %v %v", value, err)
	}
}

func TestBlitzHistoryChangeUpdatesNewInlineURLBase(t *testing.T) {
	serialBrowserTest(t)
	t.Setenv("MIMIC_STYLE_ENGINE", "blitz")
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<!doctype html><div id="box" style="width:100px"></div>`))
	}))
	defer server.Close()
	ctx := context.Background()
	if err := p.Navigate(ctx, server.URL+"/before/page"); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `(()=>{const box=document.querySelector('#box');box.getBoundingClientRect();history.replaceState({},'', '/after/page');box.style.backgroundImage='url(icon.png)';return getComputedStyle(box).backgroundImage})()`)
	if err != nil || !strings.Contains(fmt.Sprint(value), "/after/icon.png") {
		t.Fatalf("native CSS used stale document URL: %v %v", value, err)
	}
}
