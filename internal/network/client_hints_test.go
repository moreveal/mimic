package network

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/moreveal/mimic/internal/state"
	"github.com/moreveal/mimic/internal/trace"
)

func TestResponseClientHintFieldLinesApplyToNextRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/accept" {
			w.Header().Add("Accept-CH", "Sec-CH-Prefers-Color-Scheme")
			w.Header().Add("Accept-CH", "Sec-CH-UA-Arch")
			w.Header().Add("Accept-CH", "Sec-CH-UA-Bitness")
		} else {
			if got := r.Header.Get("Sec-CH-UA-Arch"); got != `"x86"` {
				t.Errorf("accepted architecture = %q, want x86", got)
			}
			if got := r.Header.Get("Sec-CH-UA-Bitness"); got != `"64"` {
				t.Errorf("accepted bitness = %q, want 64", got)
			}
		}
		_, _ = io.WriteString(w, "ok")
	}))
	defer server.Close()
	environment := testEnvironment()
	environment.Platform.Architecture = "x86_64"
	loader := NewLoaderWithSession(func() state.Environment { return environment }, NewCookieStore(), NewSessionState(), trace.New())
	for _, path := range []string{"/accept", "/probe"} {
		u, _ := url.Parse(server.URL + path)
		if _, err := loader.Load(context.Background(), Request{URL: u, Initiator: Navigation}); err != nil {
			t.Fatal(err)
		}
	}
}

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
