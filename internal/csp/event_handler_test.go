package csp

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestEventHandlerDirectivesAndHashAuthorization(t *testing.T) {
	source := "window.count++"
	digest := sha256.Sum256([]byte(source))
	hash := "'sha256-" + base64.StdEncoding.EncodeToString(digest[:]) + "'"
	for _, test := range []struct {
		policy string
		allow  bool
	}{
		{"", true},
		{"script-src-elem 'none'", true},
		{"default-src 'none'", false},
		{"script-src 'unsafe-inline'", true},
		{"script-src 'unsafe-inline'; script-src-attr 'none'", false},
		{"script-src 'none'; script-src-attr 'unsafe-inline'", true},
		{"script-src 'unsafe-inline' 'nonce-n'", false},
		{"script-src " + hash, false},
		{"script-src 'unsafe-hashes' " + hash, true},
		{"script-src 'unsafe-hashes' 'sha256-no-match'", false},
	} {
		if actual := Parse(test.policy).AllowsEventHandler(source); actual != test.allow {
			t.Errorf("%s: allowed=%v, expected %v", test.policy, actual, test.allow)
		}
	}
	if Parse("script-src 'unsafe-hashes' " + hash).AllowsEventHandler(source + ";") {
		t.Fatal("handler hashes must cover the exact source")
	}
	if Parse("script-src 'unsafe-inline'", "script-src-attr 'none'").AllowsEventHandler(source) {
		t.Fatal("all enforced policies must allow the handler")
	}
}
