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
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
)

// Opt-in bootstrap attribution. Ordinary teardown and diagnostic Go collection
// are recorded separately; neither changes the production recovery policy.
func TestGojaBootstrapChildProfile(t *testing.T) {
	serialBrowserTest(t)
	output := os.Getenv("MIMIC_GOJA_CHILD_PROFILE")
	if output == "" {
		t.Skip("set MIMIC_GOJA_CHILD_PROFILE for startup attribution")
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
				v, e := p.Evaluate(context.Background(), `(()=>{for(let i=0;i<2;i++){const e=document.createElement('iframe');document.body.appendChild(e)}return document.querySelectorAll('iframe').length===2})()`)
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
