package network

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// CookiePartitionKey is part of cookie identity, not a Cookie header attribute.
type CookiePartitionKey struct {
	TopLevelSite         string `json:"topLevelSite"`
	HasCrossSiteAncestor bool   `json:"hasCrossSiteAncestor"`
}

// CookieContext is a value snapshot captured on the document's owning task.
// Ancestors are compared with the top-level site, including intervening frames
// in A -> B -> A. Workers inherit this snapshot from their creator.
type CookieContext struct {
	TopLevelSite         string
	HasCrossSiteAncestor bool
	// Access is separate from CHIPS identity. A cross-site main navigation
	// selects its destination partition but does not acquire Strict access.
	Access              CookieSameSiteAccess
	MainFrameNavigation bool
}

type CookieSameSiteAccess uint8

const (
	CookieAccessFromSite CookieSameSiteAccess = iota
	CookieAccessCrossSite
	CookieAccessLaxUnsafe
	CookieAccessLax
	CookieAccessStrict
)

func cookieAccess(u *url.URL, contexts []CookieContext) CookieSameSiteAccess {
	if len(contexts) == 0 {
		return CookieAccessStrict
	}
	c := contexts[0]
	if c.Access != CookieAccessFromSite {
		return c.Access
	}
	if c.HasCrossSiteAncestor || c.TopLevelSite == "" || c.TopLevelSite != SchemefulSite(u) {
		return CookieAccessCrossSite
	}
	return CookieAccessStrict
}

func SchemefulSite(u *url.URL) string {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	host := cookieHost(u.Hostname())
	if host == "" {
		return ""
	}
	if net.ParseIP(host) == nil {
		if registrable, err := publicsuffix.EffectiveTLDPlusOne(host); err == nil {
			host = registrable
		}
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return (&url.URL{Scheme: strings.ToLower(u.Scheme), Host: host}).String()
}

func cookiePartition(u *url.URL, contexts []CookieContext) CookiePartitionKey {
	context := CookieContext{TopLevelSite: SchemefulSite(u)}
	if len(contexts) != 0 {
		context = contexts[0]
	}
	return CookiePartitionKey{TopLevelSite: context.TopLevelSite,
		HasCrossSiteAncestor: context.HasCrossSiteAncestor || context.TopLevelSite != SchemefulSite(u)}
}

func (r Request) cookieContext() CookieContext {
	if r.Initiator == Navigation {
		// Each main-frame redirect establishes a destination partition; there
		// are no ancestors. Its initiator still governs Fetch/SameSite semantics.
		context := CookieContext{TopLevelSite: SchemefulSite(r.URL), MainFrameNavigation: true, Access: CookieAccessStrict}
		source := r.initiatingURL()
		if r.OpaqueOrigin || r.chainSite() == "cross-site" || source != nil && SchemefulSite(source) != context.TopLevelSite {
			context.Access = CookieAccessLaxUnsafe
			switch r.Method {
			case "", http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
				context.Access = CookieAccessLax
			}
		}
		return context
	}
	top := r.TopLevelURL
	if top == nil {
		top = r.initiatingURL()
	}
	if top == nil {
		top = r.URL
	}
	context := CookieContext{TopLevelSite: SchemefulSite(top), HasCrossSiteAncestor: r.HasCrossSiteAncestor || r.OpaqueOrigin}
	if r.chainSite() == "cross-site" {
		context.Access = CookieAccessCrossSite
	}
	return context
}
