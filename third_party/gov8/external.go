//go:build windows && amd64

package gov8

import (
	"fmt"
	"math"
	"sync"
)

// External values, internal fields and host-data ownership.
//
// Ownership rules (the constraint that shapes this file): the FFI is
// syscall-based and V8 must never hold a Go pointer — a Go pointer stored
// behind the engine's back can be collected or moved while a JS object still
// references it. Host data referenced from JS therefore travels as integer
// registry handles (HostRef tokens) that stay valid until the embedder
// removes them. Raw uintptr payloads (External, aligned internal-field
// pointers) remain available for embedder-owned native memory, documented as
// embedder responsibility.

var int32max = int64(math.MaxInt32)

// --- External -----------------------------------------------------------------

// NewExternal wraps an embedder-provided pointer in a JS External value (v8
// External::New with the default pointer tag).
//
// The payload is a plain uintptr and is never interpreted by gov8: it must
// NOT be a bare Go pointer unless the embedder keeps the object alive for as
// long as the engine can reach the value (Go's GC is currently non-moving,
// but this is not a guarantee — prefer HostRef tokens for Go data).
func (s *Scope) NewExternal(payload uintptr) (Value, error) {
	if err := s.check(); err != nil {
		return Value{}, err
	}
	h, err := callHandle("NewExternal", proc("gov8_external_new"),
		s.iso.handle, s.handle, payload)
	if err != nil {
		return Value{}, err
	}
	return Value{iso: s.iso, sc: s, h: h}, nil
}

// IsExternal reports whether the value is a JS External.
func (v Value) IsExternal() (bool, error) {
	if err := v.check(); err != nil {
		return false, err
	}
	ih, err := v.iso.handleChecked()
	if err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_value_is_external").Call(ih, v.h)
	if int64(r1) < 0 {
		return false, shimError("IsExternal", r1)
	}
	return r1 == 1, nil
}

// ExternalValue returns the External's raw payload pointer.
func (v Value) ExternalValue() (uintptr, error) {
	if err := v.check(); err != nil {
		return 0, err
	}
	sh, err := v.sc.checkedHandle()
	if err != nil {
		return 0, err
	}
	h, err := callHandle("ExternalValue", proc("gov8_external_value"),
		v.iso.handle, sh, v.h)
	if err != nil {
		return 0, err
	}
	return h, nil
}

// --- HostRef: Go data behind integer tokens -------------------------------------

// hostRefs maps integer tokens to Go values for one isolate. Tokens are what
// the engine sees (as External payloads or aligned internal-field pointers),
// so no Go pointer ever reaches V8. The registry is keyed by isolate and
// released with ReleaseIsolateHostState.
type hostRefs struct {
	mu     sync.Mutex
	next   uint64
	values map[uint64]any
}

var hostRefRegistries = struct {
	mu sync.Mutex
	m  map[*Isolate]*hostRefs
}{m: make(map[*Isolate]*hostRefs)}

func refsFor(i *Isolate) *hostRefs {
	hostRefRegistries.mu.Lock()
	defer hostRefRegistries.mu.Unlock()
	r, ok := hostRefRegistries.m[i]
	if !ok {
		r = &hostRefs{values: make(map[uint64]any)}
		hostRefRegistries.m[i] = r
	}
	return r
}

// HostRefAdd stores v on the isolate and returns an 8-aligned integer token
// suitable as an External payload or aligned internal-field pointer. The
// token stays valid until HostRefRemove.
func (i *Isolate) HostRefAdd(v any) (uintptr, error) {
	if err := i.check(); err != nil {
		return 0, err
	}
	r := refsFor(i)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	token := r.next << 3 // 8-aligned, non-zero
	r.values[r.next] = v
	return uintptr(token), nil
}

func hostRefSplit(token uintptr) (uint64, bool) {
	if token == 0 || token%8 != 0 {
		return 0, false
	}
	return uint64(token >> 3), true
}

// HostRefGet resolves a token previously returned by HostRefAdd.
func (i *Isolate) HostRefGet(token uintptr) (any, bool) {
	id, ok := hostRefSplit(token)
	if !ok {
		return nil, false
	}
	r := refsFor(i)
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.values[id]
	return v, ok
}

// HostRefRemove resolves a token and hands ownership back to the caller: the
// token stops resolving and the Go value becomes the caller's again.
func (i *Isolate) HostRefRemove(token uintptr) (any, bool) {
	id, ok := hostRefSplit(token)
	if !ok {
		return nil, false
	}
	r := refsFor(i)
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.values[id]
	if ok {
		delete(r.values, id)
	}
	return v, ok
}

