package gojaengine

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestEvalHonorsContextCancellationAndRuntimeRemainsUsable(t *testing.T) {
	runtime := Factory{}.New()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := runtime.Eval(ctx, `for (;;) {}`, "infinite.js"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline, got %v", err)
	}
	value, err := runtime.Eval(context.Background(), `6 * 7`, "after.js")
	if err != nil || value.Export() != int64(42) {
		t.Fatalf("runtime unusable after interruption: value=%v err=%v", value, err)
	}
}

func TestGlobalAccessObserverPreservesAccessorReceiverAndException(t *testing.T) {
	runtime := Factory{}.New()
	runtime.SetGlobalAccessObserver(func(string, bool) {})
	value, err := runtime.Eval(context.Background(), `(() => {
		const marker = {};
		Object.defineProperty(globalThis, 'receiverProbe', {get() { return this; }});
		Object.defineProperty(globalThis, 'throwProbe', {get() { throw marker; }});
		const child = Object.create(globalThis);
		if (globalThis.receiverProbe !== globalThis || child.receiverProbe !== child) return false;
		try { globalThis.throwProbe; } catch (error) { return error === marker; }
		return false;
	})()`, "global-observer.js")
	if err != nil || value.Export() != true {
		t.Fatalf("observer changed accessor semantics: value=%v err=%v", value, err)
	}
}
