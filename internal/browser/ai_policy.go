package browser

// Features such as AI and geolocation use the self allowlist by default.
// Intersect the existing header and iframe allow parsers at each ancestor,
// rather than reading only the requesting document's Permissions-Policy.
func (r *Realm) selfPolicyAllows(feature string) bool {
	if r.inactive || r.origin == "null" {
		return false
	}
	if list, declared := hintPolicy(r.securityState().permissionsPolicy, r.origin)[feature]; declared && !hintAllows(list, r.origin) {
		return false
	}
	frame, ok := r.agent.(*Frame)
	if !ok || frame.parent == nil || frame.parent.Realm == nil {
		return true
	}
	parent := frame.parent.Realm
	if !parent.selfPolicyAllows(feature) {
		return false
	}
	if list, declared := hintPolicy(parent.securityState().permissionsPolicy, parent.origin)[feature]; declared && !hintAllows(list, r.origin) {
		return false
	}
	if list, declared := parent.frameClientHintsContainer(frame, r.origin)[feature]; declared {
		return hintAllows(list, r.origin)
	}
	return r.origin == parent.origin
}
