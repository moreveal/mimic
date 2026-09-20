package browser

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strconv"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestRuntimeLatencyProfile(t *testing.T) {
	serialBrowserTest(t)
	dir := os.Getenv("MIMIC_LATENCY_PROFILE_DIR")
	url := os.Getenv("MIMIC_LATENCY_PROFILE_URL")
	if dir == "" || url == "" {
		t.Skip("set MIMIC_LATENCY_PROFILE_DIR and MIMIC_LATENCY_PROFILE_URL for a live diagnostic")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	kind := os.Getenv("MIMIC_LATENCY_PROFILE_KIND")
	if kind == "" {
		kind = "all"
	}
	switch kind {
	case "all", "native", "go-cpu", "go-memory", "go-block", "go-mutex":
	default:
		t.Fatalf("unknown MIMIC_LATENCY_PROFILE_KIND %q", kind)
	}
	nativeProfile, cpuProfile := kind == "all" || kind == "native", kind == "all" || kind == "go-cpu"
	// A navigation-only native profiler is a separate engine opt-in. Prevent
	// an inherited setting from adding it to an explicitly Go-only run.
	if !nativeProfile {
		t.Setenv("MIMIC_V8_CPU_PROFILE", "")
	}
	write := func(name string, value any) {
		t.Helper()
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("profile-metadata.json", map[string]any{
		"kind": kind, "url": url, "startedAt": time.Now().UTC(),
		"hostsEnabled":            os.Getenv("MIMIC_PROFILE_HOSTS"),
		"textCacheEnabled":        os.Getenv("MIMIC_PROFILE_TEXT_CACHE"),
		"nativeNavigationEnabled": os.Getenv("MIMIC_V8_CPU_PROFILE"),
		"memorySampleRate":        runtime.MemProfileRate,
	})
	var profileNames []string
	var restoreMutex, stopBlock func()
	if kind == "all" || kind == "go-memory" {
		profileNames = append(profileNames, "allocs", "heap")
	}
	if kind == "all" || kind == "go-mutex" {
		previousMutexRate := runtime.SetMutexProfileFraction(1)
		restoreMutex = func() { runtime.SetMutexProfileFraction(previousMutexRate) }
		profileNames = append(profileNames, "mutex")
	}
	if kind == "all" || kind == "go-block" {
		runtime.SetBlockProfileRate(1)
		stopBlock = func() { runtime.SetBlockProfileRate(0) }
		profileNames = append(profileNames, "block")
	}
	if kind == "all" {
		profileNames = append(profileNames, "goroutine")
	}
	defer func() {
		if restoreMutex != nil {
			restoreMutex()
		}
		if stopBlock != nil {
			stopBlock()
		}
		for _, name := range profileNames {
			profile := pprof.Lookup(name)
			if profile == nil {
				continue
			}
			out, err := os.Create(filepath.Join(dir, name+".pprof"))
			if err != nil {
				t.Error(err)
				continue
			}
			if err := profile.WriteTo(out, 0); err != nil {
				t.Error(err)
			}
			if err := out.Close(); err != nil {
				t.Error(err)
			}
		}
	}()
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
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if cpuProfile {
		f, err := os.Create(filepath.Join(dir, "go.pprof"))
		if err != nil {
			t.Fatal(err)
		}
		if err = pprof.StartCPUProfile(f); err != nil {
			f.Close()
			t.Fatal(err)
		}
		defer func() {
			pprof.StopCPUProfile()
			if err := f.Close(); err != nil {
				t.Error(err)
			}
		}()
	}
	start := time.Now()
	err = p.NavigateReserved(ctx, url, p.ReserveNavigation())
	t.Logf("navigate %s error %v", time.Since(start), err)
	if err != nil {
		t.Fatal(err)
	}
	var finish func() (any, error)
	if nativeProfile {
		finish, err = p.Top.Realm.runtime.(interface {
			ProfileWorkloadCPU() (func() (any, error), error)
		}).ProfileWorkloadCPU()
		if err != nil {
			t.Fatal(err)
		}
	}
	pumpSeconds, _ := strconv.ParseFloat(os.Getenv("MIMIC_LATENCY_PROFILE_PUMP_SECONDS"), 64)
	pumpUntil := time.Now().Add(time.Duration(pumpSeconds * float64(time.Second)))
	for i := 0; i < 20 || pumpSeconds > 0 && time.Now().Before(pumpUntil); i++ {
		start = time.Now()
		_, err := p.AdvanceTimeBudget(ctx, 10*time.Millisecond, 1)
		if i < 20 || time.Since(start) > 100*time.Millisecond || err != nil {
			t.Logf("pump %d: %s error=%v", i, time.Since(start), err)
		}
		if ctx.Err() != nil {
			break
		}
		if pumpSeconds > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	for i := 0; i < 4; i++ {
		start = time.Now()
		source := os.Getenv("MIMIC_LATENCY_PROFILE_EXPRESSION")
		if source == "" {
			source = `document.querySelectorAll('main a').length`
		}
		value, err := p.EvaluateCommand(ctx, "", source)
		t.Logf("selector %d: %s result=%v error=%v", i, time.Since(start), value, err)
	}
	// Optional diagnostic input replay uses the production input dispatcher.
	// Expressions can inspect state between actions; no site behavior belongs
	// in the runtime. Waiting pumps the same Page's tasks and remains bounded
	// by the diagnostic context deadline.
	if path := os.Getenv("MIMIC_LATENCY_PROFILE_ACTIONS"); path != "" {
		var actions []struct {
			Expression string         `json:"expression"`
			Method     string         `json:"method"`
			Params     map[string]any `json:"params"`
			WaitMS     int            `json:"waitMs"`
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &actions); err != nil {
			t.Fatal(err)
		}
		for i, action := range actions {
			start := time.Now()
			var value any
			var err error
			if action.Expression != "" {
				value, err = p.EvaluateCommand(ctx, "", action.Expression)
			}
			if err == nil && action.Method != "" {
				err = p.DispatchProtocolInput(ctx, action.Method, action.Params)
			}
			t.Logf("action %d: %s result=%v error=%v", i, time.Since(start), value, err)
			if err != nil {
				t.Fatal(err)
			}
			until := time.Now().Add(time.Duration(action.WaitMS) * time.Millisecond)
			for time.Now().Before(until) {
				if _, err := p.AdvanceTimeBudget(ctx, 10*time.Millisecond, 1); err != nil {
					t.Fatal(err)
				}
				if err := ctx.Err(); err != nil {
					t.Fatal(err)
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
	if finish != nil {
		profile, err := finish()
		if err != nil {
			t.Fatal(err)
		}
		write("native.json", profile)
	}
	write("hosts.json", p.LiveDiagnostics())
	if profile := p.Top.Realm.textCacheProfile; profile != nil {
		write("text-cache.json", profile.snapshot(p.Top.Realm.textShapeCache))
	}
	diagnostics, err := p.Top.Realm.runtime.(interface{ Diagnostics() (any, error) }).Diagnostics()
	if err != nil {
		t.Fatal(err)
	}
	write("diagnostics.json", diagnostics)
}
