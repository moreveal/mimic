//go:build windows && amd64

package gov8

import (
	"fmt"
	"unsafe"
)

// NewObjectWithPrototypeAndProperties creates an object with the supplied
// prototype and own data properties. Each name must be a String or Symbol;
// every local must be live and belong to this scope's isolate. Properties are
// writable, enumerable, and configurable, matching
// v8::Object::with_prototype_and_properties.
//
// Unlike the Rust wrapper's assert_eq!, unequal name/value lengths are
// reported as an error before entering V8.
func (s *Scope) NewObjectWithPrototypeAndProperties(c *Context, prototype Value, names, values []Value) (*Object, error) {
	if err := s.check(); err != nil {
		return nil, err
	}
	if c == nil || c.iso != s.iso {
		return nil, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	if err := prototype.check(); err != nil {
		return nil, err
	}
	if prototype.iso != s.iso {
		return nil, foreignIsolate("prototype")
	}
	if len(names) != len(values) {
		return nil, fmt.Errorf("gov8: names and values have different lengths")
	}

	// Keep non-nil storage even for zero properties. V8 ignores both arrays
	// when length is zero, while the shim still validates their addresses.
	nameWires := make([]uintptr, len(names)+1)
	valueWires := make([]uintptr, len(values)+1)
	for index := range names {
		if err := names[index].requireName(); err != nil {
			return nil, fmt.Errorf("gov8: name %d: %w", index, err)
		}
		if names[index].iso != s.iso {
			return nil, foreignIsolate("name")
		}
		if err := values[index].check(); err != nil {
			return nil, fmt.Errorf("gov8: value %d: %w", index, err)
		}
		if values[index].iso != s.iso {
			return nil, foreignIsolate("value")
		}
		nameWires[index] = names[index].h
		valueWires[index] = values[index].h
	}

	var out uintptr
	r1, _, _ := proc("gov8_or_object_new_with_prototype_and_properties").Call(
		s.iso.handleAssumingCheck(), c.handle, sh, prototype.h,
		uintptr(unsafe.Pointer(&nameWires[0])), uintptr(unsafe.Pointer(&valueWires[0])),
		uintptr(len(names)), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewObjectWithPrototypeAndProperties", r1)
	}
	return &Object{Value: Value{iso: s.iso, sc: s, h: out}}, nil
}

// GetOwnPropertyNames returns only own property names, applying the pinned
// Object::get_own_property_names filter and numeric-key conversion. Unlike
// GetPropertyNames, this upstream API has no prototype or index-filter knobs.
func (o *Object) GetOwnPropertyNames(s *Scope, c *Context, propertyFilter PropertyFilter, conversion KeyConversionMode) (*Array, error) {
	if propertyFilter&^PropertyFilter(0x1f) != 0 {
		return nil, fmt.Errorf("gov8: invalid property filter")
	}
	if conversion > KeyConversionNoNumbers {
		return nil, fmt.Errorf("gov8: invalid key conversion mode")
	}
	sh, err := o.receiverArgs(s, c)
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_or_object_get_own_property_names").Call(
		o.iso.handleAssumingCheck(), c.handle, sh, o.h, uintptr(propertyFilter),
		uintptr(conversion), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("Object.GetOwnPropertyNames", r1)
	}
	return &Array{Value: Value{iso: o.iso, sc: s, h: out}}, nil
}

// PreviewEntries returns V8's debugger-style entry snapshot for Map, Set,
// WeakMap, WeakSet, and their iterators. present=false is the upstream empty
// Option for unsupported receivers. keyValue reports whether adjacent array
// elements form key/value pairs. Go takes c explicitly because Scope does not
// itself carry the current Context; Rust's PinScope supplies that implicitly.
func (o *Object) PreviewEntries(s *Scope, c *Context) (entries *Array, keyValue, present bool, err error) {
	sh, err := o.receiverArgs(s, c)
	if err != nil {
		return nil, false, false, err
	}
	var out uintptr
	var kv int32
	r1, _, _ := proc("gov8_or_object_preview_entries").Call(
		o.iso.handleAssumingCheck(), c.handle, sh, o.h, uintptr(unsafe.Pointer(&out)),
		uintptr(unsafe.Pointer(&kv)))
	if int64(r1) < 0 {
		return nil, false, false, shimError("Object.PreviewEntries", r1)
	}
	if out == 0 {
		return nil, kv == 1, false, nil
	}
	return &Array{Value: Value{iso: o.iso, sc: s, h: out}}, kv == 1, true, nil
}

// IsAPIWrapper reports V8's embedder-wrapper classification. Objects merely
// having internal fields need not be API wrappers; V8 decides the category.
func (o *Object) IsAPIWrapper() (bool, error) {
	if err := o.check(); err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_or_object_is_api_wrapper").Call(
		o.iso.handleAssumingCheck(), o.h)
	if int64(r1) < 0 {
		return false, shimError("Object.IsAPIWrapper", r1)
	}
	return r1 == 1, nil
}
