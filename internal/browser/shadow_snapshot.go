package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/moreveal/mimic/internal/dom"
)

// ShadowSnapshots reads attachment metadata at the Page command boundary.
// Canonical nodes remain owned by Document; no mutable tree is copied back.
func (r *Realm) ShadowSnapshots(ctx context.Context) ([]dom.ShadowSnapshot, error) {
	if r.shadowSnapshotCallback == nil {
		return nil, nil
	}
	value, err := r.runtime.Call(ctx, r.shadowSnapshotCallback, nil)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("shadow snapshot callback returned no result")
	}
	var roots []dom.ShadowSnapshot
	if err := json.Unmarshal([]byte(value.String()), &roots); err != nil {
		return nil, fmt.Errorf("invalid shadow snapshot: %w", err)
	}
	return roots, nil
}
