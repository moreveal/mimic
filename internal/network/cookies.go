package network

import (
	"net/http"
	"net/url"
	"sync"
	"time"
)

type CookieStore struct {
	mu  sync.RWMutex
	jar map[string]map[string]*http.Cookie
}

func NewCookieStore() *CookieStore { return &CookieStore{jar: map[string]map[string]*http.Cookie{}} }
func (s *CookieStore) Set(u *url.URL, c *http.Cookie) {
	s.mu.Lock()
	defer s.mu.Unlock()
	domain := c.Domain
	if domain == "" {
		domain = u.Hostname()
	}
	if s.jar[domain] == nil {
		s.jar[domain] = map[string]*http.Cookie{}
	}
	copy := *c
	copy.Domain = domain
	if copy.Path == "" {
		copy.Path = "/"
	}
	s.jar[domain][c.Name] = &copy
}
func (s *CookieStore) SetFromResponse(u *url.URL, h http.Header) {
	for _, c := range (&http.Response{Header: h}).Cookies() {
		s.Set(u, c)
	}
}
func (s *CookieStore) ForURL(u *url.URL) []*http.Cookie {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*http.Cookie
	now := time.Now()
	for domain, cs := range s.jar {
		if u.Hostname() != domain {
			continue
		}
		for _, c := range cs {
			if !c.Expires.IsZero() && c.Expires.Before(now) {
				continue
			}
			copy := *c
			out = append(out, &copy)
		}
	}
	return out
}
func (s *CookieStore) All() []*http.Cookie {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*http.Cookie
	for _, cs := range s.jar {
		for _, c := range cs {
			copy := *c
			out = append(out, &copy)
		}
	}
	return out
}
func (s *CookieStore) Delete(domain, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cs := s.jar[domain]; cs != nil {
		delete(cs, name)
		if len(cs) == 0 {
			delete(s.jar, domain)
		}
	}
}
func (s *CookieStore) Clear() {
	s.mu.Lock()
	s.jar = map[string]map[string]*http.Cookie{}
	s.mu.Unlock()
}
