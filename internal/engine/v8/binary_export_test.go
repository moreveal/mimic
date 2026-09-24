//go:build (windows || linux) && amd64

package v8

import (
	"context"
	"reflect"
	"testing"

	"github.com/moreveal/mimic/internal/engine"
)

func TestBinaryViewsExportTheirExactBytes(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	var captured any
	if err := runtime.Set("capture", runtime.Function(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		captured = args[0].Export()
		return nil, nil
	})); err != nil {
		t.Fatal(err)
	}
	const prefix = `const bytes = new Uint8Array([17,34,51,68,85,102,119,136]);`
	cases := []struct {
		name   string
		expr   string
		wanted []byte
	}{
		{"ArrayBuffer", "bytes.buffer", []byte{17, 34, 51, 68, 85, 102, 119, 136}},
		{"Uint8Array offset", "new Uint8Array(bytes.buffer, 2, 4)", []byte{51, 68, 85, 102}},
		{"Uint16Array offset", "new Uint16Array(bytes.buffer, 2, 2)", []byte{51, 68, 85, 102}},
		{"Int32Array offset", "new Int32Array(bytes.buffer, 4, 1)", []byte{85, 102, 119, 136}},
		{"Float64Array", "new Float64Array(bytes.buffer)", []byte{17, 34, 51, 68, 85, 102, 119, 136}},
		{"BigInt64Array", "new BigInt64Array(bytes.buffer)", []byte{17, 34, 51, 68, 85, 102, 119, 136}},
		{"DataView offset", "new DataView(bytes.buffer, 1, 5)", []byte{34, 51, 68, 85, 102}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := `(() => {` + prefix + `const view = ` + tc.expr + `; capture(view); return view; })()`
			value, err := runtime.Eval(context.Background(), source, tc.name)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(captured, tc.wanted) {
				t.Fatalf("callback bytes = %v, want %v", captured, tc.wanted)
			}
			if got := value.Export(); !reflect.DeepEqual(got, tc.wanted) {
				t.Fatalf("evaluation bytes = %v, want %v", got, tc.wanted)
			}
		})
	}
	t.Run("SharedArrayBuffer", func(t *testing.T) {
		value, err := runtime.Eval(context.Background(), `(() => {
			const buffer = new SharedArrayBuffer(4);
			new Uint8Array(buffer).set([9, 8, 7, 6]);
			capture(buffer);
			return buffer;
		})()`, "SharedArrayBuffer")
		if err != nil {
			t.Fatal(err)
		}
		wanted := []byte{9, 8, 7, 6}
		if !reflect.DeepEqual(captured, wanted) || !reflect.DeepEqual(value.Export(), wanted) {
			t.Fatalf("shared bytes: callback=%v evaluation=%v, want %v", captured, value.Export(), wanted)
		}
	})
}
