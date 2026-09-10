//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

type CppGCMarkingType uint8

const (
	CppGCMarkingAtomic CppGCMarkingType = iota
	CppGCMarkingIncremental
	CppGCMarkingIncrementalAndConcurrent
)

func (v CppGCMarkingType) String() string {
	switch v {
	case CppGCMarkingAtomic:
		return "Atomic"
	case CppGCMarkingIncremental:
		return "Incremental"
	case CppGCMarkingIncrementalAndConcurrent:
		return "IncrementalAndConcurrent"
	}
	return fmt.Sprintf("CppGCMarkingType(%d)", v)
}

type CppGCSweepingType uint8

const (
	CppGCSweepingAtomic CppGCSweepingType = iota
	CppGCSweepingIncremental
	CppGCSweepingIncrementalAndConcurrent
)

func (v CppGCSweepingType) String() string {
	switch v {
	case CppGCSweepingAtomic:
		return "Atomic"
	case CppGCSweepingIncremental:
		return "Incremental"
	case CppGCSweepingIncrementalAndConcurrent:
		return "IncrementalAndConcurrent"
	}
	return fmt.Sprintf("CppGCSweepingType(%d)", v)
}

type CppGCEmbedderStackState uint8

const (
	CppGCStackMayContainHeapPointers CppGCEmbedderStackState = iota
	CppGCStackNoHeapPointers
)

type CppGCHeapCreateParams struct {
	MarkingSupport  CppGCMarkingType
	SweepingSupport CppGCSweepingType
}

func DefaultCppGCHeapCreateParams() CppGCHeapCreateParams {
	return CppGCHeapCreateParams{CppGCMarkingIncrementalAndConcurrent, CppGCSweepingIncrementalAndConcurrent}
}

var cppgcHeapLifecycle = struct {
	sync.Mutex
	explicit         bool
	retainedPlatform bool
	heaps            map[*CppGCHeap]struct{}
}{heaps: make(map[*CppGCHeap]struct{})}

// InitializeCppGCProcess initializes cppgc independently of V8 for detached
// heap use. Go rejects duplicate initialization before rusty_v8's fatal CHECK.
func InitializeCppGCProcess() error {
	if err := loadShim(); err != nil {
		return err
	}
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	cppgcHeapLifecycle.Lock()
	defer cppgcHeapLifecycle.Unlock()
	if loadPlatform() != stateUninitialized || cppgcHeapLifecycle.explicit {
		return errors.New("gov8: cppgc process initialization requires uninitialized V8 and is one-shot until shutdown")
	}
	if err := callErr("CppGC.InitializeProcess", proc("gov8_cppgc_process_initialize")); err != nil {
		return err
	}
	cppgcHeapLifecycle.explicit = true
	return nil
}

// ShutdownCppGCProcess pairs an explicit initialization. It rejects live
// heaps, duplicate shutdown, and V8-managed cppgc state rather than forwarding
// rusty_v8's unguarded unsafe shutdown.
func ShutdownCppGCProcess() error {
	lifecycleMu.Lock()
	defer lifecycleMu.Unlock()
	cppgcHeapLifecycle.Lock()
	defer cppgcHeapLifecycle.Unlock()
	if !cppgcHeapLifecycle.explicit || loadPlatform() != stateUninitialized {
		return errors.New("gov8: cppgc process is not explicitly initialized")
	}
	if len(cppgcHeapLifecycle.heaps) != 0 {
		return fmt.Errorf("gov8: cppgc process shutdown with %d live heap(s)", len(cppgcHeapLifecycle.heaps))
	}
	if err := callErr("CppGC.ShutdownProcess", proc("gov8_cppgc_process_shutdown")); err != nil {
		return err
	}
	cppgcHeapLifecycle.explicit = false
	cppgcHeapLifecycle.retainedPlatform = true
	return nil
}

func cppgcBeforeV8Initialize() error {
	cppgcHeapLifecycle.Lock()
	defer cppgcHeapLifecycle.Unlock()
	if cppgcHeapLifecycle.explicit || len(cppgcHeapLifecycle.heaps) != 0 {
		return errors.New("gov8: shut down the explicit cppgc process and its heaps before Initialize")
	}
	if cppgcHeapLifecycle.retainedPlatform && (selectedPlatform != nil || selectedCustomPlatform != nil) {
		return errors.New("gov8: an explicit cppgc platform can only be promoted through default Initialize")
	}
	return nil
}

type CppGCHeap struct {
	mu              sync.Mutex
	handle          uintptr
	identity        uintptr
	tid             uint32
	closed          bool
	terminated      bool
	detachedEnabled bool
	claimed         bool
	transferred     bool
}

