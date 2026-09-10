//go:build windows && amd64

package gov8

import (
	"fmt"
	"unsafe"
)

// TryCatch observes exceptions thrown inside compile/run calls that are
// passed the TryCatch. It mirrors the observable surface the oracle
// characterizes: HasCaught, CanContinue, Reset, message text (with the
// engine's "Uncaught " prefix), exception ToString text, exception kind, and
// message position information (0-based character offset and column, 1-based
// line).
//
// A TryCatch registers itself with the isolate for its whole lifetime
// (between NewTryCatch and Close) and must be closed on the owning thread,
// nested consistently with the isolate's other engine work.
type TryCatch struct {
	iso    *Isolate
	handle uintptr
	closed bool
}

// NewTryCatch creates and registers a TryCatch on the isolate.
func (i *Isolate) NewTryCatch() (*TryCatch, error) {
	ih, err := i.handleChecked()
	if err != nil {
		return nil, err
	}
	h, err := callHandle("TryCatch.New", proc("gov8_trycatch_new"), ih)
	if err != nil {
		return nil, err
	}
	return &TryCatch{iso: i, handle: h}, nil
}

// check validates the TryCatch's own state and its isolate's thread
// affinity; affinity first, so wrong-thread misuse returns before the
// TryCatch-local closed flag is read.
func (t *TryCatch) check() error {
	if err := t.iso.check(); err != nil {
		return err
	}
	if t.closed {
		return fmt.Errorf("gov8: trycatch used after Close")
	}
	return nil
}

