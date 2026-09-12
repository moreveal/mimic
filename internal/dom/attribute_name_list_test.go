package dom

import "testing"

func TestAttributeNameListIsOrderedSnapshot(t *testing.T) {
	d, err := Parse("<!doctype html><html><body></body></html>")
	if err != nil {
		t.Fatal(err)
	}
	n := d.CreateElement("div")
	d.SetAttribute(n.ID, "z", "one")
	d.SetAttribute(n.ID, "a", "two")
	names := d.AttributeNameList(n.ID)
	if len(names) != 2 || names[0] != "z" || names[1] != "a" {
		t.Fatal(names)
	}
	names[0] = "corrupt"
	d.RemoveAttribute(n.ID, "z")
	d.SetAttribute(n.ID, "z", "three")
	current := d.AttributeNameList(n.ID)
	if len(current) != 2 || current[0] != "a" || current[1] != "z" {
		t.Fatal(current)
	}
}
