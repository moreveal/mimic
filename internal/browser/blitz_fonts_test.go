package browser

import (
	"context"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBlitzFontCollectionUsesCanonicalResources(t *testing.T) {
	serialBrowserTest(t)
	t.Setenv("MIMIC_STYLE_ENGINE", "blitz")
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
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<!doctype html><html><body></body></html>"))
	}))
	defer server.Close()
	if err := p.Navigate(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(context.Background(), `(async()=>{
const face=new FontFace('NativeOwnedFace','local("Courier New")');await face.load();
const e=document.createElement('span');e.style.cssText='display:inline-block;font:20px NativeOwnedFace,Arial';e.textContent='iiiiWWAV';document.body.append(e);
const measure=()=>e.getBoundingClientRect().width;
const before=measure();document.fonts.add(face);const loaded=measure();
if(before===loaded)return 'unchanged';document.fonts.delete(face);if(measure()!==before)return 'delete';
document.fonts.add(face);if(measure()!==loaded)return 'readd';face.family='Different';if(measure()!==before)return 'rename';
return 'ok';})()`)
	if err != nil || value != "ok" {
		t.Fatalf("font collection: %v %v", value, err)
	}
	if p.Top.Realm.blitz == nil || p.Top.Realm.blitz.document.Owner == nil || p.Top.Realm.blitz.fallback != "" {
		t.Fatal("font test silently used fallback")
	}
}
