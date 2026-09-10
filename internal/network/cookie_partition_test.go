package network

import (
	"net/http"
	"reflect"
	"testing"
)

func TestSchemefulCookieSite(t *testing.T) {
	for raw, want := range map[string]string{
		"https://a.example.co.uk:8443/x": "https://example.co.uk",
		"http://a.example.co.uk/":        "http://example.co.uk",
		"https://a.github.io/":           "https://a.github.io",
		"http://localhost:123/":          "http://localhost",
		"http://127.0.0.1:123/":          "http://127.0.0.1",
		"http://[::1]:123/":              "http://[::1]",
		"about:blank":                    "",
	} {
		if got := SchemefulSite(cookieURL(raw)); got != want {
			t.Errorf("%s: %q != %q", raw, got, want)
		}
	}
}

func TestCookiePartitionIdentityAndDeletion(t *testing.T) {
	s := NewCookieStore()
	u := cookieURL("https://a.example/path")
	first := CookieContext{TopLevelSite: "https://a.example"}
	nested := CookieContext{TopLevelSite: "https://a.example", HasCrossSiteAncestor: true}
	other := CookieContext{TopLevelSite: "https://b.example"}
	s.SetFromDocument(u, "id=shared; Secure; SameSite=None; Path=/")
	for i, ctx := range []CookieContext{first, nested, other} {
		value := []string{"first", "nested", "other"}[i]
		s.SetFromDocument(u, "id="+value+"; Secure; SameSite=None; Partitioned; Path=/", ctx)
	}
	for i, ctx := range []CookieContext{first, nested, other} {
		want := []string{"id=shared", "id=" + []string{"first", "nested", "other"}[i]}
		if got := cookiePairs(s.ForURL(u, ctx)); !reflect.DeepEqual(got, want) {
			t.Fatalf("%+v: %v != %v", ctx, got, want)
		}
	}
	// A document in one partition cannot remove the other partitions or the
	// unpartitioned cookie with the same name/domain/path.
	s.SetFromDocument(u, "id=; Max-Age=0; Secure; Partitioned; Path=/", nested)
	if got := cookiePairs(s.ForURL(u, nested)); !reflect.DeepEqual(got, []string{"id=shared"}) {
		t.Fatal(got)
	}
	if len(s.All()) != 3 {
		t.Fatal(s.All())
	}
	if got := cookiePairs(s.ForURL(u, first)); !reflect.DeepEqual(got, []string{"id=shared", "id=first"}) {
		t.Fatal(got)
	}
}

func TestCookiePartitionSecurityAndSnapshots(t *testing.T) {
	s := NewCookieStore()
	u := cookieURL("https://a.example/")
	ctx := CookieContext{TopLevelSite: "https://b.example", HasCrossSiteAncestor: true}
	s.SetFromDocument(u, "bad=1; Partitioned", ctx)
	s.SetFromDocument(u, "opaque=1; Secure; Partitioned", CookieContext{})
	if len(s.All()) != 0 {
		t.Fatal("invalid partition accepted")
	}
	s.SetFromResponse(u, http.Header{"Set-Cookie": {"id=secret; Secure; HttpOnly; SameSite=None; Partitioned; Path=/"}}, ctx)
	s.SetFromDocument(u, "id=changed; Secure; Partitioned; Path=/", ctx)
	s.SetFromDocument(u, "id=; Max-Age=0; Secure; Partitioned; Path=/", ctx)
	rows := s.Snapshots()
	if len(rows) != 1 || rows[0].Cookie.Value != "secret" || !rows[0].HostOnly || rows[0].PartitionKey == nil || !rows[0].PartitionKey.HasCrossSiteAncestor {
		t.Fatalf("%+v", rows)
	}
	rows[0].Cookie.Value = "mutated"
	rows[0].PartitionKey.TopLevelSite = "https://evil.example"
	if fresh := s.Snapshots(); fresh[0].Cookie.Value != "secret" || fresh[0].PartitionKey.TopLevelSite != ctx.TopLevelSite {
		t.Fatal("snapshot aliases canonical state")
	}
	if len(s.ForURL(u)) != 0 {
		t.Fatal("third-party partition leaked into first party")
	}
}

func TestSameSiteNoneRequiresSecureAttribute(t *testing.T) {
	for _, raw := range []string{"https://a.example/", "http://localhost/", "http://127.0.0.1/"} {
		s := NewCookieStore()
		u := cookieURL(raw)
		s.SetFromDocument(u, "script=1; SameSite=None")
		s.SetFromResponse(u, http.Header{"Set-Cookie": {"response=1; SameSite=None"}})
		if len(s.All()) != 0 {
			t.Fatalf("%s accepted missing Secure", raw)
		}
		s.SetFromDocument(u, "valid=1; SameSite=None; Secure")
		if len(s.All()) != 1 {
			t.Fatalf("%s rejected valid Secure", raw)
		}
	}
}

func TestRequestCookiePartitionNavigationAndAncestry(t *testing.T) {
	a, b := cookieURL("https://a.example/"), cookieURL("https://b.example/")
	r := Request{URL: b, SourceURL: a, TopLevelURL: a, Initiator: Navigation, HasCrossSiteAncestor: true}
	if got := cookiePartition(r.URL, []CookieContext{r.cookieContext()}); got != (CookiePartitionKey{TopLevelSite: "https://b.example"}) {
		t.Fatal(got)
	}
	r.Initiator = Iframe
	if got := cookiePartition(r.URL, []CookieContext{r.cookieContext()}); got != (CookiePartitionKey{TopLevelSite: "https://a.example", HasCrossSiteAncestor: true}) {
		t.Fatal(got)
	}
	r.URL = a // A -> B -> A must retain the intervening ancestor bit.
	if !cookiePartition(r.URL, []CookieContext{r.cookieContext()}).HasCrossSiteAncestor {
		t.Fatal("ancestor bit lost")
	}
}
