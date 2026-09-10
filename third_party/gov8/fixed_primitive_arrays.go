//go:build windows && amd64

package gov8

import (
	"fmt"
	"unsafe"
)

// Data is a scope-local handle to any V8 heap data. Unlike Value, Data can
// also represent metadata-only engine objects such as FixedArray and
// ModuleRequest. It follows the same scope, isolate, and thread-affinity rules
// as Value.
type Data struct {
	iso *Isolate
	sc  *Scope
	h   uintptr
}

func (d Data) check() error {
	if d.h == 0 {
		return fmt.Errorf("gov8: zero data handle")
	}
	if d.sc == nil || d.iso == nil {
		return fmt.Errorf("gov8: invalid data handle")
	}
	return d.sc.check()
}

func (d Data) predicate(name, op string) (bool, error) {
	if err := d.check(); err != nil {
		return false, err
	}
	r1, _, _ := proc(name).Call(d.iso.handleAssumingCheck(), d.h)
	if int64(r1) < 0 {
		return false, shimError(op, r1)
	}
	return r1 == 1, nil
}

// IsValue reports whether the data can be viewed as a JavaScript Value.
func (d Data) IsValue() (bool, error) {
	return d.predicate("gov8_data_is_value", "Data.IsValue")
}

// IsPrimitive reports whether the data is a JavaScript primitive Value.
func (d Data) IsPrimitive() (bool, error) {
	return d.predicate("gov8_data_is_primitive", "Data.IsPrimitive")
}

// IsFixedArray reports whether the data is a FixedArray.
func (d Data) IsFixedArray() (bool, error) {
	return d.predicate("gov8_data_is_fixed_array", "Data.IsFixedArray")
}

// IsModuleRequest reports whether the data is a ModuleRequest.
func (d Data) IsModuleRequest() (bool, error) {
	return d.predicate("gov8_data_is_module_request", "Data.IsModuleRequest")
}

// Value converts data known to be a JavaScript Value. ok is false for
// metadata-only Data.
func (d Data) Value() (Value, bool, error) {
	ok, err := d.IsValue()
	if err != nil || !ok {
		return Value{}, false, err
	}
	return Value{iso: d.iso, sc: d.sc, h: d.h}, true, nil
}

// ModuleRequestData is a local module-request metadata handle.
type ModuleRequestData struct{ Data }

// ModuleRequest converts data known to hold module request metadata.
func (d Data) ModuleRequest() (*ModuleRequestData, bool, error) {
	ok, err := d.IsModuleRequest()
	if err != nil || !ok {
		return nil, false, err
	}
	return &ModuleRequestData{Data: d}, true, nil
}

// ImportAttributes returns the raw [key, value, source-offset, ...] metadata
// array for the request. The returned local is owned by the supplied scope.
func (r *ModuleRequestData) ImportAttributes(s *Scope) (*FixedArray, error) {
	if r == nil {
		return nil, fmt.Errorf("gov8: nil module request")
	}
	if err := r.check(); err != nil {
		return nil, err
	}
	if s == nil || s.iso != r.iso {
		return nil, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_module_request_import_attributes").Call(
		r.iso.handleAssumingCheck(), sh, r.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("ModuleRequest.ImportAttributes", r1)
	}
	return &FixedArray{Data: Data{iso: r.iso, sc: s, h: out}}, nil
}

// FixedArray is V8's fixed-sized, read-only array of Data values.
type FixedArray struct{ Data }

// ModuleRequests returns a module's direct requests as their native
// FixedArray metadata container. The returned local belongs to s.
func (m *Module) ModuleRequests(s *Scope) (*FixedArray, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	if s == nil || s.iso != m.iso {
		return nil, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_module_requests_fixed_array").Call(
		m.iso.handleAssumingCheck(), sh, m.handle, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("Module.ModuleRequests", r1)
	}
	return &FixedArray{Data: Data{iso: m.iso, sc: s, h: out}}, nil
}

