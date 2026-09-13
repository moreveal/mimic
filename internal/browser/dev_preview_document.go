package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// This is an immutable projection, never a new runtime or a resource loader.
// The viewer's browser draws it; no preview fetch enters the Page network stack.
func (p *Page) previewDocument(frame *Frame) (string, error) {
	r := frame.Realm
	var root *html.Node
	var observation any
	err := r.runOnOwner(context.Background(), func(ctx context.Context) error {
		shadows, err := r.ShadowSnapshots(ctx)
		if err != nil {
			return err
		}
		forms, err := r.FormSnapshots(ctx)
		if err != nil {
			return err
		}
		root, err = r.document.PreviewTree(r.document.Root().ID, shadows, forms, r.ID)
		if err != nil {
			return err
		}
		v, err := r.runtime.Call(ctx, r.previewRead, nil, r.val("snapshot"))
		if err == nil {
			observation = v.Export()
		}
		return err
	})
	if err != nil {
		return "", err
	}
	base, _ := url.Parse(frame.URL())
	var data struct {
		Styles       []struct{ Text, Base string }
		Modals       []int64
		Scroll       map[string][]float64
		WindowScroll []float64
	}
	wire, _ := json.Marshal(observation)
	if err := json.Unmarshal(wire, &data); err != nil {
		return "", err
	}
	modals := make(map[string]int, len(data.Modals))
	for i, id := range data.Modals {
		modals[fmt.Sprintf("%s:%d", r.ID, id)] = i + 1
	}
	var head *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "head" {
			head = n
		}
		if n.Type == html.ElementNode && n.Data == "base" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					if u, e := base.Parse(a.Val); e == nil {
						base = u
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(root)
	iframes := r.document.FindAllByTagName("iframe")
	index := 0
	var clean func(*html.Node, bool) error
	clean = func(n *html.Node, shadow bool) error {
		if n.Data == "template" {
			for _, a := range n.Attr {
				if a.Key == "shadowrootmode" {
					shadow = true
				}
			}
		}
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			if c.Type == html.ElementNode {
				// The source Page has scripting enabled, but the mirror's sandbox
				// does not. Omit its inert noscript fallback before the viewer
				// reparses raw text as active markup (including styles/refreshes).
				remove := (!shadow && c.Data == "style") || c.Data == "noscript" || c.Data == "script" || c.Data == "base" || c.Data == "object" || c.Data == "embed" || c.Data == "meta" || c.Data == "link"
				if remove {
					n.RemoveChild(c)
					c = next
					continue
				}
				attrs := c.Attr[:0]
				for _, a := range c.Attr {
					if a.Key == "data-mimic-preview-modal" || a.Key == "data-mimic-preview-scroll" || a.Key == "data-mimic-preview-window-scroll" {
						continue
					}
					if strings.HasPrefix(strings.ToLower(a.Key), "on") || a.Key == "srcdoc" || a.Key == "autofocus" || a.Key == "action" || a.Key == "formaction" || (c.Data == "a" && a.Key == "href") || (c.Data == "iframe" && (a.Key == "src" || a.Key == "sandbox")) {
						continue
					}
					attrs = append(attrs, a)
				}
				c.Attr = attrs
				for _, a := range attrs {
					if a.Key == "data-mimic-preview-node" {
						if value := data.Scroll[strings.TrimPrefix(a.Val, r.ID+":")]; len(value) == 2 {
							c.Attr = append(c.Attr, html.Attribute{Key: "data-mimic-preview-scroll", Val: fmt.Sprintf("%g,%g", value[0], value[1])})
						}
					}
				}
				if c.Data == "html" && len(data.WindowScroll) == 2 {
					c.Attr = append(c.Attr, html.Attribute{Key: "data-mimic-preview-window-scroll", Val: fmt.Sprintf("%g,%g", data.WindowScroll[0], data.WindowScroll[1])})
				}
				if c.Data == "dialog" {
					for _, a := range attrs {
						if a.Key == "data-mimic-preview-node" && modals[a.Val] > 0 {
							c.Attr = append(c.Attr, html.Attribute{Key: "data-mimic-preview-modal", Val: fmt.Sprint(modals[a.Val])})
							break
						}
					}
				}
				if c.Data == "iframe" {
					markup := ""
					if index < len(iframes) {
						for _, child := range frame.Children() {
							if child.elementID == iframes[index].ID && child.Realm != nil {
								var e error
								markup, e = p.previewDocument(child)
								if e != nil {
									return e
								}
								break
							}
						}
					}
					index++
					c.Attr = append(c.Attr, html.Attribute{Key: "sandbox", Val: ""}, html.Attribute{Key: "srcdoc", Val: markup})
				}
			}
			if err := clean(c, shadow); err != nil {
				return err
			}
			c = next
		}
		return nil
	}
	if err := clean(root, false); err != nil {
		return "", err
	}
	if head != nil {
		meta := &html.Node{Type: html.ElementNode, Data: "meta", Attr: []html.Attribute{{Key: "http-equiv", Val: "Content-Security-Policy"}, {Key: "content", Val: "script-src 'none'; object-src 'none'; connect-src 'none'; form-action 'none'"}}}
		head.InsertBefore(meta, head.FirstChild)
		head.AppendChild(&html.Node{Type: html.ElementNode, Data: "base", Attr: []html.Attribute{{Key: "href", Val: base.String()}}})
		// Preserve stylesheet mutations, including rules changed via CSSOM. The
		// existing snapshot path already preserves adopted and shadow styles.
		for _, sheet := range data.Styles {
			u, e := url.Parse(sheet.Base)
			if e != nil {
				u = base
			}
			css := snapshotCSSURL.ReplaceAllStringFunc(sheet.Text, func(match string) string {
				groups := snapshotCSSURL.FindStringSubmatch(match)
				for _, raw := range groups[1:] {
					if raw != "" && !strings.HasPrefix(raw, "#") {
						if absolute, e := u.Parse(raw); e == nil {
							return strings.Replace(match, raw, absolute.String(), 1)
						}
					}
				}
				return match
			})
			style := &html.Node{Type: html.ElementNode, Data: "style"}
			style.AppendChild(&html.Node{Type: html.TextNode, Data: css})
			head.AppendChild(style)
		}
	}
	var out bytes.Buffer
	err = html.Render(&out, root)
	return out.String(), err
}
