//go:build windows && amd64

package gov8

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"
	"unsafe"
)

// This file implements the native Promise/PromiseResolver API with
// observable parity to the pinned Rust v8 crate (v8 =152.2.0), as
// characterized by rust-oracle/src/checks/host/promises.rs.
//
// Ownership and lifetime rules:
//   - Promise and PromiseResolver are scope-local values exactly like Value:
//     bound to the Scope that created them, invalid once that scope closes,
//     and valid only on the isolate's owning thread. They hold no engine
//     resource of their own, so they have no Close — the Scope's Close is
//     the explicit lifetime boundary.
//   - Native reaction handlers are backed by an INTEGER callback registry:
//     the shim only ever sees an int64 id, never a Go pointer. The two
//     process-wide dispatch trampolines (created once via syscall.NewCallback,
//     which pins them for the process lifetime) are the only function
//     pointers that cross the ABI, in the Go→C++ direction.
//   - Values delivered into a Go callback (reaction arguments, reject
//     messages) live in an engine-owned handle scope that exists only for
//     the duration of that callback; they must not be retained.
//   - NativeFunction.Close and ClearPromiseRejectCallback release the Go
//     registry entries explicitly; forgetting them leaks registry entries
//     until process end, it cannot corrupt the engine.

// PromiseState mirrors v8::PromiseState.
type PromiseState int

const (
	PromisePending   PromiseState = 0
	PromiseFulfilled PromiseState = 1
	PromiseRejected  PromiseState = 2
)

func (s PromiseState) String() string {
	switch s {
	case PromisePending:
		return "Pending"
	case PromiseFulfilled:
		return "Fulfilled"
	case PromiseRejected:
		return "Rejected"
	}
	return fmt.Sprintf("PromiseState(%d)", int(s))
}

// Promise is a scope-local JS promise (v8::Promise).
type Promise struct {
	Value
}

// PromiseResolver is a scope-local promise resolver (v8::Promise::Resolver)
// together with the promise it settles (retrieved via GetPromise).
type PromiseResolver struct {
	Value
}

// AsPromise casts a live scope-local value to a Promise after checking its
// engine kind. The returned Promise retains the value's original Scope and
// therefore becomes invalid with that Scope, matching Local::try_cast in the
// pinned Rust crate.
func AsPromise(v Value) (*Promise, error) {
	vv, err := typedCast(v, v.IsPromise, "Promise")
	if err != nil {
		return nil, err
	}
	return &Promise{Value: vv}, nil
}

var (
	promiseHotOnce         sync.Once
	promiseResolverNewAddr uintptr
	promiseGetPromiseAddr  uintptr
	promiseResolveAddr     uintptr
	promiseStateAddr       uintptr
	promiseResultAddr      uintptr
	promiseThenAddr        uintptr
	promiseIsFunctionAddr  uintptr
)

func ensurePromiseHotProcs() {
	promiseHotOnce.Do(func() {
		promiseResolverNewAddr = proc("gov8_promise_resolver_new_direct").Addr()
		promiseGetPromiseAddr = proc("gov8_promise_resolver_get_promise_direct").Addr()
		promiseResolveAddr = proc("gov8_promise_resolver_resolve_direct").Addr()
		promiseStateAddr = proc("gov8_promise_state").Addr()
		promiseResultAddr = proc("gov8_promise_result_direct").Addr()
		promiseThenAddr = proc("gov8_promise_then_direct").Addr()
		promiseIsFunctionAddr = proc("gov8_is_function").Addr()
	})
}

// NewPromiseResolver creates a resolver with a fresh pending promise in the
// context. The scope must belong to the same isolate as the context.
func (s *Scope) NewPromiseResolver(c *Context) (PromiseResolver, error) {
	if err := s.check(); err != nil {
		return PromiseResolver{}, err
	}
	if c == nil || c.iso != s.iso {
		return PromiseResolver{}, foreignIsolate("context")
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return PromiseResolver{}, err
	}
	if err := requireInitialized(); err != nil {
		return PromiseResolver{}, err
	}
	ensurePromiseHotProcs()
	r1, _, _ := syscall.Syscall(promiseResolverNewAddr, 3,
		s.iso.handleAssumingCheck(), c.handle, s.handle)
	if int64(r1) < 0 {
		return PromiseResolver{}, shimError("PromiseResolver.New", r1)
	}
	if r1 == 0 {
		return PromiseResolver{}, shimError("PromiseResolver.New", r1)
	}
	return PromiseResolver{Value{iso: s.iso, sc: s, h: r1}}, nil
}

