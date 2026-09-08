package dom

import "testing"

func TestFramePresenceCoversAllNodeCreationPaths(t *testing.T) {
	for name, create := range map[string]func(*Document){
		"create":    func(d *Document) { d.CreateElement("iframe") },
		"namespace": func(d *Document) { d.CreateElementNS("http://www.w3.org/1999/xhtml", "iframe") },
		"append":    func(d *Document) { d.AppendElement(d.Root().ID, "iframe", nil) },
		"innerHTML": func(d *Document) {
			node := d.CreateElement("div")
			if err := d.SetInnerHTML(node.ID, "<section><iframe></iframe></section>"); err != nil {
				t.Fatal(err)
			}
			d.SetInnerHTML(node.ID, "")
		},
	} {
		t.Run(name, func(t *testing.T) {
			d, err := Parse("<!doctype html><body></body>")
			if err != nil {
				t.Fatal(err)
			}
			if d.HasFrameElements() {
				t.Fatal("empty document has frame elements")
			}
			create(d)
			if !d.HasFrameElements() {
				t.Fatal("frame insertion steps disabled for a reusable iframe")
			}
		})
	}
	d, err := Parse("<!doctype html><body><iframe></iframe></body>")
	if err != nil || !d.HasFrameElements() {
		t.Fatalf("parsed iframe: %v", err)
	}
}
