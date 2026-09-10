//go:build windows && amd64

package gov8

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// Exception details and stack capture, mirroring the pinned crate's
// exception.rs observably:
//
//   - TryCatch.Message opens the caught exception's Message as a scope-local
//     handle; every getter validates the owning scope exactly like other
//     scope-local values here.
//   - StackTrace.CurrentStackTrace / Message.StackTrace / the exception
//     forms produce scope-local StackTrace handles. Frame validates both
//     bounds before calling V8: the pinned binding returns a non-empty but
//     fatally invalid handle at index == FrameCount.
//   - CaptureStackTrace attaches a ".stack" property to a plain object;
//     GetStackTrace reports the exception-attached trace; the isolate-level
//     capture toggle mirrors
//     Isolate::SetCaptureStackTraceForUncaughtExceptions.
//
// Message and trace handles are valid while their Scope is open and on the
// isolate's owning thread.

// Message is the engine's error message for a caught exception.
type Message struct {
	iso *Isolate
	sc  *Scope
	h   uintptr
}

func (m *Message) check() error {
	if m == nil {
		return fmt.Errorf("gov8: nil message")
	}
	if m.h == 0 {
		return fmt.Errorf("gov8: no message present")
	}
	return m.sc.check()
}

// Message returns the caught exception's message, ok=false when the
// TryCatch holds nothing. The message is a scope-local handle: it must be
// read while the scope is open.
func (t *TryCatch) Message(s *Scope) (*Message, bool, error) {
	if err := t.check(); err != nil {
		return nil, false, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, false, err
	}
	if s.iso != t.iso {
		return nil, false, foreignIsolate("scope")
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_tc_message").Call(
		t.iso.handleAssumingCheck(), t.handle, sh, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, false, shimError("TryCatch.Message", r1)
	}
	if out == 0 {
		return nil, false, nil
	}
	return &Message{iso: t.iso, sc: s, h: out}, true, nil
}

// Exception returns the caught exception as a scope-local value. ok is false
// when the TryCatch is empty. The explicit Scope is the Go representation of
// rusty_v8's parent-scope lifetime: the returned value remains valid after the
// TryCatch closes, but not after s closes.
func (t *TryCatch) Exception(s *Scope) (Value, bool, error) {
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
	r1, _, _ := proc("gov8_ea_tc_exception").Call(
		t.iso.handleAssumingCheck(), t.handle, sh, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, false, shimError("TryCatch.Exception", r1)
	}
	if out == 0 {
		return Value{}, false, nil
	}
	return Value{iso: t.iso, sc: s, h: out}, true, nil
}

// Rethrow schedules the caught exception for the next outer handler and
// returns V8's scope-local ReThrow result. In the pinned build that result is
// undefined, while the original exception propagates to the outer TryCatch.
// The caller must leave engine execution immediately after this call.
func (t *TryCatch) Rethrow(s *Scope) (Value, bool, error) {
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
	r1, _, _ := proc("gov8_ea_tc_rethrow").Call(
		t.iso.handleAssumingCheck(), t.handle, sh, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, false, shimError("TryCatch.Rethrow", r1)
	}
	if out == 0 {
		return Value{}, false, nil
	}
	return Value{iso: t.iso, sc: s, h: out}, true, nil
}

// StackTrace returns the caught exception's .stack property as a scope-local
// value. ok is false when nothing was caught or no stack property is present.
// This is TryCatch::StackTrace, distinct from Message.StackTrace (which is a
// structured StackTrace captured only when isolate-level capture is enabled).
func (t *TryCatch) StackTrace(s *Scope, c *Context) (Value, bool, error) {
	if err := t.check(); err != nil {
		return Value{}, false, err
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return Value{}, false, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, false, err
	}
	if s.iso != t.iso {
		return Value{}, false, foreignIsolate("scope")
	}
	if c.iso != t.iso {
		return Value{}, false, foreignIsolate("context")
	}
	var out uintptr
	r1, _, _ := proc("gov8_ea_tc_stack_trace").Call(
		t.iso.handleAssumingCheck(), c.handle, t.handle, sh,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, false, shimError("TryCatch.StackTrace", r1)
	}
	if out == 0 {
		return Value{}, false, nil
	}
	return Value{iso: t.iso, sc: s, h: out}, true, nil
}

