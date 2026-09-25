package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"github.com/moreveal/mimic/internal/network"
)

// Controlled Context workload. Request counts are server observations, while
// retained bytes are the Context's shared HTTP/CDP body store before/after
// teardown; neither should be mistaken for socket wire bytes or process RSS.
func BenchmarkResourcePolicyBrowserContext(b *testing.B) {
	var requests atomic.Int64
	var html strings.Builder
	html.WriteString("<html><body>")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&html, `<img src="/image/%d">`, i)
	}
	html.WriteString("</body></html>")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Cache-Control", "no-store")
		if strings.HasPrefix(r.URL.Path, "/image/") {
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="2" height="1"/>`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html.String()))
	}))
	defer server.Close()
	browser, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		b.Fatal(err)
	}
	defer browser.Close()
	// Warm the shared bootstrap and transport before the first measured mode.
	warm := browser.NewContext()
	warmPage, err := warm.NewPage()
	if err != nil {
		b.Fatal(err)
	}
	warmCtx, warmCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := warmPage.Navigate(warmCtx, server.URL); err != nil {
		b.Fatal(err)
	}
	warmCancel()
	if err := warm.Close(); err != nil {
		b.Fatal(err)
	}
	blocked := false
	for _, scenario := range []struct {
		name   string
		policy *network.ResourcePolicy
	}{
		{name: "off"},
		{name: "reportOnly", policy: &network.ResourcePolicy{ReportOnly: true, Rules: []network.ResourceRule{{ID: "visual", Match: network.ResourceMatch{Kinds: []string{"image"}}, Work: network.ResourceWork{CacheRead: &blocked, Network: &blocked}}}}},
		{name: "active", policy: &network.ResourcePolicy{Rules: []network.ResourceRule{{ID: "visual", Match: network.ResourceMatch{Kinds: []string{"image"}}, Work: network.ResourceWork{CacheRead: &blocked, Network: &blocked}}}}},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			requests.Store(0)
			var retainedBefore, retainedAfter int64
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				c := browser.NewContext()
				if scenario.policy != nil {
					if _, err := c.UpdateResourcePolicy(*scenario.policy); err != nil {
						b.Fatal(err)
					}
				}
				page, err := c.NewPage()
				if err != nil {
					b.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				err = page.Navigate(ctx, server.URL)
				cancel()
				if err != nil {
					b.Fatal(err)
				}
				retainedBefore += c.network.BodyStorageStats().ResidentBytes
				if err := c.Close(); err != nil {
					b.Fatal(err)
				}
				retainedAfter += c.network.BodyStorageStats().ResidentBytes
			}
			b.StopTimer()
			b.ReportMetric(float64(requests.Load())/float64(b.N), "requests/op")
			b.ReportMetric(float64(retainedBefore)/float64(b.N), "retained_B/op")
			b.ReportMetric(float64(retainedAfter)/float64(b.N), "teardown_B/op")
		})
	}
}
