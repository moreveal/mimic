package network

import (
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"
)

func cookieURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
func cookiePairs(cs []*http.Cookie) []string {
	out := []string{}
	for _, c := range cs {
		out = append(out, c.Name+"="+c.Value)
	}
	return out
}
func TestCookieDomainScope(t *testing.T) {
	s := NewCookieStore()
	origin := cookieURL("https://www.example.com/a/b")
	s.Set(origin, &http.Cookie{Name: "domain", Value: "yes", Domain: ".EXAMPLE.com"})
	s.Set(origin, &http.Cookie{Name: "host", Value: "yes"})
	s.Set(origin, &http.Cookie{Name: "foreign", Domain: "other.com"})
	s.Set(origin, &http.Cookie{Name: "suffix", Domain: "com"})
	s.Set(origin, &http.Cookie{Name: "suffix-private", Domain: "github.io"})
	for _, tc := range []struct {
		url  string
		want []string
	}{
		{"https://www.example.com/a/b", []string{"domain=yes", "host=yes"}},
		{"https://example.com/a/b", []string{"domain=yes"}},
		{"https://child.www.example.com/a/b", []string{"domain=yes"}},
		{"https://notexample.com/a/b", []string{}},
	} {
		if got := cookiePairs(s.ForURL(cookieURL(tc.url))); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v want %v", tc.url, got, tc.want)
		}
	}
	s.Delete(".example.com", "domain")
	if len(s.All()) != 1 {
		t.Fatalf("delete normalized domain: %v", s.All())
	}
}
func TestCookiePathIdentityAndOrder(t *testing.T) {
	s := NewCookieStore()
	u := cookieURL("https://example.com/a/b")
	s.Set(u, &http.Cookie{Name: "same", Value: "root", Path: "/"})
	s.Set(u, &http.Cookie{Name: "same", Value: "default"})
	s.Set(u, &http.Cookie{Name: "later", Value: "1", Path: "/a"})
	s.Set(u, &http.Cookie{Name: "same", Value: "updated", Path: "/a"})
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"/a/b", []string{"later=1", "same=updated", "same=root"}},
		{"/a", []string{"later=1", "same=updated", "same=root"}},
		{"/ab", []string{"same=root"}},
		{"/", []string{"same=root"}},
	} {
		if got := cookiePairs(s.ForURL(cookieURL("https://example.com" + tc.path))); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v want %v", tc.path, got, tc.want)
		}
	}
	s.Set(u, &http.Cookie{Name: "same", Path: "/a", MaxAge: -1})
	if got := cookiePairs(s.ForURL(u)); !reflect.DeepEqual(got, []string{"later=1", "same=root"}) {
		t.Fatal(got)
	}
}
func TestCookieExpirySecurityAndDocumentWrites(t *testing.T) {
	s := NewCookieStore()
	u := cookieURL("https://example.com/")
	s.Set(u, &http.Cookie{Name: "expired", Expires: time.Now().Add(-time.Hour)})
	s.Set(u, &http.Cookie{Name: "live", Value: "1", MaxAge: 60, Expires: time.Now().Add(-time.Hour)})
	s.Set(u, &http.Cookie{Name: "secure", Value: "1", Secure: true})
	s.Set(u, &http.Cookie{Name: "secret", Value: "1", HttpOnly: true})
	s.SetFromDocument(u, "secret=changed")
	s.SetFromDocument(u, "secret=; Max-Age=0")
	s.SetFromDocument(u, "forged=1; HttpOnly")
	s.Set(cookieURL("http://example.com/"), &http.Cookie{Name: "insecure-write", Secure: true})
	if got := cookiePairs(s.ForURL(u)); !reflect.DeepEqual(got, []string{"live=1", "secure=1", "secret=1"}) {
		t.Fatal(got)
	}
	if got := cookiePairs(s.ForURL(cookieURL("http://example.com/"))); !reflect.DeepEqual(got, []string{"live=1", "secret=1"}) {
		t.Fatal(got)
	}
	all := s.All()
	all[0].Value = "mutated"
	if s.All()[0].Value != "1" {
		t.Fatal("All leaked mutable state")
	}
	s.SetFromDocument(u, "live=; Max-Age=0")
	if len(s.All()) != 2 {
		t.Fatal("Max-Age=0 did not delete")
	}
	s.Clear()
	if len(s.All()) != 0 {
		t.Fatal("Clear retained cookies")
	}
}
func TestCookieIPAndPublicSuffixRejection(t *testing.T) {
	for _, tc := range []struct {
		origin, domain string
		count          int
	}{
		{"https://a.github.io/", "github.io", 0},
		{"https://example.com/", ".com", 0},
		{"http://127.0.0.1/", "127.0.0.1", 1},
		{"http://127.0.0.1/", "0.0.1", 0},
		{"https://example.com/", "example.com.", 0},
		{"https://example.com/", "..example.com", 0},
	} {
		s := NewCookieStore()
		s.Set(cookieURL(tc.origin), &http.Cookie{Name: "a", Domain: tc.domain})
		if len(s.All()) != tc.count {
			t.Errorf("%s Domain=%s got %v", tc.origin, tc.domain, s.All())
		}
	}
}
