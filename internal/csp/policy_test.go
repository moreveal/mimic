package csp

import (
	"net/url"
	"testing"
)

func TestFormActionHasNoDefaultSourceFallback(t *testing.T) {
	doc, _ := url.Parse("https://example.test/index")
	other, _ := url.Parse("https://other.test/submit")
	if !Parse("default-src 'none'").AllowsFormAction(doc, other) {
		t.Fatal("default-src incorrectly blocked form navigation")
	}
	if Parse("form-action 'self'").AllowsFormAction(doc, other) {
		t.Fatal("cross-origin form action accepted")
	}
	if !Parse("form-action 'self'").AllowsFormAction(doc, doc) {
		t.Fatal("same-origin form action rejected")
	}
	if append(Parse("form-action *"), Parse("form-action 'none'")...).AllowsFormAction(doc, doc) {
		t.Fatal("multiple policies did not intersect")
	}
}

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
