//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"unsafe"
)

// SnapshotCreateParams composes the existing safe CreateParams surface with
// an owned reference to a StartupData blob. It is the Go counterpart of a
// rusty_v8 CreateParams after snapshot_blob has been selected.
//
// The wrapper is single-use: NewIsolateWithSnapshotParams consumes it before
// validating and entering the native constructor, so every returned error
// after that call still consumes the wrapper. StartupData itself remains
// caller-owned and reusable; the shim copies its bytes into isolate-lifetime
// native storage.
//
// The embedded CreateParams preserves its existing fluent API, but its direct
// setters are not synchronized and are not frozen after Consumed reports true.
// Configure a SnapshotCreateParams from one goroutine before handing it to the
// constructor; do not mutate it concurrently with construction. Post-consume
// getter changes do not permit a second isolate creation.
type SnapshotCreateParams struct {
	*CreateParams

	mu       sync.Mutex
	snapshot *StartupData
	consumed bool
}

// NewSnapshotCreateParams returns default CreateParams carrying snapshot.
// Empty, truncated, or version-incompatible blobs are rejected before V8's
// fatal snapshot-version boundary.
func NewSnapshotCreateParams(snapshot *StartupData) (*SnapshotCreateParams, error) {
	p := &SnapshotCreateParams{CreateParams: NewCreateParams()}
	if err := p.SetSnapshotBlob(snapshot); err != nil {
		return nil, err
	}
	return p, nil
}

// Clone returns an independent Go-owned copy of the startup bytes and their
// known external-reference requirement. It mirrors StartupData::clone in the
// pinned crate: releasing the original does not affect consumers of the copy.
// A released StartupData cannot be cloned because Release marks the Go owner
// as retired even though its diagnostic Bytes view remains available.
func (s *StartupData) Clone() (*StartupData, error) {
	if s == nil {
		return nil, errors.New("gov8: nil startup blob")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.released {
		return nil, errors.New("gov8: startup blob has been released")
	}
	if len(s.bytes) == 0 {
		return nil, errors.New("gov8: startup blob is empty")
	}
	bytes := make([]byte, len(s.bytes))
	copy(bytes, s.bytes)
	return &StartupData{
		bytes:                      bytes,
		requiresExternalReferences: s.requiresExternalReferences,
		externalReferencesKnown:    s.externalReferencesKnown,
	}, nil
}

// SetSnapshotBlob replaces the currently selected blob, matching repeated
// rusty_v8 snapshot_blob builder calls. The previous blob remains owned by
// its caller and may be reused independently.
func (p *SnapshotCreateParams) SetSnapshotBlob(snapshot *StartupData) error {
	if p == nil {
		return errors.New("gov8: nil snapshot CreateParams")
	}
	if snapshot == nil {
		return errors.New("gov8: startup blob is nil")
	}
	snapshot.mu.Lock()
	if snapshot.released {
		snapshot.mu.Unlock()
		return errors.New("gov8: startup blob has been released")
	}
	err := snapshot.validForCreation()
	snapshot.mu.Unlock()
	if err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.consumed {
		return errors.New("gov8: snapshot CreateParams already consumed")
	}
	p.snapshot = snapshot
	return nil
}

// ConfigureHeapLimits derives V8 constraints from initial and maximum heap
// sizes. Unlike the pinned builder's fatal inverted pair, Go rejects
// initial>maximum before entering V8. Direct individual constraint setters
// remain available through the embedded CreateParams and intentionally retain
// their permissive round-trip behavior.
func (p *SnapshotCreateParams) ConfigureHeapLimits(initial, maximum uint64) error {
	if p == nil || p.CreateParams == nil {
		return errors.New("gov8: nil snapshot CreateParams")
	}
	if initial > maximum {
		return fmt.Errorf("gov8: initial heap limit %d exceeds maximum %d", initial, maximum)
	}
	p.mu.Lock()
	consumed := p.consumed
	p.mu.Unlock()
	if consumed {
		return errors.New("gov8: snapshot CreateParams already consumed")
	}
	return p.CreateParams.ConfigureHeapLimits(initial, maximum)
}

// Consumed reports whether NewIsolateWithSnapshotParams has consumed p.
func (p *SnapshotCreateParams) Consumed() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.consumed
}

