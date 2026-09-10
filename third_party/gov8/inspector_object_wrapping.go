//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// InspectorRemoteObject is an owned Inspector Runtime.RemoteObject. Its
// protocol representation is independent of the session, Inspector, context,
// and isolate that produced it. ToBytes and Close therefore remain usable
// after those resources have been closed and have no isolate-thread affinity.
type InspectorRemoteObject struct {
	mu     sync.Mutex
	handle uintptr
	closed bool
}

// InspectorObjectIDErrorKind classifies failures returned by UnwrapObject.
type InspectorObjectIDErrorKind uint8

const (
	InspectorObjectIDErrorUnknown InspectorObjectIDErrorKind = iota
	InspectorObjectIDInvalid
	InspectorObjectIDNotFound
)

// InspectorObjectIDError owns the UTF-16 Inspector diagnostic associated
// with an invalid or released/missing remote object ID.
type InspectorObjectIDError struct {
	kind    InspectorObjectIDErrorKind
	message InspectorStringView
}

func (e *InspectorObjectIDError) Error() string {
	if e == nil {
		return "gov8: nil Inspector object-ID error"
	}
	return e.message.String()
}

// Kind reports whether the ID was malformed or referred to an object that is
// no longer present. Unknown preserves future Inspector diagnostics safely.
func (e *InspectorObjectIDError) Kind() InspectorObjectIDErrorKind {
	if e == nil {
		return InspectorObjectIDErrorUnknown
	}
	return e.kind
}

// Message returns an owned copy of the Inspector diagnostic string.
func (e *InspectorObjectIDError) Message() InspectorStringView {
	if e == nil {
		return EmptyInspectorStringView()
	}
	return NewInspectorStringBuffer(e.message).StringView()
}

func classifyInspectorObjectIDError(message InspectorStringView) InspectorObjectIDErrorKind {
	switch message.String() {
	case "Invalid remote object id":
		return InspectorObjectIDInvalid
	case "Could not find object with given id":
		return InspectorObjectIDNotFound
	default:
		return InspectorObjectIDErrorUnknown
	}
}

