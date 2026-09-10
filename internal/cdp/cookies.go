package cdp

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/moreveal/mimic/internal/network"
)

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
	s.page.Cookies().SetWithPartition(u, c, key)
	return nil
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
