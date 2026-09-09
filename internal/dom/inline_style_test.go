package dom

import "testing"

func TestInlineStyleCanonicalAttributeMutation(t *testing.T) {
	d, err := Parse(`<div id="a"></div><div id="b"></div>`)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := d.Find("#a")
	b, _ := d.Find("#b")
	const text = "flex-grow: 2; flex-shrink: ; flex-basis: ;"
	const data = `[{"name":"flex-grow","value":"2","priority":""},{"name":"flex-shrink","value":"","priority":""},{"name":"flex-basis","value":"","priority":""}]`
	if _, err = d.SetInlineStyle(a.ID, text, data); err != nil {
		t.Fatal(err)
	}
	if err = d.SetAttribute(a.ID, "style", text); err != nil {
		t.Fatal(err)
	}
	if d.InlineStyleState(a.ID) != "j"+data {
		t.Fatal("same-value attribute write lost parsed state")
	}
	if err = d.SetAttributeNS(a.ID, "", "style", text); err != nil {
		t.Fatal(err)
	}
	if d.InlineStyleState(a.ID) != "j"+data {
		t.Fatal("same-value namespace write lost parsed state")
	}
	if err = d.SetAttribute(b.ID, "style", text); err != nil {
		t.Fatal(err)
	}
	d.CopyInlineStyle(a.ID, b.ID)
	if err = d.SetAttribute(a.ID, "style", "color: red;"); err != nil {
		t.Fatal(err)
	}
	if d.InlineStyleState(a.ID) != "scolor: red;" {
		t.Fatal("changed attribute retained stale parsed state")
	}
	if d.InlineStyleState(b.ID) != "j"+data {
		t.Fatal("clone shared mutable style state")
	}
	if err = d.RemoveAttribute(b.ID, "style"); err != nil {
		t.Fatal(err)
	}
	if d.InlineStyleState(b.ID) != "s" {
		t.Fatal("removed attribute retained parsed state")
	}
}
