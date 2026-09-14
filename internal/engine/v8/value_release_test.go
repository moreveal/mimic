package v8

import (
	"context"
	"reflect"
	"testing"

	"github.com/moreveal/mimic/internal/engine"
)

func TestPersistentValueReleaseKeepsOtherReferencesAlive(t *testing.T) {
	r := (Factory{}).New().(*adapter)
	defer r.Close()
	ctx := context.Background()
	value, err := r.Eval(ctx, `globalThis.saved={x:7};saved`, "release.js")
	if err != nil {
		t.Fatal(err)
	}
	other, err := r.Eval(ctx, `saved`, "other.js")
	if err != nil {
		t.Fatal(err)
	}
	before := len(r.globals)
	r.ReleaseValue(value)
	r.ReleaseValue(value)
	if len(r.globals) != before-1 {
		t.Fatalf("persistent roots after release: %d before %d", len(r.globals), before)
	}
	property := r.GetProperty(other, "x")
	if property.Export() != float64(7) {
		t.Fatalf("other handle lost object: %v", property.Export())
	}
	r.ReleaseValue(other)
	r.ReleaseValue(property)
	value, err = r.Eval(ctx, `saved.x`, "still-alive.js")
	if err != nil || value.Export() != float64(7) {
		t.Fatalf("page reference lost: %v %v", value, err)
	}
}

func TestPromiseSettlementReleasesPrivateRoots(t *testing.T) {
	r := (Factory{}).New().(*adapter)
	defer r.Close()
	ctx := context.Background()
	argument, err := r.Eval(ctx, `globalThis.saved={answer:42};saved`, "promise-value.js")
	if err != nil {
		t.Fatal(err)
	}
	defer r.ReleaseValue(argument)
	baseline := len(r.globals)
	for iteration := 0; iteration < 100; iteration++ {
		promise := r.NewPromise()
		if got := len(r.globals) - baseline; got != 3 {
			t.Fatalf("pending Promise owns %d roots, want Promise and two resolvers", got)
		}
		var input any = map[string]any{"answer": 42}
		if iteration%2 != 0 {
			input = argument
		}
		if err := promise.Resolve(input); err != nil {
			t.Fatal(err)
		}
		if err := promise.Reject("ignored second settlement"); err != nil {
			t.Fatal(err)
		}
		if got := len(r.globals) - baseline; got != 1 {
			t.Fatalf("settled Promise owns %d roots, want only the caller's Value", got)
		}
		result, done, err := r.Await(promise.Value)
		if err != nil || !done || !reflect.DeepEqual(result.Export(), map[string]any{"answer": float64(42)}) {
			t.Fatalf("settlement %d: %v %v %v", iteration, result, done, err)
		}
		r.ReleaseValue(result)
		r.ReleaseValue(promise.Value)
		if got := len(r.globals); got != baseline {
			t.Fatalf("iteration %d retained %d temporary roots", iteration, got-baseline)
		}
	}
	property := r.GetProperty(argument, "answer")
	defer r.ReleaseValue(property)
	if property.Export() != float64(42) {
		t.Fatal("settlement released the caller-owned argument")
	}
}

func TestPromiseSettlementCanReenterResolver(t *testing.T) {
	r := (Factory{}).New().(*adapter)
	defer r.Close()
	ctx := context.Background()
	promise := r.NewPromise()
	defer r.ReleaseValue(promise.Value)
	if err := r.Set("reenter", r.TransientFunction(func(engine.Value, []engine.Value) (engine.Value, error) {
		return nil, promise.Reject("ignored reentrant settlement")
	})); err != nil {
		t.Fatal(err)
	}
	value, err := r.Eval(ctx, `({answer:42,get then(){reenter();return undefined}})`, "then-getter.js")
	if err != nil {
		t.Fatal(err)
	}
	defer r.ReleaseValue(value)
	if err := promise.Resolve(value); err != nil {
		t.Fatal(err)
	}
	result, done, err := r.Await(promise.Value)
	if err != nil || !done || !r.StrictEqual(result, value) {
		t.Fatalf("reentrant settlement: %v %v %v", result, done, err)
	}
	r.ReleaseValue(result)
}

