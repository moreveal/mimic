//go:build (windows || linux) && amd64

package v8

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type diagnosticCost struct {
	Count       uint64 `json:"count"`
	Nanoseconds int64  `json:"ns"`
}

type diagnosticState struct {
	mu          sync.Mutex                 `json:"-"`
	Detailed    bool                       `json:"detailed"`
	HostOnly    bool                       `json:"host_only,omitempty"`
	Costs       map[string]diagnosticCost  `json:"costs"`
	Heaps       map[string]any             `json:"heaps"`
	CPUProfiles map[string]json.RawMessage `json:"cpu_profiles,omitempty"`
}

func newDiagnostics() *diagnosticState {
	hostOnly := os.Getenv("MIMIC_PROFILE_HOSTS") == "1"
	if os.Getenv("MIMIC_DIAGNOSTICS") != "1" && !hostOnly {
		return nil
	}
	return &diagnosticState{HostOnly: hostOnly, Detailed: !hostOnly && os.Getenv("MIMIC_PROFILE_CONVERSIONS") == "1", Costs: map[string]diagnosticCost{}, Heaps: map[string]any{}}
}

func (a *adapter) ProfileEnabled() bool { return a.profile != nil && !a.profile.HostOnly }

// ProfileWorkloadCPU spans the caller's whole task, including Promise jobs and
// asynchronous continuations that an Eval-only native sample would miss.
func (a *adapter) ProfileWorkloadCPU() (func() (any, error), error) {
	var finish func() (json.RawMessage, error)
	_, err := a.owner.execute(func(s *state) response {
		var err error
		finish, err = startNativeProfile(s.isolate, s.realms[a.realm.id])
		return response{err: err}
	})
	if err != nil {
		return nil, err
	}
	return func() (any, error) {
		return a.owner.execute(func(*state) response {
			data, err := finish()
			return response{value: data, err: err}
		})
	}, nil
}

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
	a.profile.mu.Lock()
	defer a.profile.mu.Unlock()
	cost := a.profile.Costs[name]
	cost.Count++
	cost.Nanoseconds += time.Since(start).Nanoseconds()
	a.profile.Costs[name] = cost
}

// LiveDiagnostics returns counters that are safe to inspect while JavaScript
// owns the isolate thread. It intentionally excludes heap/isolate operations,
// which must still be dispatched to the owner actor.
func (a *adapter) LiveDiagnostics() any {
	if a.profile == nil {
		return map[string]any{"enabled": false}
	}
	a.profile.mu.Lock()
	costs := make(map[string]diagnosticCost, len(a.profile.Costs))
	for name, cost := range a.profile.Costs {
		costs[name] = cost
	}
	hostOnly := a.profile.HostOnly
	a.profile.mu.Unlock()
	return map[string]any{"enabled": true, "hostOnly": hostOnly, "costs": costs}
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
		threadCPU := diagnosticThreadCPU(a.owner.actorTID)
		return response{value: map[string]any{"processor_samples": a.processorSamples, "thread": threadCPU, "heap": heap, "spaces": spaces, "host_crossings": a.callbackSeq, "persistent_handles": len(a.globals), "detail": a.profile}, err: err}
	})
}
