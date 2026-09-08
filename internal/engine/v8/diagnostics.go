//go:build windows && amd64

package v8

import (
	"encoding/json"
	"os"
	"time"
)

type diagnosticCost struct {
	Count       uint64 `json:"count"`
	Nanoseconds int64  `json:"ns"`
}

type diagnosticState struct {
	Costs       map[string]diagnosticCost  `json:"costs"`
	Heaps       map[string]any             `json:"heaps"`
	CPUProfiles map[string]json.RawMessage `json:"cpu_profiles,omitempty"`
}

func newDiagnostics() *diagnosticState {
	if os.Getenv("MIMIC_DIAGNOSTICS") != "1" {
		return nil
	}
	return &diagnosticState{Costs: map[string]diagnosticCost{}, Heaps: map[string]any{}}
}

func (a *adapter) ProfileEnabled() bool { return a.profile != nil }

// ProfileCollect is an explicit diagnostic intervention, never benchmark policy.
func (a *adapter) ProfileCollect() error {
	_, err := a.owner.execute(func(s *state) response { return response{err: s.isolate.LowMemoryNotification()} })
	return err
}

func (a *adapter) ProfileHeapSnapshot(write func([]byte) bool) error {
	_, err := a.owner.execute(func(s *state) response { return response{err: s.isolate.TakeHeapSnapshot(write)} })
	return err
}

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
		if err != nil {
			return response{err: err}
		}
		var spaces []any
		count, err := s.isolate.NumberOfHeapSpaces()
		if err != nil {
			return response{err: err}
		}
		for i := int64(0); i < count; i++ {
			space, ok, err := s.isolate.GetHeapSpaceStatistics(uint64(i))
			if err != nil {
				return response{err: err}
			}
			if ok {
				spaces = append(spaces, space)
			}
		}
		return response{value: map[string]any{"heap": heap, "spaces": spaces, "host_crossings": a.callbackSeq, "persistent_handles": len(a.globals), "detail": a.profile}, err: err}
	})
}