// GetPromise returns the resolver's associated promise.
func (r PromiseResolver) GetPromise(s *Scope) (Promise, error) {
	if err := r.check(); err != nil {
		return Promise{}, err
	}
	if s.iso != r.iso {
		return Promise{}, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Promise{}, err
	}
	ensurePromiseHotProcs()
	r1, _, _ := syscall.Syscall(promiseGetPromiseAddr, 3,
		r.iso.handleAssumingCheck(), sh, r.h)
	if int64(r1) < 0 {
		return Promise{}, shimError("PromiseResolver.GetPromise", r1)
	}
	if r1 == 0 {
		return Promise{}, shimError("PromiseResolver.GetPromise", r1)
	}
	return Promise{Value{iso: r.iso, sc: s, h: r1}}, nil
}

// checkValues validates a resolved argument: same isolate, usable handle.
func (r PromiseResolver) checkValue(v Value) error {
	if v.h == 0 {
		return fmt.Errorf("gov8: zero value handle")
	}
	if v.iso == r.iso {
		_, err := v.sc.checkedHandleAssumingIsolate()
		return err
	}
	if err := v.check(); err != nil {
		return err
	}
	return foreignIsolate("value")
}

// Resolve settles the associated promise with value. The returned bool is
// the success of the CALL, not a settlement change: resolving or rejecting
// an already-settled promise is silently ignored by the engine and still
// reports true (pinned oracle contract).
func (r PromiseResolver) Resolve(c *Context, v Value) (bool, error) {
	if err := r.check(); err != nil {
		return false, err
	}
	if c == nil || c.iso != r.iso {
		return false, foreignIsolate("context")
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return false, err
	}
	if err := r.checkValue(v); err != nil {
		return false, err
	}
	ensurePromiseHotProcs()
	r1, _, _ := syscall.Syscall6(promiseResolveAddr, 5,
		r.iso.handleAssumingCheck(), c.handle, r.sc.handle, r.h, v.h, 0)
	if int64(r1) < 0 {
		return false, shimError("PromiseResolver.Resolve", r1)
	}
	return r1 == 1, nil
}

// Reject settles the associated promise with value as the rejection reason,
// with the same call-success semantics as Resolve.
func (r PromiseResolver) Reject(c *Context, v Value) (bool, error) {
	if err := r.check(); err != nil {
		return false, err
	}
	if c == nil || c.iso != r.iso {
		return false, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return false, err
	}
	if err := r.checkValue(v); err != nil {
		return false, err
	}
	var ok int32
	r1, _, _ := proc("gov8_promise_resolver_reject").Call(
		r.iso.handle, c.handle, r.sc.handle, r.h, v.h,
		uintptr(unsafe.Pointer(&ok)))
	if int64(r1) < 0 {
		return false, shimError("PromiseResolver.Reject", r1)
	}
	return ok == 1, nil
}

// State reports the promise state.
func (p Promise) State() (PromiseState, error) {
	if err := p.check(); err != nil {
		return 0, err
	}
	if err := requireInitialized(); err != nil {
		return 0, err
	}
	ensurePromiseHotProcs()
	r1, _, _ := syscall.Syscall(promiseStateAddr, 2, p.iso.handleAssumingCheck(), p.h, 0)
	if int64(r1) < 0 {
		return 0, shimError("Promise.State", r1)
	}
	return PromiseState(r1), nil
}

