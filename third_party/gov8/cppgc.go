//go:build windows && amd64

package gov8

import (
	"fmt"
	"math"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// CppGCTag is the sandbox tag associated with a wrapped cppgc object. Tags
// are embedder-wide type identifiers; callers must use a stable tag for each
// native object family.
type CppGCTag uint16

// MaxCppGCTag is the largest tag accepted by pinned V8 152.2.0. 0 is also a
// valid tag.
const MaxCppGCTag CppGCTag = 0x7ffe

// CppGCObjectCallbacks observes cppgc tracing and final destruction. Either
// callback may be nil. V8 may trace on a GC worker, so callbacks must be
// concurrency-safe and must not call thread-affine V8 APIs. Destroy runs
// synchronously during sweeping or isolate teardown and must not re-enter its
// isolate (Isolate.Close holds the lifecycle lock during native teardown).
// A panic is a fail-fast host error because it cannot unwind through cppgc.
type CppGCObjectCallbacks struct {
	Trace   func()
	Destroy func()
}

type cppgcEntry struct {
	iso       *Isolate
	callbacks CppGCObjectCallbacks
}

var cppgcRegistry = struct {
	sync.Mutex
	next    uint64
	entries map[uint64]*cppgcEntry
}{entries: make(map[uint64]*cppgcEntry)}

const (
	cppgcDispatchTrace   = 1
	cppgcDispatchDestroy = 2
)

var goCppGCDispatch = syscall.NewCallback(func(idWord, kindWord uintptr) uintptr {
	id := uint64(idWord)
	cppgcRegistry.Lock()
	entry := cppgcRegistry.entries[id]
	if kindWord == cppgcDispatchDestroy && entry != nil {
		delete(cppgcRegistry.entries, id)
	}
	cppgcRegistry.Unlock()
	if entry == nil {
		fatalHostMisuse("gov8: cppgc callback for unknown registry ID %d", id)
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in cppgc callback: %v\n", recovered)
			proc("gov8_host_panic_abort").Call()
		}
	}()
	switch kindWord {
	case cppgcDispatchTrace:
		if entry.callbacks.Trace != nil {
			entry.callbacks.Trace()
		}
	case cppgcDispatchDestroy:
		if entry.callbacks.Destroy != nil {
			entry.callbacks.Destroy()
		}
	default:
		fatalHostMisuse("gov8: cppgc callback with invalid kind %d", kindWord)
	}
	return 0
})

func registerCppGCObject(iso *Isolate, callbacks CppGCObjectCallbacks) (uint64, error) {
	cppgcRegistry.Lock()
	defer cppgcRegistry.Unlock()
	for attempts := uint64(0); attempts < math.MaxUint64; attempts++ {
		cppgcRegistry.next++
		id := cppgcRegistry.next
		if id != 0 && cppgcRegistry.entries[id] == nil {
			cppgcRegistry.entries[id] = &cppgcEntry{iso: iso, callbacks: callbacks}
			return id, nil
		}
	}
	return 0, fmt.Errorf("gov8: cppgc callback registry exhausted")
}

func dropCppGCRegistration(id uint64) {
	cppgcRegistry.Lock()
	delete(cppgcRegistry.entries, id)
	cppgcRegistry.Unlock()
}

func liveCppGCRegistration(id uint64, iso *Isolate) bool {
	cppgcRegistry.Lock()
	entry := cppgcRegistry.entries[id]
	live := entry != nil && entry.iso == iso
	cppgcRegistry.Unlock()
	return live
}

// CppGCObject is a non-owning view of a cppgc allocation attached to a local
// API-wrapper object. V8 owns the allocation. The view is valid only while its
// wrapper's Scope is live; it never contains or exposes the native pointer.
type CppGCObject struct {
	iso        *Isolate
	wrapper    Value
	tag        CppGCTag
	registryID uint64
	objectID   int32
}

func validCppGCTag(tag CppGCTag) error {
	if tag > MaxCppGCTag {
		return fmt.Errorf("gov8: cppgc tag %#x exceeds %#x", uint16(tag), uint16(MaxCppGCTag))
	}
	return nil
}

func (object *CppGCObject) check() error {
	if object == nil || object.registryID == 0 {
		return fmt.Errorf("gov8: invalid cppgc object")
	}
	if err := object.wrapper.check(); err != nil {
		return err
	}
	if !liveCppGCRegistration(object.registryID, object.iso) {
		return fmt.Errorf("gov8: cppgc object has been destroyed")
	}
	return nil
}

// ID returns the embedder scalar stored in the cppgc allocation.
func (object *CppGCObject) ID() (int32, error) {
	if err := object.check(); err != nil {
		return 0, err
	}
	return object.objectID, nil
}

// Tag returns the exact tag used to wrap this allocation.
func (object *CppGCObject) Tag() (CppGCTag, error) {
	if err := object.check(); err != nil {
		return 0, err
	}
	return object.tag, nil
}

