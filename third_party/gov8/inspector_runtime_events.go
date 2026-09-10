//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// InspectorAsyncTaskID is an opaque identity token used to correlate an
// embedder task's schedule, start, finish, and cancellation events. V8 only
// compares this token; it never dereferences it. Zero is a valid token.
type InspectorAsyncTaskID uintptr

// InspectorStackTrace owns a V8 Inspector stack-trace snapshot. Close releases
// an unconsumed snapshot. ExceptionThrown consumes it exactly once.
type InspectorStackTrace struct {
	mu       sync.Mutex
	iso      *Isolate
	handle   uintptr
	closed   bool
	consumed bool
}

func (i *Inspector) IdleStarted() error {
	if err := i.check(); err != nil {
		return err
	}
	return callErr("Inspector.IdleStarted", proc("gov8_ire_idle_started"), i.handle)
}

func (i *Inspector) IdleFinished() error {
	if err := i.check(); err != nil {
		return err
	}
	return callErr("Inspector.IdleFinished", proc("gov8_ire_idle_finished"), i.handle)
}

// AsyncTaskScheduled records the scheduling stack for task. A non-recurring
// task loses it after its first finish; a recurring task keeps it until cancel.
func (i *Inspector) AsyncTaskScheduled(name InspectorStringView, task InspectorAsyncTaskID, recurring bool) error {
	if err := i.check(); err != nil {
		return err
	}
	is8, data, length := name.native()
	recurringWord := uintptr(0)
	if recurring {
		recurringWord = 1
	}
	err := callErr("Inspector.AsyncTaskScheduled", proc("gov8_ire_async_task_scheduled"),
		i.handle, is8, data, length, uintptr(task), recurringWord)
	runtime.KeepAlive(name)
	return err
}

func (i *Inspector) AsyncTaskCanceled(task InspectorAsyncTaskID) error {
	if err := i.check(); err != nil {
		return err
	}
	return callErr("Inspector.AsyncTaskCanceled", proc("gov8_ire_async_task_canceled"), i.handle, uintptr(task))
}

func (i *Inspector) AsyncTaskStarted(task InspectorAsyncTaskID) error {
	if err := i.check(); err != nil {
		return err
	}
	return callErr("Inspector.AsyncTaskStarted", proc("gov8_ire_async_task_started"), i.handle, uintptr(task))
}

func (i *Inspector) AsyncTaskFinished(task InspectorAsyncTaskID) error {
	if err := i.check(); err != nil {
		return err
	}
	return callErr("Inspector.AsyncTaskFinished", proc("gov8_ire_async_task_finished"), i.handle, uintptr(task))
}

func (i *Inspector) AllAsyncTasksCanceled() error {
	if err := i.check(); err != nil {
		return err
	}
	return callErr("Inspector.AllAsyncTasksCanceled", proc("gov8_ire_all_async_tasks_canceled"), i.handle)
}

// CreateInspectorStackTrace converts a scope-local V8 StackTrace to an owned
// Inspector snapshot. ok is false when trace is nil or Inspector produces no
// snapshot. The pinned build returns no snapshot for a nil input.
func (i *Inspector) CreateInspectorStackTrace(trace *StackTrace) (*InspectorStackTrace, bool, error) {
	if err := i.check(); err != nil {
		return nil, false, err
	}
	var traceHandle uintptr
	if trace != nil {
		if trace.iso != i.iso {
			return nil, false, foreignIsolate("stack trace")
		}
		if err := trace.check(); err != nil {
			return nil, false, err
		}
		traceHandle = trace.h
	}
	var out uintptr
	r, _, _ := proc("gov8_ire_create_stack_trace").Call(i.handle, traceHandle, uintptr(unsafe.Pointer(&out)))
	if int64(r) < 0 {
		return nil, false, shimError("Inspector.CreateInspectorStackTrace", r)
	}
	if out == 0 {
		return nil, false, nil
	}
	return &InspectorStackTrace{iso: i.iso, handle: out}, true, nil
}