// Text returns Message::Get text (carries the "Uncaught " prefix for
// TryCatch-caught exceptions in this build).
func (m *Message) Text(c *Context) (string, error) {
	if err := m.check(); err != nil {
		return "", err
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return "", err
	}
	if c.iso != m.iso {
		return "", foreignIsolate("context")
	}
	return callTextFn("Message.Text", func(buf *byte, cap int, outLen *int64) uintptr {
		r, _, _ := proc("gov8_ca_message_text").Call(
			m.iso.handleAssumingCheck(), c.handle, m.sc.handle, m.h,
			uintptr(unsafe.Pointer(buf)), uintptr(cap), uintptr(unsafe.Pointer(outLen)))
		return r
	})
}

// TextValue returns Message::Get as its scope-local JavaScript String. Unlike
// Text it performs no UTF-8 conversion. The returned Value is local to the
// Message's Scope and remains usable after the originating TryCatch closes.
func (m *Message) TextValue() (Value, error) {
	if err := m.check(); err != nil {
		return Value{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_message_get_value").Call(
		m.iso.handleAssumingCheck(), m.sc.handle, m.h,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("Message.TextValue", r1)
	}
	if out == 0 {
		return Value{}, fmt.Errorf("gov8: Message.TextValue returned no value")
	}
	return Value{iso: m.iso, sc: m.sc, h: out}, nil
}

// LineNumber returns the 1-based line of the error; ok=false when absent.
func (m *Message) LineNumber(c *Context) (line int32, ok bool, err error) {
	if err := m.check(); err != nil {
		return 0, false, err
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return 0, false, err
	}
	if c.iso != m.iso {
		return 0, false, foreignIsolate("context")
	}
	var out, okv int32
	r1, _, _ := proc("gov8_ca_message_line_number").Call(
		m.iso.handleAssumingCheck(), c.handle, m.sc.handle, m.h,
		uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&okv)))
	if int64(r1) < 0 {
		return 0, false, shimError("Message.LineNumber", r1)
	}
	return out, okv == 1, nil
}

// SourceLine returns the source line the error points at; ok=false when
// absent.
func (m *Message) SourceLine(c *Context) (string, bool, error) {
	if err := m.check(); err != nil {
		return "", false, err
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return "", false, err
	}
	if c.iso != m.iso {
		return "", false, foreignIsolate("context")
	}
	var present int32
	text, err := callTextFn("Message.SourceLine", func(buf *byte, cap int, outLen *int64) uintptr {
		r, _, _ := proc("gov8_ca_message_source_line").Call(
			m.iso.handleAssumingCheck(), c.handle, m.sc.handle, m.h,
			uintptr(unsafe.Pointer(buf)), uintptr(cap), uintptr(unsafe.Pointer(outLen)),
			uintptr(unsafe.Pointer(&present)))
		return r
	})
	if err != nil {
		return "", false, err
	}
	return text, present == 1, nil
}

// SourceLineValue returns the source line as its scope-local JavaScript
// String. ok=false represents Option::None; an empty String returns ok=true.
func (m *Message) SourceLineValue(c *Context) (Value, bool, error) {
	if err := m.check(); err != nil {
		return Value{}, false, err
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return Value{}, false, err
	}
	if c.iso != m.iso {
		return Value{}, false, foreignIsolate("context")
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_message_source_line_value").Call(
		m.iso.handleAssumingCheck(), c.handle, m.sc.handle, m.h,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, false, shimError("Message.SourceLineValue", r1)
	}
	if out == 0 {
		return Value{}, false, nil
	}
	return Value{iso: m.iso, sc: m.sc, h: out}, true, nil
}

