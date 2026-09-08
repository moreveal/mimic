package dom

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type Node struct {
	ID         int64             `json:"nodeId"`
	Type       string            `json:"type"`
	TagName    string            `json:"tagName,omitempty"`
	Namespace  string            `json:"namespaceURI,omitempty"`
	Text       string            `json:"text,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Parent     int64             `json:"parentId,omitempty"`
	Children   []int64           `json:"children,omitempty"`
}
type Document struct {
	mu            sync.RWMutex
	next          int64
	nodes         map[int64]*Node
	root          int64
	title, source string
}

func Parse(source string) (*Document, error) {
	root, err := html.Parse(strings.NewReader(source))
	if err != nil {
		return nil, err
	}
	d := &Document{nodes: map[int64]*Node{}, source: source}
	var walk func(*html.Node, int64)
	walk = func(n *html.Node, parent int64) {
		d.next++
		id := d.next
		node := &Node{ID: id, Parent: parent, Attributes: map[string]string{}}
		switch n.Type {
		case html.DocumentNode:
			node.Type = "document"
		case html.ElementNode:
			node.Type = "element"
			node.TagName = strings.ToUpper(n.Data)
			switch n.Namespace {
			case "svg":
				node.Namespace = "http://www.w3.org/2000/svg"
			case "math":
				node.Namespace = "http://www.w3.org/1998/Math/MathML"
			default:
				node.Namespace = "http://www.w3.org/1999/xhtml"
			}
			for _, a := range n.Attr {
				node.Attributes[a.Key] = a.Val
			}
		case html.TextNode:
			node.Type = "text"
			node.Text = n.Data
		default:
			node.Type = "other"
		}
		d.nodes[id] = node
		if parent == 0 {
			d.root = id
		} else {
			d.nodes[parent].Children = append(d.nodes[parent].Children, id)
		}
		if node.TagName == "TITLE" && n.FirstChild != nil {
			d.title = n.FirstChild.Data
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, id)
		}
	}
	walk(root, 0)
	return d, nil
}
func (d *Document) Title() string     { d.mu.RLock(); defer d.mu.RUnlock(); return d.title }
func (d *Document) SetTitle(v string) { d.mu.Lock(); d.title = v; d.mu.Unlock() }
func (d *Document) Source() string    { return d.source }
func (d *Document) Root() Node        { d.mu.RLock(); defer d.mu.RUnlock(); return *d.nodes[d.root] }
func (d *Document) Get(id int64) (Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n, ok := d.nodes[id]
	if !ok {
		return Node{}, false
	}
	copy := *n
	copy.Children = append([]int64(nil), n.Children...)
	return copy, true
}
func (d *Document) IsConnected(id int64) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for id != 0 {
		if id == d.root {
			return true
		}
		node := d.nodes[id]
		if node == nil {
			return false
		}
		id = node.Parent
	}
	return false
}
func (d *Document) GetAttribute(id int64, name string) (string, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	if n == nil {
		return "", false
	}
	v, ok := n.Attributes[strings.ToLower(name)]
	return v, ok
}
func (d *Document) SetAttribute(id int64, name, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := d.nodes[id]
	if n == nil || n.Type != "element" {
		return fmt.Errorf("element node %d does not exist", id)
	}
	if n.Attributes == nil {
		n.Attributes = map[string]string{}
	}
	n.Attributes[strings.ToLower(name)] = value
	return nil
}
func (d *Document) RemoveAttribute(id int64, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := d.nodes[id]
	if n == nil || n.Type != "element" {
		return fmt.Errorf("element node %d does not exist", id)
	}
	delete(n.Attributes, strings.ToLower(name))
	return nil
}
func (d *Document) Find(selector string) (Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for i := int64(1); i <= d.next; i++ {
		n := d.nodes[i]
		if n == nil {
			continue
		}
		if matchesSelector(n, selector) {
			return *n, true
		}
	}
	return Node{}, false
}
func (d *Document) FindWithin(parent int64, selector string) (Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	root := d.nodes[parent]
	if root == nil {
		return Node{}, false
	}
	var visit func(int64) (*Node, bool)
	visit = func(id int64) (*Node, bool) {
		n := d.nodes[id]
		if n == nil {
			return nil, false
		}
		if matchesSelector(n, selector) {
			return n, true
		}
		for _, child := range n.Children {
			if found, ok := visit(child); ok {
				return found, true
			}
		}
		return nil, false
	}
	for _, child := range root.Children {
		if found, ok := visit(child); ok {
			copy := *found
			copy.Attributes = cloneAttributes(found.Attributes)
			copy.Children = append([]int64(nil), found.Children...)
			return copy, true
		}
	}
	return Node{}, false
}
func (d *Document) FindAll(selector string) []Node { return d.findAllWithin(0, selector) }
func (d *Document) FindAllWithin(parent int64, selector string) []Node {
	return d.findAllWithin(parent, selector)
}
func (d *Document) findAllWithin(parent int64, selector string) []Node {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var roots []int64
	if parent == 0 {
		roots = []int64{d.root}
	} else if n := d.nodes[parent]; n != nil {
		roots = append(roots, n.Children...)
	}
	var out []Node
	var visit func(int64)
	visit = func(id int64) {
		n := d.nodes[id]
		if n == nil {
			return
		}
		if matchesSelector(n, selector) {
			copy := *n
			copy.Attributes = cloneAttributes(n.Attributes)
			copy.Children = append([]int64(nil), n.Children...)
			out = append(out, copy)
		}
		for _, child := range n.Children {
			visit(child)
		}
	}
	for _, root := range roots {
		visit(root)
	}
	return out
}

func matchesSelector(n *Node, selector string) bool {
	selector = strings.TrimSpace(strings.Split(selector, ",")[0])
	if n == nil || n.Type != "element" || selector == "" {
		return false
	}
	if strings.ContainsAny(selector, " >+~:") {
		return false
	}
	if strings.HasPrefix(selector, "#") {
		return n.Attributes["id"] == selector[1:]
	}
	if strings.HasPrefix(selector, ".") {
		return hasClass(n.Attributes["class"], selector[1:])
	}
	if strings.HasPrefix(selector, "[") && strings.HasSuffix(selector, "]") {
		inside := strings.TrimSpace(selector[1 : len(selector)-1])
		name, value, hasValue := strings.Cut(inside, "=")
		actual, exists := n.Attributes[strings.ToLower(strings.TrimSpace(name))]
		if !hasValue {
			return exists
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		return exists && actual == value
	}
	if dot := strings.IndexByte(selector, '.'); dot >= 0 {
		return strings.EqualFold(n.TagName, selector[:dot]) && hasClass(n.Attributes["class"], selector[dot+1:])
	}
	return selector == "*" || strings.EqualFold(n.TagName, selector)
}
func hasClass(classes, wanted string) bool {
	for _, class := range strings.Fields(classes) {
		if class == wanted {
			return true
		}
	}
	return false
}
func (d *Document) FindAllByTagName(tag string) []Node {
	d.mu.RLock()
	defer d.mu.RUnlock()
	tag = strings.ToUpper(tag)
	out := make([]Node, 0)
	for i := int64(1); i <= d.next; i++ {
		n := d.nodes[i]
		if n == nil || n.Type != "element" || (tag != "*" && n.TagName != tag) {
			continue
		}
		copy := *n
		copy.Attributes = cloneAttributes(n.Attributes)
		copy.Children = append([]int64(nil), n.Children...)
		out = append(out, copy)
	}
	return out
}

func cloneAttributes(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func (d *Document) AppendElement(parent int64, tag string, attrs map[string]string) (Node, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.nodes[parent] == nil {
		return Node{}, fmt.Errorf("parent node %d does not exist", parent)
	}
	d.next++
	n := &Node{ID: d.next, Type: "element", TagName: strings.ToUpper(tag), Attributes: attrs, Parent: parent}
	d.nodes[n.ID] = n
	d.nodes[parent].Children = append(d.nodes[parent].Children, n.ID)
	return *n, nil
}
func (d *Document) CreateElement(tag string) Node {
	return d.CreateElementNS("http://www.w3.org/1999/xhtml", tag)
}
func (d *Document) CreateElementNS(namespace, tag string) Node {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.next++
	n := &Node{ID: d.next, Type: "element", TagName: strings.ToUpper(tag), Namespace: namespace, Attributes: map[string]string{}}
	d.nodes[n.ID] = n
	return *n
}
func (d *Document) CreateComment(data string) Node {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.next++
	n := &Node{ID: d.next, Type: "comment", Text: data, Attributes: map[string]string{}}
	d.nodes[n.ID] = n
	return *n
}
func (d *Document) CreateText(data string) Node {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.next++
	n := &Node{ID: d.next, Type: "text", Text: data, Attributes: map[string]string{}}
	d.nodes[n.ID] = n
	return *n
}
func (d *Document) InsertNode(parent, child, before int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	p, c := d.nodes[parent], d.nodes[child]
	if p == nil || c == nil {
		return fmt.Errorf("insert nodes do not exist: parent=%d child=%d", parent, child)
	}
	if c.Parent != 0 {
		if old := d.nodes[c.Parent]; old != nil {
			old.Children = removeID(old.Children, child)
		}
	}
	c.Parent = parent
	if before == 0 {
		p.Children = append(p.Children, child)
		return nil
	}
	for i, id := range p.Children {
		if id == before {
			p.Children = append(p.Children[:i], append([]int64{child}, p.Children[i:]...)...)
			return nil
		}
	}
	return fmt.Errorf("reference node %d is not a child of %d", before, parent)
}
func removeID(ids []int64, wanted int64) []int64 {
	for i, id := range ids {
		if id == wanted {
			return append(ids[:i], ids[i+1:]...)
		}
	}
	return ids
}
func (d *Document) FirstElementChild(id int64) (Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	if n == nil {
		return Node{}, false
	}
	for _, child := range n.Children {
		if c := d.nodes[child]; c != nil && c.Type == "element" {
			return *c, true
		}
	}
	return Node{}, false
}
func (d *Document) FirstChild(id int64) (Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	if n == nil || len(n.Children) == 0 || d.nodes[n.Children[0]] == nil {
		return Node{}, false
	}
	return *d.nodes[n.Children[0]], true
}
func (d *Document) Children(id int64) []Node {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	var out []Node
	if n != nil {
		for _, child := range n.Children {
			if c := d.nodes[child]; c != nil {
				out = append(out, *c)
			}
		}
	}
	return out
}
func (d *Document) Parent(id int64) (Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	if n == nil || n.Parent == 0 || d.nodes[n.Parent] == nil {
		return Node{}, false
	}
	return *d.nodes[n.Parent], true
}
func (d *Document) Sibling(id int64, offset int) (Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	if n == nil || n.Parent == 0 {
		return Node{}, false
	}
	p := d.nodes[n.Parent]
	for i, child := range p.Children {
		if child == id {
			index := i + offset
			if index >= 0 && index < len(p.Children) && d.nodes[p.Children[index]] != nil {
				return *d.nodes[p.Children[index]], true
			}
			break
		}
	}
	return Node{}, false
}
func (d *Document) ElementChildren(id int64) []Node {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	var out []Node
	if n != nil {
		for _, child := range n.Children {
			if c := d.nodes[child]; c != nil && c.Type == "element" {
				out = append(out, *c)
			}
		}
	}
	return out
}
func (d *Document) RemoveNode(parent, child int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	p, c := d.nodes[parent], d.nodes[child]
	if p == nil || c == nil || c.Parent != parent {
		return fmt.Errorf("node %d is not a child of %d", child, parent)
	}
	p.Children = removeID(p.Children, child)
	c.Parent = 0
	return nil
}
func (d *Document) TextContent(id int64) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var collect func(int64) string
	collect = func(nodeID int64) string {
		n := d.nodes[nodeID]
		if n == nil {
			return ""
		}
		if n.Type == "text" || n.Type == "comment" {
			return n.Text
		}
		var b strings.Builder
		for _, child := range n.Children {
			b.WriteString(collect(child))
		}
		return b.String()
	}
	return collect(id)
}
func (d *Document) SetTextContent(id int64, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := d.nodes[id]
	if n == nil {
		return fmt.Errorf("node %d does not exist", id)
	}
	if n.Type == "text" || n.Type == "comment" {
		n.Text = value
		return nil
	}
	for _, child := range n.Children {
		if detached := d.nodes[child]; detached != nil {
			detached.Parent = 0
		}
	}
	n.Children = nil
	if value != "" {
		d.next++
		text := &Node{ID: d.next, Type: "text", Text: value, Parent: id, Attributes: map[string]string{}}
		d.nodes[text.ID] = text
		n.Children = append(n.Children, text.ID)
	}
	return nil
}
func (d *Document) InnerHTML(id int64) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	if n == nil {
		return "", fmt.Errorf("node %d does not exist", id)
	}
	var b bytes.Buffer
	for _, child := range n.Children {
		if err := html.Render(&b, d.htmlNode(child)); err != nil {
			return "", err
		}
	}
	return b.String(), nil
}
func (d *Document) htmlNode(id int64) *html.Node {
	n := d.nodes[id]
	if n == nil {
		return nil
	}
	out := &html.Node{}
	switch n.Type {
	case "element":
		out.Type, out.Data, out.DataAtom = html.ElementNode, strings.ToLower(n.TagName), atom.Lookup([]byte(strings.ToLower(n.TagName)))
		for k, v := range n.Attributes {
			out.Attr = append(out.Attr, html.Attribute{Key: k, Val: v})
		}
	case "text":
		out.Type, out.Data = html.TextNode, n.Text
	case "comment":
		out.Type, out.Data = html.CommentNode, n.Text
	default:
		out.Type = html.CommentNode
	}
	for _, child := range n.Children {
		if c := d.htmlNode(child); c != nil {
			out.AppendChild(c)
		}
	}
	return out
}
func (d *Document) SetInnerHTML(id int64, source string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	parent := d.nodes[id]
	if parent == nil || parent.Type != "element" {
		return fmt.Errorf("element node %d does not exist", id)
	}
	contextNode := &html.Node{Type: html.ElementNode, Data: strings.ToLower(parent.TagName), DataAtom: atom.Lookup([]byte(strings.ToLower(parent.TagName)))}
	fragments, err := html.ParseFragment(strings.NewReader(source), contextNode)
	if err != nil {
		return err
	}
	for _, child := range parent.Children {
		if detached := d.nodes[child]; detached != nil {
			detached.Parent = 0
		}
	}
	parent.Children = nil
	var add func(*html.Node, int64)
	add = func(raw *html.Node, parentID int64) {
		d.next++
		n := &Node{ID: d.next, Parent: parentID, Attributes: map[string]string{}}
		switch raw.Type {
		case html.ElementNode:
			n.Type, n.TagName = "element", strings.ToUpper(raw.Data)
			for _, attr := range raw.Attr {
				n.Attributes[attr.Key] = attr.Val
			}
		case html.TextNode:
			n.Type, n.Text = "text", raw.Data
		case html.CommentNode:
			n.Type, n.Text = "comment", raw.Data
		default:
			n.Type = "other"
		}
		d.nodes[n.ID] = n
		d.nodes[parentID].Children = append(d.nodes[parentID].Children, n.ID)
		for child := raw.FirstChild; child != nil; child = child.NextSibling {
			add(child, n.ID)
		}
	}
	for _, fragment := range fragments {
		add(fragment, id)
	}
	return nil
}
func (d *Document) Scripts() []Node {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []Node
	for i := int64(1); i <= d.next; i++ {
		n := d.nodes[i]
		if n != nil && n.TagName == "SCRIPT" {
			copy := *n
			for _, cid := range n.Children {
				if c := d.nodes[cid]; c != nil && c.Type == "text" {
					copy.Text += c.Text
				}
			}
			out = append(out, copy)
		}
	}
	return out
}
func (d *Document) MetaHTTPEquiv(name string) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []string
	for i := int64(1); i <= d.next; i++ {
		n := d.nodes[i]
		if n != nil && n.TagName == "META" && strings.EqualFold(n.Attributes["http-equiv"], name) {
			out = append(out, n.Attributes["content"])
		}
	}
	return out
}
