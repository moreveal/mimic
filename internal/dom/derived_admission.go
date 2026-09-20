package dom

// HasCodeUnitConnectedText checks native UTF-8 admission without copying the
// document or losing canonical DOMString code units. TextJSON can also encode
// ordinary scalar text: conservatively decline that representation until its
// adapter exists. Detached data is outside the producer's connected input.
func (d *Document) HasCodeUnitConnectedText() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	stack := []int64{d.root}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		node := d.nodes[id]
		if node.TextJSON != "" {
			return true
		}
		stack = append(stack, node.Children...)
	}
	return false
}
