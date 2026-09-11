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
	"sync"
	"time"

	"github.com/moreveal/mimic/internal/network"
	"golang.org/x/net/html"
)

// Snapshot is a static, portable document. Files are relative to index.html;
// byte slices are base64 encoded by the CDP JSON encoder.
type Snapshot struct {
	URL       string            `json:"url"`
	Files     map[string][]byte `json:"files"`
	Warnings  []string          `json:"warnings"`
	TimingsMS map[string]int64  `json:"timingsMs"`
}

type snapshotBuilder struct {
	page             *Page
	ctx              context.Context
	out              Snapshot
	seen             map[string]string
	prefetched       map[string]snapshotAssetLoad
	prefetchComplete bool
	count            int
}

type snapshotAssetSpec struct {
	key  string
	url  *url.URL
	base *url.URL
	css  bool
}

type snapshotAssetLoad struct {
	response network.Response
	err      error
}

const (
	snapshotFetchConcurrency = 32
	snapshotFetchBudget      = 8 * time.Second
)

// CaptureSnapshot exports the current DOM, without executing the exported scripts.
// Callers must serialize this operation with navigation/evaluation, as CDP does.
func (p *Page) CaptureSnapshot(ctx context.Context) (*Snapshot, error) {
	timings := map[string]int64{}
	stage := time.Now()
	mark := func(name string) {
		timings[name] = time.Since(stage).Milliseconds()
		stage = time.Now()
	}
	d, ok := p.Document()
	if !ok {
		return nil, fmt.Errorf("page has no document")
	}
	shadows, err := p.Top.Realm.ShadowSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	mark("shadowState")
	forms, err := p.Top.Realm.FormSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	mark("formState")
	root, err := d.SnapshotTreeWithFormState(d.Root().ID, shadows, forms)
	if err != nil {
		return nil, err
	}
	mark("cloneDOM")
	base, err := url.Parse(p.URL())
	if err != nil {
		return nil, err
	}
	b := &snapshotBuilder{
		page:       p,
		ctx:        ctx,
		out:        Snapshot{URL: p.URL(), Files: map[string][]byte{}, Warnings: []string{}, TimingsMS: timings},
		seen:       map[string]string{},
		prefetched: map[string]snapshotAssetLoad{},
	}
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
	mark("findBase")
	b.prefetchHTMLAssets(root, base)
	mark("fetchAssets")
	b.rewriteHTML(root, base)
	mark("rewriteDOM")
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
	mark("renderHTML")
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.out.Files["index.html"] = output.Bytes()
	return &b.out, nil
}

func snapshotAssetURL(raw string, base *url.URL) (string, *url.URL, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(strings.ToLower(raw), "data:") {
		return "", nil, false
	}
	u, err := base.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", nil, false
	}
	u.Fragment = ""
	return u.String(), u, true
}

func (b *snapshotBuilder) loadAsset(ctx context.Context, spec snapshotAssetSpec) snapshotAssetLoad {
	if response, ok := b.page.loader.CompletedURL(spec.key); ok {
		return snapshotAssetLoad{response: response}
	}
	response, err := b.page.loader.Load(ctx, network.Request{
		URL: spec.url, Method: http.MethodGet, Initiator: network.Other,
		Referrer: spec.base, SourceURL: spec.base,
	})
	return snapshotAssetLoad{response: response, err: err}
}