// ResourceName returns the script resource name ("" when absent).
func (m *Message) ResourceName(c *Context) (string, error) {
	if err := m.check(); err != nil {
		return "", err
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return "", err
	}
	if c.iso != m.iso {
		return "", foreignIsolate("context")
	}
	return callTextFn("Message.ResourceName", func(buf *byte, cap int, outLen *int64) uintptr {
		r, _, _ := proc("gov8_ca_message_resource_name").Call(
			m.iso.handleAssumingCheck(), c.handle, m.sc.handle, m.h,
			uintptr(unsafe.Pointer(buf)), uintptr(cap), uintptr(unsafe.Pointer(outLen)))
		return r
	})
}

// ResourceNameValue returns the script resource name as V8's original
// scope-local Value. No string coercion is performed; ok=false represents an
// absent handle, while an explicit undefined resource returns ok=true.
func (m *Message) ResourceNameValue() (Value, bool, error) {
	if err := m.check(); err != nil {
		return Value{}, false, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_message_resource_name_value").Call(
		m.iso.handleAssumingCheck(), m.sc.handle, m.h,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, false, shimError("Message.ResourceNameValue", r1)
	}
	if out == 0 {
		return Value{}, false, nil
	}
	return Value{iso: m.iso, sc: m.sc, h: out}, true, nil
}

func (m *Message) simpleInt(op string) (int64, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	r1, _, _ := proc(op).Call(m.iso.handleAssumingCheck(), m.sc.handle, m.h)
	if int64(r1) < 0 {
		return 0, shimError(op, r1)
	}
	return int64(r1), nil
}

func (m *Message) simpleBool(op string) (bool, error) {
	v, err := m.simpleInt(op)
	return v == 1, err
}

func (m *Message) signedLocation(which int32) (int64, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	var out int32
	r1, _, _ := proc("gov8_ec_message_location").Call(
		m.iso.handleAssumingCheck(), m.sc.handle, m.h, uintptr(which),
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("Message.Location", r1)
	}
	return int64(out), nil
}

// StartPosition returns the 0-based character offset where the error
// region starts.
func (m *Message) StartPosition() (int64, error) {
	return m.signedLocation(0)
}

// EndPosition returns the exclusive end offset of the error region.
func (m *Message) EndPosition() (int64, error) { return m.signedLocation(1) }

// StartColumn returns the 0-based column where the error region starts.
func (m *Message) StartColumn() (int64, error) { return m.signedLocation(2) }

// EndColumn returns the exclusive end column of the error region.
func (m *Message) EndColumn() (int64, error) { return m.signedLocation(3) }

// ErrorLevel returns the MessageErrorLevel of the message.
func (m *Message) ErrorLevel() (int64, error) { return m.simpleInt("gov8_ca_message_error_level") }

// IsOpaque reports the origin's is_opaque flag.
func (m *Message) IsOpaque() (bool, error) { return m.simpleBool("gov8_ca_message_is_opaque") }

// IsSharedCrossOrigin reports the origin's is_shared_cross_origin flag.
func (m *Message) IsSharedCrossOrigin() (bool, error) {
	return m.simpleBool("gov8_ca_message_is_shared_cross_origin")
}

// WasmFunctionIndex returns the Wasm function index for a Wasm-originated
// message, or -1 for a non-Wasm message.
func (m *Message) WasmFunctionIndex() (int64, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	var out int32
	r1, _, _ := proc("gov8_ea_message_wasm_function_index").Call(
		m.iso.handleAssumingCheck(), m.sc.handle, m.h,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("Message.WasmFunctionIndex", r1)
	}
	return int64(out), nil
}

// StackTrace returns the trace attached to the message; ok=false when the
// engine produced none (the default for uncaught exceptions unless
// SetCaptureStackTraceForUncaughtExceptions enabled capture).
func (m *Message) StackTrace() (*StackTrace, bool, error) {
	if err := m.check(); err != nil {
		return nil, false, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_message_get_stack_trace").Call(
		m.iso.handleAssumingCheck(), m.sc.handle, m.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, false, shimError("Message.StackTrace", r1)
	}
	if out == 0 {
		return nil, false, nil
	}
	return &StackTrace{iso: m.iso, sc: m.sc, h: out}, true, nil
}

// StackTrace is a captured JS stack trace (scope-local handle).
type StackTrace struct {
	iso *Isolate
	sc  *Scope
	h   uintptr
}

