package dom

// ElementByID returns the first descendant in tree order. IDs are literal
// DOMStrings, not CSS selectors; detached roots and duplicate IDs are supported.
func (d *Document) ElementByID(root int64, value string) int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if value == "" {
		return 0
	}
	var visit func(int64) int64
	visit = func(id int64) int64 {
		n := d.nodes[id]
		if n == nil {
			return 0
		}
		if n.Type == "element" && n.Attributes["id"] == value {
			return id
		}
		for _, child := range n.Children {
			if found := visit(child); found != 0 {
				return found
			}
		}
		return 0
	}
	if n := d.nodes[root]; n != nil {
		for _, child := range n.Children {
			if found := visit(child); found != 0 {
				return found
			}
		}
	}
	return 0
}

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

// CreateXMLDocument allocates an inert root in the same canonical arena.
// It owns nodes but never creates a Window, scheduler, or independent DOM store.
func (d *Document) CreateXMLDocument(namespace, name string) Node {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.next++
	root := &Node{ID: d.next, Type: "document", ContentType: "application/xml", Attributes: map[string]string{}}
	switch namespace {
	case "http://www.w3.org/1999/xhtml":
		root.ContentType = "application/xhtml+xml"
	case "http://www.w3.org/2000/svg":
		root.ContentType = "image/svg+xml"
	}
	d.nodes[root.ID] = root
	if name != "" {
		n := d.createDocumentElementLocked(root.ID, namespace, name)
		n.Parent = root.ID
		root.Children = []int64{n.ID}
	}
	return *root
}

func (d *Document) createDocumentElementLocked(owner int64, namespace, name string) *Node {
	d.next++
	n := &Node{ID: d.next, Type: "element", TagName: name, QualifiedName: name, Namespace: namespace, OwnerDocument: owner, Attributes: map[string]string{}}
	d.nodes[n.ID] = n
	return n
}

func (d *Document) CreateDocumentElement(owner int64, namespace, name string) Node {
	d.mu.Lock()
	defer d.mu.Unlock()
	return *d.createDocumentElementLocked(owner, namespace, name)
}

// WindowNamedElements reads the canonical connected tree in one traversal.
// It deliberately excludes detached nodes and shadow/template contents.
func (d *Document) WindowNamedElements(name string) []int64 {
	d.mu.RLock()
	defer d.mu.RUnlock()
	ids := []int64{}
	if name == "" {
		return ids
	}
	var visit func(int64)
	visit = func(id int64) {
		n := d.nodes[id]
		if n == nil {
			return
		}
		named := false
		if n.Namespace == "http://www.w3.org/1999/xhtml" {
			switch n.TagName {
			case "EMBED", "FORM", "IMG", "OBJECT", "IFRAME":
				named = n.Attributes["name"] == name
			}
		}
		if n.Type == "element" && (n.Attributes["id"] == name || named) {
			ids = append(ids, id)
		}
		for _, child := range n.Children {
			visit(child)
		}
	}
	visit(d.root)
	return ids
}
