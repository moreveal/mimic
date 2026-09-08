//go:build windows && amd64

package v8

// Diagnostics samples the isolate on its owner thread. It is deliberately not
// part of the JavaScript surface or the engine-neutral execution contract.
func (a *adapter) Diagnostics() (any, error) {
	return a.owner.execute(func(s *state) response {
		heap, err := s.isolate.GetHeapStatistics()
		return response{value: map[string]any{"heap": heap, "host_crossings": a.callbackSeq, "persistent_handles": len(a.globals)}, err: err}
	})
}
