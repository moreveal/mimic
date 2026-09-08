package csp

import (
	"net/url"
	"testing"
)

func TestScriptPolicy(t *testing.T) {
	doc, _ := url.Parse("https://example.test/index")
	self, _ := url.Parse("https://example.test/app.js")
	other, _ := url.Parse("https://cdn.test/app.js")
	p := Parse("default-src 'none'; script-src 'self' 'nonce-good'")
	if ok, _ := p.AllowsScript(doc, nil, true, false, "good"); !ok {
		t.Fatal("matching inline nonce rejected")
	}
	if ok, _ := p.AllowsScript(doc, nil, true, false, ""); ok {
		t.Fatal("inline script without nonce allowed")
	}
	if ok, _ := p.AllowsScript(doc, self, false, false, ""); !ok {
		t.Fatal("self resource rejected")
	}
	if ok, _ := p.AllowsScript(doc, other, false, false, ""); ok {
		t.Fatal("cross-origin resource allowed")
	}
}
