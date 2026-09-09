package browser

import (
	"net"
	"net/url"
	"strings"
)

// Loopback HTTP origins are potentially trustworthy without TLS. Use the same
// test for documents and their workers so they select the same secure exposure.
func potentiallyTrustworthyURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	if u.Scheme != "http" {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
