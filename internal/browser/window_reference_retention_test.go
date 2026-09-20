package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Chrome keeps a saved nested WindowProxy usable after its ancestor navigates.
// Separate evaluations force owner collection between navigation and observation.
func TestWindowReferenceRetainsDescendantContextAcrossNavigation(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/ancestor" {
			fmt.Fprint(w, `<!doctype html><body><script>const nested=document.createElement("iframe");document.body.appendChild(nested);globalThis.nestedWindow=nested.contentWindow;</script>`)
			return
		}
		fmt.Fprint(w, "<!doctype html><body>")
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		_, err := p.Evaluate(ctx, `(async()=>{
   globalThis.ancestor=document.createElement('iframe');
   document.body.appendChild(ancestor);
   await new Promise(resolve=>{ancestor.onload=resolve;ancestor.src='/ancestor'});
   globalThis.savedNested=ancestor.contentWindow.nestedWindow;
   if(!savedNested)throw Error('nested WindowProxy was not created: '+ancestor.contentDocument.documentElement.outerHTML);
   await new Promise(resolve=>{ancestor.onload=resolve;ancestor.src='/replacement'});
   return true;
  })()`)
		if err != nil {
			t.Fatal(err)
		}
		// Holding the ancestor's WindowProxy must not pin its replaced document.
		if n := len(p.realmOwners); n != 3 {
			t.Fatalf("retained %d realms, want top, current ancestor and detached nested context", n)
		}
		value, err := p.Evaluate(ctx, `savedNested.window===savedNested && savedNested.document.defaultView===null && savedNested.document.body!==null && savedNested.eval('17')===17`)
		if err != nil || value != true {
			t.Fatalf("saved nested WindowProxy: %v, %v", value, err)
		}
	})
}

// Chrome's cross-origin WindowProxy omits the Window toStringTag while retaining
// the same proxy identity; the target descriptor must allow this dynamic result.
func TestWindowProxyTagFollowsOriginAccess(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx := context.Background()
		if _, err := p.Evaluate(ctx, `const tagFrame=document.createElement('iframe');document.body.appendChild(tagFrame);globalThis.tagWindow=tagFrame.contentWindow`); err != nil {
			t.Fatal(err)
		}
		var child *Realm
		for _, f := range p.Top.Realm.childFrames {
			child = f.Realm
			break
		}
		if child == nil {
			t.Fatal("missing child realm")
		}
		origin := child.origin
		for _, tc := range []struct{ origin, tag string }{{origin, "[object Window]"}, {"https://other.example", "[object Object]"}, {origin, "[object Window]"}} {
			child.origin = tc.origin
			result, err := p.Evaluate(ctx, `Object.prototype.toString.call(tagWindow)`)
			if err != nil || result != tc.tag {
				t.Fatalf("tag: %v, %v; want %s", result, err, tc.tag)
			}
		}
	})
}
