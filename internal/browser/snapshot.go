package browser

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/moreveal/mimic/internal/network"
	"golang.org/x/net/html"
)

// Snapshot is a static, portable document. Files are relative to index.html;
// byte slices are base64 encoded by the CDP JSON encoder.
type Snapshot struct {
	URL      string            `json:"url"`
	Files    map[string][]byte `json:"files"`
	Warnings []string          `json:"warnings"`
}

type snapshotBuilder struct {
	page  *Page
	ctx   context.Context
	out   Snapshot
	seen  map[string]string
	count int
}

// CaptureSnapshot exports the current DOM, without executing the exported scripts.
// Callers must serialize this operation with navigation/evaluation, as CDP does.
func (p *Page) CaptureSnapshot(ctx context.Context) (*Snapshot, error) {
	d, ok := p.Document()
	if !ok {
		return nil, fmt.Errorf("page has no document")
	}
	shadows, err := p.Top.Realm.ShadowSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	forms, err := p.Top.Realm.FormSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	root, err := d.SnapshotTreeWithFormState(d.Root().ID, shadows, forms)
	if err != nil {
		return nil, err
	}
	base, err := url.Parse(p.URL())
	if err != nil {
		return nil, err
	}
	b := &snapshotBuilder{page: p, ctx: ctx, out: Snapshot{URL: p.URL(), Files: map[string][]byte{}, Warnings: []string{}}, seen: map[string]string{}}
	var findBase func(*html.Node) bool
	findBase = func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "base" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					if u, e := base.Parse(a.Val); e == nil {
						base = u
					}
					return true
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if findBase(c) {
				return true
			}
		}
		return false
	}
	findBase(root)
	b.rewriteHTML(root, base)
	var output bytes.Buffer
	// Portable snapshots use the historical HTML5 preamble exactly once.
	// Remove only the immutable projection's doctype, not canonical DOM state.
	for c := root.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.DoctypeNode {
			root.RemoveChild(c)
		}
		c = next
	}
	output.WriteString("<!doctype html>\n")
	if err := html.Render(&output, root); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.out.Files["index.html"] = output.Bytes()
	return &b.out, nil
}

func (b *snapshotBuilder) asset(raw string, base *url.URL, css bool) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "data:") {
		return raw
	}
	u, err := base.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	fragment := u.Fragment
	u.Fragment = ""
	key := u.String()
	if name, ok := b.seen[key]; ok {
		if fragment != "" && name != "" {
			return name + "#" + fragment
		}
		return name
	}
	if b.count >= 512 {
		b.out.Warnings = append(b.out.Warnings, "Resource limit reached: "+key)
		return ""
	}
	b.count++
	b.seen[key] = ""
	response, ok := b.page.loader.CompletedURL(key)
	if !ok {
		response, err = b.page.loader.Load(b.ctx, network.Request{URL: u, Method: http.MethodGet, Initiator: network.Other, Referrer: base, SourceURL: base})
	}
	if err != nil || response.Status < 200 || response.Status >= 300 {
		b.out.Warnings = append(b.out.Warnings, fmt.Sprintf("Resource unavailable: %s (status %d, error %v)", key, response.Status, err))
		return ""
	}
	if len(response.Body) > 32<<20 {
		b.out.Warnings = append(b.out.Warnings, "Resource too large: "+key)
		return ""
	}
	ext := path.Ext(u.Path)
	if len(ext) > 10 || strings.ContainsAny(ext, "\\/:") {
		ext = ""
	}
	// Dynamic resource endpoints need the response's media type in a portable
	// filename. Static servers cannot infer SVG image MIME from a .php URL.
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(response.Headers.Get("Content-Type"), ";")[0]))
	if mediaExt := snapshotMediaExtensions[mediaType]; mediaExt != "" {
		ext = mediaExt
	}
	css = css || strings.Contains(response.Headers.Get("Content-Type"), "text/css")
	if css {
		ext = ".css"
	}
	name := fmt.Sprintf("assets/%x%s", sha256.Sum256([]byte(key)), ext)
	b.seen[key] = name
	body := response.Body
	if css {
		resourceBase := u
		if response.URL != nil {
			resourceBase = response.URL
		}
		body = []byte(b.rewriteCSS(string(body), resourceBase, true))
	}
	b.out.Files[name] = body
	if fragment != "" {
		return name + "#" + fragment
	}
	return name
}

