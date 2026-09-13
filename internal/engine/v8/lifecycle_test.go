//go:build (windows || linux) && amd64

package v8

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
	"weak"
)

func TestDisposeReleasesHostCallbackCaptures(t *testing.T) {
	type payload struct{ bytes [1024]byte }
	retained := func() weak.Pointer[payload] {
		owner, err := NewRuntime()
		if err != nil {
			t.Fatal(err)
		}
		realm, err := owner.NewRealm()
		if err != nil {
			t.Fatal(err)
		}
		data := &payload{}
		ref := weak.Make(data)
		err = realm.SetHostObject("host", map[string]any{"read": HostFunction(func([]string) (any, error) { return int(data.bytes[0]), nil })})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := realm.Eval("host.read()", "capture.js"); err != nil {
			t.Fatal(err)
		}
		if err := owner.Dispose(); err != nil {
			t.Fatal(err)
		}
		return ref
	}()
	for i := 0; i < 5; i++ {
		runtime.GC()
	}
	if retained.Value() != nil {
		t.Fatal("disposed isolate retains Go host callback capture")
	}
}

func TestRuntimeDispatchDuringDispose(t *testing.T) {
	owner, err := NewRuntime()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _ = owner.NewRealm() }()
	}
	_ = owner.Dispose()
	wg.Wait()
	if _, err := owner.NewRealm(); err == nil {
		t.Fatal("dispatch after disposal succeeded")
	}
}

func TestClosedAdapterDoesNotStartCancellationWatcher(t *testing.T) {
	runtime := (Factory{}).New().(*adapter)
	if err := runtime.Close(); err != nil && err != errDispose {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		if _, err := runtime.Eval(context.Background(), "1", "closed.js"); err == nil {
			t.Fatal("closed runtime evaluated")
		}
	}
	if len(runtime.globals) != 0 {
		t.Fatal("persistent handles retained after Close")
	}
}

func TestCancellationDoesNotPoisonNextTurn(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, _ = runtime.Eval(ctx, "for(;;){}", "cancel.js")
	value, err := runtime.Eval(context.Background(), "42", "next.js")
	if err != nil || value.Export() != float64(42) {
		t.Fatalf("next turn: value=%v err=%v", value, err)
	}
}
