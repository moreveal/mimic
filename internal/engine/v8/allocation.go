//go:build (windows || linux) && amd64

package v8

import "github.com/moreveal/mimic/internal/engine"

func (a *adapter) SampleAllocations() (engine.AllocationSample, error) {
	value, err := a.owner.execute(func(s *state) response {
		heap, err := s.isolate.GetHeapStatistics()
		if err != nil {
			return response{err: err}
		}
		return response{value: engine.AllocationSample{UsedBytes: heap.UsedHeapSize + heap.ExternalMemory}}
	})
	if err != nil {
		return engine.AllocationSample{}, err
	}
	return value.(engine.AllocationSample), nil
}