// Result returns the [[PromiseResult]] value. The promise must not be
// pending (the engine contract; the check mirrors the Rust API).
func (p Promise) Result(s *Scope) (Value, error) {
	if err := p.check(); err != nil {
		return Value{}, err
	}
	if s.iso != p.iso {
		return Value{}, foreignIsolate("scope")
	}
	sh, err := s.checkedHandleAssumingIsolate()
	if err != nil {
		return Value{}, err
	}
	ensurePromiseHotProcs()
	r1, _, _ := syscall.Syscall(promiseResultAddr, 3,
		p.iso.handleAssumingCheck(), sh, p.h)
	if int64(r1) < 0 {
		return Value{}, shimError("Promise.Result", r1)
	}
	if r1 == 0 {
		return Value{}, shimError("Promise.Result", r1)
	}
	return Value{iso: p.iso, sc: s, h: r1}, nil
}

// HasHandler reports whether the promise has at least one derived promise
// (resolve/reject handlers, including default handlers).
func (p Promise) HasHandler() (bool, error) {
	if err := p.check(); err != nil {
		return false, err
	}
	ih, err := p.iso.handleChecked()
	if err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_promise_has_handler").Call(ih, p.h)
	if int64(r1) < 0 {
		return false, shimError("Promise.HasHandler", r1)
	}
	return r1 == 1, nil
}

// MarkAsHandled marks the promise as handled so an unhandled-rejection
// notification is suppressed.
func (p Promise) MarkAsHandled() error {
	if err := p.check(); err != nil {
		return err
	}
	ih, err := p.iso.handleChecked()
	if err != nil {
		return err
	}
	r1, _, _ := proc("gov8_promise_mark_as_handled").Call(ih, p.h)
	if int64(r1) < 0 {
		return shimError("Promise.MarkAsHandled", r1)
	}
	return nil
}

// StrictEquals reports v8 Value::StrictEquals for two values of the same
// isolate (used to prove derived promises are distinct objects).
func (p Promise) StrictEquals(other Value) (bool, error) {
	if err := p.check(); err != nil {
		return false, err
	}
	if err := other.check(); err != nil {
		return false, err
	}
	if other.iso != p.iso {
		return false, foreignIsolate("value")
	}
	r1, _, _ := proc("gov8_promise_strict_equals").Call(p.iso.handle, p.h, other.h)
	if int64(r1) < 0 {
		return false, shimError("Promise.StrictEquals", r1)
	}
	return r1 == 1, nil
}

const (
	promiseHandlerTypeError       = "gov8: promise handler is not a function"
	promiseHandlerTypeErrorDetail = "promise handler is not a function"
)

