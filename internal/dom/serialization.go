package dom

import (
	"bytes"
	"strings"

	"golang.org/x/net/html"
)

// Fragment serialization is a browser observation, not a round-trip HTML
// writer. In particular, text quotes remain literal and attribute quotes use
// &quot;. Escaping the finished output would also corrupt script/style data.
var fragmentTextEscape = strings.NewReplacer("&", "&amp;", "\u00a0", "&nbsp;", "<", "&lt;", ">", "&gt;")
var fragmentAttributeEscape = strings.NewReplacer("&", "&amp;", "\u00a0", "&nbsp;", "\"", "&quot;", "<", "&lt;", ">", "&gt;")

func rawHTMLText(tag, namespace string) bool {
	if namespace != "" && namespace != "http://www.w3.org/1999/xhtml" {
		return false
	}
	switch strings.ToLower(tag) {
	case "script", "style", "xmp", "iframe", "noembed", "noframes", "plaintext", "noscript":
		return true
	}
	return false
}

func renderFragmentNode(out *bytes.Buffer, n *html.Node, raw bool) {
	if n == nil {
		return
	}
	switch n.Type {
	case html.TextNode:
		if raw {
			out.WriteString(n.Data)
		} else {
			out.WriteString(fragmentTextEscape.Replace(n.Data))
		}
		return
	case html.CommentNode:
		out.WriteString("<!--" + n.Data + "-->")
		return
	case html.DoctypeNode:
		out.WriteString("<!DOCTYPE " + n.Data + ">")
		return
	case html.ElementNode:
		out.WriteByte('<')
		out.WriteString(n.Data)
		for _, a := range n.Attr {
			out.WriteByte(' ')
			if a.Namespace != "" {
				out.WriteString(a.Namespace)
				out.WriteByte(':')
			}
			out.WriteString(a.Key)
			out.WriteString("=\"")
			out.WriteString(fragmentAttributeEscape.Replace(a.Val))
			out.WriteByte('"')
		}
		out.WriteByte('>')
		if n.Namespace == "" {
			switch n.Data {
			case "area", "base", "basefont", "bgsound", "br", "col", "embed", "hr", "img", "input", "keygen", "link", "meta", "param", "source", "track", "wbr":
				return
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		renderFragmentNode(out, c, rawHTMLText(n.Data, n.Namespace))
	}
	if n.Type == html.ElementNode {
		out.WriteString("</" + n.Data + ">")
	}
}

// The nonce IDL attribute is a cryptographic slot, not a content-attribute
// reflection. Attribute mutation updates the slot; writing the slot does not
// create an attribute. Keeping it on Node preserves clone/adoption/CSP identity.
func (d *Document) SetNonce(id int64, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if n := d.nodes[id]; n != nil {
		n.Nonce = value
	}
	return nil
}
