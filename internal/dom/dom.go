package dom

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"sync"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type Node struct {
	ID            int64  `json:"nodeId"`
	Type          string `json:"type"`
	TagName       string `json:"tagName,omitempty"`
	Namespace     string `json:"namespaceURI,omitempty"`
	QualifiedName string `json:"qualifiedName,omitempty"`
	ContentType   string `json:"contentType,omitempty"`
	Text          string `json:"text,omitempty"`
	// TextJSON preserves DOMString code units that cannot cross a UTF-8 string
	// boundary (unpaired UTF-16 surrogates). Text is its scalar projection.
	TextJSON             string            `json:"-"`
	Attributes           map[string]string `json:"attributes,omitempty"`
	AttributeNamespaces  map[string]string `json:"attributeNamespaces,omitempty"`
	AttributeNames       []string          `json:"attributeNames,omitempty"`
	Parent               int64             `json:"parentId,omitempty"`
	Children             []int64           `json:"children,omitempty"`
	TemplateContent      int64             `json:"templateContent,omitempty"`
	TemplateHost         int64             `json:"templateHost,omitempty"`
	ScriptAlreadyStarted bool              `json:"-"`
	OwnerDocument        int64             `json:"ownerDocumentId,omitempty"`
}
type Document struct {
	mu               sync.RWMutex
	next             int64
	nodes            map[int64]*Node
	root             int64
	title, source    string
	hasFrameElements bool
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
			if n.Namespace != "" {
				node.QualifiedName = n.Data
			}
			for _, a := range n.Attr {
				node.setParsedAttribute(a)
			}
		case html.TextNode:
			node.Type = "text"
			node.Text = n.Data
		case html.CommentNode:
			node.Type, node.Text = "comment", n.Data
		case html.DoctypeNode:
			node.Type, node.TagName = "doctype", n.Data
			for _, a := range n.Attr {
				node.setAttribute(a.Key, a.Val)
			}
		default:
			node.Type = "other"
		}
		d.nodes[id] = node
		d.hasFrameElements = d.hasFrameElements || node.TagName == "IFRAME"
		if parent == 0 {
			d.root = id
		} else {
			d.nodes[parent].Children = append(d.nodes[parent].Children, id)
		}
		if node.TagName == "TITLE" && n.FirstChild != nil && d.isConnectedLocked(id) {
			d.title = n.FirstChild.Data
		}
		childParent := id
		if node.TagName == "TEMPLATE" && node.Namespace == "http://www.w3.org/1999/xhtml" {
			childParent = d.newTemplateContentLocked(node).ID
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, childParent)
		}
	}
	walk(root, 0)
	return d, nil
}
func (d *Document) Title() string     { d.mu.RLock(); defer d.mu.RUnlock(); return d.title }
func (d *Document) SetTitle(v string) { d.mu.Lock(); d.title = v; d.mu.Unlock() }
func (d *Document) Source() string    { return d.source }