func (st *StackTrace) check() error {
	if st == nil {
		return fmt.Errorf("gov8: nil stack trace")
	}
	if st.h == 0 {
		return fmt.Errorf("gov8: no stack trace present")
	}
	return st.sc.check()
}

// CurrentStackTrace captures up to frameLimit frames of the current JS
// stack; ok=false when the engine produced none.
func (s *Scope) CurrentStackTrace(frameLimit int) (*StackTrace, bool, error) {
	if frameLimit < 0 {
		return nil, false, fmt.Errorf("gov8: negative frame limit")
	}
	// rusty_v8 accepts usize but returns None when it cannot losslessly
	// convert the limit to the binding's signed 32-bit int.
	if uint64(frameLimit) > uint64(^uint32(0)>>1) {
		return nil, false, nil
	}
	sh, err := s.checkedHandle()
	if err != nil {
		return nil, false, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_stack_trace_current").Call(
		s.iso.handleAssumingCheck(), sh, uintptr(frameLimit), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, false, shimError("CurrentStackTrace", r1)
	}
	if out == 0 {
		return nil, false, nil
	}
	return &StackTrace{iso: s.iso, sc: s, h: out}, true, nil
}

// CurrentScriptNameOrSourceURL returns the script name (or source URL) of
// the topmost JS frame; ok=false when there is none.
func (s *Scope) CurrentScriptNameOrSourceURL() (string, bool, error) {
	v, ok, err := s.currentScriptNameOrSourceURLValue("CurrentScriptNameOrSourceURL")
	if err != nil || !ok {
		return "", ok, err
	}
	txt, err := v.StringValue()
	return txt, true, err
}

// CurrentScriptNameOrSourceURLValue returns the topmost script name or
// sourceURL as a scope-local JavaScript String without copying it to Go.
func (s *Scope) CurrentScriptNameOrSourceURLValue() (Value, bool, error) {
	return s.currentScriptNameOrSourceURLValue("CurrentScriptNameOrSourceURLValue")
}

func (s *Scope) currentScriptNameOrSourceURLValue(op string) (Value, bool, error) {
	sh, err := s.checkedHandle()
	if err != nil {
		return Value{}, false, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_stack_trace_current_script_name").Call(
		s.iso.handleAssumingCheck(), sh, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, false, shimError(op, r1)
	}
	if out == 0 {
		return Value{}, false, nil
	}
	return Value{iso: s.iso, sc: s, h: out}, true, nil
}

// FrameCount returns the number of frames in the trace.
func (st *StackTrace) FrameCount() (int, error) {
	if err := st.check(); err != nil {
		return 0, err
	}
	r1, _, _ := proc("gov8_ca_stack_trace_frame_count").Call(
		st.iso.handleAssumingCheck(), st.sc.handle, st.h)
	if int64(r1) < 0 {
		return 0, shimError("StackTrace.FrameCount", r1)
	}
	return int(r1), nil
}

// Frame returns the frame at index i (topmost first). Both bounds are checked
// before the frame-getter FFI. This intentionally differs from rusty_v8
// 152.2.0: get_frame(FrameCount) reports Some, but dereferencing that handle
// reproducibly access-violates, so safe Go code must never receive it.
func (st *StackTrace) Frame(i int) (*StackFrame, error) {
	if err := st.check(); err != nil {
		return nil, err
	}
	count, err := st.FrameCount()
	if err != nil {
		return nil, err
	}
	if i < 0 || i >= count {
		return nil, fmt.Errorf("gov8: frame index out of range: %d", i)
	}
	var out uintptr
	var ok int32
	r1, _, _ := proc("gov8_ca_stack_trace_get_frame").Call(
		st.iso.handleAssumingCheck(), st.sc.handle, st.h, uintptr(i),
		uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&ok)))
	if int64(r1) < 0 {
		return nil, shimError("StackTrace.Frame", r1)
	}
	if ok != 1 {
		return nil, fmt.Errorf("gov8: frame index out of range: %d", i)
	}
	return &StackFrame{iso: st.iso, sc: st.sc, h: out}, nil
}

// StackFrame is one frame of a StackTrace (scope-local handle).
type StackFrame struct {
	iso *Isolate
	sc  *Scope
	h   uintptr
}

