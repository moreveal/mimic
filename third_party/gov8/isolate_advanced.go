//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// CounterLookupCallback observes a V8 statistics-counter name. The callback
// runs synchronously on an engine thread and must not retain engine pointers or
// re-enter the isolate. Counter storage itself is owned by the shim and remains
// stable for the isolate lifetime.
type CounterLookupCallback func(name string)

// CreateParams is the safe Go counterpart of v8::CreateParams for the options
// characterized by the pinned oracle. Custom allocators, raw stack limits at
// isolate construction and snapshots are composed by dedicated APIs. A custom
// cppgc heap may be transferred exactly once with SetCppGCHeap.
type CreateParams struct {
	initialized             bool
	maxOldGeneration        uint64
	maxYoungGeneration      uint64
	codeRange               uint64
	initialOldGeneration    uint64
	initialYoungGeneration  uint64
	stackLimit              uintptr
	allowAtomicsWait        bool
	arrayBufferAllocatorSet bool
	arrayBufferAllocator    *ArrayBufferAllocator
	externalReferencesSet   bool
	externalReferences      []ExternalReference
	counterLookup           CounterLookupCallback
	cppGCHeap               *CppGCHeap
}

// SetCppGCHeap selects a custom cppgc heap for one future isolate. The heap
// is claimed immediately and ownership transfers when native construction
// accepts it; it cannot be reused by another CreateParams or as detached.
func (p *CreateParams) SetCppGCHeap(heap *CppGCHeap) error {
	if p == nil || heap == nil {
		return errors.New("gov8: nil CreateParams or cppgc heap")
	}
	p.initialize()
	if p.cppGCHeap != nil {
		return errors.New("gov8: CreateParams already has a cppgc heap")
	}
	heap.mu.Lock()
	defer heap.mu.Unlock()
	if err := heap.checkLocked(); err != nil {
		return err
	}
	if heap.claimed || heap.detachedEnabled || heap.terminated {
		return errors.New("gov8: cppgc heap is not transferable")
	}
	heap.claimed = true
	p.cppGCHeap = heap
	return nil
}

// NewCreateParams returns the Rust builder's defaults. The allocator flag is
// false before finalization and Atomics.wait is allowed.
func NewCreateParams() *CreateParams {
	return &CreateParams{initialized: true, allowAtomicsWait: true}
}

func (p *CreateParams) initialize() {
	if !p.initialized {
		p.initialized = true
		p.allowAtomicsWait = true
	}
}

func (p *CreateParams) SetMaxOldGenerationSizeInBytes(value uint64) *CreateParams {
	p.initialize()
	p.maxOldGeneration = value
	return p
}

func (p *CreateParams) SetMaxYoungGenerationSizeInBytes(value uint64) *CreateParams {
	p.initialize()
	p.maxYoungGeneration = value
	return p
}

func (p *CreateParams) SetCodeRangeSizeInBytes(value uint64) *CreateParams {
	p.initialize()
	p.codeRange = value
	return p
}

func (p *CreateParams) SetInitialOldGenerationSizeInBytes(value uint64) *CreateParams {
	p.initialize()
	p.initialOldGeneration = value
	return p
}

func (p *CreateParams) SetInitialYoungGenerationSizeInBytes(value uint64) *CreateParams {
	p.initialize()
	p.initialYoungGeneration = value
	return p
}

// SetStackLimit records a pointer for getter parity only. NewIsolateWithParams
// rejects a non-zero value because a Go-stack address is not a stable native
// stack boundary for an isolate lifetime.
func (p *CreateParams) SetStackLimit(value uintptr) *CreateParams {
	p.initialize()
	p.stackLimit = value
	return p
}

func (p *CreateParams) SetAllowAtomicsWait(value bool) *CreateParams {
	p.initialize()
	p.allowAtomicsWait = value
	return p
}

// UseDefaultArrayBufferAllocator configures the engine's default allocator.
// Arbitrary allocator callbacks are excluded because Go function/data pointers
// cannot safely implement V8's allocator lifetime contract.
func (p *CreateParams) UseDefaultArrayBufferAllocator() *CreateParams {
	p.initialize()
	p.arrayBufferAllocatorSet = true
	p.arrayBufferAllocator = nil
	return p
}

// SetArrayBufferAllocator configures a shared default or callback-backed
// allocator. The allocator must remain open until NewIsolateWithParams has
// copied its native shared reference; it may be closed immediately afterward.
func (p *CreateParams) SetArrayBufferAllocator(allocator *ArrayBufferAllocator) error {
	if p == nil || allocator == nil {
		return errors.New("gov8: nil CreateParams or ArrayBuffer allocator")
	}
	allocator.mu.Lock()
	live := !allocator.closed && allocator.handle != 0
	allocator.mu.Unlock()
	if !live {
		return errors.New("gov8: ArrayBuffer allocator used after Close")
	}
	p.initialize()
	p.arrayBufferAllocatorSet = true
	p.arrayBufferAllocator = allocator
	return nil
}

