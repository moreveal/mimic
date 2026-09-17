package layoutflat

import (
	"testing"

	"github.com/moreveal/mimic/internal/layouttaffy"
)

func TestStatePublishesImmutableRevisionSnapshot(t *testing.T) {
	state := new(State)
	request := layouttaffy.Request{Viewport: [2]float32{300, 200}, Nodes: []layouttaffy.Node{{ID: 1, Parent: -1, Display: 1, Width: layouttaffy.Length{Kind: 1, Value: 100}, Height: layouttaffy.Length{Kind: 1, Value: 20}}}}
	first, err := state.Publish(7, request)
	if err != nil {
		t.Fatal(err)
	}
	first.Boxes[1] = Box{ID: 1, Width: 999}
	if got := state.Snapshot(7).Boxes[1].Width; got == 999 {
		t.Fatal("caller mutation changed retained records")
	}
	if state.Snapshot(8) != nil {
		t.Fatal("snapshot crossed document revision")
	}
	state.Invalidate()
	if state.Snapshot(7) != nil {
		t.Fatal("invalidated snapshot remained visible")
	}
}
