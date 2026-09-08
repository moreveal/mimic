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
	"runtime/debug"
	"testing"
	"time"
	"unsafe"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"golang.org/x/sys/windows"
)

func processMemory(t *testing.T) map[string]any {
	t.Helper()
	var counters struct {
		Size, Faults                                                                                          uint32
		PeakWorkingSet, WorkingSet, PeakPaged, Paged, PeakNonpaged, Nonpaged, Pagefile, PeakPagefile, Private uintptr
	}
	counters.Size = uint32(unsafe.Sizeof(counters))
	proc := windows.NewLazySystemDLL("psapi.dll").NewProc("GetProcessMemoryInfo")
	ok, _, err := proc.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&counters)), uintptr(counters.Size))
	if ok == 0 {
		t.Fatal(err)
	}
	var goMemory runtime.MemStats
	runtime.ReadMemStats(&goMemory)
	return map[string]any{"rss": counters.WorkingSet, "private": counters.Private, "go_heap": goMemory.HeapAlloc, "go_sys": goMemory.Sys, "go_released": goMemory.HeapReleased, "goroutines": runtime.NumGoroutine()}
}

// Separates live Page ownership from allocator high-water marks. Forced Go GC
// and scavenging are diagnostic observations, never benchmark recovery policy.
func TestPerformanceDensityProfile(t *testing.T) {
	dir := os.Getenv("MIMIC_PROFILE_DIR")
	if dir == "" {
		t.Skip("set MIMIC_PROFILE_DIR for memory attribution")
	}
	os.MkdirAll(dir, 0755)
	var records []map[string]any
	for _, kind := range []string{"static", "react"} {
		b, err := New(v8engine.Factory{}, chrome152.New())
		if err != nil {
			t.Fatal(err)
		}
		c := b.NewContext()
		var pages []*Page
		var servers []*httptest.Server
		sample := func(phase string) {
			r := processMemory(t)
			r["workload"] = kind
			r["phase"] = phase
			r["pages"] = len(pages)
			var isolates []any
			for _, p := range pages {
				stats, e := p.Top.Realm.runtime.(interface{ Diagnostics() (any, error) }).Diagnostics()
				if e != nil {
					t.Fatal(e)
				}
				isolates = append(isolates, stats)
			}
			r["isolates"] = isolates
			records = append(records, r)
		}
		sample("before")
		for i := 0; i < 10; i++ {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				if r.URL.Path == "/" {
					vendor := ""
					if kind == "react" {
						vendor = `<script src="/vendor/react.production.min.js"></script><script src="/vendor/react-dom.production.min.js"></script>`
					}
					fmt.Fprintf(w, `<!doctype html><html><head><meta charset="utf-8"><link rel="icon" href="data:,"></head><body data-workload="%s"><div id="root">baseline</div>%s<script src="/workload.js"></script></body></html>`, kind, vendor)
					return
				}
				if r.URL.Path == "/data.json" {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, `{"value":7}`)
					return
				}
				if r.URL.Path == "/workload.js" || r.URL.Path == "/vendor/react.production.min.js" || r.URL.Path == "/vendor/react-dom.production.min.js" {
					w.Header().Set("Content-Type", "text/javascript")
					data, e := os.ReadFile(filepath.Join("../../benchmark/fixtures", r.URL.Path[1:]))
					if e != nil {
						http.Error(w, e.Error(), 500)
						return
					}
					w.Write(data)
					return
				}
				http.NotFound(w, r)
			}))
			servers = append(servers, srv)
			p, e := c.NewPage()
			if e != nil {
				t.Fatal(e)
			}
			pages = append(pages, p)
			if e = p.Navigate(context.Background(), srv.URL); e != nil {
				t.Fatal(e)
			}
			if _, e = p.Evaluate(context.Background(), `__benchRun()`); e != nil {
				t.Fatal(e)
			}
			if e = p.AdvanceTime(context.Background(), time.Millisecond); e != nil {
				t.Fatal(e)
			}
			result, e := p.Evaluate(context.Background(), `__bench.error===null&&__bench.done`)
			if e != nil || result != true {
				t.Fatalf("%s: result %v, %v", kind, result, e)
			}
			if i == 0 || i == 4 || i == 9 {
				sample("live")
			}
		}
		if os.Getenv("MIMIC_PROFILE_V8_GC") == "1" {
			var durations []float64
			for _, p := range pages {
				start := time.Now()
				if err := p.Top.Realm.runtime.(interface{ ProfileCollect() error }).ProfileCollect(); err != nil {
					t.Fatal(err)
				}
				durations = append(durations, float64(time.Since(start))/1e6)
			}
			sample("live_after_v8_gc")
			records[len(records)-1]["collection_ms_per_page"] = durations
		}
		for _, p := range pages {
			c.ClosePage(p.ID)
		}
		pages = nil
		sample("closed_immediate")
		time.Sleep(250 * time.Millisecond)
		sample("closed_250ms")
		runtime.GC()
		sample("closed_go_gc")
		debug.FreeOSMemory()
		sample("closed_scavenged")
		c.Close()
		for _, srv := range servers {
			srv.Close()
		}
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "density.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}
