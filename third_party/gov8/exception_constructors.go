//go:build windows && amd64

package gov8

import (
	"fmt"
	"unsafe"
)

// The pinned Exception constructors consult the current V8 context and can
// access-violate when called without one. The Go surface therefore makes the
// Context explicit and validates it, the Scope, and all local values before
// crossing the ABI.

type exceptionConstructorKind uint8

const (
	exceptionError exceptionConstructorKind = iota
	exceptionRangeError
	exceptionReferenceError
	exceptionSyntaxError
	exceptionTypeError
)

func (c *Context) exceptionScopeHandle(s *Scope) (uintptr, error) {
	if c == nil {
		return 0, fmt.Errorf("gov8: nil context")
	}
	if s == nil {
		return 0, fmt.Errorf("gov8: nil scope")
	}
	if err := c.check(); err != nil {
		return 0, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return 0, err
	}
	if s.iso != c.iso {
		return 0, foreignIsolate("scope")
	}
	return sh, nil
}

func (c *Context) newExceptionWithStringHandle(s *Scope, sh uintptr, message Value, kind exceptionConstructorKind) (Value, error) {
	var out uintptr
	r1, _, _ := proc("gov8_ec_exception_new").Call(
		c.iso.handleAssumingCheck(), c.handle, sh, uintptr(kind), message.h,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("Context.NewException", r1)
	}
	return Value{iso: c.iso, sc: s, h: out}, nil
}

func (c *Context) newException(s *Scope, message string, kind exceptionConstructorKind) (Value, error) {
	sh, err := c.exceptionScopeHandle(s)
	if err != nil {
		return Value{}, err
	}
	// String::NewFromUtf8 takes a signed 32-bit length in the pinned V8 ABI.
	if uint64(len(message)) > uint64(^uint32(0)>>1) {
		return Value{}, fmt.Errorf("gov8: exception message exceeds V8 string length")
	}
	msg, err := s.newStringAssumingCheck(message)
	if err != nil {
		return Value{}, err
	}
	// NewString produced a live String in this scope, so the hot Go-string
	// path can use the common native helper without a redundant IsString FFI.
	return c.newExceptionWithStringHandle(s, sh, msg, kind)
}

func (c *Context) newExceptionFromStringValue(s *Scope, message Value, kind exceptionConstructorKind) (Value, error) {
	sh, err := c.exceptionScopeHandle(s)
	if err != nil {
		return Value{}, err
	}
	if message.h == 0 {
		return Value{}, fmt.Errorf("gov8: zero exception message handle")
	}
	if message.iso != c.iso {
		return Value{}, foreignIsolate("exception message")
	}
	if err := message.check(); err != nil {
		return Value{}, err
	}
	isString, err := message.IsString()
	if err != nil {
		return Value{}, err
	}
	if !isString {
		return Value{}, fmt.Errorf("gov8: exception message is not a String")
	}
	return c.newExceptionWithStringHandle(s, sh, message, kind)
}

// NewError constructs an Error in c. The returned value is local to s.
func (c *Context) NewError(s *Scope, message string) (Value, error) {
	return c.newException(s, message, exceptionError)
}

// NewRangeError constructs a RangeError in c. The returned value is local to s.
func (c *Context) NewRangeError(s *Scope, message string) (Value, error) {
	return c.newException(s, message, exceptionRangeError)
}

// NewReferenceError constructs a ReferenceError in c. The returned value is local to s.
func (c *Context) NewReferenceError(s *Scope, message string) (Value, error) {
	return c.newException(s, message, exceptionReferenceError)
}

// NewSyntaxError constructs a SyntaxError in c. The returned value is local to s.
func (c *Context) NewSyntaxError(s *Scope, message string) (Value, error) {
	return c.newException(s, message, exceptionSyntaxError)
}

// NewTypeError constructs a TypeError in c. The returned value is local to s.
func (c *Context) NewTypeError(s *Scope, message string) (Value, error) {
	return c.newException(s, message, exceptionTypeError)
}

// NewErrorFromStringValue passes an existing scope-local V8 String directly to
// the Error constructor without a Go UTF-8 round-trip or coercion. This
// preserves exact UTF-16 contents and representation, including lone
// surrogates and external backing resources.
func (c *Context) NewErrorFromStringValue(s *Scope, message Value) (Value, error) {
	return c.newExceptionFromStringValue(s, message, exceptionError)
}

// NewRangeErrorFromStringValue passes an existing scope-local V8 String to the
// RangeError constructor without a Go UTF-8 round-trip or coercion.
func (c *Context) NewRangeErrorFromStringValue(s *Scope, message Value) (Value, error) {
	return c.newExceptionFromStringValue(s, message, exceptionRangeError)
}

// NewReferenceErrorFromStringValue passes an existing scope-local V8 String to
// the ReferenceError constructor without a Go UTF-8 round-trip or coercion.
func (c *Context) NewReferenceErrorFromStringValue(s *Scope, message Value) (Value, error) {
	return c.newExceptionFromStringValue(s, message, exceptionReferenceError)
}

// NewSyntaxErrorFromStringValue passes an existing scope-local V8 String to the
// SyntaxError constructor without a Go UTF-8 round-trip or coercion.
func (c *Context) NewSyntaxErrorFromStringValue(s *Scope, message Value) (Value, error) {
	return c.newExceptionFromStringValue(s, message, exceptionSyntaxError)
}

// NewTypeErrorFromStringValue passes an existing scope-local V8 String to the
// TypeError constructor without a Go UTF-8 round-trip or coercion.
func (c *Context) NewTypeErrorFromStringValue(s *Scope, message Value) (Value, error) {
	return c.newExceptionFromStringValue(s, message, exceptionTypeError)
}

func (c *Context) exceptionArgs(s *Scope, exception Value) (uintptr, error) {
	if c == nil {
		return 0, fmt.Errorf("gov8: nil context")
	}
	if s == nil {
		return 0, fmt.Errorf("gov8: nil scope")
	}
	if err := c.check(); err != nil {
		return 0, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return 0, err
	}
	if s.iso != c.iso {
		return 0, foreignIsolate("scope")
	}
	if err := exception.check(); err != nil {
		return 0, err
	}
	if exception.iso != c.iso {
		return 0, foreignIsolate("exception")
	}
	return sh, nil
}

// CreateMessage reconstructs V8's Message for exception. It accepts native
// errors, primitives, and arbitrary JavaScript values. The result is local to
// s and can recover the current JS source location when called in a callback.
func (c *Context) CreateMessage(s *Scope, exception Value) (*Message, error) {
	sh, err := c.exceptionArgs(s, exception)
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_ec_exception_create_message").Call(
		c.iso.handleAssumingCheck(), c.handle, sh, exception.h,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("Context.CreateMessage", r1)
	}
	return &Message{iso: c.iso, sc: s, h: out}, nil
}

// GetExceptionStackTrace returns the structured stack trace attached to
// exception. ok is false when the value carries none.
func (c *Context) GetExceptionStackTrace(s *Scope, exception Value) (*StackTrace, bool, error) {
	sh, err := c.exceptionArgs(s, exception)
	if err != nil {
		return nil, false, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_ec_exception_get_stack_trace").Call(
		c.iso.handleAssumingCheck(), c.handle, sh, exception.h,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, false, shimError("Context.GetExceptionStackTrace", r1)
	}
	if out == 0 {
		return nil, false, nil
	}
	return &StackTrace{iso: c.iso, sc: s, h: out}, true, nil
}
