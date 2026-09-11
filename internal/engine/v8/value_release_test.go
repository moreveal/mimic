package v8

import (
	"context"
	"testing"
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