// Close releases an Inspector stack trace that has not been consumed. The
// pinned V8StackTraceImpl destructor is isolate-independent (it is defaulted
// and owns only copied frame/protocol state), so Close remains safe after the
// originating Inspector or isolate is closed and from another thread.
func (st *InspectorStackTrace) Close() error {
	if st == nil {
		return errors.New("gov8: nil Inspector stack trace")
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.consumed {
		return errors.New("gov8: Inspector stack trace was consumed")
	}
	if st.closed {
		return errors.New("gov8: Inspector stack trace already closed")
	}
	if err := callErr("InspectorStackTrace.Close", proc("gov8_ire_stack_trace_delete"), st.handle); err != nil {
		return err
	}
	st.handle = 0
	st.closed = true
	return nil
}

func currentInspectorContext(iso *Isolate, context *Context) bool {
	contextScopeStacks.mu.Lock()
	defer contextScopeStacks.mu.Unlock()
	stack := contextScopeStacks.m[iso]
	return len(stack) != 0 && !stack[len(stack)-1].closed && stack[len(stack)-1].ctx == context
}

// ExceptionThrown reports an exception and returns its Inspector exception id.
// stackTrace may be nil and is consumed when non-nil, including when Inspector
// has no registered context and returns id zero. scope must be current,
// context must be the current entered context, and exception must be a live
// local from the same isolate.
func (i *Inspector) ExceptionThrown(scope *Scope, context *Context,
	message InspectorStringView, exception Value, detailedMessage, url InspectorStringView,
	lineNumber, columnNumber uint32, stackTrace *InspectorStackTrace, scriptID int32) (uint32, error) {
	if err := i.check(); err != nil {
		return 0, err
	}
	if scope == nil {
		return 0, errors.New("gov8: nil scope")
	}
	if scope.iso != i.iso {
		return 0, foreignIsolate("scope")
	}
	if err := scope.check(); err != nil {
		return 0, err
	}
	if err := scope.requireCurrent(); err != nil {
		return 0, err
	}
	if context == nil {
		return 0, errors.New("gov8: nil context")
	}
	if context.iso != i.iso {
		return 0, foreignIsolate("context")
	}
	if err := context.checkAssumingIsolate(); err != nil {
		return 0, err
	}
	if !currentInspectorContext(i.iso, context) {
		return 0, errors.New("gov8: context is not the current entered context")
	}
	if exception.h == 0 {
		return 0, errors.New("gov8: zero value handle")
	}
	if exception.iso != i.iso {
		return 0, foreignIsolate("exception")
	}
	if err := exception.check(); err != nil {
		return 0, err
	}

	var traceHandle uintptr
	if stackTrace != nil {
		if stackTrace.iso != i.iso {
			return 0, foreignIsolate("Inspector stack trace")
		}
		if err := stackTrace.iso.check(); err != nil {
			return 0, err
		}
		stackTrace.mu.Lock()
		defer stackTrace.mu.Unlock()
		if stackTrace.consumed {
			return 0, errors.New("gov8: Inspector stack trace was consumed")
		}
		if stackTrace.closed || stackTrace.handle == 0 {
			return 0, errors.New("gov8: Inspector stack trace used after Close")
		}
		traceHandle = stackTrace.handle
	}

	mi, mp, ml := message.native()
	di, dp, dl := detailedMessage.native()
	ui, up, ul := url.native()
	var exceptionID uint32
	traceTransfer := traceHandle
	r, _, _ := proc("gov8_ire_exception_thrown").Call(
		i.handle, context.handle, scope.handle,
		mi, mp, ml, exception.h,
		di, dp, dl, ui, up, ul,
		uintptr(lineNumber), uintptr(columnNumber),
		uintptr(unsafe.Pointer(&traceTransfer)), uintptr(uint32(scriptID)),
		uintptr(unsafe.Pointer(&exceptionID)))
	runtime.KeepAlive(message)
	runtime.KeepAlive(detailedMessage)
	runtime.KeepAlive(url)
	if stackTrace != nil && traceTransfer == 0 {
		stackTrace.handle = 0
		stackTrace.consumed = true
	}
	if int64(r) < 0 {
		return 0, shimError("Inspector.ExceptionThrown", r)
	}
	if stackTrace != nil && traceTransfer != 0 {
		return 0, fmt.Errorf("gov8: Inspector.ExceptionThrown did not consume its stack trace")
	}
	return exceptionID, nil
}