func TestHostPromiseSettlementKeepsJavaScriptReferences(t *testing.T) {
	r := (Factory{}).New().(*adapter)
	defer r.Close()
	ctx := context.Background()
	var pending []engine.Promise
	if err := r.Set("hostPromise", r.TransientFunction(func(engine.Value, []engine.Value) (engine.Value, error) {
		promise := r.NewHostPromise()
		pending = append(pending, promise)
		return promise.Value, nil
	})); err != nil {
		t.Fatal(err)
	}
	baseline := len(r.globals)
	value, err := r.Eval(ctx, `globalThis.savedPromises=Array.from({length:64},()=>hostPromise());undefined`, "pending-host-promises.js")
	if err != nil {
		t.Fatal(err)
	}
	r.ReleaseValue(value)
	if got := len(r.globals) - baseline; got != 2*len(pending) {
		t.Fatalf("pending host Promises own %d roots, want two resolvers each", got)
	}
	for index, promise := range pending {
		if err := promise.Resolve(map[string]any{"index": index}); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(r.globals); got != baseline {
		t.Fatalf("completed host Promises retained %d roots", got-baseline)
	}
	value, err = r.Eval(ctx, `Promise.all(savedPromises).then(values=>values.every((value,index)=>value.index===index))`, "retained-host-promises.js")
	if err != nil {
		t.Fatal(err)
	}
	defer r.ReleaseValue(value)
	if err := r.MicrotaskCheckpoint(); err != nil {
		t.Fatal(err)
	}
	result, done, err := r.Await(value)
	if err != nil || !done || result.Export() != true {
		t.Fatalf("JavaScript-owned Promise results: %v %v %v", result, done, err)
	}
	r.ReleaseValue(result)
}

func TestRethrownExceptionsReleaseHandledRoots(t *testing.T) {
	r := (Factory{}).New().(*adapter)
	defer r.Close()
	ctx := context.Background()
	if err := r.Set("hostCall", r.TransientFunction(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return r.Call(ctx, args[0], nil)
	})); err != nil {
		t.Fatal(err)
	}
	baseline := len(r.globals)
	value, err := r.Eval(ctx, `(()=>{const original={reason:'same identity'};for(let i=0;i<100;i++){try{hostCall(()=>{throw original})}catch(error){if(error!==original)return false}}return true})()`, "rethrow.js")
	if err != nil || value.Export() != true {
		t.Fatalf("rethrow identity: %v %v", value, err)
	}
	r.ReleaseValue(value)
	if got := len(r.globals); got != baseline {
		t.Fatalf("handled exceptions retained %d roots", got-baseline)
	}
}

func TestRetainedBorrowedValueOwnsIndependentRoot(t *testing.T) {
	r := (Factory{}).New().(*adapter)
	defer r.Close()
	ctx := context.Background()
	var retained engine.Value
	if err := r.Set("retain", r.TransientFunction(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		retained = r.RetainValue(args[0])
		return args[0], nil
	})); err != nil {
		t.Fatal(err)
	}
	baseline := len(r.globals)
	value, err := r.Eval(ctx, `globalThis.saved=()=>42;retain(saved)===saved`, "retain-borrowed.js")
	if err != nil || value.Export() != true {
		t.Fatalf("borrowed return identity: %v %v", value, err)
	}
	r.ReleaseValue(value)
	if got := len(r.globals) - baseline; got != 1 {
		t.Fatalf("retained callback owns %d roots, want one", got)
	}
	result, err := r.Call(ctx, retained, nil)
	if err != nil || result.Export() != float64(42) {
		t.Fatalf("callback after borrowed scope closed: %v %v", result, err)
	}
	r.ReleaseValue(result)
	r.ReleaseValue(retained)
	if got := len(r.globals); got != baseline {
		t.Fatalf("released callback kept %d roots", got-baseline)
	}
	value, err = r.Eval(ctx, `saved()`, "retained-by-script.js")
	if err != nil || value.Export() != float64(42) {
		t.Fatalf("retention cleanup lost JS reference: %v %v", value, err)
	}
	r.ReleaseValue(value)
}

func TestHostValueReturnReleasesOnlyTransferredRoot(t *testing.T) {
	r := (Factory{}).New().(*adapter)
	defer r.Close()
	ctx := context.Background()
	var retained engine.Value
	if err := r.Set("handoff", r.TransientFunction(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		owned := r.RetainValue(args[0])
		retained = r.RetainValue(owned)
		return r.ReturnValueAndRelease(owned), nil
	})); err != nil {
		t.Fatal(err)
	}
	baseline := len(r.globals)
	value, err := r.Eval(ctx, `globalThis.saved=()=>42;handoff(saved)===saved`, "handoff.js")
	if err != nil || value.Export() != true {
		t.Fatalf("host return identity: %v %v", value, err)
	}
	r.ReleaseValue(value)
	if got := len(r.globals) - baseline; got != 1 {
		t.Fatalf("host handoff retained %d roots, want only the independent owner", got)
	}
	// The API outside a host callback must preserve ordinary caller ownership.
	if r.ReturnValueAndRelease(retained) != retained || len(r.globals) != baseline+1 {
		t.Fatal("handoff outside a callback consumed a caller-owned value")
	}
	result, err := r.Call(ctx, retained, nil)
	if err != nil || result.Export() != float64(42) {
		t.Fatalf("independent handle after handoff: %v %v", result, err)
	}
	r.ReleaseValue(result)
	r.ReleaseValue(retained)
	if got := len(r.globals); got != baseline {
		t.Fatalf("handoff cleanup kept %d roots", got-baseline)
	}
	value, err = r.Eval(ctx, `saved()`, "handoff-js-owner.js")
	if err != nil || value.Export() != float64(42) {
		t.Fatalf("handoff lost JavaScript reference: %v %v", value, err)
	}
	r.ReleaseValue(value)
}
