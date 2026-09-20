package layoutblitz

import (
	"fmt"
	"github.com/moreveal/mimic/internal/dom"
	"slices"
)

// Document retains one native projection between observable requests. The Go
// DOM is authoritative. Reconciliation updates existing native nodes instead
// of recreating the style/layout producer on every mutation.
type Document struct {
	BaseURL                 string
	baseURL                 string
	Owner                   *Owner
	source                  *dom.Document
	revision                uint64
	nodes                   map[int64]dom.Node
	retired                 map[int64]dom.Node
	width, height           uint32
	detached                int
	retiredBytes            int64
	Builds, Updates, Reuses uint64
}

func (d *Document) Close() {
	if d.Owner != nil {
		d.Owner.Close()
	}
	d.Owner = nil
	d.source = nil
	d.nodes = nil
	d.retired = nil
	d.detached = 0
	d.retiredBytes = 0
}

// This bounds retained canonical input estimates, not total native heap use.
// Count separately bounds tiny nodes; bytes bound large text/attribute payloads.
const retiredInputByteLimit int64 = 16 << 20

func retiredInputBytes(node dom.Node) int64 {
	bytes := int64(512 + len(node.Children)*8)
	// Count string payload twice to cover canonical snapshots plus the native
	// projection. Fixed node/map metadata is deliberately conservative.
	bytes += 2 * int64(len(node.Text)+len(node.TextJSON)+len(node.StyleDeclarationsJSON)+len(node.TagName)+len(node.Namespace))
	for key, value := range node.Attributes {
		bytes += 128 + 2*int64(len(key)+len(value)+len(node.AttributeNamespaces[key]))
	}
	for _, name := range node.AttributeNames {
		bytes += 32 + 2*int64(len(name))
	}
	return bytes
}

func (d *Document) rebuild(snapshot dom.DerivedSnapshot, source *dom.Document, width, height uint32) error {
	baseURL := d.BaseURL
	if baseURL == "" {
		baseURL = "about:blank"
	}
	owner, err := FromSnapshotAtURL(snapshot, width, height, baseURL)
	if err != nil {
		return err
	}
	d.Close()
	d.Owner, d.baseURL, d.source = owner, baseURL, source
	d.nodes = make(map[int64]dom.Node, len(snapshot.Nodes))
	d.retired = make(map[int64]dom.Node)
	d.Builds++
	return nil
}

func (d *Document) Sync(source *dom.Document, width, height uint32) error {
	// Update the URL before parsing new inline declarations. Existing parsed
	// sheets keep their own URL context; a history/base transition is not a
	// reason to reparse unchanged CSS against a different URL.
	baseURL := d.BaseURL
	if baseURL == "" {
		baseURL = "about:blank"
	}
	if d.Owner != nil && baseURL != d.baseURL {
		if err := d.Owner.BaseURL(baseURL); err != nil {
			return err
		}
		d.baseURL = baseURL
	}
	if d.Owner != nil && d.source == source && source.Revision() == d.revision && d.width == width && d.height == height {
		d.Reuses++
		return nil
	}
	snapshot := source.DerivedSnapshot()
	if d.Owner == nil || d.source != source {
		if err := d.rebuild(snapshot, source, width, height); err != nil {
			return err
		}
	} else {
		if err := d.reconcile(snapshot); err != nil {
			d.Close()
			return err
		}
		// Evict in this transaction. Waiting until the next changed observation
		// allows the clean-reuse fast path to retain oversized inputs forever.
		if d.detached > len(snapshot.Nodes)+1024 || d.retiredBytes > retiredInputByteLimit {
			if err := d.rebuild(snapshot, source, width, height); err != nil {
				d.Close()
				return err
			}
		}
		if d.width != width || d.height != height {
			if err := d.Owner.Viewport(width, height); err != nil {
				d.Close()
				return err
			}
		}
		d.Updates++
	}
	for _, node := range snapshot.Nodes {
		d.nodes[node.ID] = node
	}
	d.width = width
	d.height = height
	d.revision = snapshot.Revision
	return nil
}

func (d *Document) reconcile(snapshot dom.DerivedSnapshot) error {
	next := make(map[int64]dom.Node, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		next[node.ID] = node
	}
	// Detach moved and removed nodes before attaching any new topology. This
	// handles moving a descendant above its previous ancestor without cycles.
	for id, previous := range d.nodes {
		if id == snapshot.Root {
			continue
		}
		node, exists := next[id]
		if !exists || node.Parent != previous.Parent {
			if err := d.Owner.Detach(uint64(id)); err != nil {
				return err
			}
		}
	}
	for _, node := range snapshot.Nodes {
		if node.ID == snapshot.Root {
			continue
		}
		previous, exists := d.nodes[node.ID]
		if !exists {
			previous, exists = d.retired[node.ID]
			if exists {
				d.retiredBytes -= retiredInputBytes(previous)
			}
			delete(d.retired, node.ID)
		}
		if !exists {
			var err error
			switch node.Type {
			case "element":
				name := canonicalLocalName(node)
				err = d.Owner.Element(uint64(node.ID), node.Namespace, name)
			case "text":
				err = d.Owner.Text(uint64(node.ID), node.Text, true)
			case "comment", "doctype":
				err = d.Owner.Comment(uint64(node.ID), node.Text)
			default:
				return fmt.Errorf("blitz: unsupported canonical kind %q", node.Type)
			}
			if err != nil {
				return err
			}
		}
		if node.TextJSON != "" {
			return fmt.Errorf("blitz: UTF-16 code-unit adapter required")
		}
		if exists && node.Type == "text" && previous.Text != node.Text {
			if err := d.Owner.Text(uint64(node.ID), node.Text, false); err != nil {
				return err
			}
		}
		for name := range previous.Attributes {
			if _, present := node.Attributes[name]; !present {
				if err := d.Owner.ClearAttribute(uint64(node.ID), previous.AttributeNamespaces[name], name); err != nil {
					return err
				}
			}
		}
		for _, name := range node.AttributeNames {
			old, present := previous.Attributes[name]
			if !present || old != node.Attributes[name] || previous.AttributeNamespaces[name] != node.AttributeNamespaces[name] {
				if err := d.Owner.Attribute(uint64(node.ID), node.AttributeNamespaces[name], name, node.Attributes[name]); err != nil {
					return err
				}
			}
		}
		if node.Type == "element" && node.StyleDeclarationsJSON != previous.StyleDeclarationsJSON {
			if err := d.Owner.inlineDeclarations(node); err != nil {
				return err
			}
		}
	}
	for _, node := range snapshot.Nodes {
		if slices.Equal(d.nodes[node.ID].Children, node.Children) {
			continue
		}
		// Append in final canonical order; Blitz reuses node identity on moves.
		for _, child := range node.Children {
			if err := d.Owner.Append(uint64(node.ID), uint64(child)); err != nil {
				return err
			}
		}
	}
	for id, previous := range d.nodes {
		if _, exists := next[id]; !exists {
			d.retired[id] = previous
			d.retiredBytes += retiredInputBytes(previous)
			delete(d.nodes, id)
		}
	}
	d.detached = len(d.retired)
	return nil
}