func (f *StackFrame) check() error {
	if f == nil {
		return fmt.Errorf("gov8: nil stack frame")
	}
	return f.sc.check()
}

// valueField returns a scope-local String produced by a StackFrame getter.
func (f *StackFrame) valueField(procName, op string) (Value, bool, error) {
	if err := f.check(); err != nil {
		return Value{}, false, err
	}
	var out uintptr
	r1, _, _ := proc(procName).Call(
		f.iso.handleAssumingCheck(), f.sc.handle, f.h,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, false, shimError(op, r1)
	}
	if out == 0 {
		return Value{}, false, nil
	}
	return Value{iso: f.iso, sc: f.sc, h: out}, true, nil
}

// FunctionNameValue returns the function name as a scope-local JavaScript
// String. ok=false means the frame has no function name.
func (f *StackFrame) FunctionNameValue() (Value, bool, error) {
	return f.valueField("gov8_ca_frame_function_name", "StackFrame.FunctionNameValue")
}

// ScriptNameValue returns the script name as a scope-local JavaScript String.
func (f *StackFrame) ScriptNameValue() (Value, bool, error) {
	return f.valueField("gov8_ca_frame_script_name", "StackFrame.ScriptNameValue")
}

// ScriptNameOrSourceURLValue returns the script name or sourceURL fallback as
// a scope-local JavaScript String.
func (f *StackFrame) ScriptNameOrSourceURLValue() (Value, bool, error) {
	return f.valueField("gov8_ea_frame_script_name_or_source_url",
		"StackFrame.ScriptNameOrSourceURLValue")
}

// ScriptSourceValue returns the complete script source as a scope-local
// JavaScript String.
func (f *StackFrame) ScriptSourceValue() (Value, bool, error) {
	return f.valueField("gov8_ea_frame_script_source", "StackFrame.ScriptSourceValue")
}

// SourceMappingURLValue returns the source-map URL as a scope-local JavaScript
// String.
func (f *StackFrame) SourceMappingURLValue() (Value, bool, error) {
	return f.valueField("gov8_ea_frame_script_source_mapping_url",
		"StackFrame.SourceMappingURLValue")
}

// FunctionName returns the called function's name; ok=false when anonymous.
func (f *StackFrame) FunctionName() (string, bool, error) {
	v, ok, err := f.valueField("gov8_ca_frame_function_name", "StackFrame.FunctionName")
	if err != nil || !ok {
		return "", ok, err
	}
	txt, err := v.StringValue()
	return txt, true, err
}

// ScriptName returns the frame's script name; ok=false when absent.
func (f *StackFrame) ScriptName() (string, bool, error) {
	v, ok, err := f.valueField("gov8_ca_frame_script_name", "StackFrame.ScriptName")
	if err != nil || !ok {
		return "", ok, err
	}
	txt, err := v.StringValue()
	return txt, true, err
}

func (f *StackFrame) textField(procName, op string) (string, bool, error) {
	v, ok, err := f.valueField(procName, op)
	if err != nil || !ok {
		return "", ok, err
	}
	txt, err := v.StringValue()
	return txt, true, err
}

// ScriptNameOrSourceURL returns the frame's script resource name, or its
// sourceURL directive when V8 provides that fallback. ok is false when absent.
func (f *StackFrame) ScriptNameOrSourceURL() (string, bool, error) {
	return f.textField("gov8_ea_frame_script_name_or_source_url",
		"StackFrame.ScriptNameOrSourceURL")
}

// ScriptSource returns the frame's complete script source. ok is false when
// V8 does not expose source for the frame.
func (f *StackFrame) ScriptSource() (string, bool, error) {
	return f.textField("gov8_ea_frame_script_source", "StackFrame.ScriptSource")
}

// SourceMappingURL returns the frame script's source-map URL. ok is false
// when no sourceMappingURL is present.
func (f *StackFrame) SourceMappingURL() (string, bool, error) {
	return f.textField("gov8_ea_frame_script_source_mapping_url",
		"StackFrame.SourceMappingURL")
}