// UseEmptyExternalReferences installs a process-lifetime, null-terminated
// empty external-reference table. Non-empty raw address tables are excluded.
func (p *CreateParams) UseEmptyExternalReferences() *CreateParams {
	return p.SetExternalReferences(nil)
}

func (p *CreateParams) SetCounterLookupCallback(callback CounterLookupCallback) *CreateParams {
	p.initialize()
	p.counterLookup = callback
	return p
}

func (p *CreateParams) MaxOldGenerationSizeInBytes() uint64     { return p.maxOldGeneration }
func (p *CreateParams) MaxYoungGenerationSizeInBytes() uint64   { return p.maxYoungGeneration }
func (p *CreateParams) CodeRangeSizeInBytes() uint64            { return p.codeRange }
func (p *CreateParams) InitialOldGenerationSizeInBytes() uint64 { return p.initialOldGeneration }
func (p *CreateParams) InitialYoungGenerationSizeInBytes() uint64 {
	return p.initialYoungGeneration
}
func (p *CreateParams) StackLimit() uintptr { return p.stackLimit }
func (p *CreateParams) AllowAtomicsWait() bool {
	return !p.initialized || p.allowAtomicsWait
}
func (p *CreateParams) HasSetArrayBufferAllocator() bool { return p.arrayBufferAllocatorSet }
func (p *CreateParams) HasEmptyExternalReferences() bool {
	return p.externalReferencesSet && len(p.externalReferences) == 0
}

func (p *CreateParams) setDerived(values [5]uint64) {
	p.maxOldGeneration = values[0]
	p.maxYoungGeneration = values[1]
	p.codeRange = values[2]
	p.initialOldGeneration = values[3]
	p.initialYoungGeneration = values[4]
}

// ConfigureHeapLimits derives constraints from initial and maximum heap size.
func (p *CreateParams) ConfigureHeapLimits(initial, maximum uint64) error {
	p.initialize()
	var values [5]uint64
	r1, _, _ := proc("gov8_ia_derive_constraints").Call(
		0, uintptr(initial), uintptr(maximum), uintptr(unsafe.Pointer(&values[0])))
	if int64(r1) < 0 {
		return shimError("CreateParams.ConfigureHeapLimits", r1)
	}
	p.setDerived(values)
	return nil
}

// ConfigureHeapLimitsFromSystemMemory derives constraints from physical and
// virtual memory limits using V8's platform-exact formula.
func (p *CreateParams) ConfigureHeapLimitsFromSystemMemory(physical, virtual uint64) error {
	p.initialize()
	var values [5]uint64
	r1, _, _ := proc("gov8_ia_derive_constraints").Call(
		1, uintptr(physical), uintptr(virtual), uintptr(unsafe.Pointer(&values[0])))
	if int64(r1) < 0 {
		return shimError("CreateParams.ConfigureHeapLimitsFromSystemMemory", r1)
	}
	p.setDerived(values)
	return nil
}

var isolateCounterRegistry = struct {
	sync.Mutex
	next    uintptr
	entries map[uintptr]CounterLookupCallback
}{entries: make(map[uintptr]CounterLookupCallback)}

var (
	isolateCounterDispatcherOnce sync.Once
	isolateCounterDispatcherErr  error
)

var isolateCounterDispatcher = syscall.NewCallback(func(handle, name, length uintptr) (result uintptr) {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in counter lookup callback: %v\n", recovered)
			proc("gov8_host_panic_abort").Call()
		}
	}()
	isolateCounterRegistry.Lock()
	callback := isolateCounterRegistry.entries[handle]
	isolateCounterRegistry.Unlock()
	if callback != nil {
		callback(copyCLenString(name, length))
	}
	return 1
})

func registerIsolateCounter(callback CounterLookupCallback) (uintptr, error) {
	if callback == nil {
		return 0, nil
	}
	isolateCounterDispatcherOnce.Do(func() {
		isolateCounterDispatcherErr = callErr("CounterLookup.Dispatcher",
			proc("gov8_ia_set_counter_dispatcher"), isolateCounterDispatcher)
	})
	if isolateCounterDispatcherErr != nil {
		return 0, isolateCounterDispatcherErr
	}
	isolateCounterRegistry.Lock()
	isolateCounterRegistry.next++
	handle := isolateCounterRegistry.next
	isolateCounterRegistry.entries[handle] = callback
	isolateCounterRegistry.Unlock()
	return handle, nil
}

