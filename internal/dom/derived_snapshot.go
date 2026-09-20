package dom

import "maps"

// DerivedSnapshot captures canonical input for a native producer under one
// read lock. Every map and slice in Nodes belongs to the snapshot. The native
// projection may run after releasing the arena lock without racing the parser.
type DerivedSnapshot struct {
	Revision uint64
	Root     int64
	Nodes    []Node
}

func (d *Document) DerivedSnapshot() DerivedSnapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()
	snapshot := DerivedSnapshot{Revision: d.mu.revision, Root: d.root}
	stack := []int64{d.root}
	for len(stack) != 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		node := *d.nodes[id]
		node.Attributes = maps.Clone(node.Attributes)
		node.AttributeNamespaces = maps.Clone(node.AttributeNamespaces)
		node.AttributeNames = append([]string(nil), node.AttributeNames...)
		node.Children = append([]int64(nil), node.Children...)
		snapshot.Nodes = append(snapshot.Nodes, node)
		for i := len(node.Children) - 1; i >= 0; i-- {
			stack = append(stack, node.Children[i])
		}
	}
	return snapshot
}