// HasFrameElements is conservative: detached nodes remain reusable, so once an
// iframe has existed we keep insertion steps enabled for this document.
func (d *Document) HasFrameElements() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.hasFrameElements
}
func (d *Document) Root() Node { d.mu.RLock(); defer d.mu.RUnlock(); return *d.nodes[d.root] }
func (d *Document) Get(id int64) (Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n, ok := d.nodes[id]
	if !ok {
		return Node{}, false
	}
	copy := *n
	copy.AttributeNames = append([]string(nil), n.AttributeNames...)
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

// Contains follows canonical parent links without projecting ancestor records
// through the engine boundary.
func (d *Document) Contains(parent, child int64) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for child != 0 {
		if child == parent {
			return true
		}
		node := d.nodes[child]
		if node == nil {
			return false
		}
		child = node.Parent
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
	v, ok := n.Attributes[n.attributeName(name)]
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
	n.setAttribute(n.attributeName(name), value)
	return nil
}

// ToggleToken performs the existing DOMTokenList read/modify/write in one
// canonical operation, avoiding repeated serialization of the same attribute.
// force is -1 (toggle), 0 (remove), or 1 (add).
func (d *Document) ToggleToken(id int64, attribute, token string, force int) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := d.nodes[id]
	if n == nil || n.Type != "element" {
		return false, fmt.Errorf("element node %d does not exist", id)
	}
	attribute = strings.ToLower(attribute)
	tokens := strings.FieldsFunc(n.Attributes[attribute], tokenWhitespace)
	has := false
	for _, item := range tokens {
		if item == token {
			has = true
			break
		}
	}
	add := force == 1 || (!has && force != 0)
	if !add && !has {
		return false, nil
	}
	result := make([]string, 0, len(tokens)+1)
	seen := make(map[string]bool, len(tokens)+1)
	for _, item := range tokens {
		if !add && item == token {
			continue
		}
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}
	if add && !seen[token] {
		result = append(result, token)
	}
	if n.Attributes == nil {
		n.Attributes = map[string]string{}
	}
	n.setAttribute(attribute, strings.Join(result, " "))
	return add, nil
}

// Match the existing JavaScript /\s/ token parser, including BOM, rather
// than substituting Go's slightly different Unicode whitespace predicate.
func tokenWhitespace(r rune) bool {
	return (r >= '\t' && r <= '\r') || r == ' ' || r == 0x00a0 || r == 0x1680 ||
		(r >= 0x2000 && r <= 0x200a) || r == 0x2028 || r == 0x2029 || r == 0x202f ||
		r == 0x205f || r == 0x3000 || r == 0xfeff
}

