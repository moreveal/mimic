//go:build windows && amd64

package gov8

import (
	"fmt"
	"runtime"
	"unsafe"
)

// ReThrow rethrows the caught exception and immediately closes the innermost
// TryCatch before returning. The returned local is V8's observable undefined
// result; the original exception has already propagated to the next outer
// TryCatch. This is the safe counterpart to legacy Rethrow, whose caller had
// to manually leave and close the catcher immediately.
func (t *TryCatch) ReThrow(s *Scope) (Value, bool, error) {
	if err := t.check(); err != nil {
		return Value{}, false, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, false, err
	}
	if s.iso != t.iso {
		return Value{}, false, foreignIsolate("scope")
	}
	var out uintptr
	r, _, _ := proc("gov8_tlr_tc_rethrow_and_close").Call(
		t.iso.handleAssumingCheck(), t.handle, sh, uintptr(unsafe.Pointer(&out)))
	if int64(r) < 0 {
		return Value{}, false, shimError("TryCatch.ReThrow", r)
	}
	t.closed = true
	t.handle = 0
	if out == 0 {
		return Value{}, false, nil
	}
	return Value{iso: t.iso, sc: s, h: out}, true, nil
}

// AsMessage exposes the complete safe Message getter surface for a listener
// callback. The returned handle remains valid only for the callback scope.
func (m *CallbackMessage) AsMessage() (*Message, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return &Message{iso: m.iso, sc: m.sc, h: m.h}, nil
}

// CreateMessage reconstructs a Message for exception in the callback's local
// scope, allowing listener code to compare it with the delivered Message.
func (m *CallbackMessage) CreateMessage(c *Context, exception Value) (*Message, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	if c == nil || c.iso != m.iso {
		return nil, foreignIsolate("context")
	}
	return c.CreateMessage(m.sc, exception)
}

// SameIdentity reports V8 Local<Message> identity, not merely equal text.
func (m *Message) SameIdentity(other *Message) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	if err := other.check(); err != nil {
		return false, err
	}
	if other.iso != m.iso {
		return false, foreignIsolate("message")
	}
	r, _, _ := proc("gov8_tlr_message_same").Call(
		m.iso.handleAssumingCheck(), m.sc.handle, m.h, other.h)
	if int64(r) < 0 {
		return false, shimError("Message.SameIdentity", r)
	}
	return r == 1, nil
}

// CompileUncaughtWithOrigin compiles without an internal fallback TryCatch,
// allowing syntax errors to reach message listeners. ResourceNameValue is
// rejected because this first residual listener slice only exposes the safe
// string-origin bridge used by the pinned oracle.
func (c *Context) CompileUncaughtWithOrigin(s *Scope, source string, origin *Origin) (*Script, error) {
	if c == nil {
		return nil, fmt.Errorf("gov8: nil context")
	}
	if s == nil {
		return nil, fmt.Errorf("gov8: nil scope")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, err
	}
	if s.iso != c.iso {
		return nil, foreignIsolate("scope")
	}
	if uint64(len(source)) > uint64(^uint32(0)>>1) {
		return nil, fmt.Errorf("gov8: source exceeds V8 string length")
	}
	var hasOrigin, flags uintptr
	var name, sourceMap string
	var line, column, scriptID int32
	if origin != nil {
		if origin.ResourceNameValue.h != 0 {
			return nil, fmt.Errorf("gov8: CompileUncaughtWithOrigin does not accept ResourceNameValue")
		}
		if origin.IsModule {
			return nil, fmt.Errorf("gov8: CompileUncaughtWithOrigin does not accept module origins")
		}
		hasOrigin = 1
		name, sourceMap = origin.ResourceName, origin.SourceMapURL
		line, column, scriptID = origin.LineOffset, origin.ColumnOffset, origin.ScriptID
		if origin.IsOpaque {
			flags |= 1
		}
		if origin.IsSharedCrossOrigin {
			flags |= 2
		}
		if origin.IsWasm {
			flags |= 4
		}
	}
	if uint64(len(name)) > uint64(^uint32(0)>>1) || uint64(len(sourceMap)) > uint64(^uint32(0)>>1) {
		return nil, fmt.Errorf("gov8: origin string exceeds V8 string length")
	}
	var out uintptr
	sourceBytes, nameBytes, sourceMapBytes := []byte(source), []byte(name), []byte(sourceMap)
	sp, np, mp := bytesPtr(sourceBytes), bytesPtr(nameBytes), bytesPtr(sourceMapBytes)
	r, _, _ := proc("gov8_tlr_compile_uncaught_origin").Call(
		c.iso.handleAssumingCheck(), c.handle, sh, sp, uintptr(len(source)),
		hasOrigin, np, uintptr(len(name)), uintptr(line), uintptr(column),
		uintptr(scriptID), mp, uintptr(len(sourceMap)), flags,
		uintptr(unsafe.Pointer(&out)))
	runtime.KeepAlive(sourceBytes)
	runtime.KeepAlive(nameBytes)
	runtime.KeepAlive(sourceMapBytes)
	if int64(r) < 0 {
		return nil, shimError("Context.CompileUncaughtWithOrigin", r)
	}
	return &Script{iso: c.iso, ctx: c, handle: out}, nil
}
