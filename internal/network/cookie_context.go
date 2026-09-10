package network

import (
	"net"
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
		return CookieContext{TopLevelSite: SchemefulSite(r.URL)}
	}
	top := r.TopLevelURL
	if top == nil {
		top = r.SourceURL
	}
	if top == nil {
		top = r.URL
	}
	return CookieContext{TopLevelSite: SchemefulSite(top), HasCrossSiteAncestor: r.HasCrossSiteAncestor || r.OpaqueOrigin}
}
