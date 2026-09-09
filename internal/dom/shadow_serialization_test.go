package dom

import (
	"bytes"
	"golang.org/x/net/html"
	"strings"
	"testing"
)

func TestShadowProjectionDoesNotMutateCanonicalTree(t *testing.T) {
	d, err := Parse(`<!doctype html><body><div id="host">fallback</div><section id="child">shadow &amp; safe</section>`)
	if err != nil {
		t.Fatal(err)
	}
	host, _ := d.Find("#host")
	child, _ := d.Find("#child")
	before, _ := d.OuterHTML(d.Root().ID)
	tree, err := d.SnapshotTree(d.Root().ID, []ShadowSnapshot{{HostID: host.ID, Mode: "closed", Children: []int64{child.ID}}})
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err = html.Render(&output, tree); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `<template shadowrootmode="closed"><section id="child">shadow &amp; safe</section></template>fallback`) {
		t.Fatal(output.String())
	}
	after, _ := d.OuterHTML(d.Root().ID)
	if before != after {
		t.Fatal("serialization mutated canonical DOM")
	}
	list, err := d.SerializeNodeList([]int64{child.ID})
	if err != nil || list != `<section id="child">shadow &amp; safe</section>` {
		t.Fatalf("list %s %v", list, err)
	}
}
