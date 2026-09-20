package layoutblitz

import (
	"github.com/moreveal/mimic/internal/dom"
	"strings"
	"testing"
)

func TestStyleBatchHiddenScalarParity(t *testing.T) {
	source, err := dom.Parse(`<html><body><div style="display:none;color:rgb(1,2,3);font-size:23px;--size:42px"><span style="width:var(--size)"></span></div></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := FromSnapshot(source.DerivedSnapshot(), 800, 600)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err = owner.Resolve(0); err != nil {
		t.Fatal(err)
	}
	id := uint64(source.FindAllByTagName("span")[0].ID)
	names := []string{"display", "color", "font-size", "--size", "width"}
	values, err := owner.StyleBatch(id, names)
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range names {
		scalar, err := owner.Style(id, name)
		if err != nil || scalar != values[i] {
			t.Fatalf("%s: batch %q scalar %q err %v", name, values[i], scalar, err)
		}
	}
	if has, err := owner.HasComputedStyle(id); err != nil || has {
		t.Fatalf("batch invented hidden primary style: %v %v", has, err)
	}
	// A large property value exercises the non-poisoning output capacity retry.
	long := strings.Repeat("x", 8192)
	if err := owner.Attribute(id, "", "style", "--long:"+long); err != nil {
		t.Fatal(err)
	}
	if _, err = owner.Resolve(0); err != nil {
		t.Fatal(err)
	}
	values, err = owner.StyleBatch(id, []string{"--long"})
	if err != nil || values[0] != long {
		t.Fatalf("capacity retry length %d err %v", len(values), err)
	}
	if _, err = owner.Style(id, "display"); err != nil {
		t.Fatalf("capacity retry poisoned owner: %v", err)
	}
}
