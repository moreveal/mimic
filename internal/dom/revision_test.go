package dom

import "testing"

func TestRevisionTracksCanonicalWritesNotReads(t *testing.T) {
	d, err := Parse("<div id='box'>before</div>")
	if err != nil {
		t.Fatal(err)
	}
	box := d.FindAllByTagName("div")[0]
	before := d.Revision()
	d.Get(box.ID)
	d.GetAttribute(box.ID, "id")
	d.FindAllByTagName("div")
	if d.Revision() != before {
		t.Fatal("read advanced mutation revision")
	}
	for name, write := range map[string]func(){
		"attribute":       func() { _ = d.SetAttribute(box.ID, "class", "changed") },
		"text":            func() { _ = d.SetTextContent(box.ID, "after") },
		"parsed children": func() { _ = d.SetInnerHTML(box.ID, "<span>child</span>") },
		"detached node":   func() { d.CreateElement("section") },
	} {
		before = d.Revision()
		write()
		if d.Revision() <= before {
			t.Errorf("%s did not advance mutation revision", name)
		}
	}
}
