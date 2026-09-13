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

// syncShadowSnapshots refreshes an isolated world's realm-local ShadowRoot
// wrappers from the main world's canonical node identities. It runs between
// debugger calls, never from a DOM getter or mutation callback, so it cannot
// create a main-world -> isolated-world -> main-world reentry cycle.
func (r *Realm) syncShadowSnapshots(ctx context.Context) error {
	if r.mainWorld == nil || r.shadowSnapshotRestore == nil || r.mainWorld.shadowSnapshotVersion == nil {
		return nil
	}
	var roots []dom.ShadowSnapshot
	var revision string
	if err := r.mainWorld.runOnOwner(ctx, func(ctx context.Context) error {
		value, err := r.mainWorld.runtime.Call(ctx, r.mainWorld.shadowSnapshotVersion, nil)
		if err != nil {
			return err
		}
		defer releaseDebuggerValue(r.mainWorld, value)
		if value != nil {
			revision = value.String()
		}
		if revision == r.shadowSnapshotRevision {
			return nil
		}
		var snapshotErr error
		roots, snapshotErr = r.mainWorld.ShadowSnapshots(ctx)
		return snapshotErr
	}); err != nil {
		return err
	}
	if revision == r.shadowSnapshotRevision {
		return nil
	}
	if len(roots) == 0 {
		r.shadowSnapshotRevision = revision
		return nil
	}
	encoded, err := json.Marshal(roots)
	if err != nil {
		return err
	}
	err = r.runOnOwner(ctx, func(ctx context.Context) error {
		value := r.val(string(encoded))
		defer releaseDebuggerValue(r, value)
		result, err := r.runtime.Call(ctx, r.shadowSnapshotRestore, nil, value)
		releaseDebuggerValue(r, result)
		return err
	})
	if err == nil {
		r.shadowSnapshotRevision = revision
	}
	return err
}
