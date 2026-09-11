package dom

func (d *Document) CreateDocumentFragment() Node {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.next++
	node := &Node{ID: d.next, Type: "fragment", OwnerDocument: d.root, Attributes: map[string]string{}}
	d.nodes[node.ID] = node
	return *node
}

// SharesNodeArena reports whether node IDs belong to the same ownership domain.
func (d *Document) SharesNodeArena(other *Document) bool { return d.nodeArena == other.nodeArena }

// ShareNodeArena joins a newly parsed, unpublished document to a Page's node
// arena. IDs are remapped before any JS wrapper or parser can retain them.
// Documents keep independent roots and queries; moved nodes retain one identity.
// The caller owns both documents on the Page event loop during this operation.
func (d *Document) ShareNodeArena(other *Document) {
	if d == other || d.nodeArena == other.nodeArena {
		return
	}
	other.mu.Lock()
	defer other.mu.Unlock()
	oldRoot := d.root
	ids := make(map[int64]int64, len(d.nodes))
	for id := int64(1); id <= d.next; id++ {
		if d.nodes[id] != nil {
			other.next++
			ids[id] = other.next
		}
	}
	for old, node := range d.nodes {
		node.ID = ids[old]
		node.Parent = ids[node.Parent]
		for index, child := range node.Children {
			node.Children[index] = ids[child]
		}
		node.TemplateContent = ids[node.TemplateContent]
		node.TemplateHost = ids[node.TemplateHost]
		owner := node.OwnerDocument
		if owner == 0 && node.Type != "document" {
			owner = oldRoot
		}
		node.OwnerDocument = ids[owner]
		other.nodes[node.ID] = node
	}
	other.hasFrameElements = other.hasFrameElements || d.hasFrameElements
	d.root = ids[oldRoot]
	d.nodeArena = other.nodeArena
}
