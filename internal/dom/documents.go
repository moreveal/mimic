package dom

// Inert documents share the Page's canonical node arena, but have independent
// roots and persistent ownership. A detached node keeps its document on removal;
// insertion adopts the entire subtree, without cloning or changing identity.
func (d *Document) ownerDocumentLocked(id int64) int64 {
	n := d.nodes[id]
	if n == nil {
		return 0
	}
	if n.Type == "document" {
		return id
	}
	if n.OwnerDocument != 0 {
		return n.OwnerDocument
	}
	return d.root
}

func (d *Document) OwnerDocumentID(id int64) int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	n := d.nodes[id]
	if n == nil || n.Type == "document" {
		return 0
	}
	return d.ownerDocumentLocked(id)
}

func (d *Document) adoptNodeLocked(id, owner int64) {
	n := d.nodes[id]
	if n == nil || n.Type == "document" {
		return
	}
	n.OwnerDocument = owner
	for _, child := range n.Children {
		d.adoptNodeLocked(child, owner)
	}
	if n.TemplateContent != 0 {
		d.adoptNodeLocked(n.TemplateContent, owner)
	}
}

func (d *Document) AdoptNode(id, owner int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.adoptNodeLocked(id, owner)
}

func (d *Document) CreateHTMLDocument(title *string) Node {
	d.mu.Lock()
	defer d.mu.Unlock()
	newNode := func(kind, tag string, parent int64) *Node {
		d.next++
		n := &Node{ID: d.next, Type: kind, TagName: tag, Parent: parent, Attributes: map[string]string{}}
		if kind == "element" {
			n.Namespace = "http://www.w3.org/1999/xhtml"
		}
		d.nodes[n.ID] = n
		if parent != 0 {
			d.nodes[parent].Children = append(d.nodes[parent].Children, n.ID)
			n.OwnerDocument = d.ownerDocumentLocked(parent)
		}
		return n
	}
	root := newNode("document", "", 0)
	newNode("doctype", "html", root.ID)
	html := newNode("element", "HTML", root.ID)
	head := newNode("element", "HEAD", html.ID)
	if title != nil {
		node := newNode("element", "TITLE", head.ID)
		newNode("text", "", node.ID).Text = *title
	}
	newNode("element", "BODY", html.ID)
	return *root
}
