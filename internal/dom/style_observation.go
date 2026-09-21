package dom

import "encoding/json"

// StyleObservationState is a read-only projection for one lazy style graph.
// Attribute/parent/precise inline facts are copied together under the canonical
// arena lock. No second mutable DOM or cross-Page cache is maintained here.
func (d *Document) StyleObservationState(id int64) string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	state := struct {
		Parent        int64             `json:"parent"`
		Children      []int64           `json:"children"`
		OwnerDocument int64             `json:"ownerDocument"`
		Connected     bool              `json:"connected"`
		Attributes    map[string]string `json:"attributes"`
		Inline        string            `json:"inline"`
	}{Children: []int64{}, Attributes: map[string]string{}, Inline: "s"}
	if n := d.nodes[id]; n != nil {
		state.Parent = n.Parent
		state.Children = append(state.Children, n.Children...)
		state.OwnerDocument = d.ownerDocumentLocked(id)
		state.Connected = d.isConnectedLocked(id)
		if n.Attributes != nil {
			state.Attributes = n.Attributes
		}
		if n.StyleDeclarationsJSON != "" {
			state.Inline = "j" + n.StyleDeclarationsJSON
		} else {
			state.Inline = "s" + n.Attributes["style"]
		}
	}
	// The projection contains only integers and strings; encoding cannot fail.
	data, _ := json.Marshal(state)
	return string(data)
}