// checkPromiseHandlerLocal validates the Go-owned lifetime and ownership
// metadata for a handler without asking V8 for its dynamic type. The reaction
// shims accept a Value wire and check IsFunction before casting, so successful
// calls need only one native transition.
func checkPromiseHandlerLocal(v Value, iso *Isolate) error {
	if v.iso != iso {
		// Validate the foreign value first so zero, stale-scope, isolate-lifecycle,
		// and wrong-thread errors retain their established precedence over the
		// ownership error. Never ask the engine for its type: a local handle is
		// meaningful only to its owning isolate, and the promise shim has no
		// ownership metadata with which to reject a foreign wire safely.
		if err := v.check(); err != nil {
			return err
		}
		return foreignIsolate("handler")
	}
	if v.h == 0 {
		return fmt.Errorf("gov8: zero value handle")
	}
	if _, err := v.sc.checkedHandleAssumingIsolate(); err != nil {
		return err
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	return nil
}

// checkPromiseHandlerType performs the former standalone native type check.
// It remains only for Then2's negative slow path, where the first handler's
// type error must retain precedence over a local-validation error on the
// second handler.
func checkPromiseHandlerType(v Value, iso *Isolate) error {
	ensurePromiseHotProcs()
	r1, _, _ := syscall.Syscall(promiseIsFunctionAddr, 2, iso.handleAssumingCheck(), v.h, 0)
	if int64(r1) < 0 {
		return shimError("gov8_is_function", r1)
	}
	if r1 != 1 {
		return errors.New(promiseHandlerTypeError)
	}
	return nil
}

// mapPromiseReactionError restores the public Go error used by the former
// standalone predicate while retaining every other shim status and detail.
// The mapping is failure-only; successful calls avoid last-error work.
func mapPromiseReactionError(err error) error {
	shimErr, ok := err.(*ShimError)
	if ok && shimErr.Code == errBadArg && shimErr.Detail == promiseHandlerTypeErrorDetail {
		return errors.New(promiseHandlerTypeError)
	}
	return err
}

func promiseReactionError(op string, status uintptr) error {
	return mapPromiseReactionError(shimError(op, status))
}

// Then registers handler as the fulfillment AND rejection reaction
// (PerformPromiseThen with a fresh native promise as result) and returns the
// derived promise, which is always a distinct object. Under the Explicit
// microtasks policy the reaction job runs only on a microtask checkpoint.
// handler must be a function value of the same isolate — either a native
// function from NewNativeFunction or a JS function.
func (p Promise) Then(c *Context, handler Value) (Promise, error) {
	return p.thenImpl(c, handler, Value{}, "Then")
}

// Then2 is Then with separate fulfillment and rejection reactions.
func (p Promise) Then2(c *Context, onFulfilled, onRejected Value) (Promise, error) {
	return p.thenImpl(c, onFulfilled, onRejected, "Then2")
}

// thenImpl routes to the one-handler or two-handler shim export; a zero
// onRejected selects the one-handler path (used by Then).
func (p Promise) thenImpl(c *Context, onFulfilled, onRejected Value, op string) (Promise, error) {
	if err := p.check(); err != nil {
		return Promise{}, err
	}
	if c == nil || c.iso != p.iso {
		return Promise{}, foreignIsolate("context")
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return Promise{}, err
	}
	if err := checkPromiseHandlerLocal(onFulfilled, p.iso); err != nil {
		return Promise{}, err
	}
	if onRejected.h == 0 {
		ensurePromiseHotProcs()
		r1, _, _ := syscall.Syscall6(promiseThenAddr, 5,
			p.iso.handleAssumingCheck(), c.handle, p.sc.handle, p.h, onFulfilled.h, 0)
		if int64(r1) < 0 {
			return Promise{}, promiseReactionError("Promise."+op, r1)
		}
		if r1 == 0 {
			return Promise{}, shimError("Promise."+op, r1)
		}
		return Promise{Value{iso: p.iso, sc: p.sc, h: r1}}, nil
	}
	if err := checkPromiseHandlerLocal(onRejected, p.iso); err != nil {
		// Previously the first handler's native type check ran before any
		// validation of the second handler. Preserve that observable ordering
		// only on this negative path; valid calls rely on the Then2 shim's two
		// checked casts and avoid both standalone predicates.
		if firstTypeErr := checkPromiseHandlerType(onFulfilled, p.iso); firstTypeErr != nil {
			return Promise{}, firstTypeErr
		}
		return Promise{}, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_promise_then2").Call(
		p.iso.handle, c.handle, p.sc.handle, p.h, onFulfilled.h, onRejected.h,
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Promise{}, promiseReactionError("Promise."+op, r1)
	}
	return Promise{Value{iso: p.iso, sc: p.sc, h: out}}, nil
}

// Catch registers handler as the rejection reaction. ok is false when the
// engine returned no derived promise (the oracle's Option mapping).
func (p Promise) Catch(c *Context, handler Value) (Promise, bool, error) {
	if err := p.check(); err != nil {
		return Promise{}, false, err
	}
	if c == nil || c.iso != p.iso {
		return Promise{}, false, foreignIsolate("context")
	}
	if err := c.checkAssumingIsolate(); err != nil {
		return Promise{}, false, err
	}
	if err := checkPromiseHandlerLocal(handler, p.iso); err != nil {
		return Promise{}, false, err
	}
	var out uintptr
	var ok int32
	r1, _, _ := proc("gov8_promise_catch").Call(
		p.iso.handle, c.handle, p.sc.handle, p.h, handler.h,
		uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&ok)))
	if int64(r1) < 0 {
		return Promise{}, false, promiseReactionError("Promise.Catch", r1)
	}
	if ok == 0 {
		return Promise{}, false, nil
	}
	return Promise{Value{iso: p.iso, sc: p.sc, h: out}}, true, nil
}

// ---------------------------------------------------------------------------
// Native functions backed by the Go integer callback registry.
// ---------------------------------------------------------------------------