// Length returns the number of elements.
func (a *FixedArray) Length() (int, error) {
	if a == nil {
		return 0, fmt.Errorf("gov8: nil fixed array")
	}
	if err := a.check(); err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_fixed_array_length").Call(a.iso.handleAssumingCheck(), a.h)
	if int64(r1) < 0 {
		return 0, shimError("FixedArray.Length", r1)
	}
	return int(r1), nil
}

// Get returns the element at index. Out-of-range indices return ok=false and
// never enter V8's unchecked FixedArray::Get path.
func (a *FixedArray) Get(s *Scope, index int) (Data, bool, error) {
	if a == nil {
		return Data{}, false, fmt.Errorf("gov8: nil fixed array")
	}
	if err := a.check(); err != nil {
		return Data{}, false, err
	}
	if s == nil || s.iso != a.iso {
		return Data{}, false, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Data{}, false, err
	}
	length, err := a.Length()
	if err != nil {
		return Data{}, false, err
	}
	if index < 0 || index >= length {
		return Data{}, false, nil
	}
	var out uintptr
	r1, _, _ := proc("gov8_fixed_array_get").Call(
		a.iso.handleAssumingCheck(), sh, a.h, uintptr(index), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Data{}, false, shimError("FixedArray.Get", r1)
	}
	return Data{iso: a.iso, sc: s, h: out}, true, nil
}

// PrimitiveArray is V8's fixed-sized mutable array of primitive Values.
type PrimitiveArray struct{ Data }

// NewPrimitiveArray creates a primitive array initialized with undefined.
// rusty_v8 accepts usize and truncates it to C int. gov8 preserves every safe
// non-negative result of that conversion (for example 2^32 becomes zero), but
// rejects Go-negative inputs and conversions whose int32 result is negative,
// which are process-fatal in V8.
func NewPrimitiveArray(s *Scope, length int) (*PrimitiveArray, error) {
	if s == nil {
		return nil, fmt.Errorf("gov8: nil scope")
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, err
	}
	converted := int32(length)
	if length < 0 || converted < 0 {
		return nil, fmt.Errorf("gov8: primitive array length %d converts to negative C int %d", length, converted)
	}
	var out uintptr
	r1, _, _ := proc("gov8_primitive_array_new").Call(
		s.iso.handleAssumingCheck(), sh, uintptr(converted), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewPrimitiveArray", r1)
	}
	return &PrimitiveArray{Data: Data{iso: s.iso, sc: s, h: out}}, nil
}

// Length returns the number of slots.
func (a *PrimitiveArray) Length() (int, error) {
	if a == nil {
		return 0, fmt.Errorf("gov8: nil primitive array")
	}
	if err := a.check(); err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_primitive_array_length").Call(a.iso.handleAssumingCheck(), a.h)
	if int64(r1) < 0 {
		return 0, shimError("PrimitiveArray.Length", r1)
	}
	return int(r1), nil
}

func (a *PrimitiveArray) checkAccess(s *Scope, index int) (uintptr, bool, error) {
	if a == nil {
		return 0, false, fmt.Errorf("gov8: nil primitive array")
	}
	if err := a.check(); err != nil {
		return 0, false, err
	}
	if s == nil || s.iso != a.iso {
		return 0, false, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return 0, false, err
	}
	length, err := a.Length()
	if err != nil {
		return 0, false, err
	}
	if index < 0 || index >= length {
		return sh, false, nil
	}
	return sh, true, nil
}

// Get returns the primitive at index. Out-of-range indices return ok=false
// instead of reaching V8's process-fatal API check.
func (a *PrimitiveArray) Get(s *Scope, index int) (Value, bool, error) {
	sh, valid, err := a.checkAccess(s, index)
	if err != nil || !valid {
		return Value{}, false, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_primitive_array_get").Call(
		a.iso.handleAssumingCheck(), sh, a.h, uintptr(index), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, false, shimError("PrimitiveArray.Get", r1)
	}
	return Value{iso: a.iso, sc: s, h: out}, true, nil
}

