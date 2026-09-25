package dom

import (
	"fmt"
	"strings"

	"github.com/moreveal/mimic/internal/htmlstream"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var ErrStreamPaused = htmlstream.ErrPaused

type Stream struct {
	document *Document
	parser   *htmlstream.Stream
}

// NewStream clears the document's connected children while retaining the
// Document and every detached node's identity. Abort the previous stream first.
func (d *Document) NewStream() (*Stream, error) {
	d.mu.Lock()
	root := d.nodes[d.root]
	if root == nil || root.Type != "document" {
		d.mu.Unlock()
		return nil, fmt.Errorf("HTML stream requires a document")
	}
	d.markConnectedMutationLocked(d.root)
	for _, id := range root.Children {
		if node := d.nodes[id]; node != nil {
			node.Parent = 0
		}
	}
	root.Children = nil
	d.title = ""
	d.source = ""
	d.scriptPositions = nil
	id := d.root
	d.mu.Unlock()
	s := &Stream{document: d}
	s.parser = htmlstream.New(streamBackend{d}, id)
	return s, nil
}
func (s *Stream) callback(onScript func(Node) error) func(int64) error {
	return func(id int64) error {
		if position, ok := s.parser.ScriptPosition(id); ok {
			s.document.mu.Lock()
			if s.document.scriptPositions == nil {
				s.document.scriptPositions = make(map[int64]htmlstream.ScriptPosition)
			}
			s.document.scriptPositions[id] = position
			s.document.mu.Unlock()
		}
		node, ok := s.document.Get(id)
		if !ok {
			return fmt.Errorf("stream script %d missing", id)
		}
		if onScript != nil {
			return onScript(node)
		}
		return nil
	}
}
func (s *Stream) Write(source string, onScript func(Node) error) error {
	return s.parser.Write(source, s.callback(onScript))
}
func (s *Stream) Close(onScript func(Node) error) error { return s.parser.Close(s.callback(onScript)) }
func (s *Stream) Resume(onScript func(Node) error) error {
	return s.parser.Resume(s.callback(onScript))
}
func (s *Stream) Closed() bool { return s.parser.Closed() }
func (s *Stream) Paused() bool { return s.parser.Paused() }
func (s *Stream) Abort()       { s.parser.Abort() }

type streamBackend struct{ d *Document }

func (b streamBackend) Create(data htmlstream.NodeData) int64 {
	d := b.d
	d.mu.Lock()
	defer d.mu.Unlock()
	d.next++
	node := &Node{ID: d.next, OwnerDocument: d.root}
	switch data.Type {
	case html.ElementNode:
		node.Type = "element"
		node.TagName = strings.ToUpper(data.Data)
		if data.Namespace != "" {
			node.QualifiedName = data.Data
		}
		node.Namespace = streamNamespace(data.Namespace)
	case html.TextNode:
		node.Type = "text"
		node.Text = data.Data
	case html.CommentNode:
		node.Type = "comment"
		node.Text = data.Data
	case html.DoctypeNode:
		node.Type = "doctype"
		node.TagName = data.Data
	default:
		node.Type = "other"
	}
	for _, attr := range data.Attr {
		node.setParsedAttribute(attr)
	}
	d.nodes[node.ID] = node
	d.hasFrameElements = d.hasFrameElements || node.TagName == "IFRAME"
	if node.TagName == "TEMPLATE" && node.Namespace == "http://www.w3.org/1999/xhtml" {
		d.newTemplateContentLocked(node)
	}
	return node.ID
}
func streamNamespace(namespace string) string {
	switch namespace {
	case "svg":
		return "http://www.w3.org/2000/svg"
	case "math":
		return "http://www.w3.org/1998/Math/MathML"
	default:
		return "http://www.w3.org/1999/xhtml"
	}
}
func (b streamBackend) Get(id int64) htmlstream.NodeData {
	d := b.d
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	if n == nil {
		panic(fmt.Sprintf("HTML parser node %d missing", id))
	}
	out := htmlstream.NodeData{Parent: n.Parent, Namespace: parserNamespace(n.Namespace)}
	switch n.Type {
	case "document":
		out.Type = html.DocumentNode
	case "element":
		out.Type = html.ElementNode
		out.Data = n.QualifiedName
		if out.Data == "" {
			out.Data = strings.ToLower(n.TagName)
		}
		out.DataAtom = atom.Lookup([]byte(out.Data))
	case "text":
		out.Type = html.TextNode
		out.Data = n.Text
	case "comment":
		out.Type = html.CommentNode
		out.Data = n.Text
	case "doctype":
		out.Type = html.DoctypeNode
		out.Data = n.TagName
	}
	for _, name := range n.AttributeNames {
		namespace := n.AttributeNamespaces[name]
		key := name
		if namespace != "" {
			if index := strings.IndexByte(key, ':'); index >= 0 {
				key = key[index+1:]
			}
		}
		switch namespace {
		case "http://www.w3.org/1999/xlink":
			namespace = "xlink"
		case "http://www.w3.org/XML/1998/namespace":
			namespace = "xml"
		case "http://www.w3.org/2000/xmlns/":
			namespace = "xmlns"
		}
		out.Attr = append(out.Attr, html.Attribute{Namespace: namespace, Key: key, Val: n.Attributes[name]})
	}
	children := n.Children
	if n.TemplateContent != 0 {
		children = d.nodes[n.TemplateContent].Children
	}
	if len(children) > 0 {
		out.FirstChild = children[0]
		out.LastChild = children[len(children)-1]
	}
	if parent := d.nodes[n.Parent]; parent != nil {
		for i, child := range parent.Children {
			if child == id {
				if i > 0 {
					out.PrevSibling = parent.Children[i-1]
				}
				if i+1 < len(parent.Children) {
					out.NextSibling = parent.Children[i+1]
				}
				break
			}
		}
		if parent.TemplateHost != 0 {
			out.Parent = parent.TemplateHost
		}
	}
	return out
}
func (b streamBackend) parent(id int64) int64 {
	if n, ok := b.d.Get(id); ok && n.TemplateContent != 0 {
		return n.TemplateContent
	}
	return id
}
func (b streamBackend) InsertBefore(parent, child, before int64) {
	if err := b.d.InsertNode(b.parent(parent), child, before); err != nil {
		panic(err)
	}
	b.updateTitle(parent)
}
func (b streamBackend) RemoveChild(parent, child int64) {
	if err := b.d.RemoveNode(b.parent(parent), child); err != nil {
		panic(err)
	}
	b.updateTitle(parent)
}
func (b streamBackend) SetData(id int64, text string) {
	if err := b.d.SetTextContent(id, text); err != nil {
		panic(err)
	}
	if n, ok := b.d.Get(id); ok {
		b.updateTitle(n.Parent)
	}
}
func (b streamBackend) SetNamespace(id int64, namespace string) {
	d := b.d
	d.mu.Lock()
	defer d.mu.Unlock()
	d.markConnectedMutationLocked(id)
	d.nodes[id].Namespace = streamNamespace(namespace)
}
func (b streamBackend) SetAttributes(id int64, attrs []html.Attribute) {
	d := b.d
	d.mu.Lock()
	defer d.mu.Unlock()
	n := d.nodes[id]
	d.markConnectedMutationLocked(id)
	n.Attributes = map[string]string{}
	n.AttributeNames = nil
	n.AttributeNamespaces = nil
	for _, attr := range attrs {
		n.setParsedAttribute(attr)
	}
}
func (b streamBackend) updateTitle(id int64) {
	if n, ok := b.d.Get(id); ok && n.TagName == "TITLE" && n.Namespace == "http://www.w3.org/1999/xhtml" && b.d.IsConnected(id) {
		for _, title := range b.d.FindAllByTagName("title") {
			if title.Namespace == "http://www.w3.org/1999/xhtml" {
				if title.ID == id {
					b.d.SetTitle(b.d.TextContent(id))
				}
				break
			}
		}
	}
}