// WrapObject wraps value using the inspected context selected by context and
// this session's context group. present is false when no matching inspected
// context has been registered. The object group preserves 8-bit, UTF-16, and
// embedded-NUL input exactly at the native boundary.
func (s *InspectorSession) WrapObject(scope *Scope, context *Context, value Value,
	objectGroup InspectorStringView, generatePreview bool) (*InspectorRemoteObject, bool, error) {
	if err := s.check(); err != nil {
		return nil, false, err
	}
	iso := s.inspector.iso
	if scope == nil || scope.iso != iso {
		return nil, false, foreignIsolate("scope")
	}
	if err := scope.check(); err != nil {
		return nil, false, err
	}
	if context == nil || context.iso != iso {
		return nil, false, foreignIsolate("context")
	}
	if err := context.checkAssumingIsolate(); err != nil {
		return nil, false, err
	}
	if value.iso != iso {
		return nil, false, foreignIsolate("value")
	}
	if err := value.check(); err != nil {
		return nil, false, err
	}
	is8, data, length := objectGroup.native()
	var out uintptr
	preview := uintptr(0)
	if generatePreview {
		preview = 1
	}
	err := callErr("InspectorSession.WrapObject", proc("gov8_iow_wrap_object"),
		s.handle, iso.handleAssumingCheck(), scope.handle, context.handle, value.h,
		is8, data, length, preview, uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(objectGroup)
	if err != nil {
		return nil, false, err
	}
	if out == 0 {
		return nil, false, nil
	}
	return &InspectorRemoteObject{handle: out}, true, nil
}

// UnwrapObject resolves an Inspector-generated ID. The returned Value is
// copied into scope immediately. Context is the already-registered Go Context
// whose persistent native context matches the Inspector result; this method
// never fabricates a second owning wrapper. ObjectGroup is always non-nil on
// success, including for Some("").
func (s *InspectorSession) UnwrapObject(scope *Scope, objectID InspectorStringView) (
	Value, *Context, *InspectorStringBuffer, error) {
	if err := s.check(); err != nil {
		return Value{}, nil, nil, err
	}
	iso := s.inspector.iso
	if scope == nil || scope.iso != iso {
		return Value{}, nil, nil, foreignIsolate("scope")
	}
	if err := scope.check(); err != nil {
		return Value{}, nil, nil, err
	}
	if err := scope.requireCurrent(); err != nil {
		return Value{}, nil, nil, err
	}

	contexts := make([]*Context, 0, len(s.inspector.contexts))
	contextHandles := make([]uintptr, 0, len(s.inspector.contexts))
	for context := range s.inspector.contexts {
		if context == nil || context.iso != iso {
			return Value{}, nil, nil, errors.New("gov8: Inspector has an invalid registered context")
		}
		if err := context.checkAssumingIsolate(); err != nil {
			return Value{}, nil, nil, err
		}
		contexts = append(contexts, context)
		contextHandles = append(contextHandles, context.handle)
	}

	is8, data, length := objectID.native()
	var valueRaw, contextIndex, groupRaw, errorRaw uintptr
	err := callErr("InspectorSession.UnwrapObject", proc("gov8_iow_unwrap_object"),
		s.handle, iso.handleAssumingCheck(), scope.handle, is8, data, length,
		slicePointer(contextHandles), uintptr(len(contextHandles)),
		uintptr(unsafe.Pointer(&valueRaw)), uintptr(unsafe.Pointer(&contextIndex)),
		uintptr(unsafe.Pointer(&groupRaw)), uintptr(unsafe.Pointer(&errorRaw)))
	runtime.KeepAlive(objectID)
	runtime.KeepAlive(contextHandles)
	if err != nil {
		return Value{}, nil, nil, err
	}
	if errorRaw != 0 {
		message, copyErr := copyInspectorOwnedString(errorRaw)
		if copyErr != nil {
			return Value{}, nil, nil, copyErr
		}
		view := message.StringView()
		return Value{}, nil, nil, &InspectorObjectIDError{
			kind: classifyInspectorObjectIDError(view), message: view,
		}
	}
	if valueRaw == 0 || groupRaw == 0 || contextIndex >= uintptr(len(contexts)) {
		if groupRaw != 0 {
			_ = callErr("InspectorStringBuffer.Close", proc("gov8_iow_string_delete"), groupRaw)
		}
		return Value{}, nil, nil, errors.New("gov8: Inspector UnwrapObject returned an invalid result")
	}
	group, err := copyInspectorOwnedString(groupRaw)
	if err != nil {
		return Value{}, nil, nil, err
	}
	return Value{iso: iso, sc: scope, h: valueRaw}, contexts[contextIndex], group, nil
}

func copyInspectorOwnedString(raw uintptr) (result *InspectorStringBuffer, err error) {
	if raw == 0 {
		return nil, errors.New("gov8: null owned Inspector string")
	}
	defer func() {
		closeErr := callErr("InspectorStringBuffer.Close", proc("gov8_iow_string_delete"), raw)
		if closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}()
	var is8Word, lengthWord uintptr
	if err = callErr("InspectorStringBuffer.Info", proc("gov8_iow_string_info"), raw,
		uintptr(unsafe.Pointer(&is8Word)), uintptr(unsafe.Pointer(&lengthWord))); err != nil {
		return nil, err
	}
	if is8Word != 0 && is8Word != 1 {
		return nil, fmt.Errorf("gov8: invalid Inspector string encoding %d", is8Word)
	}
	maxInt := uintptr(^uint(0) >> 1)
	if lengthWord > maxInt {
		return nil, errors.New("gov8: Inspector string length exceeds max int")
	}
	length := int(lengthWord)
	if is8Word != 0 {
		data := make([]byte, length)
		if err = callErr("InspectorStringBuffer.Copy", proc("gov8_iow_string_copy"),
			raw, slicePointer(data), uintptr(len(data))); err != nil {
			return nil, err
		}
		return NewInspectorStringBuffer(NewInspectorStringView8(data)), nil
	}
	if lengthWord > maxInt/2 {
		return nil, errors.New("gov8: Inspector UTF-16 string byte length exceeds max int")
	}
	units := make([]uint16, length)
	if err = callErr("InspectorStringBuffer.Copy", proc("gov8_iow_string_copy"),
		raw, slicePointer(units), uintptr(len(units)*2)); err != nil {
		return nil, err
	}
	return NewInspectorStringBuffer(NewInspectorStringView16(units)), nil
}

// ToBytes returns a fresh copy of the RemoteObject's CRDTP/CBOR encoding.
func (r *InspectorRemoteObject) ToBytes() ([]byte, error) {
	if r == nil {
		return nil, errors.New("gov8: nil Inspector remote object")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.handle == 0 {
		return nil, errors.New("gov8: Inspector remote object used after Close")
	}
	var lengthWord uintptr
	status, _, _ := proc("gov8_iow_remote_to_bytes").Call(
		r.handle, 0, 0, uintptr(unsafe.Pointer(&lengthWord)))
	if int64(status) < 0 && int64(status) != -4 {
		return nil, shimError("InspectorRemoteObject.ToBytes", status)
	}
	if lengthWord > uintptr(^uint(0)>>1) {
		return nil, errors.New("gov8: Inspector remote object length exceeds max int")
	}
	result := make([]byte, int(lengthWord))
	if err := callErr("InspectorRemoteObject.ToBytes", proc("gov8_iow_remote_to_bytes"),
		r.handle, slicePointer(result), uintptr(len(result)),
		uintptr(unsafe.Pointer(&lengthWord))); err != nil {
		return nil, err
	}
	if lengthWord != uintptr(len(result)) {
		return nil, errors.New("gov8: Inspector remote object serialization length changed")
	}
	return result, nil
}

// Close releases the owned protocol object. It is valid after isolate
// disposal and is synchronized with ToBytes.
func (r *InspectorRemoteObject) Close() error {
	if r == nil {
		return errors.New("gov8: nil Inspector remote object")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.handle == 0 {
		return errors.New("gov8: Inspector remote object already closed")
	}
	if err := callErr("InspectorRemoteObject.Close", proc("gov8_iow_remote_delete"), r.handle); err != nil {
		return err
	}
	r.closed = true
	r.handle = 0
	return nil
}
