//go:build windows && amd64

package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestIframeSrcdocMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "iframe_srcdoc")
}

// Keep the two evaluations from the saved Chrome repro: collecting retired
// owners between them must not lose the exported descendant WindowProxy.
func TestIframeSrcdocRetainsExportedDescendant(t *testing.T) {
	setup, err := os.ReadFile("testdata/navigation_nested_window_setup.js")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "<!doctype html><body>")
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		if _, err := p.Evaluate(ctx, string(setup)); err != nil {
			t.Fatal(err)
		}
		value, err := p.Evaluate(ctx, `(()=>{const w=globalThis.__retainedNestedWindow;return !!w&&w.window===w&&w.document.title==='retained nested'&&w.document.defaultView===null&&w.document.body!==null&&w.eval('17')===17})()`)
		if err != nil || value != true {
			t.Fatalf("retained srcdoc descendant: %v, %v", value, err)
		}
	})
}
