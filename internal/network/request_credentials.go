package network

import (
	"golang.org/x/net/publicsuffix"
	"net"
	"net/url"
	"strings"
)

func sameRequestOrigin(a, b *url.URL) bool {
	if a == nil || b == nil || a.Scheme != b.Scheme || !strings.EqualFold(a.Hostname(), b.Hostname()) {
		return false
	}
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		if u.Scheme == "https" {
			return "443"
		}
		if u.Scheme == "http" {
			return "80"
		}
		return ""
	}
	return port(a) == port(b)
}
func requestIncludesCredentials(r Request) bool {
	switch r.Credentials {
	case "omit":
		return false
	case "same-origin":
		return !r.OpaqueOrigin && sameRequestOrigin(r.initiatingURL(), r.URL) && r.chainSite() == "same-origin"
	default:
		return true
	}
}
func requestSameSite(a, b *url.URL) bool {
	if a == nil || b == nil || a.Scheme != b.Scheme {
		return false
	}
	site := func(u *url.URL) string {
		host := strings.ToLower(u.Hostname())
		if net.ParseIP(host) != nil {
			return host
		}
		if domain, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
			return domain
		}
		return host
	}
	return site(a) == site(b)
}
func trustworthyRequest(u *url.URL) bool {
	if u == nil {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	if u.Scheme != "http" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	ip := net.ParseIP(host)
	return host == "localhost" || strings.HasSuffix(host, ".localhost") || ip != nil && ip.IsLoopback()
}
func applyStorageAccessHeader(r *Request, cookiesEnabled bool) {
	// Top-level navigation establishes a first-party context at the destination.
	if r.Initiator == Navigation || !requestIncludesCredentials(*r) || !trustworthyRequest(r.URL) {
		return
	}
	top := r.TopLevelURL
	if top == nil {
		top = r.initiatingURL()
	}
	if top == nil || !r.OpaqueOrigin && requestSameSite(top, r.URL) {
		return
	}
	value := "none"
	if cookiesEnabled {
		value = "active"
	}
	r.Headers.Set("Sec-Fetch-Storage-Access", value)
}
