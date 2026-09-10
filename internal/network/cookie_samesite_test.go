package network

import (
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestSameSiteCookieRequestSelection(t *testing.T) {
	u := cookieURL("https://a.example/")
	s := NewCookieStore()
	for _, header := range []string{"none=1; Secure; SameSite=None", "lax=1; Secure; SameSite=Lax", "strict=1; Secure; SameSite=Strict", "default=1; Secure"} {
		s.SetFromDocument(u, header)
	}
	for _, test := range []struct {
		access CookieSameSiteAccess
		want   []string
	}{
		{CookieAccessStrict, []string{"none=1", "lax=1", "strict=1", "default=1"}},
		{CookieAccessLax, []string{"none=1", "lax=1", "default=1"}},
		{CookieAccessLaxUnsafe, []string{"none=1", "default=1"}},
		{CookieAccessCrossSite, []string{"none=1"}},
	} {
		if got := cookiePairs(s.ForURL(u, CookieContext{TopLevelSite: SchemefulSite(u), Access: test.access})); !reflect.DeepEqual(got, test.want) {
			t.Fatalf("%d: %v != %v", test.access, got, test.want)
		}
	}
	// Age the store's clock-independent fixture, without sleeping or changing
	// production clocks. Explicit Lax never gets the unsafe-method allowance.
	for key, entry := range s.jar {
		entry.created = time.Now().Add(-3 * time.Minute)
		s.jar[key] = entry
	}
	s.SetFromDocument(u, "default=updated; Secure")
	if got := cookiePairs(s.ForURL(u, CookieContext{Access: CookieAccessLaxUnsafe})); !reflect.DeepEqual(got, []string{"none=1"}) {
		t.Fatal("replacement refreshed original creation age", got)
	}
}

func TestSameSiteCrossSiteWritesAndHttpOnly(t *testing.T) {
	u := cookieURL("https://a.example/")
	s := NewCookieStore()
	cross := CookieContext{TopLevelSite: "https://b.example"}
	for _, value := range []string{"default=1; Secure", "lax=1; Secure; SameSite=Lax", "strict=1; Secure; SameSite=Strict"} {
		s.SetFromDocument(u, value, cross)
		s.SetFromResponse(u, http.Header{"Set-Cookie": {value}}, cross)
	}
	if len(s.All()) != 0 {
		t.Fatal(s.All())
	}
	s.SetFromResponse(u, http.Header{"Set-Cookie": {"none=secret; Secure; SameSite=None; HttpOnly"}}, cross)
	s.SetFromDocument(u, "none=changed; Secure; SameSite=None", cross)
	if got := s.All(); len(got) != 1 || got[0].Value != "secret" {
		t.Fatal(got)
	}
	cross.MainFrameNavigation = true
	s.SetFromResponse(u, http.Header{"Set-Cookie": {"strict=1; Secure; SameSite=Strict"}}, cross)
	if len(s.All()) != 2 {
		t.Fatal("main-frame response rejected SameSite cookie", s.All())
	}
}

func TestSameSiteNavigationContextIsNotPartitionIdentity(t *testing.T) {
	a, b := cookieURL("https://a.example/"), cookieURL("https://b.example/")
	r := Request{URL: a, SourceURL: b, Initiator: Navigation, Method: http.MethodPost}
	if got := r.cookieContext(); got.Access != CookieAccessLaxUnsafe || got.TopLevelSite != "https://a.example" || got.HasCrossSiteAncestor {
		t.Fatal(got)
	}
	r.Method = http.MethodGet
	if r.cookieContext().Access != CookieAccessLax {
		t.Fatal(r.cookieContext())
	}
	r.SourceURL = a
	r.redirectCount = 2 // Chrome A -> B -> A navigation retains Strict access.
	if r.cookieContext().Access != CookieAccessStrict {
		t.Fatal(r.cookieContext())
	}
	r.Initiator = Fetch
	r.TopLevelURL = a
	r.HasCrossSiteAncestor = true
	if cookieAccess(a, []CookieContext{r.cookieContext()}) != CookieAccessCrossSite {
		t.Fatal("frame ancestry was confused with a redirect chain")
	}
}