// NativePromiseHandler is the Go implementation behind a native function
// used as a promise reaction handler (or any other caller of that
// function). It runs on the isolate's owning thread during engine
// execution: inside reaction jobs on PerformMicrotaskCheckpoint, or
// synchronously during Resolve/Reject when handlers are attached to an
// already-settled promise.
//
// args are scope-local values valid ONLY for the duration of the call (the
// engine-owned handle scope around the callback closes when it returns);
// they must not be retained. Returning (Value{}, _) — or ok=false — leaves
// the JS return value undefined, which makes the derived promise fulfill
// with undefined. A returned Value must belong to an open scope of the same
// isolate. A panic prints a diagnostic and fail-fast aborts rather than
// unwinding into V8.
type NativePromiseHandler func(args []Value) (result Value, ok bool)

type promiseNativeEntry struct {
	iso *Isolate
	sc  *Scope
	fn  NativePromiseHandler
}

type promiseRejectEntry struct {
	iso *Isolate
	sc  *Scope
	fn  PromiseRejectCallback
}

var (
	// promiseRegMu guards the registries and the id counter. Callback
	// dispatch reads it on isolate owner threads; registration may come
	// from any thread that owns an isolate.
	promiseRegMu     sync.Mutex
	promiseRegID     int64
	promiseNativeReg = map[int64]promiseNativeEntry{}
	promiseRejectReg = map[int64]promiseRejectEntry{}
	// promiseRejectByIso tracks the live reject registration per isolate
	// (V8 keeps one reject callback per isolate; a new one supersedes it).
	promiseRejectByIso = map[*Isolate]int64{}
)

// NativeFunction is a scope-local JS function backed by a Go callback
// through the integer registry. The V8 function object lives (and dies)
// with the creating Scope; Close unregisters the Go side explicitly.
type NativeFunction struct {
	v      Value
	id     int64
	closed bool
}

// Value returns the scope-local function value for use with promise APIs
// (and anything else that takes a function). It remains a valid V8 function
// until the creating scope closes, even after Close — but after Close its
// Go callback is unregistered and invoking it simply returns undefined.
func (f *NativeFunction) Value() Value { return f.v }

// Close unregisters the Go callback entry. It does not touch the engine:
// the function object is scope-owned. Close is safe from any thread.
func (f *NativeFunction) Close() error {
	promiseRegMu.Lock()
	defer promiseRegMu.Unlock()
	if f.closed {
		return errors.New("gov8: native function already closed")
	}
	f.closed = true
	delete(promiseNativeReg, f.id)
	return nil
}

// promiseInstallEntries registers the process-wide dispatch trampolines
// with the shim exactly once. syscall.NewCallback entries are pinned by the
// Go runtime for the process lifetime, so the shim can retain them.
func promiseInstallEntries() {
	promiseEntriesOnce.Do(func() {
		native := syscall.NewCallback(promiseNativeDispatch)
		reject := syscall.NewCallback(promiseRejectDispatch)
		_, _, _ = proc("gov8_promise_fn_set_entry").Call(native)
		_, _, _ = proc("gov8_promise_reject_set_entry").Call(reject)
	})
}

var promiseEntriesOnce sync.Once

// nextRegistryID allocates a fresh positive registry id.
func nextRegistryID() int64 {
	promiseRegMu.Lock()
	defer promiseRegMu.Unlock()
	promiseRegID++
	return promiseRegID
}

// NewNativeFunction creates a native function in the context whose
// invocations dispatch to fn through the integer registry. The function is
// created in the scope and must be used while that scope is open.
func (s *Scope) NewNativeFunction(c *Context, fn NativePromiseHandler) (*NativeFunction, error) {
	if fn == nil {
		return nil, errors.New("gov8: nil native promise handler")
	}
	if err := s.check(); err != nil {
		return nil, err
	}
	if c == nil || c.iso != s.iso {
		return nil, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return nil, err
	}
	ih, err := s.iso.handleChecked()
	if err != nil {
		return nil, err
	}
	promiseInstallEntries()
	id := nextRegistryID()
	promiseRegMu.Lock()
	promiseNativeReg[id] = promiseNativeEntry{iso: s.iso, sc: s, fn: fn}
	promiseRegMu.Unlock()
	var out uintptr
	r1, _, _ := proc("gov8_promise_fn_new").Call(
		ih, c.handle, s.handle, uintptr(id), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		promiseRegMu.Lock()
		delete(promiseNativeReg, id)
		promiseRegMu.Unlock()
		return nil, shimError("NewNativeFunction", r1)
	}
	return &NativeFunction{v: Value{iso: s.iso, sc: s, h: out}, id: id}, nil
}

