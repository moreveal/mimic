//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestNavigationRealmRetentionAndTeardown(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "<!doctype html><body>") }))
	defer server.Close()
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err = p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	// Merely navigating does not export any cross-realm objects to the parent.
	if _, err = p.Evaluate(ctx, `(async()=>{globalThis.frame=document.createElement('iframe');document.body.append(frame);for(let i=0;i<20;i++)await new Promise(resolve=>{frame.onload=resolve;frame.src='about:blank?generation='+i});return true})()`); err != nil {
		t.Fatal(err)
	}
	if n := len(p.realmOwners); n != 2 {
		t.Fatalf("unobserved history retained %d realms, want active parent and child", n)
	}
	if result, evalErr := p.Evaluate(ctx, `(async()=>{globalThis.oldDocument=frame.contentDocument;oldDocument.marker=37;await new Promise(resolve=>{frame.onload=resolve;frame.src='about:blank?retained'});return oldDocument.marker===37&&oldDocument!==frame.contentDocument})()`); evalErr != nil || result != true {
		t.Fatalf("retained document: result=%v err=%v", result, evalErr)
	}
	if n := len(p.realmOwners); n != 3 {
		t.Fatalf("exported document must retain exactly its owning realm: %d", n)
	}
	retained := make([]*Realm, 0, len(p.realmOwners))
	for _, r := range p.realmOwners {
		retained = append(retained, r)
	}
	// A new importing realm has no bridge-cache references to the old graph.
	if err = p.Navigate(ctx, server.URL+"/replacement"); err != nil {
		t.Fatal(err)
	}
	if n := len(p.realmOwners); n != 1 {
		t.Fatalf("retired importer graph not released: %d realms", n)
	}
	for _, r := range retained {
		if !r.closed {
			t.Fatalf("realm %s retained after its importer retired", r.ID)
		}
	}
	current := p.Top.Realm
	if err = p.Close(); err != nil {
		t.Fatal(err)
	}
	if len(p.realmOwners) != 0 || !current.closed {
		t.Fatal("Page.Close retained realm runtimes")
	}
	if err = p.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestUnreachableRealmReferenceCyclesAreCollected(t *testing.T) {
	parallelBrowserTest(t)
	p := testPage(t)
	ctx := context.Background()
	if _, err := p.Evaluate(ctx, `document.body.appendChild(document.createElement('iframe'));document.body.appendChild(document.createElement('iframe'));true`); err != nil {
		t.Fatal(err)
	}
	var children []*Realm
	for _, f := range p.Top.children {
		children = append(children, f.Realm)
	}
	if len(children) != 2 {
		t.Fatalf("children: %d", len(children))
	}
	children[0].retainRealm(children[1])
	children[1].retainRealm(children[0])
	p.mu.Lock()
	p.removeDescendantFramesLocked(p.Top)
	p.Top.Realm.childFrames = map[int64]*Frame{}
	p.mu.Unlock()
	for _, r := range children {
		p.retireRealm(r)
	}
	p.collectRealmOwners()
	for _, r := range children {
		if !r.closed {
			t.Fatal("unreachable realm cycle leaked")
		}
	}
}