func (f *StackFrame) simpleInt(op string) (int64, error) {
	if err := f.check(); err != nil {
		return 0, err
	}
	r1, _, _ := proc(op).Call(f.iso.handleAssumingCheck(), f.sc.handle, f.h)
	if int64(r1) < 0 {
		return 0, shimError(op, r1)
	}
	return int64(r1), nil
}

// LineNumber returns the frame's 1-based line.
func (f *StackFrame) LineNumber() (int64, error) { return f.simpleInt("gov8_ca_frame_line_number") }

// Column returns the frame's 1-based column.
func (f *StackFrame) Column() (int64, error) { return f.simpleInt("gov8_ca_frame_column") }

// ScriptID returns the frame's script id (positive for normal scripts).
func (f *StackFrame) ScriptID() (int64, error) { return f.simpleInt("gov8_ca_frame_script_id") }

// IsEval reports whether the frame came from an eval call.
func (f *StackFrame) IsEval() (bool, error) {
	v, err := f.simpleInt("gov8_ca_frame_is_eval")
	return v == 1, err
}

// IsConstructor reports whether the frame is a construct call.
func (f *StackFrame) IsConstructor() (bool, error) {
	v, err := f.simpleInt("gov8_ca_frame_is_constructor")
	return v == 1, err
}

// IsWasm reports whether the frame is a Wasm frame.
func (f *StackFrame) IsWasm() (bool, error) {
	v, err := f.simpleInt("gov8_ca_frame_is_wasm")
	return v == 1, err
}

// IsUserJavaScript reports whether the frame is user JavaScript.
func (f *StackFrame) IsUserJavaScript() (bool, error) {
	v, err := f.simpleInt("gov8_ca_frame_is_user_javascript")
	return v == 1, err
}

// NewError builds a JS Error through the isolate's explicitly entered
// context. It is retained for compatibility with Context.Enter users; new
// code should prefer Context.NewError, whose context cannot be omitted.
// The current-context check is safety-critical: the pinned no-context
// constructor path access-violates rather than returning an empty handle.
func (s *Scope) NewError(message string) (Value, error) {
	sh, err := s.checkedHandle()
	if err != nil {
		return Value{}, err
	}
	current, err := s.iso.CurrentContext(s)
	if err != nil {
		return Value{}, err
	}
	if current.h == 0 {
		return Value{}, fmt.Errorf("gov8: NewError requires an entered context; use Context.NewError")
	}
	msg, err := s.newStringAssumingCheck(message)
	if err != nil {
		return Value{}, err
	}
	h, err := callHandle("Scope.NewError", proc("gov8_exception_error"),
		s.iso.handleAssumingCheck(), sh, msg.h)
	if err != nil {
		return Value{}, err
	}
	return Value{iso: s.iso, sc: s, h: h}, nil
}

// CaptureStackTrace attaches a ".stack" property to a plain object
// (Exception::CaptureStackTrace). ok mirrors the engine's MaybeBool.
func CaptureStackTrace(c *Context, s *Scope, obj Value) (bool, error) {
	if err := c.check(); err != nil {
		return false, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return false, err
	}
	if s.iso != c.iso {
		return false, foreignIsolate("scope")
	}
	if err := obj.check(); err != nil {
		return false, err
	}
	if obj.iso != c.iso {
		return false, foreignIsolate("value")
	}
	var ok int32
	r1, _, _ := proc("gov8_ca_exception_capture_stack_trace").Call(
		c.iso.handleAssumingCheck(), c.handle, sh, obj.h, uintptr(unsafe.Pointer(&ok)))
	if int64(r1) < 0 {
		return false, shimError("CaptureStackTrace", r1)
	}
	return ok == 1, nil
}

// ExceptionStackTrace returns the stack trace attached to an exception
// value; ok=false when it carries none (natively created errors never do
// in this build -- the pinned gap).
func ExceptionStackTrace(s *Scope, exc Value) (*StackTrace, bool, error) {
	if err := exc.check(); err != nil {
		return nil, false, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return nil, false, err
	}
	if s.iso != exc.iso {
		return nil, false, foreignIsolate("exception")
	}
	var out uintptr
	r1, _, _ := proc("gov8_ca_exception_get_stack_trace").Call(
		exc.iso.handleAssumingCheck(), sh, exc.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, false, shimError("ExceptionStackTrace", r1)
	}
	if out == 0 {
		return nil, false, nil
	}
	return &StackTrace{iso: exc.iso, sc: s, h: out}, true, nil
}

