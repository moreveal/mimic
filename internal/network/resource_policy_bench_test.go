package network

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/moreveal/mimic/internal/trace"
)

// The local server makes the body read and allocation cost visible without
// depending on a remote site or modifying the frozen workload harness.
func BenchmarkResourcePolicyImageBody(b *testing.B) {
	body := make([]byte, 256<<10)
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(body)
	}))
	defer server.Close()
	target, _ := url.Parse(server.URL)
	no := false
	for _, tc := range []struct {
		name       string
		work       ResourceWork
		reportOnly bool
	}{
		{"off", ResourceWork{}, false},
		{"reportOnly", ResourceWork{CacheRead: &no, Network: &no}, true},
		{"full", ResourceWork{}, false},
		{"headers", ResourceWork{Body: "none", DebugRetain: &no}, false},
		{"prefix", ResourceWork{Body: "prefix", PrefixBytes: 4096, DebugRetain: &no}, false},
		{"block", ResourceWork{CacheRead: &no, Network: &no}, false},
	} {
		b.Run(tc.name, func(b *testing.B) {
			state := &ResourcePolicyState{}
			if tc.name != "off" {
				_, err := state.Update(ResourcePolicy{ReportOnly: tc.reportOnly, Rules: []ResourceRule{{ID: tc.name, Match: ResourceMatch{Kinds: []string{"image"}}, Work: tc.work}}})
				if err != nil {
					b.Fatal(err)
				}
			}
			loader := NewLoader(testEnvironment, NewCookieStore(), trace.New())
			if tc.name != "off" {
				loader.SetResourcePolicy(state)
			}
			defer loader.CloseResponseBodies()
			requests.Store(0)
			var delivered int64
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				response, err := loader.Load(context.Background(), Request{URL: target, Initiator: Image})
				delivered += int64(len(response.Body))
				if (tc.name == "full" || tc.name == "off" || tc.reportOnly) != (err == nil) {
					b.Fatalf("load %s: %v", tc.name, err)
				}
			}
			b.StopTimer()
			b.ReportMetric(float64(requests.Load())/float64(b.N), "requests/op")
			b.ReportMetric(float64(delivered)/float64(b.N), "body_B/op")
		})
	}
}
