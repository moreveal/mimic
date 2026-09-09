package browser

// updateSelectorTarget resolves a navigation fragment to a canonical element.
// It deliberately runs for fragment navigation, not history state URL changes.
// The caller supplies net/url's already-decoded Fragment.
func (r *Realm) updateSelectorTarget(fragment string) {
	r.selectorTargetID = 0
	if fragment == "" {
		return
	}
	nodes := r.document.FindAllIDs(0, "*")
	for _, id := range nodes {
		if value, ok := r.document.GetAttribute(id, "id"); ok && value == fragment {
			r.selectorTargetID = id
			return
		}
	}
	for _, id := range nodes {
		node, ok := r.document.Get(id)
		if ok && node.TagName == "A" {
			if value, ok := r.document.GetAttribute(id, "name"); ok && value == fragment {
				r.selectorTargetID = id
				return
			}
		}
	}
}
