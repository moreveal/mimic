package dom

import "testing"

func TestHTMLScriptIdentityIncludesNamespace(t *testing.T) {
	d, err := Parse("<!doctype html><body><script></script><svg><script></script></svg><div></div>")
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range d.FindAll("*") {
		want := node.TagName == "SCRIPT" && node.Namespace == "http://www.w3.org/1999/xhtml"
		if got := d.IsHTMLScript(node.ID); got != want {
			t.Fatalf("%s in %s: got %v, want %v", node.TagName, node.Namespace, got, want)
		}
	}
	if d.IsHTMLScript(0) || d.IsHTMLScript(d.Root().ID) || d.IsHTMLScript(d.CreateText("script").ID) {
		t.Fatal("a missing or non-element node was treated as an HTML script")
	}
}
