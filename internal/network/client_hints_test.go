package network

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/moreveal/mimic/internal/trace"
)

func TestClientHintsRedirectRechecksDocumentPermission(t *testing.T) {
	child := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Sec-CH-UA-Platform-Version"); got != "" {
			t.Errorf("redirect leaked hint: %q", got)
		}
		if r.Header.Get("Sec-CH-UA") == "" {
			t.Error("low entropy identity unexpectedly suppressed")
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer child.Close()
	top := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Sec-CH-UA-Platform-Version") == "" {
			t.Error("same-origin accepted hint absent")
		}
		http.Redirect(w, r, child.URL, http.StatusFound)
	}))
	defer top.Close()
	u, _ := url.Parse(top.URL)
	session := NewSessionState()
	session.AcceptClientHints(u, "Sec-CH-UA-Platform-Version")
	loader := NewLoaderWithSession(testEnvironment, NewCookieStore(), session, trace.New())
	_, err := loader.Load(context.Background(), Request{URL: u, Initiator: Fetch, ClientHints: &ClientHintsContext{TopLevelURL: u, Allowed: map[string][]string{"sec-ch-ua-platform-version": {top.URL}}}})
	if err != nil {
		t.Fatal(err)
	}
}
