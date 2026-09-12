package engine

// AllocationSample is an optional engine input to the browser's logical memory
// accounting. These are implementation measurements, NOT web-visible values.
// Browser policy removes bootstrap cost, buckets observations and derives a
// capacity from Environment. No Go/process statistics cross this boundary.
type AllocationSample struct{ UsedBytes uint64 }
type AllocationRuntime interface {
	SampleAllocations() (AllocationSample, error)
}
