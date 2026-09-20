package layoutblitz

import (
	"fmt"
	"github.com/moreveal/mimic/internal/dom"
	"strings"
)

// FromSnapshot initializes a persistent native projection with canonical IDs.
// An incomplete projection is destroyed and never published to observers.
// Shadow roots and external resources require additional input adapters before
// this initializer can be used by the browser's default producer.
func FromSnapshot(snapshot dom.DerivedSnapshot, width, height uint32) (*Owner, error) {
	return FromSnapshotAtURL(snapshot, width, height, "about:blank")
}
func FromSnapshotAtURL(snapshot dom.DerivedSnapshot, width, height uint32, baseURL string) (*Owner, error) {
	owner, err := New(uint64(snapshot.Root), width, height)
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			owner.Close()
		}
	}()
	if err := owner.BaseURL(baseURL); err != nil {
		return nil, err
	}
	for _, node := range snapshot.Nodes {
		if node.ID == snapshot.Root {
			continue
		}
		switch node.Type {
		case "element":
			name := node.TagName
			if node.Namespace == "http://www.w3.org/1999/xhtml" {
				name = strings.ToLower(name)
			}
			if err := owner.Element(uint64(node.ID), node.Namespace, name); err != nil {
				return nil, err
			}
			for _, name := range node.AttributeNames {
				if err := owner.Attribute(uint64(node.ID), node.AttributeNamespaces[name], name, node.Attributes[name]); err != nil {
					return nil, err
				}
			}
		case "text":
			if node.TextJSON != "" {
				return nil, fmt.Errorf("blitz: UTF-16 code-unit text adapter required for node %d", node.ID)
			}
			if err := owner.Text(uint64(node.ID), node.Text, true); err != nil {
				return nil, err
			}
		case "comment", "doctype":
			// Doctype participates in canonical identity but has no layout box.
			// Document quirks mode is a separate producer input.
			if err := owner.Comment(uint64(node.ID), node.Text); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("blitz: unsupported canonical node kind %q", node.Type)
		}
		if err := owner.Append(uint64(node.Parent), uint64(node.ID)); err != nil {
			return nil, err
		}
	}
	failed = false
	return owner, nil
}