// SetCaptureStackTraceForUncaughtExceptions toggles the isolate-wide
// capture of stack traces for uncaught exceptions with the given frame
// limit (Isolate::SetCaptureStackTraceForUncaughtExceptions).
func (i *Isolate) SetCaptureStackTraceForUncaughtExceptions(enable bool, frameLimit int) error {
	if err := i.check(); err != nil {
		return err
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	if frameLimit < 0 {
		return fmt.Errorf("gov8: negative frame limit")
	}
	if uint64(frameLimit) > uint64(^uint32(0)>>1) {
		return fmt.Errorf("gov8: frame limit exceeds V8 int32 range")
	}
	capture := uintptr(0)
	if enable {
		capture = 1
	}
	return callErr("SetCaptureStackTraceForUncaughtExceptions",
		proc("gov8_ca_set_capture_stack_trace_uncaught"), i.handle, capture, uintptr(frameLimit))
}

// --- interrupts -----------------------------------------------------------------

// InterruptCallback is the callback for RequestInterrupt. It runs on the
// isolate's thread, inside engine execution, at the engine's next interrupt
// check. The data uintptr is passed through verbatim; like every value
// crossing the engine boundary it must not be a Go pointer.
type InterruptCallback func(i *Isolate, data uintptr)

type interruptEntry struct {
	iso  *Isolate
	data uintptr
	cb   InterruptCallback
}

var interruptRegistry = struct {
	mu      sync.Mutex
	next    int64
	entries map[int64]*interruptEntry
}{entries: make(map[int64]*interruptEntry)}

var (
	interruptDispatcherOnce sync.Once
	interruptDispatcherErr  error
)

// goInterruptDispatch is the single entry point handed to the shim; all
// interrupt callbacks funnel through it. The engine hands back the integer
// id stored at request time.
var goInterruptDispatch = syscall.NewCallback(func(iso, data uintptr) uintptr {
	id := int64(data)
	interruptRegistry.mu.Lock()
	entry := interruptRegistry.entries[id]
	interruptRegistry.mu.Unlock()
	if entry == nil {
		return 1
	}
	defer func() {
		if r := recover(); r != nil {
			// A panic unwinding through engine interrupt frames would
			// corrupt the engine; convert it to the process abort
			// documented for native callbacks.
			fmt.Fprintf(os.Stderr, "gov8: panic in interrupt callback: %v\n", r)
			proc("gov8_host_panic_abort").Call()
		}
	}()
	entry.cb(entry.iso, entry.data)
	return 1
})

// RequestInterrupt schedules cb to run once on the isolate's thread during
// its next JS execution, with data handed back verbatim. Returns false if
// the isolate was already closed (the callback never runs then). Safe to
// call from any goroutine (the engine posts the request).
func (h *ThreadSafeHandle) RequestInterrupt(cb InterruptCallback, data uintptr) bool {
	if cb == nil {
		return false
	}
	ih, ok := h.liveHandle()
	if !ok {
		return false
	}
	interruptDispatcherOnce.Do(func() {
		interruptDispatcherErr = callErr("SetInterruptDispatcher",
			proc("gov8_ca_interrupt_set_entry"), goInterruptDispatch)
	})
	if interruptDispatcherErr != nil {
		return false
	}
	interruptRegistry.mu.Lock()
	interruptRegistry.next++
	id := interruptRegistry.next
	interruptRegistry.entries[id] = &interruptEntry{iso: h.iso, data: data, cb: cb}
	interruptRegistry.mu.Unlock()
	if err := callErr("RequestInterrupt", proc("gov8_ca_isolate_request_interrupt"), ih, uintptr(id)); err != nil {
		interruptRegistry.mu.Lock()
		delete(interruptRegistry.entries, id)
		interruptRegistry.mu.Unlock()
		return false
	}
	return true
}
