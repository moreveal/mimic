package layoutblitz

import (
	"github.com/moreveal/mimic/internal/dom"
	"testing"
)

func TestCanonicalSnapshot(t *testing.T) {
	document, err := dom.Parse(`<html><head><style>.box {width:123px;height:45px}</style></head><body><div class="box"></div></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := document.DerivedSnapshot()
	owner, err := FromSnapshot(snapshot, 1280, 800)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	if _, err := owner.Resolve(0); err != nil {
		t.Fatal(err)
	}
	var target int64
	for _, node := range snapshot.Nodes {
		if node.Attributes["class"] == "box" {
			target = node.ID
		}
	}
	rect, err := owner.Rect(uint64(target))
	if err != nil || rect.Width != 123 || rect.Height != 45 {
		t.Fatalf("rect=%+v error=%v", rect, err)
	}
}

func TestCanonicalUpdateAndLifetime(t *testing.T) {
	owner, err := New(1, 1280, 800)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	for i, name := range []string{"html", "body", "div"} {
		id := uint64(i + 2)
		if err := owner.Element(id, "http://www.w3.org/1999/xhtml", name); err != nil {
			t.Fatal(err)
		}
		if err := owner.Append(id-1, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := owner.Attribute(4, "", "style", "width:800px;height:32px"); err != nil {
		t.Fatal(err)
	}
	generation, err := owner.Resolve(0)
	if err != nil {
		t.Fatal(err)
	}
	rect, err := owner.Rect(4)
	if err != nil || rect.Width != 800 || rect.Height != 32 {
		t.Fatalf("rect=%+v error=%v", rect, err)
	}
	next, err := owner.Resolve(0)
	if err != nil || next != generation {
		t.Fatalf("clean resolve=%d expected=%d error=%v", next, generation, err)
	}
	if err := owner.Attribute(4, "", "style", "width:640px;height:40px"); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Resolve(0); err != nil {
		t.Fatal(err)
	}
	rect, err = owner.Rect(4)
	if err != nil || rect.Width != 640 || rect.Height != 40 {
		t.Fatalf("updated rect=%+v error=%v", rect, err)
	}
	owner.Close()
	if _, err := owner.Rect(4); err == nil {
		t.Fatal("read after disposal succeeded")
	}
}
