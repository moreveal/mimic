package network

import (
	"net/http"
	"net/url"
	"testing"
)

func TestRequestCredentialsAndStorageAccessContext(t *testing.T) {
	parse := func(raw string) *url.URL {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	for _, tc := range []struct {
		name, source, target, top, credentials, want string
		opaque, include                              bool
	}{
		{"first party", "https://a.example/", "https://a.example/", "https://a.example/", "same-origin", "", false, true},
		{"cross omitted", "https://a.example/", "https://b.test/", "https://a.example/", "omit", "", false, false},
		{"cross default", "https://a.example/", "https://b.test/", "https://a.example/", "same-origin", "", false, false},
		{"cross include", "https://a.example/", "https://b.test/", "https://a.example/", "include", "active", false, true},
		{"third party frame", "https://b.test/", "https://b.test/", "https://a.example/", "same-origin", "active", false, true},
		{"same site", "https://a.example.com/", "https://b.example.com/", "https://a.example.com/", "include", "", false, true},
		{"insecure target", "http://a.example/", "http://b.test/", "http://a.example/", "include", "", false, true},
		{"opaque include", "https://a.example/", "https://a.example/", "https://a.example/", "include", "active", true, true},
		{"different loopback IPs", "http://127.0.0.1/", "http://127.1.0.1/", "http://127.0.0.1/", "include", "active", false, true},
		{"opaque default", "https://a.example/", "https://a.example/", "https://a.example/", "same-origin", "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Request{URL: parse(tc.target), SourceURL: parse(tc.source), TopLevelURL: parse(tc.top), Credentials: tc.credentials, OpaqueOrigin: tc.opaque, Initiator: Fetch, Headers: make(http.Header)}
			if got := requestIncludesCredentials(r); got != tc.include {
				t.Fatalf("credentials %v", got)
			}
			applyBrowserRequestHeaders(&r)
			applyStorageAccessHeader(&r, true)
			if got := r.Headers.Get("Sec-Fetch-Storage-Access"); got != tc.want {
				t.Fatalf("storage %q", got)
			}
			if tc.opaque && (r.Headers.Get("Origin") != "null" || r.Headers.Get("Sec-Fetch-Site") != "cross-site") {
				t.Fatal("opaque origin metadata", r.Headers)
			}
		})
	}
}

func TestInheritedClientOriginKeepsDocumentReferrerSeparate(t *testing.T) {
	blank, _ := url.Parse("about:blank")
	origin, _ := url.Parse("https://example.test")
	target, _ := url.Parse("https://example.test/echo")
	request := Request{SourceURL: blank, SourceOrigin: origin, Referrer: blank, URL: target, Initiator: Fetch, Method: http.MethodPost, Credentials: "same-origin", Headers: make(http.Header)}
	request.beginChain()
	applyBrowserRequestHeaders(&request)
	if fetchCrossOrigin(request) || !requestIncludesCredentials(request) || request.Headers.Get("Sec-Fetch-Site") != "same-origin" || request.Headers.Get("Origin") != origin.String() || request.ReferrerValue() != "" {
		t.Fatalf("inherited client origin projection: headers=%v origin=%s referrer=%s", request.Headers, fetchOrigin(request), request.ReferrerValue())
	}
	request.OpaqueOrigin = true
	if !fetchCrossOrigin(request) || requestIncludesCredentials(request) || fetchOrigin(request) != "null" {
		t.Fatal("opaque origin acquired inherited credentials or CORS access")
	}
}