var snapshotMediaExtensions = map[string]string{
	"image/svg+xml": ".svg", "image/png": ".png", "image/jpeg": ".jpg",
	"image/gif": ".gif", "image/webp": ".webp", "image/avif": ".avif",
	"image/x-icon": ".ico", "image/vnd.microsoft.icon": ".ico",
	"font/woff": ".woff", "font/woff2": ".woff2", "font/ttf": ".ttf", "font/otf": ".otf",
}

// Match ordinary CSS url() and quoted @import forms. Exotic escaped CSS URLs
// are outside the current snapshot contract.
var snapshotCSSURL = regexp.MustCompile(`(?i)url\(\s*(?:"([^"\r\n]*)"|'([^'\r\n]*)'|([^)'"\s]*))\s*\)|@import\s+(?:"([^"\r\n]*)"|'([^'\r\n]*)')`)

func (b *snapshotBuilder) rewriteCSS(source string, base *url.URL, external bool) string {
	return snapshotCSSURL.ReplaceAllStringFunc(source, func(match string) string {
		groups := snapshotCSSURL.FindStringSubmatch(match)
		raw := ""
		for _, v := range groups[1:] {
			if v != "" {
				raw = v
				break
			}
		}
		// Self-contained URLs need no relocation. Preserve their CSS quoting:
		// an SVG data URL may contain literal double quotes inside a single-
		// quoted url(), which would become invalid if blindly wrapped in "".
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "data:") || strings.HasPrefix(strings.TrimSpace(raw), "#") {
			return match
		}
		imported := strings.HasPrefix(strings.ToLower(match), "@import")
		name := b.asset(raw, base, imported)
		if external {
			name = strings.TrimPrefix(name, "assets/")
		}
		if imported {
			return `@import "` + name + `"`
		}
		return `url("` + name + `")`
	})
}

func (b *snapshotBuilder) rewriteHTML(n *html.Node, base *url.URL) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if c.Type == html.ElementNode && (c.Data == "script" || c.Data == "base" || c.Data == "iframe" || c.Data == "object" || c.Data == "embed") {
			if c.Data == "iframe" {
				b.out.Warnings = append(b.out.Warnings, "Embedded frame omitted")
			}
			n.RemoveChild(c)
			c = next
			continue
		}
		if c.Type == html.ElementNode && c.Data == "meta" {
			remove := false
			for _, a := range c.Attr {
				if a.Key == "http-equiv" || a.Key == "charset" {
					remove = true
				}
			}
			if remove {
				n.RemoveChild(c)
				c = next
				continue
			}
		}
		b.rewriteHTML(c, base)
		c = next
	}
	if n.Type != html.ElementNode {
		return
	}
	attrs := n.Attr
	n.Attr = nil
	rel := ""
	for _, a := range attrs {
		if a.Key == "rel" {
			rel = strings.ToLower(a.Val)
		}
	}
	for _, a := range attrs {
		if strings.HasPrefix(a.Key, "on") || a.Key == "integrity" || a.Key == "crossorigin" || a.Key == "srcset" || a.Key == "ping" || a.Key == "action" || a.Key == "formaction" {
			continue
		}
		if a.Key == "style" {
			a.Val = b.rewriteCSS(a.Val, base, false)
		}
		if a.Key == "src" || a.Key == "poster" || a.Key == "background" || (a.Key == "href" && n.Data == "link" && (strings.Contains(rel, "stylesheet") || strings.Contains(rel, "icon"))) {
			a.Val = b.asset(a.Val, base, n.Data == "link" && strings.Contains(rel, "stylesheet"))
		} else if a.Key == "href" {
			if strings.HasPrefix(strings.TrimSpace(strings.ToLower(a.Val)), "javascript:") {
				continue
			}
			if !strings.HasPrefix(a.Val, "#") {
				if u, e := base.Parse(a.Val); e == nil {
					a.Val = u.String()
				}
			}
		}
		n.Attr = append(n.Attr, a)
	}
	if n.Data == "style" {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				c.Data = b.rewriteCSS(c.Data, base, false)
			}
		}
	}
	if n.Data == "head" {
		policy := &html.Node{Type: html.ElementNode, Data: "meta", Attr: []html.Attribute{{Key: "http-equiv", Val: "Content-Security-Policy"}, {Key: "content", Val: "default-src 'self' file: data:; script-src 'none'; connect-src 'none'; frame-src 'none'; object-src 'none'; style-src 'self' file: 'unsafe-inline'; form-action 'none'"}}}
		n.InsertBefore(policy, n.FirstChild)
		meta := &html.Node{Type: html.ElementNode, Data: "meta", Attr: []html.Attribute{{Key: "charset", Val: "utf-8"}}}
		n.InsertBefore(meta, n.FirstChild)
	}
}
