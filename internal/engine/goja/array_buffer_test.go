package gojaengine

import (
	"context"
	"github.com/moreveal/mimic/internal/engine"
	"testing"
)

func TestArrayBufferDetachmentPreservesBrandAndInvalidatesViews(t *testing.T) {
	r := Factory{}.New()
	defer r.Close()
	_, err := r.Eval(context.Background(), `globalThis.buffer=new ArrayBuffer(16);globalThis.view=new Uint8Array(buffer);view[0]=17;Object.defineProperty(buffer,Symbol.toStringTag,{value:'Changed'});globalThis.fake={get arbitrary(){throw Error('getter observed')}}`, "detach.js")
	if err != nil {
		t.Fatal(err)
	}
	d := r.(engine.ArrayBufferDetacher)
	if err = d.DetachArrayBuffer(r.Get("fake")); err == nil {
		t.Fatal("non-buffer accepted")
	}
	if err = d.DetachArrayBuffer(r.Get("buffer")); err != nil {
		t.Fatal(err)
	}
	v, err := r.Eval(context.Background(), `buffer.byteLength===0&&view.byteLength===0&&view[0]===undefined`, "detached.js")
	if err != nil || v.Export() != true {
		t.Fatalf("%v %v", v, err)
	}
	if err = d.DetachArrayBuffer(r.Get("buffer")); err != nil {
		t.Fatal(err)
	}
}
