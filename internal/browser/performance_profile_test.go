package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// A diagnostic replay, never a replacement for the immutable timed harness.
// Opt in because full allocation sampling substantially changes execution cost.
func TestPerformanceProfile(t *testing.T) {
	dir := os.Getenv("MIMIC_PROFILE_DIR")
	if dir == "" {
		t.Skip("set MIMIC_PROFILE_DIR for diagnostic replay")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("../../benchmark/fixtures/workload.js")
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/workload.js" {
			w.Header().Set("Content-Type", "application/javascript")
			w.Write(fixture)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><link rel="icon" href="data:,"></head><body data-workload="dom"><div id="root">baseline</div><script src="/workload.js"></script></body></html>`))
	}))
	defer server.Close()
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	var records []map[string]any
	sample := func(phase string, p *Page, elapsed time.Duration) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		record := map[string]any{"phase": phase, "ms": float64(elapsed) / 1e6, "go_heap": m.HeapAlloc, "go_total_alloc": m.TotalAlloc, "go_sys": m.Sys, "goroutines": runtime.NumGoroutine()}
		if p != nil {
			stats, e := p.Top.Realm.runtime.(interface{ Diagnostics() (any, error) }).Diagnostics()
			if e != nil {
				t.Fatal(e)
			}
			encoded, _ := json.Marshal(stats)
			var snapshot any
			json.Unmarshal(encoded, &snapshot)
			record["v8"] = snapshot
			record["dom_wrappers"] = p.Top.Realm.profileWrappers
		}
		records = append(records, record)
	}
	sample("before", nil, 0)
	for i := 0; i < 3; i++ {
		c = b.NewContext()
		start := time.Now()
		p, e := c.NewPage()
		if e != nil {
			t.Fatal(e)
		}
		sample("create", p, time.Since(start))
		start = time.Now()
		if e = p.Navigate(context.Background(), server.URL); e != nil {
			t.Fatal(e)
		}
		sample("navigate", p, time.Since(start))
		start = time.Now()
		if _, e = p.Evaluate(context.Background(), `__benchRun()`); e != nil {
			t.Fatal(e)
		}
		sample("dom", p, time.Since(start))
		result, e := p.Evaluate(context.Background(), `JSON.stringify(__bench)`)
		if e != nil {
			t.Fatal(e)
		}
		t.Log(result)
		if !strings.Contains(fmt.Sprint(result), `"count":3000`) {
			t.Fatal("incorrect DOM result")
		}
		start = time.Now()
		c.ClosePage(p.ID)
		c.Close()
		sample("close", nil, time.Since(start))
	}
	runtime.GC()
	sample("after_gc", nil, 0)
	for _, name := range []string{"heap", "allocs", "goroutine", "mutex", "block"} {
		f, e := os.Create(filepath.Join(dir, name+".pprof"))
		if e != nil {
			t.Fatal(e)
		}
		pprof.Lookup(name).WriteTo(f, 0)
		f.Close()
	}
	data, _ := json.MarshalIndent(records, "", "  ")
	if err = os.WriteFile(filepath.Join(dir, "phases.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}
