//go:build windows && amd64

package v8

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/engine"
)

func TestBootstrapSnapshotsWithDistinctReadOnlyLayouts(t *testing.T) {
	// Start before any other isolate exists. A regression in this ownership
	// boundary can terminate V8; keep its native diagnostics in the child output.
	const child = "MIMIC_SNAPSHOT_DISTINCT_LAYOUT_CHILD"
	if os.Getenv(child) != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBootstrapSnapshotsWithDistinctReadOnlyLayouts$", "-test.v")
		command.Env = append(os.Environ(), child+"=1")
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("distinct snapshot child: %v\n%s", err, output)
		}
		t.Logf("child diagnostics:\n%s", output)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var snapshots []engine.BootstrapSnapshot
	var runtimes []engine.Runtime
	defer func() {
		for _, runtime := range runtimes {
			_ = runtime.Close()
		}
		for _, snapshot := range snapshots {
			_ = snapshot.Close()
		}
	}()
	for _, prefix := range []string{"firstSnapshotKey", "aDifferentLongerSnapshotKey"} {
		var source strings.Builder
		source.WriteString("globalThis.seed={")
		for i := 0; i < 2500; i++ {
			fmt.Fprintf(&source, "%s%d:%d,", prefix, i, i)
		}
		source.WriteString("};")
		snapshot, err := (Factory{}).BuildBootstrapSnapshot(ctx, source.String())
		if err != nil {
			t.Fatal(err)
		}
		snapshots = append(snapshots, snapshot)
		// Frozen V8's startup header stores the read-only checksum at byte 12.
		// Log it for native diagnosis; observable restored state is the assertion.
		blob := snapshot.(*bootstrapSnapshot).blob.Bytes()
		t.Logf("%s read-only checksum: %08x", prefix, binary.LittleEndian.Uint32(blob[12:16]))
	}
	for i, snapshot := range snapshots {
		runtime, err := snapshot.NewRuntime()
		if err != nil {
			t.Fatal(err)
		}
		runtimes = append(runtimes, runtime)
		prefix := []string{"firstSnapshotKey", "aDifferentLongerSnapshotKey"}[i]
		value, err := runtime.Eval(ctx, fmt.Sprintf(`Object.keys(seed).length===2500 && Object.keys(seed)[0]===%q && seed[%q]===2499`, prefix+"0", prefix+"2499"), "distinct-snapshot-state")
		if err != nil || value.Export() != true {
			t.Fatalf("snapshot %d state: %v, %v", i, value, err)
		}
	}
	// Both snapshots must remain live while an ordinary isolate executes too.
	ordinary := (Factory{}).New()
	defer ordinary.Close()
	if value, err := ordinary.Eval(ctx, "typeof seed==='undefined' && new Map([['key',42]]).get('key')===42", "ordinary-state"); err != nil || value.Export() != true {
		t.Fatalf("ordinary state: %v, %v", value, err)
	}
	for i, runtime := range runtimes {
		prefix := []string{"firstSnapshotKey", "aDifferentLongerSnapshotKey"}[i]
		value, err := runtime.Eval(ctx, fmt.Sprintf(`Object.keys(seed).length===2500 && seed[%q]===2499`, prefix+"2499"), "coexisting-snapshot-state")
		if err != nil || value.Export() != true {
			t.Fatalf("coexisting snapshot %d state: %v, %v", i, value, err)
		}
	}
}
