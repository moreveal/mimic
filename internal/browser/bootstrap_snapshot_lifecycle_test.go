//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/engine"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestBootstrapSnapshotPageLifecycle(t *testing.T) {
	serialBrowserTest(t)

	bootstrapSnapshotPageLifecycle(t, v8engine.Factory{}, true)
}

func TestBootstrapSnapshotConcurrentPageLifecycles(t *testing.T) {
	serialBrowserTest(t)
	if os.Getenv("MIMIC_SNAPSHOT_LIFECYCLE_STRESS") != "1" {
		t.Skip("opt-in native crash reproducer; see docs/compatibility/snapshot-lifecycle-audit-2026-09-11.md")
	}
	for i := 0; i < 4; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			bootstrapSnapshotPageLifecycle(t, v8engine.Factory{}, true)
		})
	}
}

type ordinaryLifecycleFactory struct{ v8engine.Factory }

func (ordinaryLifecycleFactory) BootstrapSnapshotsEnabled() bool { return false }

type sequentialSnapshotLifecycleFactory struct {
	v8engine.Factory
	gate    *sync.Mutex
	barrier *snapshotLifecycleBarrier
}

type snapshotLifecycleBarrier struct {
	mu    sync.Mutex
	count int
	start chan struct{}
}

func (b *snapshotLifecycleBarrier) arrive(ctx context.Context) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	b.count++
	if b.count == 4 {
		close(b.start)
	}
	start := b.start
	b.mu.Unlock()
	select {
	case <-start:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f sequentialSnapshotLifecycleFactory) BuildBootstrapSnapshot(ctx context.Context, sources ...string) (engine.BootstrapSnapshot, error) {
	if err := f.barrier.arrive(ctx); err != nil {
		return nil, err
	}
	if f.gate != nil {
		f.gate.Lock()
		defer f.gate.Unlock()
	}
	return f.Factory.BuildBootstrapSnapshot(ctx, sources...)
}

func TestBootstrapSnapshotBarrierConcurrentBuilders(t *testing.T) {
	serialBrowserTest(t)

	bootstrapSnapshotBarrierLifecycles(t, false)
}

func TestBootstrapSnapshotBarrierSequentialBuilders(t *testing.T) {
	serialBrowserTest(t)
	bootstrapSnapshotBarrierLifecycles(t, true)
}

func bootstrapSnapshotBarrierLifecycles(t *testing.T, sequential bool) {
	if os.Getenv("MIMIC_SNAPSHOT_LIFECYCLE_STRESS") != "1" {
		t.Skip("test-only simultaneous builder start diagnostic")
	}
	barrier := &snapshotLifecycleBarrier{start: make(chan struct{})}
	var gate *sync.Mutex
	if sequential {
		gate = new(sync.Mutex)
	}
	for i := 0; i < 4; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			bootstrapSnapshotPageLifecycle(t, sequentialSnapshotLifecycleFactory{gate: gate, barrier: barrier}, true)
		})
	}
}

func TestBootstrapSnapshotConcurrentPagesSequentialBuilders(t *testing.T) {
	serialBrowserTest(t)
	if os.Getenv("MIMIC_SNAPSHOT_LIFECYCLE_STRESS") != "1" {
		t.Skip("test-only serialization-overlap diagnostic")
	}
	gate := new(sync.Mutex)
	for i := 0; i < 4; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			bootstrapSnapshotPageLifecycle(t, sequentialSnapshotLifecycleFactory{gate: gate}, true)
		})
	}
}

func TestOrdinaryConcurrentPageLifecycles(t *testing.T) {
	serialBrowserTest(t)
	if os.Getenv("MIMIC_SNAPSHOT_LIFECYCLE_STRESS") != "1" {
		t.Skip("ordinary-bootstrap control for the opt-in snapshot lifecycle reproducer")
	}
	for i := 0; i < 4; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			bootstrapSnapshotPageLifecycle(t, ordinaryLifecycleFactory{}, false)
		})
	}
}

func bootstrapSnapshotPageLifecycle(t *testing.T, factory engine.Factory, expectSnapshot bool) {
	t.Helper()
	if expectSnapshot && os.Getenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT") == "1" {
		t.Skip("snapshots disabled")
	}
	b, err := New(factory, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<!doctype html><html><head><title>Lifecycle</title></head><body></body></html>")
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	for i := 0; i < 32; i++ {
		p, err := c.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		if err = p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		value, err := p.Evaluate(ctx, `document.body !== null`)
		if err != nil || value != true {
			t.Fatalf("page %d: %v %v", i, value, err)
		}
		c.ClosePage(p.ID)
	}
	if err := b.bootstrapSnapshots.wait(ctx); err != nil {
		t.Fatal(err)
	}
	if expectSnapshot {
		p, err := c.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		defer c.ClosePage(p.ID)
		if err = p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		if !p.Top.Realm.bootstrapRestored {
			t.Fatal("completed snapshot was not restored after repeated Page teardown")
		}
		value, err := p.Evaluate(ctx, `document.body !== null`)
		if err != nil || value != true {
			t.Fatalf("restored page: %v %v", value, err)
		}
	}
}
