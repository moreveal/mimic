package gojaengine

import (
	"context"
	"testing"
)

func TestTypeOfPreservesPrimitiveKindsWithoutObservingObjects(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	ctx := context.Background()
	for _, test := range []struct{ source, want string }{
		{"undefined", "undefined"}, {"null", "object"}, {"true", "boolean"},
		{"'text'", "string"}, {"42", "number"}, {"42n", "bigint"},
		{"Symbol('key')", "symbol"}, {"(()=>{})", "function"},
		{"({get property(){throw Error('getter executed')},toString(){throw Error('conversion executed')}})", "object"},
		{"new Proxy({}, {get(){throw Error('get trap executed')},ownKeys(){throw Error('keys trap executed')}})", "object"},
	} {
		value, err := runtime.Eval(ctx, test.source, "typeof.js")
		if err != nil {
			t.Fatal(err)
		}
		if got := runtime.TypeOf(value); got != test.want {
			t.Errorf("typeof %s = %s, want %s", test.source, got, test.want)
		}
	}
}
