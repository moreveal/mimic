package browser

type blitzControlValue struct {
	ID        uint64 `json:"id"`
	Value     string `json:"value"`
	Multiline bool   `json:"multiline"`
}

func (state *blitzDocument) syncControlValues(values []blitzControlValue, revision uint64) error {
	next := make(map[uint64]blitzControlValue, len(values))
	for _, value := range values {
		old, exists := state.controls[value.ID]
		// A canonical attribute transaction may have refreshed the native editor
		// from its default attribute. Reapply authoritative live state afterwards.
		if !exists || old != value || state.controlRevision != revision {
			if err := state.document.Owner.ControlValue(value.ID, value.Value, value.Multiline); err != nil {
				return err
			}
		}
		next[value.ID] = value
	}
	// Removed controls belong to detached native nodes or a different input type;
	// canonical reconciliation owns their lifecycle. Empty/reset values remain
	// explicit entries, so they repair the editor instead of retaining old text.
	state.controls = next
	state.controlRevision = revision
	return nil
}