func dropIsolateCounter(handle uintptr) {
	if handle == 0 {
		return
	}
	isolateCounterRegistry.Lock()
	delete(isolateCounterRegistry.entries, handle)
	isolateCounterRegistry.Unlock()
}

// NewIsolateWithParams creates an entered, thread-affine isolate with the
// selected safe CreateParams surface.
func NewIsolateWithParams(params *CreateParams) (*Isolate, error) {
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	configuration := NewCreateParams()
	if params != nil {
		copy := *params
		if !copy.initialized {
			copy.initialize()
		}
		configuration = &copy
	}
	if configuration.stackLimit != 0 {
		return nil, errors.New("gov8: CreateParams stack limit cannot safely reference a Go stack")
	}
	var allocatorReference uintptr
	allocatorReferenceTransferred := false
	if configuration.arrayBufferAllocator != nil {
		var allocatorErr error
		allocatorReference, allocatorErr = configuration.arrayBufferAllocator.cloneHandle()
		if allocatorErr != nil {
			return nil, allocatorErr
		}
		defer func() {
			if !allocatorReferenceTransferred {
				_ = callErr("ArrayBufferAllocator.clone.Close", proc("gov8_aba_dispose"), allocatorReference)
			}
		}()
	}
	var cppHeap *CppGCHeap
	if configuration.cppGCHeap != nil {
		cppHeap = configuration.cppGCHeap
		cppHeap.mu.Lock()
		if err := cppHeap.checkLocked(); err != nil {
			cppHeap.mu.Unlock()
			return nil, err
		}
		if !cppHeap.claimed || cppHeap.detachedEnabled || cppHeap.terminated {
			cppHeap.mu.Unlock()
			return nil, errors.New("gov8: cppgc heap is not transferable")
		}
	}
	if cppHeap != nil {
		defer cppHeap.mu.Unlock()
	}
	counterHandle, err := registerIsolateCounter(configuration.counterLookup)
	if err != nil {
		return nil, err
	}
	runtime.LockOSThread()
	tid := currentThreadID()
	if err := beginIsolateCreate(); err != nil {
		dropIsolateCounter(counterHandle)
		runtime.UnlockOSThread()
		return nil, err
	}
	allowAtomics := uintptr(0)
	if configuration.allowAtomicsWait {
		allowAtomics = 1
	}
	referencesSet := uintptr(0)
	if configuration.externalReferencesSet {
		referencesSet = 1
	}
	referenceWords := externalReferenceWords(configuration.externalReferences)
	var referencePointer uintptr
	if len(referenceWords) != 0 {
		referencePointer = uintptr(unsafe.Pointer(&referenceWords[0]))
	}
	var cppHeapHandle uintptr
	if cppHeap != nil {
		cppHeapHandle = cppHeap.handle
	}
	var cppHeapConsumed int32
	var cppHeapIdentity uintptr
	handle, err := callHandle("Isolate.NewWithParams", proc("gov8_ia_isolate_new"),
		uintptr(configuration.maxOldGeneration), uintptr(configuration.maxYoungGeneration),
		uintptr(configuration.codeRange), uintptr(configuration.initialOldGeneration),
		uintptr(configuration.initialYoungGeneration), allowAtomics, referencesSet,
		referencePointer, uintptr(len(referenceWords)), counterHandle, cppHeapHandle,
		uintptr(unsafe.Pointer(&cppHeapConsumed)), uintptr(unsafe.Pointer(&cppHeapIdentity)),
		allocatorReference)
	runtime.KeepAlive(referenceWords)
	if cppHeapConsumed == 1 && cppHeap != nil {
		cppHeap.handle = 0
		cppHeap.identity = cppHeapIdentity
		cppHeap.transferred = true
		cppgcHeapLifecycle.Lock()
		delete(cppgcHeapLifecycle.heaps, cppHeap)
		cppgcHeapLifecycle.Unlock()
		runtime.UnlockOSThread()
	}
	if err != nil {
		abandonIsolateCreate()
		dropIsolateCounter(counterHandle)
		runtime.UnlockOSThread()
		return nil, err
	}
	isolate := &Isolate{handle: handle, tid: tid, advancedCounterHandle: counterHandle,
		advancedExternalReferences:    configuration.externalReferencesSet,
		customCppGCHeap:               cppHeapConsumed == 1,
		arrayBufferAllocatorReference: allocatorReference}
	allocatorReferenceTransferred = true
	finishIsolateCreate(isolate)
	return isolate, nil
}

