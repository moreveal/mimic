package htmlstream

import (
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type Tokenizer = html.Tokenizer
type Token = html.Token
type TokenType = html.TokenType
type Attribute = html.Attribute
type NodeType = html.NodeType

const (
	ErrorToken                   = html.ErrorToken
	TextToken                    = html.TextToken
	StartTagToken                = html.StartTagToken
	EndTagToken                  = html.EndTagToken
	SelfClosingTagToken          = html.SelfClosingTagToken
	CommentToken                 = html.CommentToken
	DoctypeToken                 = html.DoctypeToken
	DocumentNode                 = html.DocumentNode
	ElementNode                  = html.ElementNode
	TextNode                     = html.TextNode
	CommentNode                  = html.CommentNode
	DoctypeNode                  = html.DoctypeNode
	scopeMarkerNode     NodeType = 255
)

// NodeData is a short-lived projection of canonical DOM state. The parser
// keeps handles, not a parallel tree; every read below asks the owning DOM.
type NodeData struct {
	Type                                                    NodeType
	DataAtom                                                atom.Atom
	Data, Namespace                                         string
	Attr                                                    []Attribute
	Parent, FirstChild, LastChild, PrevSibling, NextSibling int64
}

type Backend interface {
	Create(NodeData) int64
	Get(int64) NodeData
	InsertBefore(parent, child, before int64)
	RemoveChild(parent, child int64)
	SetData(int64, string)
	SetNamespace(int64, string)
	SetAttributes(int64, []Attribute)
}

type tree struct {
	backend Backend
	handles map[int64]*Node
}
type Node struct {
	tree *tree
	id   int64
}

var scopeMarker Node

func (t *tree) node(id int64) *Node {
	if id == 0 {
		return nil
	}
	if n := t.handles[id]; n != nil {
		return n
	}
	n := &Node{tree: t, id: id}
	t.handles[id] = n
	return n
}
func (n *Node) data() NodeData {
	if n.tree == nil {
		return NodeData{Type: scopeMarkerNode}
	}
	return n.tree.backend.Get(n.id)
}
func (n *Node) Type() NodeType          { return n.data().Type }
func (n *Node) DataAtom() atom.Atom     { return n.data().DataAtom }
func (n *Node) Data() string            { return n.data().Data }
func (n *Node) Namespace() string       { return n.data().Namespace }
func (n *Node) Attr() []Attribute       { return n.data().Attr }
func (n *Node) Parent() *Node           { return n.tree.node(n.data().Parent) }
func (n *Node) FirstChild() *Node       { return n.tree.node(n.data().FirstChild) }
func (n *Node) LastChild() *Node        { return n.tree.node(n.data().LastChild) }
func (n *Node) PrevSibling() *Node      { return n.tree.node(n.data().PrevSibling) }
func (n *Node) NextSibling() *Node      { return n.tree.node(n.data().NextSibling) }
func (n *Node) AppendChild(child *Node) { n.tree.backend.InsertBefore(n.id, child.id, 0) }
func (n *Node) InsertBefore(child, before *Node) {
	id := int64(0)
	if before != nil {
		id = before.id
	}
	n.tree.backend.InsertBefore(n.id, child.id, id)
}
func (n *Node) RemoveChild(child *Node)       { n.tree.backend.RemoveChild(n.id, child.id) }
func (n *Node) setData(data string)           { n.tree.backend.SetData(n.id, data) }
func (n *Node) setNamespace(namespace string) { n.tree.backend.SetNamespace(n.id, namespace) }
func (n *Node) setAttr(attrs []Attribute)     { n.tree.backend.SetAttributes(n.id, attrs) }
func (n *Node) clone() *Node {
	data := n.data()
	data.Parent = 0
	data.FirstChild = 0
	data.LastChild = 0
	data.PrevSibling = 0
	data.NextSibling = 0
	return n.tree.node(n.tree.backend.Create(data))
}
func (p *parser) newNode(data NodeData) *Node { return p.tree.node(p.tree.backend.Create(data)) }
func reparentChildren(dst, src *Node) {
	for child := src.FirstChild(); child != nil; child = src.FirstChild() {
		src.RemoveChild(child)
		dst.AppendChild(child)
	}
}
