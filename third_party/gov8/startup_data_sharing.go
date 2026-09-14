//go:build (windows || linux) && amd64

package gov8

import "errors"

// ShareImmutableBytes creates an independent consumer owner over the same
// read-only Go snapshot storage. Each owner still tracks and releases its own
// native isolate copies; releasing either owner cannot retire the other.
// Bytes, as with all StartupData byte views, must never be mutated.
func (s *StartupData) ShareImmutableBytes() (*StartupData, error) {
	if s == nil {
		return nil, errors.New("gov8: nil startup blob")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.released {
		return nil, errors.New("gov8: startup blob has been released")
	}
	if len(s.bytes) == 0 {
		return nil, errors.New("gov8: startup blob is empty")
	}
	return &StartupData{
		bytes:                      s.bytes,
		requiresExternalReferences: s.requiresExternalReferences,
		externalReferencesKnown:    s.externalReferencesKnown,
	}, nil
}