// Set replaces the primitive at index. ok=false reports an out-of-range
// index; non-primitive and cross-isolate values return an error.
func (a *PrimitiveArray) Set(s *Scope, index int, item Value) (bool, error) {
	sh, valid, err := a.checkAccess(s, index)
	if err != nil || !valid {
		return false, err
	}
	if err := item.check(); err != nil {
		return false, err
	}
	if item.iso != a.iso {
		return false, foreignIsolate("item")
	}
	primitive, err := item.IsPrimitive()
	if err != nil {
		return false, err
	}
	if !primitive {
		return false, fmt.Errorf("gov8: primitive array item is not primitive")
	}
	r1, _, _ := proc("gov8_primitive_array_set").Call(
		a.iso.handleAssumingCheck(), sh, a.h, uintptr(index), item.h)
	if int64(r1) < 0 {
		return false, shimError("PrimitiveArray.Set", r1)
	}
	return true, nil
}

// IsPrimitive reports whether the value is undefined, null, a boolean,
// string, symbol, number, or bigint.
func (v Value) IsPrimitive() (bool, error) {
	return v.predicate("gov8_value_is_primitive")
}

// PrimitiveArrayGlobal is a strong persistent handle to a PrimitiveArray.
// It permits reopening the array in a later scope or context of the same
// isolate, matching Global<PrimitiveArray> in rusty_v8.
type PrimitiveArrayGlobal struct {
	iso    *Isolate
	cell   uintptr
	closed bool
}

// NewPrimitiveArrayGlobal roots array in a persistent cell.
func NewPrimitiveArrayGlobal(s *Scope, array *PrimitiveArray) (*PrimitiveArrayGlobal, error) {
	if array == nil {
		return nil, fmt.Errorf("gov8: nil primitive array")
	}
	if err := array.check(); err != nil {
		return nil, err
	}
	if s == nil || s.iso != array.iso {
		return nil, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	cell, err := callHandle("PrimitiveArrayGlobal.New", proc("gov8_global_new"),
		array.iso.handleAssumingCheck(), sh, array.h)
	if err != nil {
		return nil, err
	}
	return &PrimitiveArrayGlobal{iso: array.iso, cell: cell}, nil
}

func (g *PrimitiveArrayGlobal) check() error {
	if g == nil {
		return fmt.Errorf("gov8: nil primitive array global")
	}
	if g.closed {
		return fmt.Errorf("gov8: primitive array global used after Close")
	}
	return g.iso.check()
}

// ToLocal reopens the persistent array in s.
func (g *PrimitiveArrayGlobal) ToLocal(s *Scope) (*PrimitiveArray, error) {
	if err := g.check(); err != nil {
		return nil, err
	}
	if s == nil || s.iso != g.iso {
		return nil, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	var out uintptr
	if err := callErr("PrimitiveArrayGlobal.ToLocal", proc("gov8_global_to_local"),
		g.iso.handleAssumingCheck(), sh, g.cell, uintptr(unsafe.Pointer(&out))); err != nil {
		return nil, err
	}
	return &PrimitiveArray{Data: Data{iso: g.iso, sc: s, h: out}}, nil
}

// Close releases the persistent cell. Closing after the isolate is a safe
// no-op, consistent with the module's generic Global handle.
func (g *PrimitiveArrayGlobal) Close() error {
	if g == nil {
		return fmt.Errorf("gov8: nil primitive array global")
	}
	if g.closed {
		return fmt.Errorf("gov8: primitive array global already closed")
	}
	if isolateClosed(g.iso) {
		g.closed = true
		return nil
	}
	if err := g.iso.check(); err != nil {
		return err
	}
	err := callErr("PrimitiveArrayGlobal.Close", proc("gov8_global_reset"), g.iso.handleAssumingCheck(), g.cell)
	g.closed = true
	return err
}
