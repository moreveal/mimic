package gojaengine

import (
	"context"
	"testing"
)

func TestHostValueBindingsPreserveJavaScriptIdentity(t *testing.T) {
	runtime := (Factory{}).New()
	defer runtime.Close()
	ctx := context.Background()
	object, err := runtime.Eval(ctx, `globalThis.original={get value(){throw Error('unexpected export')}};original`, "identity.js")
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Set("alias", object); err != nil {
		t.Fatal(err)
	}
	if err := runtime.SetProperty(object, "self", object); err != nil {
		t.Fatal(err)
	}
	if !runtime.StrictEqual(runtime.Value(object), object) {
		t.Fatal("Value copied a realm-owned object")
	}
	got, err := runtime.Eval(ctx, "alias===original&&original.self===original", "check.js")
	if err != nil || got.Export() != true {
		t.Fatalf("binding changed identity: %v %v", got, err)
	}
	undefined, err := runtime.Eval(ctx, "undefined", "undefined.js")
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.SetProperty(object, "missing", undefined); err != nil {
		t.Fatal(err)
	}
	if runtime.TypeOf(runtime.GetProperty(object, "missing")) != "undefined" {
		t.Fatal("undefined wrapper became an object")
	}
}
