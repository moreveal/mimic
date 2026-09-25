package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"github.com/moreveal/mimic/internal/profile"
)

// Run in separate fresh processes for concurrency 8 and 100. These are PoC
// observations on a small local fixture, not the frozen performance gate.
func TestProfilePoCDensity(t *testing.T) {
	value := os.Getenv("MIMIC_PROFILE_POC_CONCURRENCY")
	if value == "" {
		t.Skip("set MIMIC_PROFILE_POC_CONCURRENCY=8 or 100")
	}
	concurrency, err := strconv.Atoi(value)
	if err != nil || (concurrency != 8 && concurrency != 100) {
		t.Fatal("expected concurrency 8 or 100")
	}
	identityMode := os.Getenv("MIMIC_PROFILE_POC_IDENTITY_MODE")
	if identityMode == "" {
		identityMode = "distinct"
	}
	if identityMode != "distinct" && identityMode != "same" && identityMode != "ordinary" {
		t.Fatal("invalid identity mode")
	}
	serialBrowserTest(t)
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<!doctype html><title>Profile PoC</title><body>")
		for i := 0; i < 200; i++ {
			fmt.Fprintf(w, "<a href='/item/%d'>Item %d</a>", i, i)
		}
		fmt.Fprint(w, "</body>")
	}))
	defer server.Close()
	jobs := 300
	if raw := os.Getenv("MIMIC_PROFILE_POC_JOBS"); raw != "" {
		jobs, err = strconv.Atoi(raw)
		if err != nil || jobs < 100 || jobs > 1000 || jobs%100 != 0 {
			t.Fatal("MIMIC_PROFILE_POC_JOBS must be a multiple of 100 from 100 to 1000")
		}
	}
	started := time.Now()
	var samples []map[string]any
	sample := func(phase string, completed int) {
		row := map[string]any{"phase": phase, "completed": completed, "contexts": len(b.Contexts()), "elapsedMillis": time.Since(started).Milliseconds()}
		b.bootstrapSnapshots.mu.Lock()
		cache := map[string]any{"entries": len(b.bootstrapSnapshots.entries), "snapshots": 0, "seedBytes": 0}
		for _, entry := range b.bootstrapSnapshots.entries {
			cache["seedBytes"] = cache["seedBytes"].(int) + bootstrapSeedBytes(entry.seed)
			if entry.snapshot != nil {
				cache["snapshots"] = cache["snapshots"].(int) + 1
			}
		}
		b.bootstrapSnapshots.mu.Unlock()
		row["bootstrapCache"] = cache
		restored := 0
		for _, c := range b.Contexts() {
			for _, p := range c.Pages() {
				if p.Top.Realm.bootstrapRestored {
					restored++
				}
			}
		}
		row["restoredPages"] = restored
		if profileProcessMemory != nil {
			row["memory"] = profileProcessMemory(t)
		}
		samples = append(samples, row)
		if phase == "live" && completed == 0 {
			if path := os.Getenv("MIMIC_PROFILE_POC_HEAP_PROFILE"); path != "" {
				runtime.GC()
				file, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				if err := pprof.WriteHeapProfile(file); err != nil {
					_ = file.Close()
					t.Fatal(err)
				}
				if err := file.Close(); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	sample("ready", 0)
	for round := 0; round < jobs/100; round++ {
		for start := 0; start < 100; start += concurrency {
			n := min(concurrency, 100-start)
			ready, done := make(chan error, n), make(chan error, n)
			release := make(chan struct{})
			for i := 0; i < n; i++ {
				go func(index int) {
					var c *Context
					var d profile.Document
					var err error
					if identityMode == "ordinary" {
						c = b.NewContext()
						d = c.Profile()
					} else {
						seed := fmt.Sprintf("poc-%d-%d", round, index)
						if identityMode == "same" {
							seed = "poc-shared"
						}
						raw := []byte(fmt.Sprintf(`{"seed":%q}`, seed))
						var descriptor profile.Descriptor
						d, descriptor, err = profile.Generate(raw, b.env)
						if err == nil {
							c, err = b.NewProfileContext(descriptor.Token(), nil, nil)
						}
					}
					if err != nil {
						ready <- err
						done <- nil
						return
					}
					p, err := c.NewPage()
					if err == nil {
						ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
						defer cancel()
						err = p.Navigate(ctx, server.URL)
						if err == nil {
							var got any
							got, err = p.Evaluate(ctx, fmt.Sprintf(`document.title==='Profile PoC'&&document.querySelectorAll('a').length===200&&innerWidth===%d&&innerHeight===%d&&navigator.hardwareConcurrency===%d`, d.Window.ViewportWidth, d.Window.ViewportHeight, d.Hardware.LogicalProcessors))
							if err == nil && got != true {
								err = fmt.Errorf("incorrect extraction or profile: %v", got)
							}
						}
					}
					ready <- err
					<-release
					done <- c.Close()
				}(start + i)
			}
			var first error
			for i := 0; i < n; i++ {
				if err := <-ready; err != nil && first == nil {
					first = err
				}
			}
			sample("live", round*100+start)
			if len(b.Contexts()) != n && first == nil {
				first = fmt.Errorf("expected %d simultaneously live contexts, got %d", n, len(b.Contexts()))
			}
			close(release)
			for i := 0; i < n; i++ {
				if err := <-done; err != nil && first == nil {
					first = err
				}
			}
			sample("closed", round*100+start+n)
			if first != nil {
				t.Fatal(first)
			}
			if len(b.Contexts()) != 0 {
				t.Fatal("retained contexts after close")
			}
		}
	}
	base := profile.FromEnvironment(b.env, b.env.ProfileID, profile.Proxy{})
	widths := base.Display.AvailableWidth - (base.Window.OuterWidth - base.Window.ViewportWidth) - 800 + 1
	heights := base.Display.AvailableHeight - (base.Window.OuterHeight - base.Window.ViewportHeight) - 480 + 1
	widthPositions := int64(widths) * int64(widths+1) / 2
	heightPositions := int64(heights) * int64(heights+1) / 2
	output, _ := json.MarshalIndent(map[string]any{"concurrency": concurrency, "profileMode": identityMode, "pages": jobs, "baseProfile": base.BaseProfile, "display": base.Display, "window": base.Window, "recipeCombinationUpperBound": widthPositions * heightPositions * 8, "samples": samples}, "", "  ")
	if path := os.Getenv("MIMIC_PROFILE_POC_OUTPUT"); path != "" {
		if err := os.WriteFile(path, output, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log(string(output))
}
