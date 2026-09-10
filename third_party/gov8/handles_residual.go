//go:build windows && amd64

package gov8

import (
	"fmt"
	"unsafe"
)

// Eternal is V8's set-once-style persistent handle. The pinned engine also
// accepts another Set while non-empty and makes the new value observable; this
// wrapper deliberately preserves that characterized overwrite behavior.
//
// Eternal differs from Global: clearing only empties the Eternal wrapper. V8
// owns the eternal-table entry for the isolate lifetime. Close destroys the
// small host wrapper and must be called deterministically.
type Eternal struct {
	iso    *Isolate
	handle uintptr
	closed bool
}

// EmptyEternal constructs an empty Eternal, corresponding to
// v8::Eternal::empty. The pinned API has no constructor that takes a Local;
// initialize (and later overwrite or reuse) it with Set.
func EmptyEternal() (*Eternal, error) {
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	handle, err := callHandle("Eternal.Empty", proc("gov8_hr_eternal_empty"))
	if err != nil {
		return nil, err
	}
	return &Eternal{handle: handle}, nil
}

func (e *Eternal) checkOpen() error {
	if e == nil {
		return fmt.Errorf("gov8: nil Eternal")
	}
	if e.closed {
		return fmt.Errorf("gov8: Eternal used after Close")
	}
	if e.handle == 0 {
		return fmt.Errorf("gov8: invalid Eternal handle")
	}
	return nil
}

func (e *Eternal) scope(s *Scope) (uintptr, error) {
	if err := e.checkOpen(); err != nil {
		return 0, err
	}
	if s == nil {
		return 0, fmt.Errorf("gov8: nil scope")
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return 0, err
	}
	if e.iso != nil && s.iso != e.iso {
		return 0, foreignIsolate("scope")
	}
	return sh, nil
}

// IsEmpty reports whether the Eternal currently contains a value. A handle
// bound to a live isolate remains thread-affine. Once that isolate is closed,
// this pure opaque-slot query remains safe, as established by the oracle.
func (e *Eternal) IsEmpty() (bool, error) {
	if err := e.checkOpen(); err != nil {
		return false, err
	}
	if e.iso != nil && !isolateClosed(e.iso) {
		if err := e.iso.check(); err != nil {
			return false, err
		}
	}
	r1, _, _ := proc("gov8_hr_eternal_is_empty").Call(e.handle)
	if int64(r1) < 0 {
		return false, shimError("Eternal.IsEmpty", r1)
	}
	return r1 == 1, nil
}

// Set stores v in the Eternal. Repeated Set calls on the same isolate are
// allowed and overwrite the value on the pinned V8 build.
func (e *Eternal) Set(s *Scope, v Value) error {
	sh, err := e.scope(s)
	if err != nil {
		return err
	}
	if err := v.check(); err != nil {
		return err
	}
	if v.iso != s.iso {
		return foreignIsolate("value")
	}
	r1, _, _ := proc("gov8_hr_eternal_set").Call(
		s.iso.handleAssumingCheck(), sh, e.handle, v.h)
	if int64(r1) < 0 {
		return shimError("Eternal.Set", r1)
	}
	if e.iso == nil {
		e.iso = s.iso
	}
	return nil
}

// Get reopens the Eternal as a local in s. ok is false while it is empty.
// The scope must belong to the isolate on which the Eternal was first set.
func (e *Eternal) Get(s *Scope) (value Value, ok bool, err error) {
	sh, err := e.scope(s)
	if err != nil {
		return Value{}, false, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_hr_eternal_get").Call(
		s.iso.handleAssumingCheck(), sh, e.handle,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, false, shimError("Eternal.Get", r1)
	}
	if out == 0 {
		return Value{}, false, nil
	}
	return Value{iso: s.iso, sc: s, h: out}, true, nil
}

// Clear empties the Eternal. While its isolate is live, Clear is
// thread-affine. The pinned subprocess oracle also proves Clear safe after the
// isolate is closed, including when the Eternal was still non-empty.
func (e *Eternal) Clear() error {
	if err := e.checkOpen(); err != nil {
		return err
	}
	if e.iso != nil && !isolateClosed(e.iso) {
		if err := e.iso.check(); err != nil {
			return err
		}
	}
	return callErr("Eternal.Clear", proc("gov8_hr_eternal_clear"), e.handle)
}

