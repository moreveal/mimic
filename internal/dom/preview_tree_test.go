package dom

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestPreviewTreeIdentityDoesNotMutateCanonicalDOM(t *testing.T) {
	d, err := Parse(`<html><body><input id="field" data-mimic-preview-node="author"></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	before := d.Revision()
	first, err := d.PreviewTree(d.Root().ID, nil, nil, "realm-a")
	if err != nil {
		t.Fatal(err)
	}
	var a bytes.Buffer
	if err := html.Render(&a, first); err != nil {
		t.Fatal(err)
	}
	second, err := d.PreviewTree(d.Root().ID, nil, nil, "realm-a")
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	html.Render(&b, second)
	if a.String() != b.String() || !strings.Contains(a.String(), `data-mimic-preview-node="realm-a:`) {
		t.Fatal("unstable preview identities")
	}
	if d.Revision() != before {
		t.Fatal("preview mutated canonical revision")
	}
	n, _ := d.Find("#field")
	if n.Attributes["data-mimic-preview-node"] != "author" {
		t.Fatal("preview mutated author attribute")
	}
	ordinary, err := d.SnapshotTree(d.Root().ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	html.Render(&out, ordinary)
	if strings.Contains(out.String(), "realm-a:") {
		t.Fatal("debug identities leaked into ordinary export")
	}
}
