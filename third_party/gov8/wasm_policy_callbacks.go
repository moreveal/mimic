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

// AllowWasmCodeGenerationCallback decides whether synchronous WebAssembly
// compilation may proceed in the originating context. source is V8's source
// string (empty for the WebAssembly.Module constructor in the pinned engine).
// The callback may schedule a JavaScript exception with CallbackScope.
type AllowWasmCodeGenerationCallback func(*CallbackScope, Value) bool

// WasmAsyncSuccess is the completion verdict passed to the asynchronous Wasm
// promise callback.
type WasmAsyncSuccess int32

const (
	WasmAsyncSuccessSuccess WasmAsyncSuccess = 0
	WasmAsyncSuccessFail    WasmAsyncSuccess = 1
)

func (s WasmAsyncSuccess) String() string {
	switch s {
	case WasmAsyncSuccessSuccess:
		return "Success"
	case WasmAsyncSuccessFail:
		return "Fail"
	default:
		return fmt.Sprintf("WasmAsyncSuccess(%d)", int32(s))
	}
}

// WasmAsyncResolvePromiseCallback receives V8's resolver and result in the
// originating context. V8 does not settle the resolver after a custom
// callback is installed; the callback must call resolution.Settle (or use
// CallbackScope.SettleCallbackPromise explicitly).
type WasmAsyncResolvePromiseCallback func(*WasmAsyncResolution)

// WasmAsyncResolution is callback-local. Resolver, Result and CallbackScope
// become invalid as soon as the callback returns.
type WasmAsyncResolution struct {
	CallbackScope *CallbackScope
	Resolver      PromiseResolver
	Result        Value
	Success       WasmAsyncSuccess
}

// Promise returns the exact promise associated with Resolver.
func (r *WasmAsyncResolution) Promise() (Promise, error) {
	if r == nil || r.CallbackScope == nil {
		return Promise{}, errors.New("gov8: invalid wasm async resolution")
	}
	return r.Resolver.GetPromise(r.CallbackScope.Scope())
}

// Settle resolves on Success and rejects on Fail using V8's supplied result.
func (r *WasmAsyncResolution) Settle() (bool, error) {
	if r == nil || r.CallbackScope == nil {
		return false, errors.New("gov8: invalid wasm async resolution")
	}
	if r.Success != WasmAsyncSuccessSuccess && r.Success != WasmAsyncSuccessFail {
		return false, errors.New("gov8: invalid wasm async success value")
	}
	return r.CallbackScope.SettleCallbackPromise(
		r.Resolver, r.Result, r.Success == WasmAsyncSuccessFail)
}

type wasmPolicyAllowFrame struct {
	isolate     uintptr
	contextWire uintptr
	scopeWire   uintptr
	sourceWire  uintptr
}

type wasmPolicyAsyncFrame struct {
	isolate      uintptr
	contextWire  uintptr
	scopeWire    uintptr
	resolverWire uintptr
	resultWire   uintptr
	success      int32
	_pad         int32
}

var (
	wasmPolicyDispatchersOnce sync.Once
	wasmPolicyDispatchersErr  error
)

// Marker combinations deliberately use otherwise impossible host-entry
// shapes. This lets the shared host registry own callback lifetime and lets
// ReleaseIsolateHostState drain policy callbacks without another hook.
func wasmPolicyAllowMarker(*CallbackScope, Value, PropertyCallbackArguments, ReturnValue) Intercepted {
	return InterceptedNo
}

func wasmPolicyAsyncMarker(*CallbackScope, uint32, PropertyCallbackArguments, ReturnValue) Intercepted {
	return InterceptedNo
}

func findWasmPolicyEntry(iso *Isolate, async bool) (uint64, *hostCallbackEntry) {
	hostCallbackRegistry.mu.Lock()
	defer hostCallbackRegistry.mu.Unlock()
	var selected uint64
	var result *hostCallbackEntry
	for id, entry := range hostCallbackRegistry.entries {
		if entry == nil || entry.iso != iso || entry.fn == nil {
			continue
		}
		marked := entry.ndesc != nil && entry.idesc == nil
		if async {
			marked = entry.idesc != nil && entry.ndesc == nil
		}
		if marked && id > selected {
			selected, result = id, entry
		}
	}
	return selected, result
}

