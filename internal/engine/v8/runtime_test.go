//go:build (windows || linux) && amd64

package v8

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/engine"
)

func TestEngineRuntimeHostObjectsAndPromises(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	if err := runtime.Set("host", map[string]any{
		"read": runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
			return runtime.Value(map[string]any{"number": 42, "flag": true, "items": []any{"a", 2}}), nil
		}),
	}); err != nil {
		t.Fatal(err)
	}
	value, err := runtime.Eval(context.Background(), `host.read()`, "host.js")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"number": float64(42), "flag": true, "items": []any{"a", float64(2)}}
	if !reflect.DeepEqual(value.Export(), want) {
		t.Fatalf("host result = %#v, want %#v", value.Export(), want)
	}

	promise := runtime.NewPromise()
	if err := promise.Resolve(map[string]any{"ok": true}); err != nil {
		t.Fatal(err)
	}
	if err := runtime.MicrotaskCheckpoint(); err != nil {
		t.Fatal(err)
	}
	result, settled, err := runtime.Await(promise.Value)
	if err != nil || !settled || !reflect.DeepEqual(result.Export(), map[string]any{"ok": true}) {
		t.Fatalf("promise = %#v, settled=%v, err=%v", result, settled, err)
	}
}

func TestEngineRuntimeInterruptsExpiredEvaluation(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := runtime.Eval(ctx, `for(;;){}`, "infinite.js"); err == nil {
		t.Fatal("infinite evaluation was not interrupted")
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("evaluation interruption took %v", elapsed)
	}
}

func TestEngineRuntimeInterruptsExpiredMicrotaskCheckpoint(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	if _, err := runtime.Eval(context.Background(), `Promise.resolve().then(function loop(){Promise.resolve().then(loop)})`, "infinite-microtasks.js"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	checkpoint := runtime.(interface{ MicrotaskCheckpointContext(context.Context) error })
	if err := checkpoint.MicrotaskCheckpointContext(ctx); err == nil {
		t.Fatal("infinite microtask checkpoint was not interrupted")
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("microtask interruption took %v", elapsed)
	}
}

func TestEngineRuntimeCreatesPromiseInsideHostCallback(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()

	var created engine.Promise
	if err := runtime.Set("makePromise", runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		created = runtime.NewPromise()
		return created.Value, nil
	})); err != nil {
		t.Fatal(err)
	}

	value, err := runtime.Eval(context.Background(), `globalThis.callbackPromise = makePromise(); callbackPromise`, "callback-promise.js")
	if err != nil {
		t.Fatal(err)
	}
	if created.Value == nil {
		t.Fatal("host callback did not create a promise")
	}
	if err := created.Resolve("resolved after callback"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.MicrotaskCheckpoint(); err != nil {
		t.Fatal(err)
	}
	result, settled, err := runtime.Await(value)
	if err != nil || !settled || result.Export() != "resolved after callback" {
		t.Fatalf("promise = %#v, settled=%v, err=%v", result, settled, err)
	}
}

func TestEngineRuntimeDefersPromiseJobsUntilCheckpoint(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()

	if _, err := runtime.Eval(context.Background(), `globalThis.jobRan=false;Promise.resolve().then(()=>jobRan=true)`, "microtasks.js"); err != nil {
		t.Fatal(err)
	}
	before, err := runtime.Eval(context.Background(), `jobRan`, "before-checkpoint.js")
	if err != nil {
		t.Fatal(err)
	}
	if before.Export() != false {
		t.Fatalf("promise job bypassed browser checkpoint: %v", before.Export())
	}
	if err := runtime.MicrotaskCheckpoint(); err != nil {
		t.Fatal(err)
	}
	after, err := runtime.Eval(context.Background(), `jobRan`, "after-checkpoint.js")
	if err != nil {
		t.Fatal(err)
	}
	if after.Export() != true {
		t.Fatalf("promise job did not run at checkpoint: %v", after.Export())
	}
}
