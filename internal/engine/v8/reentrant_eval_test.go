//go:build windows && amd64

package v8

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/engine"
)

func TestGetReentersHostCallback(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	if err := runtime.Set("hostGet", runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return runtime.Get(args[0].String()), nil
	})); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	value, err := runtime.Eval(ctx, `globalThis.identity={};globalThis.reads=0;Object.defineProperty(globalThis,'answer',{get(){reads++;return identity}});hostGet('answer')===identity&&reads===1&&hostGet('absent')===undefined`, "reentrant-get.js")
	if err != nil || value.Export() != true {
		t.Fatalf("callback global lookup: %v %v", value, err)
	}
}

func TestEvalReentersHostCallbackWithoutMicrotaskCheckpoint(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	if err := runtime.Set("hostEval", runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		value, err := runtime.Eval(context.Background(), args[0].String(), "nested-script.js")
		if err == nil {
			err = runtime.MicrotaskCheckpoint()
			_ = runtime.(*adapter).NativeTasksPending()
		}
		return value, err
	})); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	value, err := runtime.Eval(ctx, `globalThis.steps=[]; const result=hostEval("steps.push('nested');Promise.resolve().then(()=>steps.push('microtask'));globalThis.written=41;({answer:written+1})");steps.push('outer');JSON.stringify({steps,result,written})`, "outer.js")
	if err != nil || value.String() != `{"steps":["nested","outer"],"result":{"answer":42},"written":41}` {
		t.Fatalf("nested evaluation: value=%v err=%v", value, err)
	}
	if err := runtime.MicrotaskCheckpoint(); err != nil {
		t.Fatal(err)
	}
	value, err = runtime.Eval(ctx, `steps.join(',')`, "after.js")
	if err != nil || value.String() != "nested,outer,microtask" {
		t.Fatalf("checkpoint: value=%v err=%v", value, err)
	}
}

func TestEvalReentrantExceptionsRetainIdentityAndSource(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	var nestedError error
	if err := runtime.Set("hostEval", runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		value, err := runtime.Eval(context.Background(), args[0].String(), "nested-error.js")
		nestedError = err
		return value, err
	})); err != nil {
		t.Fatal(err)
	}
	value, err := runtime.Eval(context.Background(), `globalThis.marker={};let same=false;try{hostEval('throw marker')}catch(e){same=e===marker};same`, "outer.js")
	if err != nil || value.Export() != true {
		t.Fatalf("exception identity: value=%v err=%v", value, err)
	}
	if nestedError == nil || !strings.Contains(nestedError.Error(), "nested-error.js") {
		t.Fatalf("missing nested source name: %v", nestedError)
	}
	value, err = runtime.Eval(context.Background(), `try{hostEval('let =')}catch(e){e instanceof SyntaxError}`, "outer.js")
	if err != nil || value.Export() != true {
		t.Fatalf("compile exception: value=%v err=%v", value, err)
	}
}

func TestEvalReentrantCancellationDoesNotPoisonNextTurn(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	if err := runtime.Set("hostEval", runtime.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		return runtime.Eval(ctx, "for(;;){}", "nested-cancel.js")
	})); err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.Eval(context.Background(), "hostEval()", "outer.js"); err == nil {
		t.Fatal("nested infinite evaluation was not canceled")
	}
	value, err := runtime.Eval(context.Background(), "42", "next.js")
	if err != nil || value.Export() != float64(42) {
		t.Fatalf("next turn: value=%v err=%v", value, err)
	}
}
