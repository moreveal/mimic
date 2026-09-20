package layoutblitz

import (
	"github.com/moreveal/mimic/internal/dom"
	"github.com/moreveal/mimic/internal/textmetrics"
	"testing"
)

func TestCanonicalFontCollectionReplacement(t *testing.T) {
	metrics := textmetrics.New()
	defer metrics.Close()
	id, err := metrics.LocalFont("Courier New")
	if err != nil || id == "" {
		t.Skip("Courier New unavailable")
	}
	data, index, err := metrics.FontResourceBytes(id)
	if err != nil || index != 0 {
		t.Fatalf("font bytes: %d %v", index, err)
	}
	document, err := dom.Parse(`<html><body><span id="target" style="display:inline-block;font:20px NativeAlias,Arial">iiiiWWAV</span></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := document.DerivedSnapshot()
	owner, err := FromSnapshot(snapshot, 1280, 800)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	var target uint64
	for _, node := range snapshot.Nodes {
		if node.Attributes["id"] == "target" {
			target = uint64(node.ID)
		}
	}
	measure := func() float64 {
		t.Helper()
		if _, err := owner.Resolve(0); err != nil {
			t.Fatal(err)
		}
		r, err := owner.Rect(target)
		if err != nil {
			t.Fatal(err)
		}
		return r.Width
	}
	before := measure()
	if err := owner.ReplaceFonts([]Font{{Family: "NativeAlias", Weight: 400, Bytes: data}}); err != nil {
		t.Fatal(err)
	}
	after := measure()
	if after == before {
		t.Fatalf("font activation did not affect native width: %v", before)
	}
	if err := owner.ReplaceFonts(nil); err != nil {
		t.Fatal(err)
	}
	if removed := measure(); removed != before {
		t.Fatalf("removed font retained: %v != %v", removed, before)
	}
}