// --- internal fields ---------------------------------------------------------------

// InternalFieldCount returns the number of internal fields configured on the
// object's instance template (0 for plain objects).
func (v Value) InternalFieldCount() (int, error) {
	if err := v.check(); err != nil {
		return 0, err
	}
	sh, err := v.sc.checkedHandle()
	if err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_object_internal_field_count").Call(
		v.iso.handle, sh, v.h)
	if int64(r1) < 0 {
		return 0, shimError("InternalFieldCount", r1)
	}
	return int(r1), nil
}

// SetInternalField stores a Data value in internal field index. ok is false
// when the index is out of bounds — this wrapper is the bounds check (the
// engine's release build performs none, so reaching V8 with an out-of-range
// index would corrupt memory).
func (v Value) SetInternalField(index int, data Value) (bool, error) {
	if err := v.check(); err != nil {
		return false, err
	}
	if err := data.check(); err != nil {
		return false, err
	}
	if data.iso != v.iso {
		return false, foreignIsolate("value")
	}
	sh, err := v.sc.checkedHandle()
	if err != nil {
		return false, err
	}
	count, err := v.InternalFieldCount()
	if err != nil {
		return false, err
	}
	if index < 0 || index >= count {
		return false, nil
	}
	return true, callErr("SetInternalField",
		proc("gov8_object_set_internal_field"),
		v.iso.handle, sh, v.h, uintptr(int32(index)), data.h)
}

// GetInternalField reads internal field index. ok is false when the index is
// out of bounds or the field was never set.
func (v Value) GetInternalField(index int) (Value, bool, error) {
	if err := v.check(); err != nil {
		return Value{}, false, err
	}
	sh, err := v.sc.checkedHandle()
	if err != nil {
		return Value{}, false, err
	}
	count, err := v.InternalFieldCount()
	if err != nil {
		return Value{}, false, err
	}
	if index < 0 || index >= count {
		return Value{}, false, nil
	}
	h, err := callHandle("GetInternalField",
		proc("gov8_object_get_internal_field"),
		v.iso.handle, sh, v.h, uintptr(int32(index)))
	if err != nil {
		return Value{}, false, err
	}
	if h == 0 {
		return Value{}, false, nil
	}
	return Value{iso: v.iso, sc: v.sc, h: h}, true, nil
}

// SetAlignedPointerInInternalField stores an embedder pointer in internal
// field index under a type tag (0-15; the engine aborts on out-of-range
// tags). The pointer must be 8-aligned. Use HostRefAdd tokens for Go data;
// raw pointers must reference embedder-owned native memory that outlives the
// object.
func (v Value) SetAlignedPointerInInternalField(index int, ptr uintptr, tag int) error {
	if err := v.check(); err != nil {
		return err
	}
	if tag < 0 || tag > 15 {
		return fmt.Errorf("gov8: embedder data tag %d out of range 0..15", tag)
	}
	if ptr%8 != 0 {
		return fmt.Errorf("gov8: aligned internal-field pointer %x is not 8-aligned", ptr)
	}
	sh, err := v.sc.checkedHandle()
	if err != nil {
		return err
	}
	count, err := v.InternalFieldCount()
	if err != nil {
		return err
	}
	if index < 0 || index >= count {
		return fmt.Errorf("gov8: internal field index %d out of bounds (%d fields)", index, count)
	}
	return callErr("SetAlignedPointerInInternalField",
		proc("gov8_object_set_aligned_pointer_in_internal_field"),
		v.iso.handle, sh, v.h, uintptr(int32(index)), ptr, uintptr(tag))
}

// GetAlignedPointerFromInternalField reads an aligned pointer previously
// stored with the same tag. ok is false for out-of-bounds indices. A stored
// null pointer reads back as (0, true, nil): the shim encodes a null result
// as the zero word with no error text, which this wrapper distinguishes from
// real failures (those carry a shim status and detail message).
func (v Value) GetAlignedPointerFromInternalField(index int, tag int) (uintptr, bool, error) {
	if err := v.check(); err != nil {
		return 0, false, err
	}
	if tag < 0 || tag > 15 {
		return 0, false, fmt.Errorf("gov8: embedder data tag %d out of range 0..15", tag)
	}
	sh, err := v.sc.checkedHandle()
	if err != nil {
		return 0, false, err
	}
	count, err := v.InternalFieldCount()
	if err != nil {
		return 0, false, err
	}
	if index < 0 || index >= count {
		return 0, false, nil
	}
	h, err := callHandle("GetAlignedPointerFromInternalField",
		proc("gov8_object_get_aligned_pointer_from_internal_field"),
		v.iso.handle, sh, v.h, uintptr(int32(index)), uintptr(tag))
	if err != nil {
		// The shim's success path with a null pointer arrives as a zero
		// return word and an empty thread-local error message; everything
		// else is a genuine failure.
		if se, isShim := err.(*ShimError); isShim && se.Code == 0 && se.Detail == "" {
			return 0, true, nil
		}
		return 0, false, err
	}
	return h, true, nil
}