func withWasmPolicyCallback(iso *Isolate, entry *hostCallbackEntry, scopeWire, contextWire uintptr, fn func(*hostCallbackEntry, *CallbackScope)) uintptr {
	if err := iso.check(); err != nil {
		fatalHostMisuse("wasm policy callback lifecycle: %v", err)
		return 1
	}
	beginWasmCallback(iso)
	defer endWasmCallback(iso)
	borrowed := &Scope{iso: iso, handle: scopeWire, borrowed: true}
	cs := &CallbackScope{iso: iso, sc: borrowed, ctxWire: contextWire}
	defer func() {
		borrowed.closed = true
		cs.iso, cs.sc, cs.ctxWire = nil, nil, 0
	}()
	if entry == nil {
		fatalHostMisuse("wasm policy callback has no registered host entry")
		return 1
	}
	fn(entry, cs)
	return 0
}

var wasmPolicyAllowDispatcher = syscall.NewCallback(func(frame *wasmPolicyAllowFrame) (result uintptr) {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in wasm allow-code-generation callback: %v\n", recovered)
			proc("gov8_host_panic_abort").Call()
			panic(recovered)
		}
	}()
	if frame == nil {
		fatalHostMisuse("nil wasm allow callback frame")
		return 0
	}
	iso := isolateForNativeHandle(frame.isolate)
	if iso == nil {
		fatalHostMisuse("wasm allow callback for unknown isolate")
		return 0
	}
	_, entry := findWasmPolicyEntry(iso, false)
	if entry == nil {
		fatalHostMisuse("wasm allow callback has no registered entry")
		return 0
	}
	allowed := false
	_ = withWasmPolicyCallback(iso, entry, frame.scopeWire, frame.contextWire, func(entry *hostCallbackEntry, cs *CallbackScope) {
		fake := &hostCallbackFrame{propertyWire: frame.sourceWire}
		entry.fn(cs, FunctionCallbackArguments{cs: cs, frame: fake}, ReturnValue{})
		allowed = fake.outIntercepted == 1
	})
	if allowed {
		return 1
	}
	return 0
})

var wasmPolicyAsyncDispatcher = syscall.NewCallback(func(frame *wasmPolicyAsyncFrame) (result uintptr) {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in wasm async-resolve callback: %v\n", recovered)
			proc("gov8_host_panic_abort").Call()
			panic(recovered)
		}
	}()
	if frame == nil {
		fatalHostMisuse("nil wasm async callback frame")
		return 1
	}
	iso := isolateForNativeHandle(frame.isolate)
	if iso == nil {
		fatalHostMisuse("wasm async callback for unknown isolate")
		return 1
	}
	_, entry := findWasmPolicyEntry(iso, true)
	if entry == nil {
		fatalHostMisuse("wasm async callback has no registered entry")
		return 1
	}
	return withWasmPolicyCallback(iso, entry, frame.scopeWire, frame.contextWire, func(entry *hostCallbackEntry, cs *CallbackScope) {
		fake := &hostCallbackFrame{dataWire: frame.resolverWire, valueWire: frame.resultWire, flags: frame.success}
		entry.fn(cs, FunctionCallbackArguments{cs: cs, frame: fake}, ReturnValue{})
	})
})

func isolateForNativeHandle(handle uintptr) *Isolate {
	hostCallbackRegistry.mu.Lock()
	defer hostCallbackRegistry.mu.Unlock()
	for _, entry := range hostCallbackRegistry.entries {
		if entry != nil && entry.iso != nil && entry.iso.handle == handle &&
			(entry.ndesc != nil || entry.idesc != nil) {
			return entry.iso
		}
	}
	return nil
}

