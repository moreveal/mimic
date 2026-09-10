//go:build windows && amd64

package gov8

// Thread-safe termination control and memory-pressured GC, mirroring the
// pinned crate's IsolateHandle:
//
// - ThreadSafeHandle is Cloneable/Send/Sync in Rust; in Go the handle is a
//   small shared struct whose methods may be called from any goroutine.
//   TerminateExecution and CancelTerminateExecution are accepted by the
//   engine from any thread (V8 interrupt machinery); IsExecutionTerminating
//   is exposed on the handle for API parity but reads engine execution
//   state, so it observably belongs to the isolate thread (the pinned
//   oracle only reads it there).
// - The request is asynchronous by design: terminate_execution only
//   enqueues an interrupt; the terminating flag flips when the interrupt is
//   delivered at the next interrupt check, not at the request site, and
//   V8 clears it once the termination exception has fully unwound to the
//   embedder. The durable post-abort observable is the TryCatch's
//   terminated state.
// - cancel_terminate_execution restores the isolate to a fully usable
//   state. All three handle methods report false after the isolate was
//   closed (Rust: "Returns false if Isolate was already destroyed").
// - LowMemoryNotification forces a major GC; weak callbacks run inside it,
//   synchronously on the isolate thread.

// ThreadSafeHandle is a thread-safe reference to an isolate. Its main use
// is to terminate execution of a running isolate from another thread. It is
// created with Isolate.ThreadSafeHandle and remains safe to call after the
// isolate was closed (answering false, like the pinned IsolateHandle).
type ThreadSafeHandle struct {
	iso *Isolate
}

// ThreadSafeHandle returns a handle that may terminate this isolate's
// execution from any goroutine.
func (i *Isolate) ThreadSafeHandle() *ThreadSafeHandle {
	return &ThreadSafeHandle{iso: i}
}

// liveHandle returns the engine isolate pointer if the isolate is still
// open. The mutex serializes against Isolate.Close, so a concurrent close
// can never leave this call using a disposed engine isolate.
func (h *ThreadSafeHandle) liveHandle() (uintptr, bool) {
	h.iso.mu.Lock()
	defer h.iso.mu.Unlock()
	if h.iso.closed {
		return 0, false
	}
	return h.iso.handle, true
}

// TerminateExecution forcefully terminates JS execution in the isolate. It
// may be called from any goroutine. The request only takes effect at the
// target's next interrupt check. Returns false if the isolate was already
// closed.
func (h *ThreadSafeHandle) TerminateExecution() bool {
	ih, ok := h.liveHandle()
	if !ok {
		return false
	}
	r1, _, _ := proc("gov8_isolate_terminate_execution").Call(ih)
	return int64(r1) >= 0
}

// CancelTerminateExecution resumes execution capability after a previous
// TerminateExecution once the termination exception has unwound to the
// embedder. Returns false if the isolate was already closed.
func (h *ThreadSafeHandle) CancelTerminateExecution() bool {
	ih, ok := h.liveHandle()
	if !ok {
		return false
	}
	r1, _, _ := proc("gov8_isolate_cancel_terminate_execution").Call(ih)
	return int64(r1) >= 0
}

// IsExecutionTerminating reports whether JS execution is currently
// terminating because of TerminateExecution (the termination exception is
// still unwinding). Exposed on the handle for parity with the pinned
// IsolateHandle; the engine state it reads belongs to the isolate thread.
// Returns false if the isolate was already closed.
func (h *ThreadSafeHandle) IsExecutionTerminating() bool {
	ih, ok := h.liveHandle()
	if !ok {
		return false
	}
	r1, _, _ := proc("gov8_isolate_is_execution_terminating").Call(ih)
	return int64(r1) == 1
}

// TerminateExecution requests termination from the isolate's own thread
// (the isolate-level form of the pinned Isolate::terminate_execution,
// which always accepts the request). The request is delivered at the next
// interrupt check, not synchronously.
func (i *Isolate) TerminateExecution() error {
	if err := i.check(); err != nil {
		return err
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	return callErr("TerminateExecution", proc("gov8_isolate_terminate_execution"), i.handle)
}

// CancelTerminateExecution clears the termination state and restores the
// isolate to a fully usable state.
func (i *Isolate) CancelTerminateExecution() error {
	if err := i.check(); err != nil {
		return err
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	return callErr("CancelTerminateExecution", proc("gov8_isolate_cancel_terminate_execution"), i.handle)
}

// IsExecutionTerminating reports whether execution is currently terminating
// because of a TerminateExecution request.
func (i *Isolate) IsExecutionTerminating() (bool, error) {
	if err := i.check(); err != nil {
		return false, err
	}
	if err := requireInitialized(); err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_isolate_is_execution_terminating").Call(i.handle)
	if int64(r1) < 0 {
		return false, shimError("IsExecutionTerminating", r1)
	}
	return r1 == 1, nil
}

// LowMemoryNotification (Isolate.LowMemoryNotification) lives in buffer.go
// of the buffers slice: the same crate API backs both features and the
// first landed declaration wins (forced-GC weak-callback behavior is
// exercised by the weak-handle tests).

// HasTerminated reports whether the TryCatch terminated because execution
// was forcefully terminated (the durable post-abort observable of a
// termination that unwound through the TryCatch).
func (t *TryCatch) HasTerminated() (bool, error) {
	if err := t.check(); err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_tc_has_terminated").Call(t.handle)
	if int64(r1) < 0 {
		return false, shimError("HasTerminated", r1)
	}
	return r1 == 1, nil
}
