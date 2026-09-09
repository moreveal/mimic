package network

import (
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

type cookieKey struct{ domain, path, name string }
type storedCookie struct {
	cookie   http.Cookie
	hostOnly bool
	order    uint64
}

type CookieStore struct {
	mu        sync.RWMutex
	jar       map[cookieKey]storedCookie
	nextOrder uint64
}

func NewCookieStore() *CookieStore { return &CookieStore{jar: make(map[cookieKey]storedCookie)} }

func cookieHost(host string) string {
	host = strings.ToLower(host)
	if net.ParseIP(host) != nil {
		return host
	}
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return ""
	}
	return ascii
}
func cookieDomainMatches(host, domain string) bool {
	return host == domain || (net.ParseIP(host) == nil && strings.HasSuffix(host, "."+domain))
}
func cookiePathMatches(path, scope string) bool {
	return path == scope || (strings.HasPrefix(path, scope) && (strings.HasSuffix(scope, "/") || len(path) > len(scope) && path[len(scope)] == '/'))
}
func defaultCookiePath(u *url.URL) string {
	path := u.EscapedPath()
	if !strings.HasPrefix(path, "/") {
		return "/"
	}
	if i := strings.LastIndex(path, "/"); i > 0 {
		return path[:i]
	}
	return "/"
}
func secureCookieURL(u *url.URL) bool {
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	ip := net.ParseIP(host)
	return u.Scheme == "https" || host == "localhost" || strings.HasSuffix(host, ".localhost") || ip != nil && ip.IsLoopback()
}
func (s *CookieStore) Set(u *url.URL, c *http.Cookie) { s.set(u, c, false) }
func (s *CookieStore) set(u *url.URL, c *http.Cookie, script bool) {
	if u == nil || c == nil || (u.Scheme != "http" && u.Scheme != "https") {
		return
	}
	host := cookieHost(u.Hostname())
	if host == "" || c.Secure && !secureCookieURL(u) || script && c.HttpOnly {
		return
	}
	domain, hostOnly := host, c.Domain == ""
	if !hostOnly {
		domain = cookieHost(strings.TrimPrefix(c.Domain, "."))
		if domain == "" || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || !cookieDomainMatches(host, domain) {
			return
		}
		suffix, _ := publicsuffix.PublicSuffix(domain)
		if suffix == domain {
			if host != domain {
				return
			}
			hostOnly = true
		}
		if net.ParseIP(host) != nil {
			hostOnly = true
		}
	}
	copy := *c
	copy.Domain = domain
	if !strings.HasPrefix(copy.Path, "/") {
		copy.Path = defaultCookiePath(u)
	}
	key := cookieKey{domain, copy.Path, copy.Name}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	old, exists := s.jar[key]
	if exists && script && old.cookie.HttpOnly && (old.cookie.Expires.IsZero() || old.cookie.Expires.After(now)) {
		return
	}
	if copy.MaxAge < 0 || copy.MaxAge == 0 && !copy.Expires.IsZero() && !copy.Expires.After(now) {
		delete(s.jar, key)
		return
	}
	if copy.MaxAge > 0 {
		copy.Expires = now.Add(time.Duration(min(copy.MaxAge, 400*24*60*60)) * time.Second)
	}
	// Chrome 152 moves a replaced cookie behind cookies created earlier at
	// the same path length (including document.cookie replacements).
	order := s.nextOrder
	s.nextOrder++
	s.jar[key] = storedCookie{copy, hostOnly, order}
}
func (s *CookieStore) SetFromResponse(u *url.URL, h http.Header) {
	for _, c := range (&http.Response{Header: h}).Cookies() {
		s.Set(u, c)
	}
}

// Document writes share the network jar but cannot create or replace HttpOnly cookies.
func (s *CookieStore) SetFromDocument(u *url.URL, value string) {
	h := http.Header{"Set-Cookie": {value}}
	for _, c := range (&http.Response{Header: h}).Cookies() {
		s.set(u, c, true)
	}
}
func (s *CookieStore) matching(u *url.URL) []*http.Cookie {
	s.mu.RLock()
	defer s.mu.RUnlock()
	now := time.Now()
	entries := []storedCookie{}
	var host, path string
	var secure bool
	if u != nil {
		host, path, secure = cookieHost(u.Hostname()), u.EscapedPath(), secureCookieURL(u)
		if path == "" {
			path = "/"
		}
	}
	for _, entry := range s.jar {
		c := entry.cookie
		if !c.Expires.IsZero() && !c.Expires.After(now) {
			continue
		}
		if u != nil {
			if (entry.hostOnly && host != c.Domain) || !cookieDomainMatches(host, c.Domain) || !cookiePathMatches(path, c.Path) || c.Secure && !secure {
				continue
			}
		}
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		if u != nil && len(entries[i].cookie.Path) != len(entries[j].cookie.Path) {
			return len(entries[i].cookie.Path) > len(entries[j].cookie.Path)
		}
		return entries[i].order < entries[j].order
	})
	out := make([]*http.Cookie, 0, len(entries))
	for _, entry := range entries {
		copy := entry.cookie
		out = append(out, &copy)
	}
	return out
}
func (s *CookieStore) ForURL(u *url.URL) []*http.Cookie {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil
	}
	return s.matching(u)
}
func (s *CookieStore) All() []*http.Cookie { return s.matching(nil) }
func (s *CookieStore) Delete(domain, name string) {
	domain = cookieHost(strings.TrimPrefix(domain, "."))
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.jar {
		if key.domain == domain && key.name == name {
			delete(s.jar, key)
		}
	}
}
func (s *CookieStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jar = make(map[cookieKey]storedCookie)
	s.nextOrder = 0
}
