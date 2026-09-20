package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNavigationRemovesRetiredDescendantTasks(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<!doctype html><body>" + r.URL.Path))
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, page *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := page.Navigate(ctx, server.URL+"/first"); err != nil {
			t.Fatal(err)
		}
		if _, err := page.Evaluate(ctx, `const child=document.createElement('iframe');document.body.appendChild(child);child.contentWindow.eval('setTimeout(()=>{globalThis.oldTimerFired=true},1000)')`); err != nil {
			t.Fatal(err)
		}
		if err := page.Navigate(ctx, server.URL+"/second"); err != nil {
			t.Fatal(err)
		}
		if err := page.AdvanceTime(ctx, 0); err != nil {
			t.Fatal(err)
		}
		if err := page.AdvanceTime(ctx, 2*time.Second); err != nil {
			t.Fatalf("retired realm task was pumped: %v", err)
		}
		if len(page.frames) != 1 || len(page.Top.children) != 0 {
			t.Fatalf("retired frame remains active: frames=%d children=%d", len(page.frames), len(page.Top.children))
		}
	})
}

func TestChildNavigationRemovesRetiredGrandchildTasks(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<!doctype html><body>replacement"))
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, page *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, err := page.Evaluate(ctx, fmt.Sprintf(`(async()=>{
 const child=document.createElement('iframe');document.body.appendChild(child);
 child.contentWindow.eval("const grandchild=document.createElement('iframe');document.body.appendChild(grandchild);grandchild.contentWindow.eval('setTimeout(()=>{globalThis.oldTimerFired=true},1000)')");
 await new Promise(resolve=>{child.onload=resolve;child.src=%q});return true;
 })()`, server.URL))
		if err != nil {
			t.Fatal(err)
		}
		if err := page.AdvanceTime(ctx, 2*time.Second); err != nil {
			t.Fatalf("retired grandchild task was pumped: %v", err)
		}
		if len(page.frames) != 2 || len(page.Top.children) != 1 {
			t.Fatalf("unexpected active frames: %d", len(page.frames))
		}
		for _, child := range page.Top.children {
			if len(child.children) != 0 {
				t.Fatal("retired grandchild remains active")
			}
		}
	})
}