func ensureWasmPolicyDispatchers() error {
	wasmPolicyDispatchersOnce.Do(func() {
		wasmPolicyDispatchersErr = callErr("WasmPolicy.SetDispatchers",
			proc("gov8_wpc_set_dispatchers"), wasmPolicyAllowDispatcher, wasmPolicyAsyncDispatcher)
	})
	return wasmPolicyDispatchersErr
}

// SetAllowWasmCodeGenerationCallback replaces the isolate's policy callback.
// The pinned API has no clear operation; install another callback to replace
// it. ReleaseIsolateHostState drains the Go registration before isolate close.
func (i *Isolate) SetAllowWasmCodeGenerationCallback(callback AllowWasmCodeGenerationCallback) error {
	if callback == nil {
		return errors.New("gov8: wasm code-generation callback is required")
	}
	if err := i.check(); err != nil {
		return err
	}
	if err := ensureWasmPolicyDispatchers(); err != nil {
		return err
	}
	old, _ := findWasmPolicyEntry(i, false)
	entry := &hostCallbackEntry{ndesc: wasmPolicyAllowMarker}
	entry.fn = func(cs *CallbackScope, args FunctionCallbackArguments, _ ReturnValue) {
		if callback(cs, cs.wrap(args.frame.propertyWire)) {
			args.frame.outIntercepted = 1
		}
	}
	id, err := registerHostEntry(i, entry, Value{})
	if err != nil {
		return err
	}
	if err := callErr("Isolate.SetAllowWasmCodeGenerationCallback",
		proc("gov8_wpc_set_allow"), i.handleAssumingCheck()); err != nil {
		dropHostCallback(id)
		return err
	}
	if old != 0 {
		dropHostCallback(old)
	}
	return nil
}

// SetWasmAsyncResolvePromiseCallback replaces the callback V8 invokes for
// WebAssembly.compile/instantiate promise completion. There is no clear API.
func (i *Isolate) SetWasmAsyncResolvePromiseCallback(callback WasmAsyncResolvePromiseCallback) error {
	if callback == nil {
		return errors.New("gov8: wasm async-resolve callback is required")
	}
	if err := i.check(); err != nil {
		return err
	}
	if err := ensureWasmPolicyDispatchers(); err != nil {
		return err
	}
	old, _ := findWasmPolicyEntry(i, true)
	entry := &hostCallbackEntry{idesc: wasmPolicyAsyncMarker}
	entry.fn = func(cs *CallbackScope, args FunctionCallbackArguments, _ ReturnValue) {
		resolution := &WasmAsyncResolution{
			CallbackScope: cs,
			Resolver:      PromiseResolver{Value{iso: cs.iso, sc: cs.sc, h: args.frame.dataWire}},
			Result:        cs.wrap(args.frame.valueWire),
			Success:       WasmAsyncSuccess(args.frame.flags),
		}
		defer func() {
			resolution.CallbackScope = nil
			resolution.Resolver = PromiseResolver{}
			resolution.Result = Value{}
		}()
		callback(resolution)
	}
	id, err := registerHostEntry(i, entry, Value{})
	if err != nil {
		return err
	}
	if err := callErr("Isolate.SetWasmAsyncResolvePromiseCallback",
		proc("gov8_wpc_set_async"), i.handleAssumingCheck()); err != nil {
		dropHostCallback(id)
		return err
	}
	if old != 0 {
		dropHostCallback(old)
	}
	return nil
}

// CurrentContextGlobal returns the originating context's global object as a
// callback-local value.
func (cs *CallbackScope) CurrentContextGlobal() (Value, error) {
	if err := cs.check(); err != nil {
		return Value{}, err
	}
	var out uintptr
	r, _, _ := proc("gov8_wpc_context_global").Call(
		cs.iso.handleAssumingCheck(), cs.ctxWire, cs.sc.handle, uintptr(unsafe.Pointer(&out)))
	if int64(r) < 0 {
		return Value{}, shimError("CallbackScope.CurrentContextGlobal", r)
	}
	return cs.wrap(out), nil
}
