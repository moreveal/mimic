package cdp

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/moreveal/mimic/internal/browser"
	"github.com/moreveal/mimic/internal/network"
)

func frameURLs(frame *browser.Frame) []string {
	urls := []string{frame.URL()}
	for _, child := range frame.Children() {
		urls = append(urls, frameURLs(child)...)
	}
	return urls
}

func partitionParameter(p map[string]any) (*network.CookiePartitionKey, error) {
	raw, exists := p["partitionKey"]
	if !exists {
		return nil, nil
	}
	object, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("partitionKey must be an object")
	}
	u, err := url.Parse(stringValue(object["topLevelSite"]))
	if err != nil || u.Host == "" || network.SchemefulSite(u) == "" {
		return nil, fmt.Errorf("partitionKey.topLevelSite must be an HTTP(S) site; opaque partitions are unsupported")
	}
	ancestor, ok := object["hasCrossSiteAncestor"].(bool)
	if !ok {
		return nil, fmt.Errorf("partitionKey.hasCrossSiteAncestor must be a boolean")
	}
	return &network.CookiePartitionKey{TopLevelSite: network.SchemefulSite(u), HasCrossSiteAncestor: ancestor}, nil
}

func (s *session) setCookie(p map[string]any) error {
	return setCookie(s.page.Cookies(), p)
}

func setCookie(store *network.CookieStore, p map[string]any) error {
	key, err := partitionParameter(p)
	if err != nil {
		return err
	}
	secure, _ := p["secure"].(bool)
	httpOnly, _ := p["httpOnly"].(bool)
	rawURL := stringValue(p["url"])
	if rawURL == "" && stringValue(p["domain"]) != "" {
		scheme := "http"
		if secure {
			scheme = "https"
		}
		rawURL = scheme + "://" + strings.TrimPrefix(stringValue(p["domain"]), ".") + "/"
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" || network.SchemefulSite(u) == "" {
		return fmt.Errorf("valid HTTP(S) url or domain is required")
	}
	if _, specified := p["secure"]; !specified {
		secure = u.Scheme == "https"
	}
	c := &http.Cookie{Name: stringValue(p["name"]), Value: stringValue(p["value"]), Domain: stringValue(p["domain"]), Path: stringValue(p["path"]), Secure: secure, HttpOnly: httpOnly, Partitioned: key != nil}
	if c.Path == "" {
		c.Path = "/"
	}
	switch stringValue(p["sameSite"]) {
	case "":
	case "None":
		c.SameSite = http.SameSiteNoneMode
	case "Lax":
		c.SameSite = http.SameSiteLaxMode
	case "Strict":
		c.SameSite = http.SameSiteStrictMode
	default:
		return fmt.Errorf("invalid sameSite")
	}
	if expires, ok := p["expires"].(float64); ok && expires >= 0 {
		c.Expires = time.Unix(0, int64(expires*1e9))
	}
	if c.Partitioned && !c.Secure {
		return fmt.Errorf("partitioned cookies require Secure")
	}
	store.SetWithPartition(u, c, key)
	return nil
}

func (s *session) storageCookies(p map[string]any) (*network.CookieStore, error) {
	c := s.server.Context
	if id := stringValue(p["browserContextId"]); id != "" {
		var ok bool
		c, ok = s.server.Browser.Context(id)
		if !ok {
			return nil, fmt.Errorf("Failed to find browser context for id %s", id)
		}
	}
	return c.Cookies(), nil
}

func (s *session) handleStorageCookies(method string, p map[string]any) (any, bool, error) {
	if method != "Storage.getCookies" && method != "Storage.setCookies" && method != "Storage.clearCookies" {
		return nil, false, nil
	}
	store, err := s.storageCookies(p)
	if err != nil {
		return nil, true, err
	}
	if method == "Storage.getCookies" {
		return map[string]any{"cookies": cookieRows(store.Snapshots())}, true, nil
	}
	if method == "Storage.clearCookies" {
		store.Clear()
	} else {
		items, _ := p["cookies"].([]any)
		for _, raw := range items {
			cookie, _ := raw.(map[string]any)
			if err := setCookie(store, cookie); err != nil {
				return nil, true, err
			}
		}
	}
	return map[string]any{}, true, nil
}

func (s *session) cookiesForURLs(p map[string]any) []any {
	var urls []*url.URL
	if raw, exists := p["urls"].([]any); exists {
		for _, value := range raw {
			if u, err := url.Parse(stringValue(value)); err == nil && u.Host != "" {
				urls = append(urls, u)
			}
		}
	} else {
		for _, raw := range frameURLs(s.page.Top) {
			if u, err := url.Parse(raw); err == nil && u.Host != "" {
				urls = append(urls, u)
			}
		}
	}
	return cookieRows(s.page.Cookies().SnapshotsForURLs(urls))
}

func (s *session) deleteCookies(p map[string]any) error {
	key, err := partitionParameter(p)
	if err != nil {
		return err
	}
	domain, path := stringValue(p["domain"]), stringValue(p["path"])
	if raw := stringValue(p["url"]); domain == "" && raw != "" {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" {
			return fmt.Errorf("valid url is required")
		}
		domain = u.Hostname()
	}
	if domain == "" {
		return fmt.Errorf("url or domain is required")
	}
	s.page.Cookies().DeleteScoped(domain, path, stringValue(p["name"]), key)
	return nil
}
