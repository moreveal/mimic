//go:build windows && amd64

package v8

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/engine"
)

func installNestedEval(t *testing.T, caller, target *adapter, name, source string) {
	t.Helper()
	if err := caller.Set(name, caller.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		if currentThreadID() != caller.owner.actorTID {
			return nil, fmt.Errorf("callback left its actor thread")
		}
		var exported any
		err := caller.RunNested(context.Background(), func(ctx context.Context) error {
			value, err := target.Eval(ctx, source, name+".js")
			if err == nil {
				exported = value.Export()
			}
			return err
		})
		if err != nil {
			return nil, err
		}
		return caller.Value(exported), nil
	})); err != nil {
		t.Fatal(err)
	}
}

func TestNestedActorsRoundTripAndException(t *testing.T) {
	a, b, c := (Factory{}).New().(*adapter), (Factory{}).New().(*adapter), (Factory{}).New().(*adapter)
	defer a.Close()
	defer b.Close()
	defer c.Close()
	installNestedEval(t, a, b, "toB", "toC()")
	installNestedEval(t, b, c, "toC", "toA()")
	installNestedEval(t, c, a, "toA", "steps.push('back');42")
	value, err := a.Eval(context.Background(), `globalThis.steps=['before'];const answer=toB();steps.push('after');JSON.stringify({answer,steps})`, "outer.js")
	if err != nil || value.String() != `{"answer":42,"steps":["before","back","after"]}` {
		t.Fatalf("round trip: value=%v err=%v", value, err)
	}
	installNestedEval(t, a, b, "roundB", "roundA()")
	installNestedEval(t, b, a, "roundA", "roundBAgain()")
	installNestedEval(t, a, b, "roundBAgain", "41+1")
	value, err = a.Eval(context.Background(), "roundB()", "two-actor-depth.js")
	if err != nil || value.Export() != float64(42) {
		t.Fatalf("A-B-A-B chain: value=%v err=%v", value, err)
	}
	installNestedEval(t, a, b, "throwFromB", "throw new Error('nested failure')")
	value, err = a.Eval(context.Background(), `try{throwFromB()}catch(e){String(e).includes('nested failure')}`, "outer-error.js")
	if err != nil || value.Export() != true {
		t.Fatalf("nested exception: value=%v err=%v", value, err)
	}
}

func TestNestedActorCancellationReachesCallee(t *testing.T) {
	a, b := (Factory{}).New().(*adapter), (Factory{}).New().(*adapter)
	defer a.Close()
	defer b.Close()
	installNestedEval(t, a, b, "toB", "for(;;){}")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := a.Eval(ctx, "toB()", "cancel.js"); err == nil {
		t.Fatal("outer cancellation did not interrupt nested actor")
	}
	for _, runtime := range []*adapter{a, b} {
		value, err := runtime.Eval(context.Background(), "42", "after-cancel.js")
		if err != nil || value.Export() != float64(42) {
			t.Fatalf("actor poisoned after cancellation: value=%v err=%v", value, err)
		}
	}
}

func TestNestedActorDefersConcurrentTeardown(t *testing.T) {
	a, b := (Factory{}).New().(*adapter), (Factory{}).New().(*adapter)
	defer a.Close()
	defer b.Close()
	started, release := make(chan struct{}), make(chan struct{})
	if err := b.Set("gate", b.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		close(started)
		<-release
		return b.Value(42), nil
	})); err != nil {
		t.Fatal(err)
	}
	installNestedEval(t, a, b, "toB", "gate()")
	evaluated := make(chan error, 1)
	go func() { _, err := a.Eval(context.Background(), "toB()", "outer.js"); evaluated <- err }()
	<-started
	closed := make(chan error, 1)
	go func() { closed <- a.Close() }()
	select {
	case err := <-closed:
		close(release)
		t.Fatalf("disposed active caller: %v", err)
	case <-time.After(10 * time.Millisecond):
	}
	close(release)
	if err := <-evaluated; err != nil {
		t.Fatal(err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
}