// Close unregisters and releases the TryCatch.
func (t *TryCatch) Close() error {
	if err := t.iso.check(); err != nil {
		return err
	}
	if t.closed {
		return fmt.Errorf("gov8: trycatch already closed")
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	r1, _, _ := proc("gov8_trycatch_dispose").Call(t.handle)
	t.closed = true
	if int64(r1) < 0 {
		return shimError("TryCatch.Close", r1)
	}
	return nil
}

// HasCaught reports whether an exception was caught.
func (t *TryCatch) HasCaught() (bool, error) {
	if err := t.check(); err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_tc_has_caught").Call(t.handle)
	if int64(r1) < 0 {
		return false, shimError("HasCaught", r1)
	}
	return r1 == 1, nil
}

// CanContinue reports whether execution can continue after the exception.
func (t *TryCatch) CanContinue() (bool, error) {
	if err := t.check(); err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_tc_can_continue").Call(t.handle)
	if int64(r1) < 0 {
		return false, shimError("CanContinue", r1)
	}
	return r1 == 1, nil
}

// IsVerbose reports whether caught exceptions are also reported to the
// isolate's message listeners.
func (t *TryCatch) IsVerbose() (bool, error) {
	if err := t.check(); err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_ea_tc_is_verbose").Call(t.handle)
	if int64(r1) < 0 {
		return false, shimError("TryCatch.IsVerbose", r1)
	}
	return r1 == 1, nil
}

// SetVerbose controls whether caught exceptions are also reported to the
// isolate's message listeners. It is false by default.
func (t *TryCatch) SetVerbose(value bool) error {
	if err := t.check(); err != nil {
		return err
	}
	var enabled uintptr
	if value {
		enabled = 1
	}
	return callErr("TryCatch.SetVerbose", proc("gov8_ea_tc_set_verbose"),
		t.handle, enabled)
}

// SetCaptureMessage controls whether subsequently caught exceptions capture
// a Message. It is true by default. Disabling it does not suppress Exception.
func (t *TryCatch) SetCaptureMessage(value bool) error {
	if err := t.check(); err != nil {
		return err
	}
	var enabled uintptr
	if value {
		enabled = 1
	}
	return callErr("TryCatch.SetCaptureMessage",
		proc("gov8_ea_tc_set_capture_message"), t.handle, enabled)
}

// Reset clears the caught exception; subsequent HasCaught reports false and
// later scripts run normally.
func (t *TryCatch) Reset() error {
	if err := t.check(); err != nil {
		return err
	}
	return callErr("Reset", proc("gov8_tc_reset"), t.handle)
}

// MessageText returns v8 Message::Get() text (carries the "Uncaught " prefix
// for TryCatch-caught exceptions in this build). Empty when nothing was
// caught.
func (t *TryCatch) MessageText(s *Scope, c *Context) (string, error) {
	return t.utf8Text("gov8_tc_message_utf8", "MessageText", s, c)
}

// ExceptionText returns the ECMAScript ToString of the caught exception
// (no "Uncaught " prefix). Empty when nothing was caught.
func (t *TryCatch) ExceptionText(s *Scope, c *Context) (string, error) {
	return t.utf8Text("gov8_tc_exception_utf8", "ExceptionText", s, c)
}

func (t *TryCatch) utf8Text(procName, op string, s *Scope, c *Context) (string, error) {
	if err := t.check(); err != nil {
		return "", err
	}
	// t.check proved the isolate's state and affinity for this operation,
	// so the context/scope checks below only inspect their own closed flags
	// (the old code re-ran the isolate validation up to three times).
	if err := c.checkAssumingIsolate(); err != nil {
		return "", err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return "", err
	}
	if c.iso != t.iso {
		return "", foreignIsolate("context")
	}
	if s.iso != t.iso {
		return "", foreignIsolate("scope")
	}
	return callTextFn(op, func(buf *byte, cap int, outLen *int64) uintptr {
		r, _, _ := proc(procName).Call(
			t.iso.handleAssumingCheck(), t.handle, c.handle, sh,
			uintptr(unsafe.Pointer(buf)), uintptr(cap), uintptr(unsafe.Pointer(outLen)))
		return r
	})
}

// ExceptionIsString reports whether the caught exception is a JS string.
func (t *TryCatch) ExceptionIsString() (bool, error) {
	if err := t.check(); err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_tc_exception_is_string").Call(t.handle)
	if int64(r1) < 0 {
		return false, shimError("ExceptionIsString", r1)
	}
	return r1 == 1, nil
}

// StartPosition returns the 0-based character offset of the message in the
// source (0 when no message is present). The scope must belong to the same
// isolate as the TryCatch.
func (t *TryCatch) StartPosition(s *Scope) (int64, error) {
	if err := t.check(); err != nil {
		return 0, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return 0, err
	}
	if s.iso != t.iso {
		return 0, foreignIsolate("scope")
	}
	r1, _, _ := proc("gov8_tc_start_position").Call(t.iso.handleAssumingCheck(), t.handle, sh)
	if int64(r1) < 0 {
		return 0, shimError("StartPosition", r1)
	}
	return int64(r1), nil
}

// StartColumn returns the 0-based column of the message (0 when absent).
// The scope must belong to the same isolate as the TryCatch.
func (t *TryCatch) StartColumn(s *Scope) (int64, error) {
	if err := t.check(); err != nil {
		return 0, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return 0, err
	}
	if s.iso != t.iso {
		return 0, foreignIsolate("scope")
	}
	r1, _, _ := proc("gov8_tc_start_column").Call(t.iso.handleAssumingCheck(), t.handle, sh)
	if int64(r1) < 0 {
		return 0, shimError("StartColumn", r1)
	}
	return int64(r1), nil
}

// LineNumber returns the 1-based line of the message; ok is false when
// absent. The scope and context must belong to the same isolate as the
// TryCatch.
func (t *TryCatch) LineNumber(s *Scope, c *Context) (line int32, ok bool, err error) {
	if err := t.check(); err != nil {
		return 0, false, err
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return 0, false, err
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return 0, false, err
	}
	if c.iso != t.iso {
		return 0, false, foreignIsolate("context")
	}
	if s.iso != t.iso {
		return 0, false, foreignIsolate("scope")
	}
	var out, okv int32
	r1, _, _ := proc("gov8_tc_line_number").Call(
		t.iso.handleAssumingCheck(), t.handle, c.handle, sh,
		uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&okv)))
	if int64(r1) < 0 {
		return 0, false, shimError("LineNumber", r1)
	}
	return out, okv == 1, nil
}