func NewCppGCHeap(params CppGCHeapCreateParams) (*CppGCHeap, error) {
	if params.MarkingSupport > CppGCMarkingIncrementalAndConcurrent || params.SweepingSupport > CppGCSweepingIncrementalAndConcurrent {
		return nil, errors.New("gov8: invalid cppgc heap support enum")
	}
	cppgcHeapLifecycle.Lock()
	defer cppgcHeapLifecycle.Unlock()
	allowed := cppgcHeapLifecycle.explicit || loadPlatform() == stateInitialized
	if !allowed {
		return nil, errors.New("gov8: cppgc process is not initialized")
	}
	runtime.LockOSThread()
	handle, err := callHandle("CppGCHeap.New", proc("gov8_cppgc_heap_create"), uintptr(params.MarkingSupport), uintptr(params.SweepingSupport))
	if err != nil {
		runtime.UnlockOSThread()
		return nil, err
	}
	heap := &CppGCHeap{handle: handle, identity: handle, tid: currentThreadID()}
	cppgcHeapLifecycle.heaps[heap] = struct{}{}
	return heap, nil
}

func (h *CppGCHeap) checkLocked() error {
	if h == nil || h.closed || h.handle == 0 {
		return errors.New("gov8: cppgc heap used after Close or transfer")
	}
	if currentThreadID() != h.tid {
		return errors.New("gov8: cppgc heap thread affinity violated")
	}
	return nil
}

func (h *CppGCHeap) EnableDetachedGarbageCollectionsForTesting() error {
	if h == nil {
		return errors.New("gov8: nil cppgc heap")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.checkLocked(); err != nil {
		return err
	}
	if h.detachedEnabled {
		return errors.New("gov8: detached cppgc collection already enabled")
	}
	if h.claimed {
		return errors.New("gov8: attached/claimed cppgc heap cannot enter detached testing mode")
	}
	if err := callErr("CppGCHeap.EnableDetached", proc("gov8_cppgc_heap_enable_detached"), h.handle); err != nil {
		return err
	}
	h.detachedEnabled = true
	return nil
}

func (h *CppGCHeap) CollectGarbageForTesting(state CppGCEmbedderStackState) error {
	if h == nil {
		return errors.New("gov8: nil cppgc heap")
	}
	if state > CppGCStackNoHeapPointers {
		return errors.New("gov8: invalid cppgc stack state")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.checkLocked(); err != nil {
		return err
	}
	return callErr("CppGCHeap.CollectGarbageForTesting", proc("gov8_cppgc_heap_collect"), h.handle, uintptr(state))
}

// CppGCHeapAllocation observes an unrooted managed leaf without retaining or
// exposing its native address.
type CppGCHeapAllocation struct {
	registryID uint64
	objectID   int32
}

func (a *CppGCHeapAllocation) Alive() bool {
	return a != nil && a.registryID != 0 && liveCppGCRegistration(a.registryID, nil)
}
func (a *CppGCHeapAllocation) ID() int32 {
	if a == nil {
		return 0
	}
	return a.objectID
}

func (h *CppGCHeap) AllocateLeaf(objectID int32, callbacks CppGCObjectCallbacks) (*CppGCHeapAllocation, error) {
	if h == nil {
		return nil, errors.New("gov8: nil cppgc heap")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.checkLocked(); err != nil {
		return nil, err
	}
	id, err := registerCppGCObject(nil, callbacks)
	if err != nil {
		return nil, err
	}
	var consumed int32
	r1, _, _ := proc("gov8_cppgc_heap_allocate_leaf").Call(h.handle, uintptr(id), uintptr(objectID), goCppGCDispatch, uintptr(unsafe.Pointer(&consumed)))
	if int64(r1) < 0 {
		if consumed == 0 {
			dropCppGCRegistration(id)
		}
		return nil, shimError("CppGCHeap.AllocateLeaf", r1)
	}
	return &CppGCHeapAllocation{registryID: id, objectID: objectID}, nil
}

func (h *CppGCHeap) Terminate() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.transferred {
		return errors.New("gov8: cppgc heap ownership transferred to isolate")
	}
	if h.closed {
		return nil
	}
	if currentThreadID() != h.tid {
		return errors.New("gov8: cppgc heap thread affinity violated")
	}
	if h.terminated {
		return nil
	}
	if err := callErr("CppGCHeap.Terminate", proc("gov8_cppgc_heap_terminate"), h.handle); err != nil {
		return err
	}
	h.terminated = true
	return nil
}

func (h *CppGCHeap) Close() error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.transferred || h.closed {
		return nil
	}
	if currentThreadID() != h.tid {
		return errors.New("gov8: cppgc heap Close thread affinity violated")
	}
	if err := callErr("CppGCHeap.Close", proc("gov8_cppgc_heap_close"), h.handle); err != nil {
		return err
	}
	h.handle = 0
	h.closed = true
	cppgcHeapLifecycle.Lock()
	delete(cppgcHeapLifecycle.heaps, h)
	cppgcHeapLifecycle.Unlock()
	runtime.UnlockOSThread()
	return nil
}

func (h *CppGCHeap) AttachedTo(i *Isolate) (bool, error) {
	if h == nil || i == nil {
		return false, errors.New("gov8: nil cppgc heap or isolate")
	}
	if err := i.check(); err != nil {
		return false, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.transferred || h.identity == 0 {
		return false, errors.New("gov8: cppgc heap was not transferred")
	}
	var matched int32
	if err := callErr("CppGCHeap.AttachedTo", proc("gov8_cppgc_heap_matches_isolate"), h.identity, i.handleAssumingCheck(), uintptr(unsafe.Pointer(&matched))); err != nil {
		return false, err
	}
	return matched == 1, nil
}