// abiWordToPtr reinterprets a pointer-sized word received across the ABI as
// a Go pointer. The word originates from the shim's stack (the argv array
// and the rv out-slot exist only for the duration of this synchronous
// callback) and is never retained. The indirect round-trip through a local
// is the vet-clean form of unsafe.Pointer(w).
func abiWordToPtr(w uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&w))
}

// promiseNativeDispatch is the shim's entry into Go for native function
// invocation: (id, argc, argv, rvOut) -> 1 when a return value was written.
// A panic is recovered only to prevent unwinding into V8, then translated
// into the same process fail-fast abort as the pinned Rust callback boundary.
func promiseNativeDispatch(id, argc, argvPtr, rvOut uintptr) (handled uintptr) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in native promise handler: %v\n", r)
			proc("gov8_host_panic_abort").Call()
			panic(r) // unreachable: the abort does not return
		}
	}()
	n := int(int64(argc))
	promiseRegMu.Lock()
	e, ok := promiseNativeReg[int64(id)]
	promiseRegMu.Unlock()
	if !ok {
		return 0
	}
	// The callback fires on the isolate's owning thread mid-execution;
	// refuse closed/cross-thread misuse instead of touching the engine.
	if err := e.iso.check(); err != nil {
		return 0
	}
	if err := e.sc.check(); err != nil {
		return 0
	}
	var args []Value
	if n > 0 && argvPtr != 0 {
		wires := unsafe.Slice((*uintptr)(abiWordToPtr(argvPtr)), n)
		args = make([]Value, n)
		for i, w := range wires {
			// Slot words from the trampoline's engine-owned scope: valid
			// exactly for this call, bound to the registration scope.
			args[i] = Value{iso: e.iso, sc: e.sc, h: w}
		}
	}
	result, ok2 := e.fn(args)
	if !ok2 || result.h == 0 {
		return 0
	}
	if result.iso != e.iso {
		return 0
	}
	if err := result.check(); err != nil {
		return 0
	}
	*(*uintptr)(abiWordToPtr(rvOut)) = result.h
	return 1
}

// ---------------------------------------------------------------------------
// Promise-reject callback.
// ---------------------------------------------------------------------------

// PromiseRejectEvent mirrors v8::PromiseRejectEvent. The AfterResolved
// events were removed from V8 and never fire on the pinned build; they are
// kept for API parity.
type PromiseRejectEvent int

const (
	PromiseRejectWithNoHandler     PromiseRejectEvent = 0
	PromiseHandlerAddedAfterReject PromiseRejectEvent = 1
	PromiseRejectAfterResolved     PromiseRejectEvent = 2
	PromiseResolveAfterResolved    PromiseRejectEvent = 3
)

// String returns the short oracle event names used in normalized output.
func (e PromiseRejectEvent) String() string {
	switch e {
	case PromiseRejectWithNoHandler:
		return "WithNoHandler"
	case PromiseHandlerAddedAfterReject:
		return "HandlerAddedAfterReject"
	case PromiseRejectAfterResolved:
		return "RejectAfterResolved"
	case PromiseResolveAfterResolved:
		return "ResolveAfterResolved"
	}
	return fmt.Sprintf("PromiseRejectEvent(%d)", int(e))
}

// PromiseRejectMessage is delivered to the isolate's promise-reject
// callback synchronously: at reject time when no handler exists
// (WithNoHandler), when a handler is attached to a previously rejected
// promise (HandlerAddedAfterReject), or from a reaction job when a derived
// promise is left rejected and unhandled (WithNoHandler again).
//
// Promise (and the value, when present) are scope-local handles valid only
// for the duration of the callback; they are bound to the scope the
// callback was registered with, which must still be open.
type PromiseRejectMessage struct {
	Event   PromiseRejectEvent
	Promise Promise
	v       Value
}