func (d *Document) RemoveAttribute(id int64, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := d.nodes[id]
	if n == nil || n.Type != "element" {
		return fmt.Errorf("element node %d does not exist", id)
	}
	name = n.attributeName(name)
	delete(n.AttributeNamespaces, name)
	delete(n.Attributes, name)
	for i, item := range n.AttributeNames {
		if item == name {
			n.AttributeNames = append(n.AttributeNames[:i], n.AttributeNames[i+1:]...)
			break
		}
	}
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
		if d.matchesSelector(n, selector) {
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
		if d.matchesSelector(n, selector) {
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
			copy.AttributeNamespaces = cloneAttributes(found.AttributeNamespaces)
			copy.AttributeNames = append([]string(nil), found.AttributeNames...)
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
		if d.matchesSelector(n, selector) {
			copy := *n
			copy.Attributes = cloneAttributes(n.Attributes)
			copy.AttributeNamespaces = cloneAttributes(n.AttributeNamespaces)
			copy.AttributeNames = append([]string(nil), n.AttributeNames...)
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

// FindAllIDs snapshots membership in tree order without projecting attributes,
// text and child arrays that an existing realm wrapper already owns.
func (d *Document) FindAllIDs(parent int64, selector string) []int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var roots []int64
	if parent == 0 {
		roots = []int64{d.root}
	} else if n := d.nodes[parent]; n != nil {
		roots = n.Children
	}
	out := make([]int64, 0)
	var visit func(int64)
	visit = func(id int64) {
		n := d.nodes[id]
		if n == nil {
			return
		}
		if d.matchesSelector(n, selector) {
			out = append(out, id)
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

func hasClass(classes, wanted string) bool {
	for _, class := range strings.FieldsFunc(classes, func(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f' }) {
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
	var visit func(int64)
	visit = func(id int64) {
		n := d.nodes[id]
		if n == nil {
			return
		}
		if n.Type == "element" && (tag == "*" || n.TagName == tag) {
			copy := *n
			copy.Attributes = cloneAttributes(n.Attributes)
			copy.AttributeNamespaces = cloneAttributes(n.AttributeNamespaces)
			copy.AttributeNames = append([]string(nil), n.AttributeNames...)
			copy.Children = append([]int64(nil), n.Children...)
			out = append(out, copy)
		}
		for _, child := range n.Children {
			visit(child)
		}
	}
	visit(d.root)
	return out
}

func cloneAttributes(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Attribute values are indexed for lookup, while the ordered names preserve
// parser/insertion order for DOM enumeration and HTML serialization.
func (n *Node) setAttribute(name, value string) {
	if n.Attributes == nil {
		n.Attributes = map[string]string{}
	}
	if _, exists := n.Attributes[name]; !exists {
		n.AttributeNames = append(n.AttributeNames, name)
	}
	n.Attributes[name] = value
}
func (d *Document) AppendElement(parent int64, tag string, attrs map[string]string) (Node, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.nodes[parent] == nil {
		return Node{}, fmt.Errorf("parent node %d does not exist", parent)
	}
	d.next++
	n := &Node{ID: d.next, Type: "element", TagName: strings.ToUpper(tag), Attributes: attrs, Parent: parent}
	for name := range attrs {
		n.AttributeNames = append(n.AttributeNames, name)
	}
	sort.Strings(n.AttributeNames) // This internal map-based caller has no source order.
	d.nodes[n.ID] = n
	d.hasFrameElements = d.hasFrameElements || n.TagName == "IFRAME"
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
	if namespace != "http://www.w3.org/1999/xhtml" {
		n.QualifiedName = tag
	}
	d.nodes[n.ID] = n
	d.hasFrameElements = d.hasFrameElements || n.TagName == "IFRAME"
	if n.TagName == "TEMPLATE" && n.Namespace == "http://www.w3.org/1999/xhtml" {
		d.newTemplateContentLocked(n)
	}
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
	if d.hostIncludingContainsLocked(child, parent) {
		return fmt.Errorf("insertion would create a host-inclusive cycle")
	}
	if c.Parent != 0 {
		if old := d.nodes[c.Parent]; old != nil {
			old.Children = removeID(old.Children, child)
		}
	}
	c.Parent = parent
	d.adoptNodeLocked(child, d.ownerDocumentLocked(parent))
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
func (d *Document) ChildCount(id int64) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if n := d.nodes[id]; n != nil {
		return len(n.Children)
	}
	return 0
}
func (d *Document) ChildAt(id int64, index int) (Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	if n == nil || index < 0 || index >= len(n.Children) {
		return Node{}, false
	}
	child := d.nodes[n.Children[index]]
	if child == nil {
		return Node{}, false
	}
	return *child, true
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
			// Descendant text content excludes comments; querying a Comment's
			// own textContent above still returns its character data.
			if c := d.nodes[child]; c != nil && c.Type == "comment" {
				continue
			}
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
		n.TextJSON = ""
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
		text := &Node{ID: d.next, Type: "text", Text: value, Parent: id, OwnerDocument: d.ownerDocumentLocked(id), Attributes: map[string]string{}}
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
	children := n.Children
	if n.TemplateContent != 0 {
		children = d.nodes[n.TemplateContent].Children
	}
	for _, child := range children {
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
	case "document", "fragment":
		out.Type = html.DocumentNode
	case "doctype":
		out.Type, out.Data = html.DoctypeNode, n.TagName
		for _, key := range []string{"public", "system"} {
			if value, ok := n.Attributes[key]; ok {
				out.Attr = append(out.Attr, html.Attribute{Key: key, Val: value})
			}
		}
	case "element":
		out.Type, out.Data, out.DataAtom = html.ElementNode, strings.ToLower(n.TagName), atom.Lookup([]byte(strings.ToLower(n.TagName)))
		out.Namespace = parserNamespace(n.Namespace)
		if n.QualifiedName != "" {
			out.Data = n.QualifiedName
			out.DataAtom = atom.Lookup([]byte(out.Data))
		}
		for _, key := range n.AttributeNames {
			out.Attr = append(out.Attr, html.Attribute{Key: key, Val: n.Attributes[key]})
		}
	case "text":
		out.Type, out.Data = html.TextNode, n.Text
	case "comment":
		out.Type, out.Data = html.CommentNode, n.Text
	default:
		out.Type = html.CommentNode
	}
	children := n.Children
	if n.TemplateContent != 0 {
		children = d.nodes[n.TemplateContent].Children
	}
	for _, child := range children {
		if c := d.htmlNode(child); c != nil {
			out.AppendChild(c)
		}
	}
	return out
}

// OuterHTML serializes the current canonical tree, including detached nodes.
// Source is intentionally not consulted: it predates all script mutations.
func (d *Document) OuterHTML(id int64) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.nodes[id] == nil {
		return "", fmt.Errorf("node %d does not exist", id)
	}
	var out bytes.Buffer
	if err := html.Render(&out, d.htmlNode(id)); err != nil {
		return "", err
	}
	return out.String(), nil
}
func (d *Document) SetInnerHTML(id int64, source string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	parent := d.nodes[id]
	if parent == nil || parent.Type != "element" {
		return fmt.Errorf("element node %d does not exist", id)
	}
	contextName := strings.ToLower(parent.TagName)
	if parent.QualifiedName != "" {
		contextName = parent.QualifiedName
	}
	contextNode := &html.Node{Type: html.ElementNode, Data: contextName, DataAtom: atom.Lookup([]byte(contextName)), Namespace: parserNamespace(parent.Namespace)}
	fragments, err := html.ParseFragment(strings.NewReader(source), contextNode)
	if err != nil {
		return err
	}
	if parent.TemplateContent != 0 {
		parent = d.nodes[parent.TemplateContent]
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
		n := &Node{ID: d.next, Parent: parentID, OwnerDocument: d.ownerDocumentLocked(parentID), Attributes: map[string]string{}}
		switch raw.Type {
		case html.ElementNode:
			n.Type, n.TagName = "element", strings.ToUpper(raw.Data)
			n.ScriptAlreadyStarted = n.TagName == "SCRIPT"
			switch raw.Namespace {
			case "svg":
				n.Namespace = "http://www.w3.org/2000/svg"
			case "math":
				n.Namespace = "http://www.w3.org/1998/Math/MathML"
			default:
				n.Namespace = "http://www.w3.org/1999/xhtml"
			}
			if raw.Namespace != "" {
				n.QualifiedName = raw.Data
			}
			for _, attr := range raw.Attr {
				n.setParsedAttribute(attr)
			}
		case html.TextNode:
			n.Type, n.Text = "text", raw.Data
		case html.CommentNode:
			n.Type, n.Text = "comment", raw.Data
		default:
			n.Type = "other"
		}
		d.nodes[n.ID] = n
		d.hasFrameElements = d.hasFrameElements || n.TagName == "IFRAME"
		d.nodes[parentID].Children = append(d.nodes[parentID].Children, n.ID)
		childParent := n.ID
		if n.TagName == "TEMPLATE" && n.Namespace == "http://www.w3.org/1999/xhtml" {
			childParent = d.newTemplateContentLocked(n).ID
		}
		for child := raw.FirstChild; child != nil; child = child.NextSibling {
			add(child, childParent)
		}
	}
	for _, fragment := range fragments {
		add(fragment, parent.ID)
	}
	return nil
}
func (d *Document) Scripts() []Node {
	d.mu.RLock()
	defer d.mu.RUnlock()
	var out []Node
	for i := int64(1); i <= d.next; i++ {
		n := d.nodes[i]
		if n != nil && n.TagName == "SCRIPT" && d.isConnectedLocked(i) {
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
		if n != nil && n.TagName == "META" && d.isConnectedLocked(i) && strings.EqualFold(n.Attributes["http-equiv"], name) {
			out = append(out, n.Attributes["content"])
		}
	}
	return out
}
