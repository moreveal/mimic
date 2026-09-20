package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// A diagnostic replay of the frozen static fixture on independent origins.
// Its profiles attribute residual cross-isolate contention; it does not replace
// the CDP harness or its throughput denominator.
func TestPerformanceParallelPages(t *testing.T) {
	serialBrowserTest(t)
	dir := os.Getenv("MIMIC_PROFILE_DIR")
	if dir == "" {
		t.Skip("diagnostic replay")
	}
	fixture, err := os.ReadFile("../../benchmark/fixtures/workload.js")
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	const count = 50
	servers := make([]*httptest.Server, count)
	for i := range servers {
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/workload.js" {
				w.Header().Set("Content-Type", "text/javascript")
				w.Write(fixture)
				return
			}
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!doctype html><html><head><link rel="icon" href="data:,"></head><body data-workload="static"><div id="root">baseline</div><script src="/workload.js"></script></body></html>`))
		}))
		defer servers[i].Close()
	}
	start := make(chan struct{})
	var work sync.WaitGroup
	records := make([]map[string]any, count)
	begin := time.Now()
	for i := range servers {
		work.Add(1)
		go func(i int) {
			defer work.Done()
			<-start
			tick := time.Now()
			p, e := c.NewPage()
			if e != nil {
				t.Error(e)
				return
			}
			create := time.Since(tick)
			tick = time.Now()
			if e = p.Navigate(context.Background(), servers[i].URL); e != nil {
				t.Error(e)
				return
			}
			navigate := time.Since(tick)
			tick = time.Now()
			if _, e = p.Evaluate(context.Background(), `__benchRun()`); e != nil {
				t.Error(e)
				return
			}
			v, e := p.Evaluate(context.Background(), `__bench.done&&__bench.error===null&&__bench.result.text==='baseline'&&__bench.result.count===1`)
			if e != nil || v != true {
				t.Errorf("static result %v %v", v, e)
				return
			}
			records[i] = map[string]any{"create_ms": float64(create) / 1e6, "navigate_ms": float64(navigate) / 1e6, "execute_ms": float64(time.Since(tick)) / 1e6}
		}(i)
	}
	close(start)
	work.Wait()
	elapsed := time.Since(begin)
	begin = time.Now()
	c.Close()
	data, _ := json.MarshalIndent(map[string]any{"pages": count, "wall_ms": float64(elapsed) / 1e6, "close_ms": float64(time.Since(begin)) / 1e6, "records": records}, "", "  ")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "parallel.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}
