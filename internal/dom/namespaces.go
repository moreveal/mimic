package dom

import (
	"fmt"
	"golang.org/x/net/html"
	"strings"
)

func parserNamespace(uri string) string {
	switch uri {
	case "http://www.w3.org/2000/svg":
		return "svg"
	case "http://www.w3.org/1998/Math/MathML":
		return "math"
	}
	return ""
}
func (n *Node) attributeName(name string) string {
	if n.Namespace == "http://www.w3.org/1999/xhtml" || n.Namespace == "" && n.QualifiedName == "" {
		return strings.ToLower(name)
	}
	return name
}
func (n *Node) setParsedAttribute(a html.Attribute) {
	name, namespace := a.Key, ""
	switch a.Namespace {
	case "xlink":
		namespace = "http://www.w3.org/1999/xlink"
	case "xml":
		namespace = "http://www.w3.org/XML/1998/namespace"
	case "xmlns":
		namespace = "http://www.w3.org/2000/xmlns/"
	}
	if a.Namespace != "" {
		name = a.Namespace + ":" + name
	}
	n.setAttribute(name, a.Val)
	if namespace != "" {
		if n.AttributeNamespaces == nil {
			n.AttributeNamespaces = map[string]string{}
		}
		n.AttributeNamespaces[name] = namespace
	}
}
func attributeLocalName(name string) string {
	if _, local, ok := strings.Cut(name, ":"); ok {
		return local
	}
	return name
}

// Namespace lookup is over the canonical attribute collection, independent of
// the prefix used to serialize the qualified name.
func (d *Document) AttributeNameNS(id int64, namespace, local string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	if n == nil {
		return ""
	}
	for _, name := range n.AttributeNames {
		if n.AttributeNamespaces[name] == namespace && attributeLocalName(name) == local {
			return name
		}
	}
	return ""
}
func (d *Document) SetAttributeNS(id int64, namespace, name, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := d.nodes[id]
	if n == nil || n.Type != "element" {
		return fmt.Errorf("element node %d does not exist", id)
	}
	local := attributeLocalName(name)
	for _, old := range n.AttributeNames {
		if n.AttributeNamespaces[old] == namespace && attributeLocalName(old) == local {
			// setAttributeNS changes the value of the existing Attr, retaining its prefix.
			if old == "style" && namespace == "" && n.Attributes[old] != value {
				n.StyleDeclarationsJSON = ""
			}
			n.Attributes[old] = value
			return nil
		}
	}
	if _, exists := n.Attributes[name]; exists && n.AttributeNamespaces[name] != namespace {
		return fmt.Errorf("attributes with the same qualified name in different namespaces are unsupported")
	}
	n.setAttribute(name, value)
	if namespace != "" {
		if n.AttributeNamespaces == nil {
			n.AttributeNamespaces = map[string]string{}
		}
		n.AttributeNamespaces[name] = namespace
	}
	return nil
}
