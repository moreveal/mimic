package browser

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/pprof"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestRuntimeLatencyProfile(t *testing.T) {
	dir := os.Getenv("MIMIC_LATENCY_PROFILE_DIR")
	url := os.Getenv("MIMIC_LATENCY_PROFILE_URL")
	if dir == "" || url == "" {
		t.Skip("set MIMIC_LATENCY_PROFILE_DIR and MIMIC_LATENCY_PROFILE_URL for a live diagnostic")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
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
	f, err := os.Create(filepath.Join(dir, "go.pprof"))
	if err != nil {
		t.Fatal(err)
	}
	if err = pprof.StartCPUProfile(f); err != nil {
		t.Fatal(err)
	}
	defer func() { pprof.StopCPUProfile(); f.Close() }()
	start := time.Now()
	err = p.NavigateReserved(ctx, url, p.ReserveNavigation())
	t.Logf("navigate %s error %v", time.Since(start), err)
	if err != nil {
		t.Fatal(err)
	}
	finish, err := p.Top.Realm.runtime.(interface {
		ProfileWorkloadCPU() (func() (any, error), error)
	}).ProfileWorkloadCPU()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		start = time.Now()
		_, err := p.AdvanceTimeBudget(ctx, 10*time.Millisecond, 1)
		t.Logf("pump %d: %s error=%v", i, time.Since(start), err)
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
	profile, err := finish()
	if err != nil {
		t.Fatal(err)
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
	write("native.json", profile)
	write("hosts.json", p.LiveDiagnostics())
	diagnostics, err := p.Top.Realm.runtime.(interface{ Diagnostics() (any, error) }).Diagnostics()
	if err != nil {
		t.Fatal(err)
	}
	write("diagnostics.json", diagnostics)
}
