//go:build (windows || linux) && amd64

package v8

import (
	"context"
	"testing"
)

func TestBootstrapSnapshotSharedBytesOwners(t *testing.T) {
	snapshot, err := (Factory{}).BuildBootstrapSnapshot(context.Background(), `globalThis.seed={answer:42,items:[]}`)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Close()
	first, err := snapshot.NewRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := snapshot.NewRuntime()
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	source := snapshot.(*bootstrapSnapshot).blob
	consumer1 := first.(*adapter).owner.snapshot
	consumer2 := second.(*adapter).owner.snapshot
	if consumer1 == source || consumer2 == source || consumer1 == consumer2 {
		t.Fatal("consumers share native ownership records")
	}
	if &source.Bytes()[0] != &consumer1.Bytes()[0] || &source.Bytes()[0] != &consumer2.Bytes()[0] {
		t.Fatal("immutable bytes were copied")
	}
	if err := snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := source.ShareImmutableBytes(); err == nil {
		t.Fatal("closed source admitted another consumer")
	}
	value, err := first.Eval(context.Background(), `seed.items.push(1);seed.answer`, "first")
	if err != nil || value.String() != "42" {
		t.Fatalf("live first consumer after artifact Close: %v %v", value, err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	value, err = second.Eval(context.Background(), `seed.answer===42 && seed.items.length===0`, "second")
	if err != nil || value.Export() != true {
		t.Fatalf("second consumer after first Close: %v %v", value, err)
	}
}
