package dom

import (
	"errors"
	"testing"
)

func TestInsertionValidationIsAtomic(t *testing.T) {
	d, err := Parse("<body><div id=source><b id=child></b><i id=reference></i></div><div id=target></div>")
	if err != nil {
		t.Fatal(err)
	}
	source, _ := d.Find("#source")
	child, _ := d.Find("#child")
	reference, _ := d.Find("#reference")
	target, _ := d.Find("#target")
	if err := d.InsertNode(target.ID, child.ID, reference.ID); !errors.Is(err, ErrInsertionReference) {
		t.Fatalf("invalid reference: %v", err)
	}
	if err := d.InsertNode(child.ID, source.ID, 0); !errors.Is(err, ErrInsertionCycle) {
		t.Fatalf("cycle: %v", err)
	}
	if n, _ := d.Get(child.ID); n.Parent != source.ID || d.ChildCount(source.ID) != 2 || d.ChildCount(target.ID) != 0 {
		t.Fatal("failed insertion changed canonical membership")
	}
}

func TestDrainFragmentPreservesOwnershipAndIdentity(t *testing.T) {
	d, _ := Parse("<body></body>")
	fragment := d.CreateDocumentFragment()
	children := []int64{d.CreateText("text").ID, d.CreateComment("comment").ID}
	for _, id := range children {
		if err := d.InsertNode(fragment.ID, id, 0); err != nil {
			t.Fatal(err)
		}
	}
	ids, err := d.DrainFragment(fragment.ID)
	if err != nil || len(ids) != 2 || ids[0] != children[0] || ids[1] != children[1] || d.ChildCount(fragment.ID) != 0 {
		t.Fatalf("fragment transfer: %v, %v", ids, err)
	}
	for _, id := range ids {
		if n, ok := d.Get(id); !ok || n.Parent != 0 || n.OwnerDocument != fragment.OwnerDocument {
			t.Fatal("transfer changed identity or ownership")
		}
	}
}
