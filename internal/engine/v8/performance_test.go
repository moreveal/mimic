package v8

import (
	"context"
	"testing"

	"github.com/moreveal/mimic/internal/engine"
)

func TestTransientHostValuesAndRetainedCallbacks(t *testing.T) {
	r := (Factory{}).New().(*adapter)
	defer r.Close()
	var retained engine.Value
	r.Set("keep", r.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) { retained = args[0]; return nil, nil }))
	r.Set("echo", r.TransientFunction(func(_ engine.Value, args []engine.Value) (engine.Value, error) { return args[0], nil }))
	r.Eval(context.Background(), `keep(()=>42)`, "keep.js")
	before := len(r.globals)
	v, err := r.Eval(context.Background(), `(()=>{for(let i=0;i<1000;i++)if(echo(i)!==i)throw Error('echo');return echo('Привет 😀')})()`, "echo.js")
	if err != nil || v.Export() != "Привет 😀" {
		t.Fatalf("%v %v", v, err)
	}
	if len(r.globals) > before+1 {
		t.Fatal("transient arguments created persistent roots")
	}
	v, err = r.Call(context.Background(), retained, nil)
	if err != nil || v.Export() != float64(42) {
		t.Fatalf("retained function: %v %v", v, err)
	}
}

func TestBatchedCallbackRecordsPreserveCollectionValues(t *testing.T) {
	r := (Factory{}).New().(*adapter)
	defer r.Close()
	r.Set("records", r.TransientFunction(func(engine.Value, []engine.Value) (engine.Value, error) {
		return r.Value([]map[string]any{{"empty": []int64(nil), "map": map[string]string(nil), "bytes": []byte{1, 2}, "text": "Привет 😀"}}), nil
	}))
	v, err := r.Eval(context.Background(), `JSON.stringify(records())`, "records.js")
	if err != nil || v.String() != `[{"bytes":[1,2],"empty":[],"map":{},"text":"Привет 😀"}]` {
		t.Fatalf("%v %v", v, err)
	}
}
