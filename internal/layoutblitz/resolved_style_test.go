package layoutblitz

import (
	"github.com/moreveal/mimic/internal/dom"
	"testing"
)

func TestComputedStyleLogicalUsedValues(t *testing.T) {
	document, err := dom.Parse(`<html><body><div id="target" style="width:120px;height:40px;margin-inline-start:10px;direction:rtl"></div></body></html>`)
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
	var target uint64
	for _, node := range snapshot.Nodes {
		if node.Attributes["id"] == "target" {
			target = uint64(node.ID)
		}
	}
	for property, want := range map[string]string{"inline-size": "120px", "block-size": "40px", "min-inline-size": "0px", "margin-inline-start": "10px", "-webkit-min-logical-height": "0px"} {
		got, err := owner.ComputedStyle(target, property)
		if err != nil || got != want {
			t.Errorf("%s=%q, want %q, err=%v", property, got, want, err)
		}
	}
	if got, err := owner.ComputedStyle(target, "not-a-supported-property"); err != nil || got != "" {
		t.Fatalf("unsupported=%q err=%v", got, err)
	}
	if err := owner.Attribute(target, "", "style", "writing-mode:vertical-rl;direction:rtl;width:120px;height:40px;padding:1px 2px 3px 4px;border:2px solid red"); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.Resolve(0); err != nil {
		t.Fatal(err)
	}
	properties := []string{"inline-size", "block-size", "padding-inline-start", "padding-block-start", "border-left", "transform-origin", "perspective-origin", "font-family"}
	want := []string{"40px", "120px", "3px", "2px", "2px solid rgb(255, 0, 0)", "65px 24px", "65px 24px", `"Times New Roman"`}
	batch, err := owner.StyleBatch(target, properties)
	if err != nil {
		t.Fatal(err)
	}
	for index, property := range properties {
		got, err := owner.ComputedStyle(target, property)
		if err != nil || got != want[index] || batch[index] != got {
			t.Errorf("%s scalar=%q batch=%q want=%q err=%v", property, got, batch[index], want[index], err)
		}
	}
}
