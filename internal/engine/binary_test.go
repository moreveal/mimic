package engine_test

import (
	"context"
	"testing"

	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestBinaryBufferCopiesIntoRealmOwnedStorage(t *testing.T) {
	for name, factory := range map[string]engine.Factory{"goja": gojaengine.Factory{}, "v8": v8engine.Factory{}} {
		t.Run(name, func(t *testing.T) {
			r := factory.New()
			defer r.Close()
			source := []byte{0, 128, 255, 65}
			if err := r.Set("record", map[string]any{"body": engine.BinaryBuffer(source), "empty": engine.BinaryBuffer(nil)}); err != nil {
				t.Fatal(err)
			}
			source[0] = 99
			result, err := r.Eval(context.Background(), `(()=>{const view=new Uint8Array(record.body);if(!(record.body instanceof ArrayBuffer)||!(record.empty instanceof ArrayBuffer)||record.empty.byteLength!==0||String(view)!=='0,128,255,65')return false;view[1]=7;return true})()`, "binary.js")
			if err != nil || result.Export() != true || source[1] != 128 {
				t.Fatalf("binary storage must be independent: %v, %v, %v", result, err, source)
			}
			// Native conversion must not call author-replaced constructors.
			if _, err := r.Eval(context.Background(), `globalThis.ArrayBuffer=function(){throw Error('author constructor')}`, "poison.js"); err != nil {
				t.Fatal(err)
			}
			if err := r.Set("readBinary", r.Function(func(engine.Value, []engine.Value) (engine.Value, error) {
				return r.Value(map[string]any{"nested": []any{engine.BinaryBuffer(source)}}), nil
			})); err != nil {
				t.Fatal(err)
			}
			promise := r.NewPromise()
			if err := r.Set("pendingBinary", promise.Value); err != nil {
				t.Fatal(err)
			}
			if err := promise.Resolve(map[string]any{"body": engine.BinaryBuffer(source)}); err != nil {
				t.Fatal(err)
			}
			source[0] = 111
			result, err = r.Eval(context.Background(), `globalThis.binaryResult=false;pendingBinary.then(x=>{binaryResult=String(new Uint8Array(x.body))==='99,128,255,65'&&String(new Uint8Array(readBinary().nested[0]))==='111,128,255,65'})`, "promise-binary.js")
			if err != nil {
				t.Fatal(err)
			}
			if err := r.MicrotaskCheckpoint(); err != nil {
				t.Fatal(err)
			}
			if r.Get("binaryResult").Export() != true {
				t.Fatal("callback or Promise lost native binary storage")
			}
		})
	}
}
