package network

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/moreveal/mimic/internal/trace"
)

func TestNavigationRedirectChainPreservesCrossSiteContext(t *testing.T) {
	var final http.Header
	var finalMethod string
	var base string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", base+"/final")
			status, _ := strconv.Atoi(r.URL.Query().Get("status"))
			w.WriteHeader(status)
			return
		}
		final, finalMethod = r.Header.Clone(), r.Method
	}))
	defer server.Close()
	base = server.URL
	source, _ := url.Parse(base + "/document")
	for _, tc := range []struct {
		method                      string
		status                      int
		wantMethod, cookies, origin string
	}{
		{"GET", 302, "GET", "none=1; lax=1; default=1", ""},
		{"POST", 303, "GET", "none=1; lax=1; default=1", ""},
		{"POST", 307, "POST", "none=1; default=1", "null"},
	} {
		t.Run(tc.method+strconv.Itoa(tc.status), func(t *testing.T) {
			jar := NewCookieStore()
			jar.SetFromResponse(source, http.Header{"Set-Cookie": {"none=1; Secure; SameSite=None; Path=/", "lax=1; SameSite=Lax; Path=/", "strict=1; SameSite=Strict; Path=/", "default=1; Path=/"}})
			target, _ := url.Parse(strings.Replace(base, "127.0.0.1", "localhost", 1) + "/redirect?status=" + strconv.Itoa(tc.status))
			_, err := NewLoader(testEnvironment, jar, trace.New()).Load(context.Background(), Request{URL: target, SourceURL: source, Initiator: Navigation, Method: tc.method})
			if err != nil {
				t.Fatal(err)
			}
			if finalMethod != tc.wantMethod || final.Get("Cookie") != tc.cookies || final.Get("Sec-Fetch-Site") != "cross-site" || final.Get("Origin") != tc.origin {
				t.Fatalf("method=%s headers=%v", finalMethod, final)
			}
		})
	}
}

func TestRedirectChainProjections(t *testing.T) {
	parse := func(raw string) *url.URL {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	a, b, c := parse("https://a.example.test/"), parse("https://b.example.test/"), parse("https://other.test/")
	r := Request{URL: a, SourceURL: a, Initiator: Fetch, Credentials: "same-origin"}
	r.beginChain()
	r.redirectChain(b)
	r.URL = b
	if r.chainSite() != "same-site" || r.redirectTaintedOrigin() {
		t.Fatal("first same-site cross-origin hop", r.chainSite(), r.redirectTaintedOrigin())
	}
	r.redirectChain(a)
	r.URL = a
	if r.chainSite() != "same-site" || !r.redirectTaintedOrigin() || requestIncludesCredentials(r) || fetchOrigin(r) != "null" || !fetchCrossOrigin(r) {
		t.Fatal("returned origin erased chain")
	}
	r.redirectChain(c)
	r.URL = c
	r.redirectChain(a)
	r.URL = a
	if r.chainSite() != "cross-site" || r.cookieContext().Access != CookieAccessCrossSite {
		t.Fatal("returned site erased chain")
	}
	// The redirect list affects access, not the independently owned CHIPS key.
	if r.cookieContext().HasCrossSiteAncestor {
		t.Fatal("redirect mutated ancestor partition identity")
	}
	plain := Request{URL: a, SourceURL: a, Initiator: Navigation}
	plain.beginChain()
	plain.redirectChain(parse("https://a.example.test/other"))
	plain.URL = plain.chain.urls[1]
	if plain.chainSite() != "same-origin" || plain.redirectTaintedOrigin() || plain.cookieContext().Access != CookieAccessStrict {
		t.Fatal("same-origin redirect changed access")
	}
}

func TestFetchRedirectUsesTaintedOriginAndCORS(t *testing.T) {
	var base string
	var finalOrigin string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		if r.URL.Path == "/redirect" {
			w.Header().Set("Location", base+"/final")
			w.WriteHeader(302)
			return
		}
		finalOrigin = r.Header.Get("Origin")
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	base = server.URL
	source, _ := url.Parse(base + "/document")
	target, _ := url.Parse(strings.Replace(base, "127.0.0.1", "localhost", 1) + "/redirect")
	res, err := NewLoader(testEnvironment, NewCookieStore(), trace.New()).Load(context.Background(), Request{URL: target, SourceURL: source, Initiator: Fetch, Mode: "cors", Credentials: "same-origin"})
	if err != nil || finalOrigin != "null" || res.Type != "cors" || string(res.Body) != "ok" {
		t.Fatalf("response=%+v origin=%q error=%v", res, finalOrigin, err)
	}
}