func (b *snapshotBuilder) loadAssetBatch(ctx context.Context, specs []snapshotAssetSpec) {
	if len(specs) == 0 {
		return
	}
	type result struct {
		key  string
		load snapshotAssetLoad
	}
	workerCount := min(snapshotFetchConcurrency, len(specs))
	jobs := make(chan snapshotAssetSpec)
	results := make(chan result, len(specs))
	var workers sync.WaitGroup
	workers.Add(workerCount)
	defer workers.Wait()
	for range workerCount {
		go func() {
			defer workers.Done()
			for spec := range jobs {
				results <- result{key: spec.key, load: b.loadAsset(ctx, spec)}
			}
		}()
	}
	go func() {
		for _, spec := range specs {
			jobs <- spec
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()
	remaining := len(specs)
	for remaining > 0 {
		select {
		case result, ok := <-results:
			if !ok {
				return
			}
			b.prefetched[result.key] = result.load
			remaining--
		case <-ctx.Done():
			for _, spec := range specs {
				if _, ok := b.prefetched[spec.key]; !ok {
					b.prefetched[spec.key] = snapshotAssetLoad{err: ctx.Err()}
				}
			}
			return
		}
	}
}

func collectSnapshotCSSAssets(source string, base *url.URL, add func(snapshotAssetSpec)) {
	for _, groups := range snapshotCSSURL.FindAllStringSubmatch(source, -1) {
		raw := ""
		for _, value := range groups[1:] {
			if value != "" {
				raw = value
				break
			}
		}
		if key, resource, ok := snapshotAssetURL(raw, base); ok {
			add(snapshotAssetSpec{key: key, url: resource, base: base, css: strings.HasPrefix(strings.ToLower(groups[0]), "@import")})
		}
	}
}

func collectSnapshotHTMLAssets(n *html.Node, base *url.URL, add func(snapshotAssetSpec)) {
	if n.Type == html.ElementNode {
		// These subtrees are omitted by rewriteHTML. Do not spend the fetch
		// budget (or issue new requests) on content that cannot be exported.
		switch n.Data {
		case "script", "base", "iframe", "object", "embed":
			return
		}
		rel := ""
		for _, attribute := range n.Attr {
			if attribute.Key == "rel" {
				rel = strings.ToLower(attribute.Val)
			}
		}
		for _, attribute := range n.Attr {
			if attribute.Key == "style" {
				collectSnapshotCSSAssets(attribute.Val, base, add)
			}
			isStylesheet := n.Data == "link" && strings.Contains(rel, "stylesheet")
			isAsset := attribute.Key == "src" || attribute.Key == "poster" || attribute.Key == "background" ||
				(attribute.Key == "href" && n.Data == "link" && (isStylesheet || strings.Contains(rel, "icon")))
			if isAsset {
				if key, resource, ok := snapshotAssetURL(attribute.Val, base); ok {
					add(snapshotAssetSpec{key: key, url: resource, base: base, css: isStylesheet})
				}
			}
		}
		if n.Data == "style" {
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode {
					collectSnapshotCSSAssets(child.Data, base, add)
				}
			}
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		collectSnapshotHTMLAssets(child, base, add)
	}
}

func (b *snapshotBuilder) prefetchHTMLAssets(root *html.Node, base *url.URL) {
	prefetchContext, cancel := context.WithTimeout(b.ctx, snapshotFetchBudget)
	defer cancel()
	defer func() { b.prefetchComplete = true }()
	queued := map[string]bool{}
	queue := make([]snapshotAssetSpec, 0)
	add := func(spec snapshotAssetSpec) {
		if len(queued) >= 512 || queued[spec.key] {
			return
		}
		queued[spec.key] = true
		queue = append(queue, spec)
	}
	collectSnapshotHTMLAssets(root, base, add)
	for len(queue) > 0 {
		batch := queue
		queue = nil
		b.loadAssetBatch(prefetchContext, batch)
		for _, spec := range batch {
			load := b.prefetched[spec.key]
			response := load.response
			if load.err != nil || response.Status < 200 || response.Status >= 300 || len(response.Body) > 32<<20 {
				continue
			}
			mediaType := strings.ToLower(strings.TrimSpace(strings.Split(response.Headers.Get("Content-Type"), ";")[0]))
			if !spec.css && mediaType != "text/css" {
				continue
			}
			resourceBase := spec.url
			if response.URL != nil {
				resourceBase = response.URL
			}
			collectSnapshotCSSAssets(string(response.Body), resourceBase, add)
		}
		if prefetchContext.Err() != nil {
			break
		}
	}
}

func (b *snapshotBuilder) asset(raw string, base *url.URL, css bool) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(strings.ToLower(raw), "data:") {
		return raw
	}
	key, u, ok := snapshotAssetURL(raw, base)
	if !ok {
		return ""
	}
	fragment := ""
	if parsed, err := base.Parse(raw); err == nil {
		fragment = parsed.Fragment
	}
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
	load, prefetched := b.prefetched[key]
	if !prefetched {
		if b.prefetchComplete {
			load.err = fmt.Errorf("resource was not reached within the snapshot fetch budget")
		} else {
			load = b.loadAsset(b.ctx, snapshotAssetSpec{key: key, url: u, base: base, css: css})
		}
	}
	response, err := load.response, load.err
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