// NewIsolateWithSnapshotParams creates an entered, thread-affine isolate from
// the snapshot while applying every safe CreateParams field supported by
// NewIsolateWithParams: resource constraints, Atomics.wait, shared
// ArrayBuffer allocators, custom cppgc heaps, external references, and counter
// lookup.
func NewIsolateWithSnapshotParams(params *SnapshotCreateParams) (*Isolate, error) {
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	if params == nil || params.CreateParams == nil {
		return nil, errors.New("gov8: snapshot CreateParams are nil")
	}
	params.mu.Lock()
	if params.consumed {
		params.mu.Unlock()
		return nil, errors.New("gov8: snapshot CreateParams already consumed")
	}
	params.consumed = true
	snapshot := params.snapshot
	configuration := *params.CreateParams
	params.mu.Unlock()

	if snapshot == nil {
		return nil, errors.New("gov8: startup blob is nil")
	}
	// Mark construction as pending while copying and registering the native
	// holder. Release then fails instead of waiting, which keeps a user counter
	// callback free to call Release without deadlocking on this mutex.
	snapshot.mu.Lock()
	if snapshot.released {
		snapshot.mu.Unlock()
		return nil, errors.New("gov8: startup blob has been released")
	}
	if err := snapshot.validForCreation(); err != nil {
		snapshot.mu.Unlock()
		return nil, err
	}
	snapshot.pendingCreates++
	snapshotBytes := snapshot.bytes
	snapshot.mu.Unlock()
	var consumer *Isolate
	var holder uintptr
	defer func() {
		snapshot.mu.Lock()
		snapshot.pendingCreates--
		if consumer != nil && holder != 0 {
			if snapshot.refs == nil {
				snapshot.refs = make(map[*Isolate]uintptr)
			}
			snapshot.refs[consumer] = holder
		}
		snapshot.mu.Unlock()
	}()
	if !configuration.initialized {
		configuration.initialize()
	}
	if configuration.stackLimit != 0 {
		return nil, errors.New("gov8: CreateParams stack limit cannot safely reference a Go stack")
	}
	if len(snapshotBytes) > math.MaxInt32 {
		return nil, errors.New("gov8: startup blob exceeds V8's int32 size limit")
	}
	if snapshot.requiresExternalReferences && !externalReferencesUsed(configuration.externalReferences) {
		return nil, errors.New("gov8: snapshot requires a non-empty external-reference table")
	}
	if !snapshot.externalReferencesKnown && !configuration.externalReferencesSet {
		return nil, errors.New("gov8: snapshot external-reference requirements are unknown; configure an explicit table (which may be empty)")
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
				_ = callErr("ArrayBufferAllocator.snapshot.clone.Close", proc("gov8_aba_dispose"), allocatorReference)
			}
		}()
	}

	counterHandle, err := registerIsolateCounter(configuration.counterLookup)
	if err != nil {
		return nil, err
	}
	referenceWords := externalReferenceWords(configuration.externalReferences)
	var referencePointer uintptr
	if len(referenceWords) != 0 {
		referencePointer = uintptr(unsafe.Pointer(&referenceWords[0]))
	}
	allowAtomics := uintptr(0)
	if configuration.allowAtomicsWait {
		allowAtomics = 1
	}
	referencesSet := uintptr(0)
	if configuration.externalReferencesSet {
		referencesSet = 1
	}
	var cppHeap *CppGCHeap
	if configuration.cppGCHeap != nil {
		cppHeap = configuration.cppGCHeap
		cppHeap.mu.Lock()
		if err := cppHeap.checkLocked(); err != nil {
			cppHeap.mu.Unlock()
			dropIsolateCounter(counterHandle)
			return nil, err
		}
		if !cppHeap.claimed || cppHeap.detachedEnabled || cppHeap.terminated {
			cppHeap.mu.Unlock()
			dropIsolateCounter(counterHandle)
			return nil, errors.New("gov8: cppgc heap is not transferable")
		}
		defer cppHeap.mu.Unlock()
	}

	runtime.LockOSThread()
	tid := currentThreadID()
	if err := beginIsolateCreate(); err != nil {
		dropIsolateCounter(counterHandle)
		runtime.UnlockOSThread()
		return nil, err
	}
	var isolateHandle uintptr
	var cppHeapHandle uintptr
	if cppHeap != nil {
		cppHeapHandle = cppHeap.handle
	}
	var cppHeapConsumed int32
	var cppHeapIdentity uintptr
	r1, _, _ := proc("gov8_cps_isolate_new").Call(
		uintptr(unsafe.Pointer(&snapshotBytes[0])), uintptr(len(snapshotBytes)),
		uintptr(configuration.maxOldGeneration), uintptr(configuration.maxYoungGeneration),
		uintptr(configuration.codeRange), uintptr(configuration.initialOldGeneration),
		uintptr(configuration.initialYoungGeneration), allowAtomics, referencesSet,
		referencePointer, uintptr(len(referenceWords)), counterHandle,
		cppHeapHandle, uintptr(unsafe.Pointer(&cppHeapConsumed)),
		uintptr(unsafe.Pointer(&cppHeapIdentity)), allocatorReference,
		uintptr(unsafe.Pointer(&holder)), uintptr(unsafe.Pointer(&isolateHandle)))
	runtime.KeepAlive(snapshotBytes)
	runtime.KeepAlive(referenceWords)
	if cppHeapConsumed == 1 && cppHeap != nil {
		cppHeap.handle = 0
		cppHeap.identity = cppHeapIdentity
		cppHeap.transferred = true
		cppgcHeapLifecycle.Lock()
		delete(cppgcHeapLifecycle.heaps, cppHeap)
		cppgcHeapLifecycle.Unlock()
		// NewCppGCHeap and this constructor each contributed one nested
		// LockOSThread. Discharge the transferred heap's lock; the remaining
		// lock keeps the returned isolate pinned until Isolate.Close.
		runtime.UnlockOSThread()
	}
	if int64(r1) < 0 {
		abandonIsolateCreate()
		dropIsolateCounter(counterHandle)
		runtime.UnlockOSThread()
		return nil, shimError("NewIsolateWithSnapshotParams", r1)
	}
	if holder == 0 || isolateHandle == 0 {
		abandonIsolateCreate()
		dropIsolateCounter(counterHandle)
		runtime.UnlockOSThread()
		return nil, errors.New("gov8: snapshot isolate constructor returned incomplete ownership")
	}
	consumer = &Isolate{
		handle:                        isolateHandle,
		tid:                           tid,
		advancedCounterHandle:         counterHandle,
		advancedExternalReferences:    configuration.externalReferencesSet,
		customCppGCHeap:               cppHeapConsumed == 1,
		arrayBufferAllocatorReference: allocatorReference,
	}
	allocatorReferenceTransferred = true
	finishIsolateCreate(consumer)
	return consumer, nil
}
