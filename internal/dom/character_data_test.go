package dom

import "testing"

func TestParsedCommentsRetainHydrationMarkers(t *testing.T) {
	d, err := Parse("<main><!--$-->content<!--/$--></main>")
	if err != nil {
		t.Fatal(err)
	}
	root, _ := d.Find("main")
	children := d.Children(root.ID)
	if len(children) != 3 || children[0].Type != "comment" || children[0].Text != "$" || children[2].Text != "/$" {
		t.Fatalf("markers: %+v", children)
	}
	if text := d.TextContent(root.ID); text != "content" {
		t.Fatalf("descendant text: %q", text)
	}
}

func TestCanonicalDOMStringPreservesLoneSurrogates(t *testing.T) {
	d, _ := Parse("<main></main>")
	root, _ := d.Find("main")
	node := d.CreateText("initial")
	d.InsertNode(root.ID, node.ID, 0)
	if err := d.SetCharacterDataJSON(node.ID, `"\ud800x"`); err != nil {
		t.Fatal(err)
	}
	if got := d.TextContentJSON(root.ID); got != `"\ud800x"` {
		t.Fatal(got)
	}
	d.SetTextContent(node.ID, "new")
	if got := d.TextContentJSON(root.ID); got != `"new"` {
		t.Fatal(got)
	}
}
