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
	"strconv"
	"strings"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

var profileProcessMemory func(*testing.T) map[string]any

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
	kind := os.Getenv("MIMIC_PROFILE_WORKLOAD")
	if kind == "" {
		kind = "dom"
	}
	switch kind {
	case "dom", "static", "cpu", "react", "wasm":
	default:
		t.Fatal("unsupported diagnostic workload", kind)
	}
	iterations := 3
	if value := os.Getenv("MIMIC_PROFILE_ITERATIONS"); value != "" {
		var err error
		iterations, err = strconv.Atoi(value)
		if err != nil || iterations < 1 || iterations > 100 {
			t.Fatal("invalid profile iteration count")
		}
	}
	fixture, err := os.ReadFile("../../benchmark/fixtures/workload.js")
	if err != nil {
		t.Fatal(err)
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/workload.js" {
			w.Header().Set("Content-Type", "application/javascript")
			w.Write(fixture)
			return
		}
		if r.URL.Path == "/data.json" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"value":7}`))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/vendor/") {
			if r.URL.Path != "/vendor/react.production.min.js" && r.URL.Path != "/vendor/react-dom.production.min.js" {
				http.NotFound(w, r)
				return
			}
			data, err := os.ReadFile(filepath.Join("../../benchmark/fixtures", r.URL.Path[1:]))
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.Header().Set("Content-Type", "text/javascript")
			w.Write(data)
			return
		}
		vendor := ""
		if kind == "react" {
			vendor = `<script src="/vendor/react.production.min.js"></script><script src="/vendor/react-dom.production.min.js"></script>`
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><link rel="icon" href="data:,"></head><body data-workload="%s"><div id="root">baseline</div>%s<script src="/workload.js"></script></body></html>`, kind, vendor)
	})
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	var records []map[string]any
	iteration := -1
	sample := func(phase string, p *Page, elapsed time.Duration) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		record := map[string]any{"phase": phase, "workload": kind, "iteration": iteration, "ms": float64(elapsed) / 1e6, "go_heap": m.HeapAlloc, "go_total_alloc": m.TotalAlloc, "go_sys": m.Sys, "go_gc_cycles": m.NumGC, "go_gc_pause_ns": m.PauseTotalNs, "go_mallocs": m.Mallocs, "go_frees": m.Frees, "goroutines": runtime.NumGoroutine()}
		if profileProcessMemory != nil {
			record["process"] = profileProcessMemory(t)
		}
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
			record["bootstrap_phases_ms"] = p.Top.Realm.profilePhases
		}
		records = append(records, record)
	}
	sample("before", nil, 0)
	for i := 0; i < iterations; i++ {
		iteration = i
		server := httptest.NewServer(handler)
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		start := time.Now()
		p, e := c.NewPage()
		if e != nil {
			t.Fatal(e)
		}
		sample("create", p, time.Since(start))

		start = time.Now()
		if e = p.Navigate(ctx, server.URL); e != nil {
			t.Fatal(e)
		}
		sample("navigate", p, time.Since(start))
		var restoreQoS func()
		if os.Getenv("MIMIC_PROFILE_ACTIVE_QOS") == "1" {
			restoreQoS, e = p.Top.Realm.runtime.(interface{ ProfileActiveQoS() (func(), error) }).ProfileActiveQoS()
			if e != nil {
				t.Fatal(e)
			}
		}
		var finishCPU func() (any, error)
		if os.Getenv("MIMIC_PROFILE_WHOLE_CPU") == "1" && (os.Getenv("MIMIC_PROFILE_CPU_EDGES") != "1" || i < 10 || i >= iterations-10) {
			finishCPU, e = p.Top.Realm.runtime.(interface {
				ProfileWorkloadCPU() (func() (any, error), error)
			}).ProfileWorkloadCPU()
			if e != nil {
				t.Fatal(e)
			}
		}
		var goCPU *os.File
		if os.Getenv("MIMIC_PROFILE_EXEC_GO_CPU") == "1" {
			goCPU, e = os.Create(filepath.Join(dir, fmt.Sprintf("execute-%02d-cpu.pprof", i)))
			if e != nil {
				t.Fatal(e)
			}
			if e = pprof.StartCPUProfile(goCPU); e != nil {
				goCPU.Close()
				t.Fatal(e)
			}
		}
		start = time.Now()
		if _, e = p.Evaluate(ctx, `__benchRun()`); e != nil {
			if goCPU != nil {
				pprof.StopCPUProfile()
				goCPU.Close()
			}
			if finishCPU != nil {
				_, _ = finishCPU()
			}
			t.Fatal(e)
		}
		execution := time.Since(start)
		if goCPU != nil {
			pprof.StopCPUProfile()
			goCPU.Close()
		}
		var cpuProfile any
		if finishCPU != nil {
			cpuProfile, e = finishCPU()
			if e != nil {
				t.Fatal(e)
			}
		}
		sample("execute", p, execution)
		if cpuProfile != nil {
			records[len(records)-1]["workload_cpu_profile"] = cpuProfile
		}
		if i == 0 && os.Getenv("MIMIC_V8_HEAP_SNAPSHOT") == "1" {
			snapshot, e := os.Create(filepath.Join(dir, "v8-dom.heapsnapshot"))
			if e != nil {
				t.Fatal(e)
			}
			var writeErr error
			e = p.Top.Realm.runtime.(interface{ ProfileHeapSnapshot(func([]byte) bool) error }).ProfileHeapSnapshot(func(chunk []byte) bool {
				_, writeErr = snapshot.Write(chunk)
				return writeErr == nil
			})
			snapshot.Close()
			if e != nil || writeErr != nil {
				t.Fatalf("V8 snapshot: %v %v", e, writeErr)
			}
		}
		result, e := p.Evaluate(ctx, `JSON.stringify(__bench)`)
		if e != nil {
			t.Fatal(e)
		}
		t.Log(result)
		if !strings.Contains(fmt.Sprint(result), `"done":true`) || !strings.Contains(fmt.Sprint(result), `"error":null`) || (kind == "dom" && !strings.Contains(fmt.Sprint(result), `"count":3000`)) {
			t.Fatal("incorrect workload result")
		}
		start = time.Now()
		if restoreQoS != nil {
			restoreQoS()
		}
		c.ClosePage(p.ID)
		sample("close", nil, time.Since(start))
		cancel()
		server.Close()
		if i == 0 || i == iterations-1 {
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("closed-%02d-heap.pprof", i)))
			if err != nil {
				t.Fatal(err)
			}
			pprof.Lookup("heap").WriteTo(f, 0)
			f.Close()
		}
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
