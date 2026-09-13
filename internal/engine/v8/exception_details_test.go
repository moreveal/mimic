//go:build (windows || linux) && amd64

package v8

import (
	"context"
	"testing"

	"github.com/moreveal/mimic/internal/engine"
)

func TestExceptionDetailsDoNotReadPublicStack(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	var details engine.ExceptionDetails
	var inspected bool
	if err := runtime.Set("describe", runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		details, inspected = runtime.(engine.ExceptionInspector).DescribeException(args[0])
		return nil, nil
	})); err != nil {
		t.Fatal(err)
	}
	v, err := runtime.Eval(context.Background(), `let hooks=0;Error.prepareStackTrace=()=>{hooks++;throw Error('hook')};const error=new Error('sentinel');Object.defineProperty(error,'stack',{get(){hooks++;throw Error('public stack')}});describe(error);hooks`, "exception-oracle.js")
	if err != nil || v.Export() != float64(0) || !inspected {
		t.Fatalf("%v %v inspected=%v", v, err, inspected)
	}
	if details.Message != "Uncaught Error: sentinel" || details.Filename != "exception-oracle.js" || details.Line != 1 || details.Column <= 0 {
		t.Fatalf("%+v", details)
	}
}
