//go:build windows && amd64

package v8

import (
	"context"
	"strings"
	"testing"

	"github.com/moreveal/mimic/internal/engine"
)

func TestCallPreservesJavaScriptException(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	ctx := context.Background()
	if err := runtime.Set("invoke", runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		return runtime.Call(ctx, args[0], args[1])
	})); err != nil {
		t.Fatal(err)
	}
	receiver, err := runtime.Eval(ctx, `({})`, "receiver.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ source, message string }{
		{`Function.prototype.bind`, "Bind must be called on a function"},
		{`Function.prototype.toString`, "Function.prototype.toString requires that 'this' be a Function"},
		{`(()=>{throw new TypeError('call sentinel')})`, "call sentinel"},
	} {
		fn, err := runtime.Eval(ctx, test.source, "callback.js")
		if err != nil {
			t.Fatal(err)
		}
		_, err = runtime.Call(ctx, fn, receiver)
		if err == nil || !strings.Contains(err.Error(), test.message) {
			t.Errorf("Call(%s) error = %v", test.source, err)
		}
		result, err := runtime.Eval(ctx, `(()=>{try{invoke(`+test.source+`,{});return 'not caught'}catch(error){return String(error)}})()`, "reentrant-call.js")
		if err != nil || result == nil || !strings.Contains(result.String(), test.message) {
			t.Errorf("reentrant Call(%s): %v %v", test.source, result, err)
		}
	}
	identity, err := runtime.Eval(ctx, `(()=>{for(const sentinel of [new TypeError('identity'),{},null,undefined,17]){try{invoke(()=>{throw sentinel},{});return false}catch(error){if(error!==sentinel)return false}}return true})()`, "exception-identity.js")
	if err != nil || identity == nil || identity.Export() != true {
		t.Fatalf("exception identity: %v %v", identity, err)
	}
	v, err := runtime.Eval(ctx, `6*7`, "after-call.js")
	if err != nil || v.Export() != float64(42) {
		t.Fatalf("runtime after caught calls: %v %v", v, err)
	}
}
