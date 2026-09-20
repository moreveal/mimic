package layoutblitz

import (
	"github.com/moreveal/mimic/internal/dom"
	"strings"
	"testing"
)

func TestRetiredInputsEvictWithinDetachingSync(t *testing.T) {
	source, err := dom.Parse(`<html><body></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	body := source.FindAllByTagName("body")[0].ID
	var document Document
	defer document.Close()
	sync := func() {
		t.Helper()
		if err := document.Sync(source, 800, 600); err != nil {
			t.Fatal(err)
		}
	}
	sync()
	for i := 0; i < 2; i++ {
		text := source.CreateText(strings.Repeat("x", int(retiredInputByteLimit/2)+1))
		if err := source.InsertNode(body, text.ID, 0); err != nil {
			t.Fatal(err)
		}
		sync()
		owner, builds := document.Owner, document.Builds
		if err := source.RemoveNode(body, text.ID); err != nil {
			t.Fatal(err)
		}
		sync()
		if document.Owner == owner || document.Builds != builds+1 || owner.handle != nil {
			t.Fatal("detaching transaction did not rebuild and dispose native owner")
		}
		if document.retiredBytes != 0 || len(document.retired) != 0 {
			t.Fatalf("retained input after eviction: %d bytes/%d nodes", document.retiredBytes, len(document.retired))
		}
		owner, builds = document.Owner, document.Builds
		sync()
		if document.Owner != owner || document.Builds != builds {
			t.Fatal("clean observation rebuilt")
		}
	}
	document.Close()
	if document.Owner != nil || document.retiredBytes != 0 || document.retired != nil || document.nodes != nil {
		t.Fatal("close retained document products")
	}
}

func TestRetiredCountEvictsWithinDetachingSync(t *testing.T) {
	source, err := dom.Parse(`<html><body><div></div></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	container := source.FindAllByTagName("div")[0]
	for i := 0; i < 1100; i++ {
		if _, err := source.AppendElement(container.ID, "span", nil); err != nil {
			t.Fatal(err)
		}
	}
	var document Document
	defer document.Close()
	if err := document.Sync(source, 800, 600); err != nil {
		t.Fatal(err)
	}
	if err := source.RemoveNode(container.Parent, container.ID); err != nil {
		t.Fatal(err)
	}
	if err := document.Sync(source, 800, 600); err != nil {
		t.Fatal(err)
	}
	if document.Builds != 2 || document.detached != 0 {
		t.Fatalf("count bound not applied: builds%d detached%d", document.Builds, document.detached)
	}
}
