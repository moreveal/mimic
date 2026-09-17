// Package layoutflat owns retained, document-scoped layout records.
//
// The package deliberately contains no DOM wrappers or JavaScript values. A
// producer supplies a flat formatting-tree input for one document revision;
// every consumer can then read the same immutable records without invoking a
// recursive geometry evaluator.
package layoutflat

import (
	"encoding/json"
	"sync"

	"github.com/moreveal/mimic/internal/layouttaffy"
)

type Box struct {
	ID            uint64
	X             float32
	Y             float32
	Width         float32
	Height        float32
	ContentWidth  float32
	ContentHeight float32
}

type Snapshot struct {
	Revision uint64
	Boxes    map[uint64]Box
}

type State struct {
	mu       sync.RWMutex
	snapshot *Snapshot
}

func (s *State) Snapshot(revision uint64) *Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.snapshot == nil || s.snapshot.Revision != revision {
		return nil
	}
	return cloneSnapshot(s.snapshot)
}

func (s *State) Publish(revision uint64, request layouttaffy.Request) (*Snapshot, error) {
	boxes, err := layouttaffy.Layout(request)
	if err != nil {
		return nil, err
	}
	result := &Snapshot{Revision: revision, Boxes: make(map[uint64]Box, len(boxes))}
	for _, box := range boxes {
		result.Boxes[box.ID] = Box{ID: box.ID, X: box.X, Y: box.Y, Width: box.Width, Height: box.Height, ContentWidth: box.ContentWidth, ContentHeight: box.ContentHeight}
	}
	s.mu.Lock()
	s.snapshot = result
	s.mu.Unlock()
	return cloneSnapshot(result), nil
}

func (s *State) PublishJSON(revision uint64, input string) (string, error) {
	var request layouttaffy.Request
	if err := json.Unmarshal([]byte(input), &request); err != nil {
		return "", err
	}
	boxes, err := layouttaffy.Layout(request)
	if err != nil {
		return "", err
	}
	result := &Snapshot{Revision: revision, Boxes: make(map[uint64]Box, len(boxes))}
	for _, box := range boxes {
		result.Boxes[box.ID] = Box{ID: box.ID, X: box.X, Y: box.Y, Width: box.Width, Height: box.Height, ContentWidth: box.ContentWidth, ContentHeight: box.ContentHeight}
	}
	s.mu.Lock()
	s.snapshot = result
	s.mu.Unlock()
	encoded, err := json.Marshal(struct {
		Boxes []layouttaffy.Box `json:"boxes,omitempty"`
	}{Boxes: boxes})
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (s *State) Invalidate() {
	s.mu.Lock()
	s.snapshot = nil
	s.mu.Unlock()
}

func cloneSnapshot(snapshot *Snapshot) *Snapshot {
	if snapshot == nil {
		return nil
	}
	boxes := make(map[uint64]Box, len(snapshot.Boxes))
	for id, box := range snapshot.Boxes {
		boxes[id] = box
	}
	return &Snapshot{Revision: snapshot.Revision, Boxes: boxes}
}
