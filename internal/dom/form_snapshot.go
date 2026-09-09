package dom

import "golang.org/x/net/html"

// FormSnapshot contains only runtime-owned current control state. Nil fields
// leave the parsed default alone; pointers preserve explicit false/empty values.
type FormSnapshot struct {
	NodeID   int64   `json:"nodeID"`
	Value    *string `json:"value,omitempty"`
	Checked  *bool   `json:"checked,omitempty"`
	Selected *bool   `json:"selected,omitempty"`
}

func projectFormSnapshot(out *html.Node, state FormSnapshot) {
	set := func(name string, value *string) {
		attrs := out.Attr[:0]
		for _, attr := range out.Attr {
			if attr.Namespace != "" || attr.Key != name {
				attrs = append(attrs, attr)
			}
		}
		out.Attr = attrs
		if value != nil {
			out.Attr = append(out.Attr, html.Attribute{Key: name, Val: *value})
		}
	}
	boolean := func(name string, value *bool) {
		if value == nil {
			return
		}
		if *value {
			empty := ""
			set(name, &empty)
		} else {
			set(name, nil)
		}
	}
	switch out.Data {
	case "input":
		if state.Value != nil {
			set("value", state.Value)
		}
		boolean("checked", state.Checked)
	case "option":
		boolean("selected", state.Selected)
	case "textarea":
		if state.Value != nil {
			for out.FirstChild != nil {
				out.RemoveChild(out.FirstChild)
			}
			out.AppendChild(&html.Node{Type: html.TextNode, Data: *state.Value})
		}
	}
}
