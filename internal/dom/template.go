package dom

// Template contents are ordinary canonical fragment nodes. Their host link is
// separate from Parent: queries, connectivity and text traversal must not cross
// it, but insertion cycle validation and HTML serialization must account for it.
func (d *Document) newTemplateContentLocked(template *Node) *Node {
	if template.TemplateContent != 0 {
		return d.nodes[template.TemplateContent]
	}
	d.next++
	fragment := &Node{ID: d.next, Type: "fragment", TemplateHost: template.ID, OwnerDocument: d.ownerDocumentLocked(template.ID)}
	d.nodes[fragment.ID] = fragment
	template.TemplateContent = fragment.ID
	return fragment
}

func (d *Document) TemplateContent(id int64) (Node, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	template := d.nodes[id]
	if template == nil || template.TemplateContent == 0 {
		return Node{}, false
	}
	fragment := *d.nodes[template.TemplateContent]
	fragment.Children = append([]int64(nil), fragment.Children...)
	return fragment, true
}

func (d *Document) isConnectedLocked(id int64) bool {
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

func (d *Document) hostIncludingContainsLocked(parent, child int64) bool {
	for child != 0 {
		if child == parent {
			return true
		}
		node := d.nodes[child]
		if node == nil {
			return false
		}
		if node.Parent != 0 {
			child = node.Parent
		} else {
			child = node.TemplateHost
		}
	}
	return false
}

func (d *Document) HostIncludingContains(parent, child int64) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.hostIncludingContainsLocked(parent, child)
}

// CopyNodeState applies non-attribute clone state without copying listeners or
// introducing another tree. Fragment-parser scripts retain their inert flag.
func (d *Document) CopyNodeState(source, target int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	from, to := d.nodes[source], d.nodes[target]
	if from != nil && to != nil {
		to.ScriptAlreadyStarted = from.ScriptAlreadyStarted
		to.Nonce = from.Nonce
	}
}

func (d *Document) ScriptStarted(id int64) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	node := d.nodes[id]
	return node != nil && node.ScriptAlreadyStarted
}

// Script async state is shared by all worlds wrapping the same canonical node.
func (d *Document) ClearScriptForceAsync(id int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if node := d.nodes[id]; node != nil && node.TagName == "SCRIPT" {
		node.ScriptForceAsync = false
	}
}

func (d *Document) MarkScriptStarted(id int64) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if node := d.nodes[id]; node != nil {
		node.ScriptAlreadyStarted = true
	}
}