// Close destroys the host-side Eternal wrapper. It does not need a live
// isolate; the pinned destructor is safe after isolate disposal.
func (e *Eternal) Close() error {
	if err := e.checkOpen(); err != nil {
		return err
	}
	if e.iso != nil && !isolateClosed(e.iso) {
		if err := e.iso.check(); err != nil {
			return err
		}
	}
	err := callErr("Eternal.Close", proc("gov8_hr_eternal_dispose"), e.handle)
	if err == nil {
		e.handle = 0
		e.closed = true
	}
	return err
}

// TracedReference is V8's embedder-traced handle. It is not a strong root.
// V8 expects it to be embedded in an externally traced C++/cppgc owner; gov8
// does not currently expose that owner integration. Consequently Get is only
// safe while the caller independently knows the target is live (for example,
// while a Local or Eternal roots it). Never rely on a TracedReference alone to
// keep a JavaScript object alive across garbage collection.
type TracedReference struct {
	iso    *Isolate
	handle uintptr
	closed bool
}

// EmptyTracedReference constructs an empty TracedReference.
func EmptyTracedReference() (*TracedReference, error) {
	if err := requireInitialized(); err != nil {
		return nil, err
	}
	handle, err := callHandle("TracedReference.Empty", proc("gov8_hr_traced_empty"))
	if err != nil {
		return nil, err
	}
	return &TracedReference{handle: handle}, nil
}

// NewTracedReference constructs a TracedReference containing v.
func NewTracedReference(s *Scope, v Value) (*TracedReference, error) {
	r, err := EmptyTracedReference()
	if err != nil {
		return nil, err
	}
	if err := r.Reset(s, &v); err != nil {
		_ = r.Close()
		return nil, err
	}
	return r, nil
}

func (r *TracedReference) checkOpen() error {
	if r == nil {
		return fmt.Errorf("gov8: nil TracedReference")
	}
	if r.closed {
		return fmt.Errorf("gov8: TracedReference used after Close")
	}
	if r.handle == 0 {
		return fmt.Errorf("gov8: invalid TracedReference handle")
	}
	return nil
}

func (r *TracedReference) scope(s *Scope) (uintptr, error) {
	if err := r.checkOpen(); err != nil {
		return 0, err
	}
	if s == nil {
		return 0, fmt.Errorf("gov8: nil scope")
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return 0, err
	}
	if r.iso != nil && s.iso != r.iso {
		return 0, foreignIsolate("scope")
	}
	return sh, nil
}

// Reset always clears the old reference, then stores value when non-nil.
// Passing nil corresponds to rusty_v8 reset(scope, None). The reference stays
// bound to its first isolate even after reset-to-empty, so cross-isolate Get
// fails deterministically before the engine boundary.
func (r *TracedReference) Reset(s *Scope, value *Value) error {
	sh, err := r.scope(s)
	if err != nil {
		return err
	}
	var valueHandle uintptr
	if value != nil {
		if err := value.check(); err != nil {
			return err
		}
		if value.iso != s.iso {
			return foreignIsolate("value")
		}
		valueHandle = value.h
	}
	r1, _, _ := proc("gov8_hr_traced_reset").Call(
		s.iso.handleAssumingCheck(), sh, r.handle, valueHandle)
	if int64(r1) < 0 {
		return shimError("TracedReference.Reset", r1)
	}
	if value != nil && r.iso == nil {
		r.iso = s.iso
	}
	return nil
}

// Get reopens the reference as a local in s. ok is false while empty. This
// method cannot prove traced reachability; see TracedReference's type comment.
func (r *TracedReference) Get(s *Scope) (value Value, ok bool, err error) {
	sh, err := r.scope(s)
	if err != nil {
		return Value{}, false, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_hr_traced_get").Call(
		s.iso.handleAssumingCheck(), sh, r.handle,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, false, shimError("TracedReference.Get", r1)
	}
	if out == 0 {
		return Value{}, false, nil
	}
	return Value{iso: s.iso, sc: s, h: out}, true, nil
}

// Close destroys the host wrapper. Like the pinned TracedReference Drop, it
// intentionally does not Reset and remains safe after isolate disposal.
func (r *TracedReference) Close() error {
	if err := r.checkOpen(); err != nil {
		return err
	}
	if r.iso != nil && !isolateClosed(r.iso) {
		if err := r.iso.check(); err != nil {
			return err
		}
	}
	err := callErr("TracedReference.Close", proc("gov8_hr_traced_dispose"), r.handle)
	if err == nil {
		r.handle = 0
		r.closed = true
	}
	return err
}
