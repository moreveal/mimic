package dom

import "fmt"

func (d *Document) CopyInlineStyle(source, destination int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	a, b := d.nodes[source], d.nodes[destination]
	if a != nil && b != nil && a.Attributes["style"] == b.Attributes["style"] {
		if b.StyleDeclarationsJSON != a.StyleDeclarationsJSON {
			d.markConnectedMutationLocked(destination)
		}
		b.StyleDeclarationsJSON = a.StyleDeclarationsJSON
	}
}

// InlineStyleState returns either the parsed CSSOM state or the attribute that
// still needs parsing. Parsed pending-substitution values cannot round-trip
// through cssText; the DOM node owns them along with its attribute projection.
func (d *Document) InlineStyleState(id int64) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	if n == nil {
		return "s"
	}
	if n.StyleDeclarationsJSON != "" {
		return "j" + n.StyleDeclarationsJSON
	}
	return "s" + n.Attributes["style"]
}

func (d *Document) SetInlineStyle(id int64, text, declarationsJSON string) (any, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := d.nodes[id]
	if n == nil || n.Type != "element" {
		return nil, fmt.Errorf("element node %d does not exist", id)
	}
	var old any
	if value, ok := n.Attributes["style"]; ok {
		old = value
	}
	n.setAttribute("style", text)
	n.StyleDeclarationsJSON = declarationsJSON
	d.recordMutationLocked("attribute", id, "style")
	d.recordConnectedMutationLocked("attribute", id, "style")
	return old, nil
}
