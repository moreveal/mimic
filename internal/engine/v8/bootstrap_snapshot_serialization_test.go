//go:build windows && amd64

package v8

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConcurrentBootstrapSnapshotSerialization(t *testing.T) {
	const child = "MIMIC_SNAPSHOT_SERIALIZATION_CHILD"
	if os.Getenv(child) != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestConcurrentBootstrapSnapshotSerialization$", "-test.v")
		command.Env = append(os.Environ(), child+"=1")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("concurrent serializer child: %v\n%s", err, output)
		}
		t.Logf("child diagnostics:\n%s", output)
		return
	}
	// Exercise creation and finalization alone. A consumer restore is not needed
	// to trigger shared read-only-space repair against already sealed pages.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	for round := 0; round < 8; round++ {
		t.Logf("serialization round %d", round)
		start := make(chan struct{})
		failures := make(chan error, 4)
		var workers sync.WaitGroup
		for worker := 0; worker < 4; worker++ {
			workers.Add(1)
			go func(worker int) {
				defer workers.Done()
				<-start
				var source strings.Builder
				source.WriteString("globalThis.graph={")
				for i := 0; i < 2500; i++ {
					fmt.Fprintf(&source, "builder%dRound%dKey%04d:{index:%d,text:'distinct%d_%d_%d'},", worker, round, i, i, worker, round, i)
				}
				source.WriteString("};")
				blob, err := (Factory{}).BuildBootstrapSnapshot(ctx, source.String())
				if err != nil {
					failures <- err
					return
				}
				if err := blob.Close(); err != nil {
					failures <- err
				}
			}(worker)
		}
		close(start)
		workers.Wait()
		close(failures)
		for err := range failures {
			t.Fatal(err)
		}
	}
}
