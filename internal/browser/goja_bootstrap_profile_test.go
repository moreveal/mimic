//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
)

// Opt-in bootstrap attribution. Ordinary teardown and diagnostic Go collection
// are recorded separately; neither changes the production recovery policy.
func TestGojaBootstrapProfile(t *testing.T) {
	output := os.Getenv("MIMIC_GOJA_BOOTSTRAP_PROFILE")
	if output == "" {
		t.Skip("set MIMIC_GOJA_BOOTSTRAP_PROFILE for startup attribution")
	}
	b, err := New(gojaengine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	var records []map[string]any
	sample := func(phase string, elapsed time.Duration) {
		v := processMemory(t)
		v["phase"] = phase
		v["elapsed_ms"] = float64(elapsed) / float64(time.Millisecond)
		records = append(records, v)
	}
	sample("initial", 0)
	for _, phase := range []string{"cold", "warm"} {
		c := b.NewContext()
		pages := make([]*Page, 5)
		errors := make(chan error, 5)
		start := make(chan struct{})
		var wg sync.WaitGroup
		begin := time.Now()
		for i := range pages {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				p, e := c.NewPage()
				if e != nil {
					errors <- e
					return
				}
				pages[i] = p
				v, e := p.Evaluate(context.Background(), `(()=>{const e=document.createElement('div');e.id='owned';document.body.appendChild(e);return document.getElementById('owned')===e})()`)
				if e != nil {
					errors <- e
					return
				}
				if v != true {
					errors <- fmt.Errorf("realm-local DOM identity: %v", v)
				}
			}(i)
		}
		close(start)
		wg.Wait()
		close(errors)
		for e := range errors {
			t.Error(e)
		}
		sample(phase+"_live5", time.Since(begin))
		if err = c.Close(); err != nil {
			t.Fatal(err)
		}
		sample(phase+"_closed", 0)
		time.Sleep(250 * time.Millisecond)
		sample(phase+"_closed250ms", 0)
	}
	runtime.GC()
	sample("diagnostic_go_gc", 0)
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(output, data, 0644); err != nil {
		t.Fatal(err)
	}
}

type gojaPhaseFactory struct{}
type gojaPhaseRuntime struct{ engine.Runtime }

func (gojaPhaseFactory) New() engine.Runtime   { return &gojaPhaseRuntime{gojaengine.Factory{}.New()} }
func (*gojaPhaseRuntime) ProfileEnabled() bool { return true }
func (r *gojaPhaseRuntime) EvalBootstrap(ctx context.Context, source, name string) (engine.Value, error) {
	return r.Runtime.(engine.BootstrapRuntime).EvalBootstrap(ctx, source, name)
}

func TestGojaBootstrapPhases(t *testing.T) {
	output := os.Getenv("MIMIC_GOJA_BOOTSTRAP_PHASES")
	if output == "" {
		t.Skip("set MIMIC_GOJA_BOOTSTRAP_PHASES for phase attribution")
	}
	b, err := New(gojaPhaseFactory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	var records []map[string]any
	for i := 0; i < 2; i++ {
		start := time.Now()
		p, err := c.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = p.Evaluate(context.Background(), "true"); err != nil {
			t.Fatal(err)
		}
		records = append(records, map[string]any{"iteration": i, "total_ms": float64(time.Since(start)) / float64(time.Millisecond), "phases": p.Top.Realm.profilePhases})
		if err = p.Close(); err != nil {
			t.Fatal(err)
		}
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(output, data, 0644); err != nil {
		t.Fatal(err)
	}
}