// Value returns the rejection value; ok is false for events that carry no
// value (HandlerAddedAfterReject).
func (m PromiseRejectMessage) Value() (Value, bool) {
	return m.v, m.v.h != 0
}

// PromiseRejectCallback observes promise rejection events. It runs on the
// isolate's owning thread during engine execution. A panic prints a
// diagnostic and fail-fast aborts rather than unwinding into V8.
type PromiseRejectCallback func(PromiseRejectMessage)

// SetPromiseRejectCallback installs cb as the isolate's promise-reject
// callback. Installing again replaces the previous callback (V8 keeps one
// per isolate); the superseded Go registration is dropped.
//
// The scope anchors the handles delivered to the callback: it must belong
// to the isolate and stay open while the callback is installed (any Go
// scope used to drive the engine during a rejection satisfies this in
// practice). This parameter is a Go-side safety binding with no Rust
// counterpart — the Rust API takes only the callback.
func (i *Isolate) SetPromiseRejectCallback(s *Scope, cb PromiseRejectCallback) error {
	if cb == nil {
		return errors.New("gov8: nil promise reject callback")
	}
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	if s == nil || s.iso != i {
		return foreignIsolate("scope")
	}
	if _, err := s.checkedHandle(); err != nil {
		return err
	}
	promiseInstallEntries()
	id := nextRegistryID()
	promiseRegMu.Lock()
	promiseRejectReg[id] = promiseRejectEntry{iso: i, sc: s, fn: cb}
	previous, had := promiseRejectByIso[i]
	promiseRejectByIso[i] = id
	if had {
		delete(promiseRejectReg, previous)
	}
	promiseRegMu.Unlock()
	r1, _, _ := proc("gov8_isolate_set_promise_reject_callback").Call(ih, uintptr(id))
	if int64(r1) < 0 {
		promiseRegMu.Lock()
		delete(promiseRejectReg, id)
		if had {
			// Shim call failed; keep a consistent registry.
			promiseRejectByIso[i] = previous
		} else {
			delete(promiseRejectByIso, i)
		}
		promiseRegMu.Unlock()
		return shimError("SetPromiseRejectCallback", r1)
	}
	return nil
}

// ClearPromiseRejectCallback removes the isolate's promise-reject callback
// and its Go registration. Clearing without a prior registration is a
// no-op on the engine side and returns nil.
func (i *Isolate) ClearPromiseRejectCallback() error {
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	promiseRegMu.Lock()
	id, ok := promiseRejectByIso[i]
	if ok {
		delete(promiseRejectByIso, i)
		delete(promiseRejectReg, id)
	}
	promiseRegMu.Unlock()
	r1, _, _ := proc("gov8_isolate_clear_promise_reject_callback").Call(ih)
	if int64(r1) < 0 {
		return shimError("ClearPromiseRejectCallback", r1)
	}
	return nil
}

// promiseRejectDispatch is the shim's entry into Go for rejection events:
// (id, event, value wire or 0, promise wire). Panics use the same fail-fast
// translation as promiseNativeDispatch.
func promiseRejectDispatch(id, event, valueWire, promiseWire uintptr) (handled uintptr) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in promise reject callback: %v\n", r)
			proc("gov8_host_panic_abort").Call()
			panic(r) // unreachable: the abort does not return
		}
	}()
	promiseRegMu.Lock()
	e, ok := promiseRejectReg[int64(id)]
	promiseRegMu.Unlock()
	if !ok {
		return 0
	}
	if err := e.iso.check(); err != nil {
		return 0
	}
	if err := e.sc.check(); err != nil {
		return 0
	}
	msg := PromiseRejectMessage{
		Event:   PromiseRejectEvent(int64(event)),
		Promise: Promise{Value{iso: e.iso, sc: e.sc, h: promiseWire}},
		v:       Value{iso: e.iso, sc: e.sc, h: valueWire},
	}
	e.fn(msg)
	return 1
}
