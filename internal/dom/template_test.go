package dom

import "testing"

func TestTemplateCanonicalContents(t *testing.T) {
	d, err := Parse(`<!doctype html><title>active</title><body><template id="t"><title>inert</title><span id="hidden">A</span><template><b>B</b></template><script>inert()</script></template><script>active()</script>`)
	if err != nil {
		t.Fatal(err)
	}
	template, ok := d.Find("#t")
	if !ok {
		t.Fatal("missing template")
	}
	content, ok := d.TemplateContent(template.ID)
	if !ok || content.Type != "fragment" || content.Parent != 0 || content.TemplateHost != template.ID {
		t.Fatalf("content %#v", content)
	}
	if len(template.Children) != 0 || len(content.Children) != 4 {
		t.Fatalf("template children=%v content=%v", template.Children, content.Children)
	}
	if _, found := d.FindWithin(d.Root().ID, "#hidden"); found {
		t.Fatal("document query traversed template contents")
	}
	if _, found := d.FindWithin(content.ID, "#hidden"); !found {
		t.Fatal("content query missed canonical child")
	}
	if d.IsConnected(content.ID) || d.Contains(template.ID, content.ID) {
		t.Fatal("template host link leaked into parent traversal")
	}
	if d.Title() != "active" {
		t.Fatalf("inert title became document title: %q", d.Title())
	}
	scripts := d.Scripts()
	if len(scripts) != 1 || scripts[0].Text != "active()" {
		t.Fatalf("parser scripts include inert template contents: %#v", scripts)
	}
}

func TestTemplateInnerHTMLAndHostInclusiveCycle(t *testing.T) {
	d, err := Parse(`<body><template id="t"><b>old</b></template>`)
	if err != nil {
		t.Fatal(err)
	}
	template, _ := d.Find("#t")
	content, _ := d.TemplateContent(template.ID)
	old := content.Children[0]
	if err := d.SetInnerHTML(template.ID, `<span>A<template><i>B</i></template>C</span>`); err != nil {
		t.Fatal(err)
	}
	updated, _ := d.TemplateContent(template.ID)
	if updated.ID != content.ID {
		t.Fatal("content identity changed")
	}
	oldNode, _ := d.Get(old)
	if oldNode.Parent != 0 {
		t.Fatal("replaced content retains parent")
	}
	html, err := d.InnerHTML(template.ID)
	if err != nil || html != `<span>A<template><i>B</i></template>C</span>` {
		t.Fatalf("markup=%q err=%v", html, err)
	}
	if err := d.InsertNode(content.ID, template.ID, 0); err == nil {
		t.Fatal("accepted template host cycle")
	}
	if err := d.InsertNode(updated.Children[0], template.ID, 0); err == nil {
		t.Fatal("accepted descendant host cycle")
	}
	if d.TextContent(content.ID) != "AC" {
		t.Fatalf("ordinary text traversal included nested content: %q", d.TextContent(content.ID))
	}
}

func TestTemplateScriptCloneState(t *testing.T) {
	d, _ := Parse(`<body><template id="t"><script>parsed()</script></template>`)
	template, _ := d.Find("#t")
	content, _ := d.TemplateContent(template.ID)
	if d.ScriptStarted(content.Children[0]) {
		t.Fatal("template parser script already started")
	}
	if err := d.SetInnerHTML(template.ID, `<script>fragment()</script>`); err != nil {
		t.Fatal(err)
	}
	content, _ = d.TemplateContent(template.ID)
	script := content.Children[0]
	if !d.ScriptStarted(script) {
		t.Fatal("fragment parser script should be inert")
	}
	clone := d.CreateElement("script")
	d.CopyNodeState(script, clone.ID)
	if !d.ScriptStarted(clone.ID) {
		t.Fatal("clone discarded script state")
	}
}