// CounterValue returns the current engine-owned value for a named counter.
func (i *Isolate) CounterValue(name string) (value int32, found bool, err error) {
	if err = i.check(); err != nil {
		return
	}
	if i.advancedCounterHandle == 0 {
		return 0, false, nil
	}
	bytes := []byte(name)
	var foundInt int32
	r1, _, _ := proc("gov8_ia_counter_value").Call(
		i.handleAssumingCheck(), bytesArg(bytes), uintptr(len(bytes)),
		uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&foundInt)))
	runtime.KeepAlive(bytes)
	if int64(r1) < 0 {
		return 0, false, shimError("Isolate.CounterValue", r1)
	}
	return value, foundInt != 0, nil
}

// HeapSpaceStatistics is one V8 heap-space snapshot.
type HeapSpaceStatistics struct {
	Name          string
	Size          uint64
	UsedSize      uint64
	AvailableSize uint64
	PhysicalSize  uint64
}

// GetHeapSpaceStatistics returns ok=false for every out-of-range index without
// forwarding it to V8's size_t API.
func (i *Isolate) GetHeapSpaceStatistics(index uint64) (*HeapSpaceStatistics, bool, error) {
	if err := i.check(); err != nil {
		return nil, false, err
	}
	count, err := i.NumberOfHeapSpaces()
	if err != nil {
		return nil, false, err
	}
	if index >= uint64(count) {
		return nil, false, nil
	}
	var name [64]byte
	var nameLength int64
	var values [4]uint64
	var found int32
	r1, _, _ := proc("gov8_ia_heap_space_statistics").Call(
		i.handleAssumingCheck(), uintptr(index), uintptr(unsafe.Pointer(&name[0])),
		uintptr(len(name)), uintptr(unsafe.Pointer(&nameLength)),
		uintptr(unsafe.Pointer(&values[0])), uintptr(unsafe.Pointer(&found)))
	if int64(r1) < 0 {
		return nil, false, shimError("Isolate.GetHeapSpaceStatistics", r1)
	}
	if found == 0 {
		return nil, false, nil
	}
	if nameLength < 0 || nameLength > int64(len(name)) {
		return nil, false, errors.New("gov8: invalid heap space name length")
	}
	return &HeapSpaceStatistics{
		Name: string(name[:nameLength]), Size: values[0], UsedSize: values[1],
		AvailableSize: values[2], PhysicalSize: values[3],
	}, true, nil
}

// HeapCodeStatistics is V8's code/bytecode metadata snapshot.
type HeapCodeStatistics struct {
	CodeAndMetadataSize      uint64
	BytecodeAndMetadataSize  uint64
	ExternalScriptSourceSize uint64
	CPUProfilerMetadataSize  uint64
}

func (i *Isolate) GetHeapCodeAndMetadataStatistics() (*HeapCodeStatistics, bool, error) {
	if err := i.check(); err != nil {
		return nil, false, err
	}
	var values [4]uint64
	var available int32
	r1, _, _ := proc("gov8_ia_heap_code_statistics").Call(
		i.handleAssumingCheck(), uintptr(unsafe.Pointer(&values[0])),
		uintptr(unsafe.Pointer(&available)))
	if int64(r1) < 0 {
		return nil, false, shimError("Isolate.GetHeapCodeAndMetadataStatistics", r1)
	}
	if available == 0 {
		return nil, false, nil
	}
	return &HeapCodeStatistics{
		CodeAndMetadataSize: values[0], BytecodeAndMetadataSize: values[1],
		ExternalScriptSourceSize: values[2], CPUProfilerMetadataSize: values[3],
	}, true, nil
}

// HasCppHeap reports whether V8 attached its default cppgc heap.
func (i *Isolate) HasCppHeap() (bool, error) {
	if err := i.check(); err != nil {
		return false, err
	}
	var present int32
	r1, _, _ := proc("gov8_ia_has_cpp_heap").Call(
		i.handleAssumingCheck(), uintptr(unsafe.Pointer(&present)))
	if int64(r1) < 0 {
		return false, shimError("Isolate.HasCppHeap", r1)
	}
	return present != 0, nil
}

func (i *Isolate) UseDetailedSourcePositionsForProfiling() error {
	handle, err := i.handleChecked()
	if err != nil {
		return err
	}
	return callErr("Isolate.UseDetailedSourcePositionsForProfiling",
		proc("gov8_ia_profiler_controls"), handle, 0, 0)
}

// CollectCPUProfilerSample collects a sample; nil omits the trace identifier.
func (i *Isolate) CollectCPUProfilerSample(traceID *uint64) error {
	handle, err := i.handleChecked()
	if err != nil {
		return err
	}
	operation := uintptr(1)
	var value uint64
	if traceID != nil {
		operation = 2
		value = *traceID
	}
	return callErr("Isolate.CollectCPUProfilerSample",
		proc("gov8_ia_profiler_controls"), handle, operation, uintptr(value))
}
