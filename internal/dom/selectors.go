package dom

import "strings"

// This restricted legacy leaf matcher is used by internal native lookups and
// the validated simple-selector fast path. Web-platform selector parsing and
// all structural matching live in the shared mature JavaScript domain layer.
func (d *Document) Matches(id int64, selector string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.matchesSelector(d.nodes[id], selector)
}

func (d *Document) matchesSelector(n *Node, selector string) bool {
	selector = strings.TrimSpace(strings.Split(selector, ",")[0])
	if n == nil || n.Type != "element" || selector == "" {
		return false
	}
	if strings.ContainsAny(selector, " >+~:") {
		return false
	}
	if strings.HasPrefix(selector, "#") {
		return n.Attributes["id"] == selector[1:]
	}
	if strings.HasPrefix(selector, ".") {
		return hasClass(n.Attributes["class"], selector[1:])
	}
	if strings.HasPrefix(selector, "[") && strings.HasSuffix(selector, "]") {
		inside := strings.TrimSpace(selector[1 : len(selector)-1])
		name, value, hasValue := strings.Cut(inside, "=")
		actual, exists := n.Attributes[strings.ToLower(strings.TrimSpace(name))]
		if !hasValue {
			return exists
		}
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		return exists && actual == value
	}
	if dot := strings.IndexByte(selector, '.'); dot >= 0 {
		return strings.EqualFold(n.TagName, selector[:dot]) && hasClass(n.Attributes["class"], selector[dot+1:])
	}
	return selector == "*" || strings.EqualFold(n.TagName, selector)
}
