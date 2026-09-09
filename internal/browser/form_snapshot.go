package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/moreveal/mimic/internal/dom"
)

func (r *Realm) FormSnapshots(ctx context.Context) ([]dom.FormSnapshot, error) {
	if r.formSnapshotCallback == nil {
		return nil, nil
	}
	value, err := r.runtime.Call(ctx, r.formSnapshotCallback, nil)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("form snapshot callback returned no result")
	}
	var forms []dom.FormSnapshot
	if err = json.Unmarshal([]byte(value.String()), &forms); err != nil {
		return nil, fmt.Errorf("invalid form snapshot: %w", err)
	}
	return forms, nil
}
