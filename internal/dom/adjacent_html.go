package dom

import (
	"fmt"
	"strings"
)

// InsertAdjacentHTML inserts a parsed fragment without replacing existing nodes.
// The returned name is a DOMException boundary, distinct from parser failures.
func (d *Document) InsertAdjacentHTML(id int64, position, source string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	node := d.nodes[id]
	if node == nil || node.Type != "element" {
		return "", fmt.Errorf("element node %d does not exist", id)
	}
	var parent, context *Node
	var before int64
	position = strings.ToLower(position)
	switch position {
	case "beforebegin", "afterend":
		parent = d.nodes[node.Parent]
		if parent == nil || parent.Type == "document" {
			return "NoModificationAllowedError", nil
		}
		context = parent
		if position == "beforebegin" {
			before = id
		} else {
			for i, child := range parent.Children {
				if child == id && i+1 < len(parent.Children) {
					before = parent.Children[i+1]
					break
				}
			}
		}
	case "afterbegin", "beforeend":
		parent, context = node, node
		if position == "afterbegin" && len(parent.Children) > 0 {
			before = parent.Children[0]
		}
	default:
		return "SyntaxError", nil
	}
	if context.Type != "element" || context.TagName == "HTML" {
		context = &Node{TagName: "BODY", Namespace: "http://www.w3.org/1999/xhtml"}
	}
	d.markConnectedMutationLocked(parent.ID)
	return "", d.insertHTMLLocked(parent, context, source, before, false)
}

func (d *Document) SetOuterHTML(id int64, source string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	node := d.nodes[id]
	if node == nil || node.Type != "element" {
		return "", fmt.Errorf("element node %d does not exist", id)
	}
	parent := d.nodes[node.Parent]
	if parent == nil {
		return "", nil
	}
	if parent.Type == "document" {
		return "NoModificationAllowedError", nil
	}
	context := parent
	if context.Type != "element" {
		context = &Node{TagName: "BODY", Namespace: "http://www.w3.org/1999/xhtml"}
	}
	d.markConnectedMutationLocked(parent.ID)
	if err := d.insertHTMLLocked(parent, context, source, id, false); err != nil {
		return "", err
	}
	for i, child := range parent.Children {
		if child == id {
			parent.Children = append(parent.Children[:i], parent.Children[i+1:]...)
			break
		}
	}
	node.Parent = 0
	return "", nil
}
