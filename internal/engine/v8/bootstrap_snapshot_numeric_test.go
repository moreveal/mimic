//go:build (windows || linux) && amd64

package v8

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/engine"
)

func TestConcurrentNumericSnapshotsPreserveLiveGraphs(t *testing.T) {
	const child = "MIMIC_NUMERIC_SNAPSHOT_CHILD"
	if os.Getenv(child) != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestConcurrentNumericSnapshotsPreserveLiveGraphs$", "-test.v")
		command.Env = append(os.Environ(), child+"=1")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("concurrent numeric snapshot child: %v\n%s", err, output)
		}
		t.Logf("child diagnostics:\n%s", output)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	type graphs struct {
		blobs    []engine.BootstrapSnapshot
		runtimes []engine.Runtime
		err      error
	}
	const builders = 4
	results := make(chan graphs, builders)
	start := make(chan struct{})
	for worker := 0; worker < builders; worker++ {
		go func(worker int) {
			var owned graphs
			defer func() { results <- owned }()
			<-start
			for round := 1; round <= 8; round++ {
				n := round*1000 + worker*17
				source := fmt.Sprintf(`class C{constructor(i){this.value=i}read(){return this.value}}globalThis.graph=Array.from({length:%d},(_,i)=>new C(i));globalThis.check=()=>graph.length===%d&&graph[%d].read()===%d`, n, n, n-1, n-1)
				t.Logf("builder %d round %d create", worker, round)
				blob, err := (Factory{}).BuildBootstrapSnapshot(ctx, source)
				if err != nil {
					owned.err = err
					return
				}
				owned.blobs = append(owned.blobs, blob)
				r, err := blob.NewRuntime()
				if err != nil {
					owned.err = err
					return
				}
				owned.runtimes = append(owned.runtimes, r)
				for index, runtime := range owned.runtimes {
					value, err := runtime.Eval(ctx, "check()", "live-numeric-graph")
					if err != nil {
						owned.err = err
						return
					}
					if value.Export() != true {
						owned.err = fmt.Errorf("builder %d retained graph %d changed", worker, index)
						return
					}
				}
			}
		}(worker)
	}
	close(start)
	var all []graphs
	for i := 0; i < builders; i++ {
		all = append(all, <-results)
	}
	// No earlier graph or snapshot is closed while another builder is active.
	for _, owned := range all {
		if owned.err != nil {
			t.Error(owned.err)
		}
		for _, runtime := range owned.runtimes {
			value, err := runtime.Eval(ctx, "check()", "final-numeric-graph")
			if err != nil || value.Export() != true {
				t.Errorf("final retained graph: %v, %v", value, err)
			}
			if err := runtime.Close(); err != nil {
				t.Error(err)
			}
		}
		for _, blob := range owned.blobs {
			if err := blob.Close(); err != nil {
				t.Error(err)
			}
		}
	}
}
