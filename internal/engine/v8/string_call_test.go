//go:build windows && amd64

package v8

import (
	"context"
	"testing"
	"time"

	"github.com/moreveal/mimic/internal/engine"
)

func TestStringCallPreservesValuesWithoutRetainingScratchRoots(t *testing.T) {
	r := (Factory{}).New().(*adapter)
	defer r.Close()
	ctx := context.Background()
	fn, err := r.Eval(ctx, `(prefix,obj,n)=>{obj.count++;return prefix+':'+n+':'+obj.count}`, "string-call")
	if err != nil {
		t.Fatal(err)
	}
	obj, err := r.Eval(ctx, `({count:0})`, "object")
	if err != nil {
		t.Fatal(err)
	}
	err = r.RunOnOwner(ctx, func(ctx context.Context) error {
		before := len(r.globals)
		for i := 0; i < 100; i++ {
			if _, err := r.CallString(ctx, fn, "Привет\x00🌍", obj, i); err != nil {
				return err
			}
		}
		if got := len(r.globals); got != before {
			t.Fatalf("scratch roots retained: before=%d after=%d", before, got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := r.CallString(ctx, fn, "Привет\x00🌍", obj, 7); err != nil || got != "Привет\x00🌍:7:101" {
		t.Fatalf("string result %q, %v", got, err)
	}
}

func TestStringCallNestedExceptionJobsAndCancellation(t *testing.T) {
	r := (Factory{}).New().(*adapter)
	defer r.Close()
	ctx := context.Background()
	fn, err := r.Eval(ctx, `value=>{if(value.fail)throw value;Promise.resolve().then(()=>globalThis.finished=true);return value.text}`, "string-callback")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Set("bridge", r.TransientFunction(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		value, err := r.CallString(ctx, fn, args[0])
		return r.Value(value), err
	})); err != nil {
		t.Fatal(err)
	}
	value, err := r.Eval(ctx, `(()=>{const object={text:'ok'};if(bridge(object)!=='ok'||globalThis.finished)return false;const error={fail:true};try{bridge(error)}catch(e){return e===error}return false})()`, "nested-string-call")
	if err != nil || value.Export() != true {
		t.Fatalf("nested call: %v %v", value, err)
	}
	if err := r.MicrotaskCheckpoint(); err != nil || r.Get("finished").Export() != true {
		t.Fatalf("checkpoint: %v", err)
	}
	bad, _ := r.Eval(ctx, `()=>({toString(){throw Error('must not coerce')}})`, "non-string")
	if _, err := r.CallString(ctx, bad); err == nil {
		t.Fatal("accepted a non-string result")
	}
	loop, _ := r.Eval(ctx, `()=>{for(;;){}}`, "interrupt")
	deadline, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	if _, err := r.CallString(deadline, loop); err == nil {
		t.Fatal("infinite callback was not interrupted")
	}
	if value, err := r.Eval(ctx, `6*7`, "after-interrupt"); err != nil || value.String() != "42" {
		t.Fatalf("runtime did not recover: %v %v", value, err)
	}
}
