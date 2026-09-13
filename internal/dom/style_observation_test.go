package dom

import (
	"encoding/json"
	"testing"
)

func TestStyleObservationStateTracksCanonicalWrites(t *testing.T) {
	d, err := Parse(`<!doctype html><div id="x" data-value="one" style="width:10px"></div>`)
	if err != nil {
		t.Fatal(err)
	}
	n, ok := d.Find("#x")
	if !ok {
		t.Fatal("missing fixture")
	}
	read := func() struct {
		Parent     int64
		Attributes map[string]string
		Inline     string
	} { t.Helper(); var state struct {
		Parent     int64
		Attributes map[string]string
		Inline     string
	}; if err := json.Unmarshal([]byte(d.StyleObservationState(n.ID)), &state); err != nil {
		t.Fatal(err)
	}; return state }
	before := d.Revision()
	first := read()
	if first.Parent != n.Parent || first.Attributes["data-value"] != "one" || first.Inline != "swidth:10px" || d.Revision() != before {
		t.Fatalf("initial state: %+v", first)
	}
	if err := d.SetAttribute(n.ID, "data-value", "two"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetInlineStyle(n.ID, "width: 20px;", `[{"name":"width","value":"20px","priority":""}]`); err != nil {
		t.Fatal(err)
	}
	next := read()
	if next.Attributes["data-value"] != "two" || next.Inline != `j[{"name":"width","value":"20px","priority":""}]` || first.Attributes["data-value"] != "one" {
		t.Fatalf("updated state: %+v", next)
	}
}