// Same reports whether two views identify the same cppgc allocation. It does
// not compare native addresses.
func (object *CppGCObject) Same(other *CppGCObject) (bool, error) {
	if err := object.check(); err != nil {
		return false, err
	}
	if err := other.check(); err != nil {
		return false, err
	}
	return object.iso == other.iso && object.registryID == other.registryID, nil
}

// WrapCppGCObject atomically allocates a native cppgc object and attaches it
// to wrapper. wrapper must be an API wrapper (for example, an object created
// by invoking a FunctionTemplate-backed constructor). target is retained by a
// native TracedReference visited from the cppgc object's Trace method.
//
// The allocation and Object::Wrap happen in one shim call: no raw, unrooted
// cppgc pointer crosses into Go.
func (s *Scope) WrapCppGCObject(wrapper *Object, target Value, objectID int32, tag CppGCTag, callbacks CppGCObjectCallbacks) (*CppGCObject, error) {
	if s == nil {
		return nil, fmt.Errorf("gov8: nil scope")
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	if err := s.requireCurrent(); err != nil {
		return nil, err
	}
	if wrapper == nil {
		return nil, fmt.Errorf("gov8: nil cppgc wrapper")
	}
	if err := wrapper.check(); err != nil {
		return nil, err
	}
	if err := target.check(); err != nil {
		return nil, err
	}
	if wrapper.iso != s.iso {
		return nil, foreignIsolate("wrapper")
	}
	if target.iso != s.iso {
		return nil, foreignIsolate("target")
	}
	if err := validCppGCTag(tag); err != nil {
		return nil, err
	}
	apiWrapper, err := wrapper.IsAPIWrapper()
	if err != nil {
		return nil, err
	}
	if !apiWrapper {
		return nil, fmt.Errorf("gov8: cppgc wrapper is not an API wrapper")
	}
	registryID, err := registerCppGCObject(s.iso, callbacks)
	if err != nil {
		return nil, err
	}
	var consumed int32
	observeTrace := uintptr(0)
	if callbacks.Trace != nil {
		observeTrace = 1
	}
	r1, _, _ := proc("gov8_cppgc_allocate_and_wrap").Call(
		s.iso.handleAssumingCheck(), sh, wrapper.h, target.h,
		uintptr(registryID), uintptr(objectID), uintptr(tag), observeTrace, goCppGCDispatch,
		uintptr(unsafe.Pointer(&consumed)))
	if int64(r1) < 0 {
		if consumed == 0 {
			dropCppGCRegistration(registryID)
		}
		return nil, shimError("Scope.WrapCppGCObject", r1)
	}
	return &CppGCObject{
		iso:        s.iso,
		wrapper:    wrapper.Value,
		tag:        tag,
		registryID: registryID,
		objectID:   objectID,
	}, nil
}

// UnwrapCppGCObject returns the managed allocation and its traced target.
// ok is false for an unwrapped object, a mismatched tag, or a wrapper managed
// by another native object family. Go deliberately verifies exact tag and
// family identity even though the pinned raw unsafe Rust API does not.
func (s *Scope) UnwrapCppGCObject(wrapper *Object, tag CppGCTag) (object *CppGCObject, target Value, ok bool, err error) {
	if s == nil {
		return nil, Value{}, false, fmt.Errorf("gov8: nil scope")
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, Value{}, false, err
	}
	if err := s.requireCurrent(); err != nil {
		return nil, Value{}, false, err
	}
	if wrapper == nil {
		return nil, Value{}, false, fmt.Errorf("gov8: nil cppgc wrapper")
	}
	if err := wrapper.check(); err != nil {
		return nil, Value{}, false, err
	}
	if wrapper.iso != s.iso {
		return nil, Value{}, false, foreignIsolate("wrapper")
	}
	if err := validCppGCTag(tag); err != nil {
		return nil, Value{}, false, err
	}
	var registryID uint64
	var objectID int32
	var targetWire uintptr
	r1, _, _ := proc("gov8_cppgc_unwrap").Call(
		s.iso.handleAssumingCheck(), sh, wrapper.h, uintptr(tag),
		uintptr(unsafe.Pointer(&registryID)), uintptr(unsafe.Pointer(&objectID)),
		uintptr(unsafe.Pointer(&targetWire)))
	if int64(r1) < 0 {
		return nil, Value{}, false, shimError("Scope.UnwrapCppGCObject", r1)
	}
	if registryID == 0 {
		return nil, Value{}, false, nil
	}
	if targetWire == 0 || !liveCppGCRegistration(registryID, s.iso) {
		return nil, Value{}, false, fmt.Errorf("gov8: cppgc unwrap returned stale metadata")
	}
	object = &CppGCObject{
		iso:        s.iso,
		wrapper:    wrapper.Value,
		tag:        tag,
		registryID: registryID,
		objectID:   objectID,
	}
	return object, Value{iso: s.iso, sc: s, h: targetWire}, true, nil
}