// --- value identity -----------------------------------------------------------------

// StrictEquals reports ECMAScript strict equality between two values from
// the same isolate (v8 Value::StrictEquals).
func (v Value) StrictEquals(other Value) (bool, error) {
	if err := v.check(); err != nil {
		return false, err
	}
	if err := other.check(); err != nil {
		return false, err
	}
	if other.iso != v.iso {
		return false, foreignIsolate("value")
	}
	ih, err := v.iso.handleChecked()
	if err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_value_strict_equals").Call(ih, v.h, other.h)
	if int64(r1) < 0 {
		return false, shimError("StrictEquals", r1)
	}
	return r1 == 1, nil
}

// --- isolate slots --------------------------------------------------------------------
//
// v8::Isolate::set_slot/get_slot/remove_slot keyed by type in the crate map
// to an explicit `any` key here (Go has no TypeId-keyed singletons). Storage
// is host-side only: the engine is never involved.
//
// Ownership contract (documented deviation from the Rust destructors):
//   - SetSlot replacing an existing value releases the old value immediately
//     when it implements slotReleaser.
//   - RemoveSlot hands ownership back; the caller releases.
//   - Values still stored at teardown are released by
//     ReleaseIsolateHostState, the explicit Go equivalent of the Rust
//     Isolate::drop destructor (Go has no destructors and a finalizer would
//     run after engine teardown).

// slotReleaser is implemented by slot values that need deterministic release
// instead of relying on Go GC.
type slotReleaser interface {
	ReleaseSlotValue()
}

type isolateSlots struct {
	mu   sync.Mutex
	next uint64
	m    map[any]any
}

var slotRegistries = struct {
	mu sync.Mutex
	m  map[*Isolate]*isolateSlots
}{m: make(map[*Isolate]*isolateSlots)}

func slotsFor(i *Isolate) *isolateSlots {
	slotRegistries.mu.Lock()
	defer slotRegistries.mu.Unlock()
	s, ok := slotRegistries.m[i]
	if !ok {
		s = &isolateSlots{m: make(map[any]any)}
		slotRegistries.m[i] = s
	}
	return s
}

// SetSlot stores value under key. It reports whether the slot was previously
// empty; when it was not, the replaced value is released immediately if it
// implements slotReleaser (matching the oracle's replace-drops-old
// semantics) and otherwise simply becomes unreachable.
func (i *Isolate) SetSlot(key, value any) (wasEmpty bool) {
	s := slotsFor(i)
	s.mu.Lock()
	defer s.mu.Unlock()
	old, existed := s.m[key]
	s.m[key] = value
	if existed {
		releaseSlotValue(old)
	}
	return !existed
}

// GetSlot returns the value stored under key.
func (i *Isolate) GetSlot(key any) (any, bool) {
	s := slotsFor(i)
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	return v, ok
}

// RemoveSlot removes and returns the value stored under key, handing
// ownership back to the caller (no release hook runs).
func (i *Isolate) RemoveSlot(key any) (any, bool) {
	s := slotsFor(i)
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	if ok {
		delete(s.m, key)
	}
	return v, ok
}

func releaseSlotValue(v any) {
	if r, ok := v.(slotReleaser); ok {
		r.ReleaseSlotValue()
	}
}

func releaseSlots(i *Isolate) {
	slotRegistries.mu.Lock()
	s, ok := slotRegistries.m[i]
	if ok {
		delete(slotRegistries.m, i)
	}
	hostRefRegistries.mu.Lock()
	r, refsOK := hostRefRegistries.m[i]
	if refsOK {
		delete(hostRefRegistries.m, i)
	}
	hostRefRegistries.mu.Unlock()
	slotRegistries.mu.Unlock()

	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.m {
		releaseSlotValue(v)
	}
	s.m = make(map[any]any)
	_ = r
}
