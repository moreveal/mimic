//go:build windows && amd64

package v8

import (
	"os"
	"time"
)

type diagnosticCost struct {
	Count       uint64 `json:"count"`
	Nanoseconds int64  `json:"ns"`
}

type diagnosticState struct {
	Costs map[string]diagnosticCost `json:"costs"`
	Heaps map[string]any            `json:"heaps"`
}

func newDiagnostics() *diagnosticState {
	if os.Getenv("MIMIC_DIAGNOSTICS") != "1" {
		return nil
	}
	return &diagnosticState{Costs: map[string]diagnosticCost{}, Heaps: map[string]any{}}
}

func (a *adapter) ProfileEnabled() bool { return a.profile != nil }

func (a *adapter) recordCost(name string, start time.Time) {
	if a.profile == nil {
		return
	}
	cost := a.profile.Costs[name]
	cost.Count++
	cost.Nanoseconds += time.Since(start).Nanoseconds()
	a.profile.Costs[name] = cost
}

// Diagnostics samples the isolate on its owner thread. It is deliberately not
// part of the JavaScript surface or the engine-neutral execution contract.
func (a *adapter) Diagnostics() (any, error) {
	return a.owner.execute(func(s *state) response {
		heap, err := s.isolate.GetHeapStatistics()
		return response{value: map[string]any{"heap": heap, "host_crossings": a.callbackSeq, "persistent_handles": len(a.globals), "detail": a.profile}, err: err}
	})
}
