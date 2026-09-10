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

// Serializer / deserializer DELEGATE completion (the companion of
// serializer.go, which ships the base wire surface with default delegate
// behaviors). This file routes every remaining hook of the pinned crate's
// ValueSerializerImpl / ValueDeserializerImpl to Go.
//
// Rust surface mapped here (pinned crate v8 =152.2.0; semantics pinned by
// rust-oracle/src/bin/conformance-serializer-delegates.rs):
//
//	ValueSerializerImpl (throw_data_clone_error is required and lives on
//	ValueSerializerDelegate in serializer.go; the rest are optional)
//	  has_custom_host_object(isolate) -> bool
//	  is_host_object(scope, object) -> Option<bool>
//	  write_host_object(scope, object, ser) -> Option<bool>
//	  get_shared_array_buffer_id(scope, sab) -> Option<u32>
//	  get_wasm_module_transfer_id(scope, module) -> Option<u32>
//	ValueDeserializerImpl (all hooks optional; the empty delegate is valid)
//	  read_host_object(scope, deser) -> Option<Local<Object>>
//	  get_shared_array_buffer_from_id(id) -> Option<Local<SharedArrayBuffer>>
//	  get_wasm_module_from_id(id) -> Option<Local<WasmModuleObject>>
//
// Hook detection. A delegate opts into a hook by implementing its
// single-method interface; the constructor computes a bitmask, and the shim
// consults Go ONLY for implemented hooks. Unimplemented hooks run the
// pinned crate's trait defaults verbatim inside the shim, un-instrumented
// (the oracle pins that their counters stay 0).
//
// Pinned completion semantics reproduced by this pairing:
//
//   - write_host_object: the release build ignores the delegate's
//     ok/answered result once no exception is pending — only a thrown
//     exception aborts the write, right after the 0x5c tag the engine
//     wrote first. A delegate that returns (false, true) without throwing
//     therefore still succeeds.
//   - is_host_object answered=false (None): the write fails with no
//     exception (V8 propagates the Nothing).
//   - get_shared_array_buffer_id answered=false: the pinned build throws
//     V8's OWN "#<SharedArrayBuffer> could not be cloned." — not routed
//     through ThrowDataCloneError.
//   - get_wasm_module_transfer_id answered=false: the module silently
//     disappears from the wire while the enclosing write succeeds (the
//     pinned asymmetry with the SAB path).
//   - Deserializer hooks answered=false: never a clean failure — the
//     engine throws "Unable to deserialize cloned data.", which the
//     caller's TryCatch observes.
//
// Delegate model. Identical to the native-callback registries: delegates
// live in an integer registry below; the engine only ever sees an int64 id.
// One process-wide trampoline (syscall.NewCallback, pinned by the Go
// runtime) is registered with the shim once. No Go pointer ever crosses
// into the engine.
//
// Panic boundary. A panic inside any hook is recovered and deliberately
// translated into the process fail-fast abort (gov8_host_panic_abort, exit
// code 0xC0000409 on Windows) after printing the message — the observable
// equivalent of the pinned oracle, where a panic unwinding out of the
// crate's extern "C" delegate trampolines aborts the process. Returning a
// panic into the engine instead would unwind through C++ frames, which is
// unsupported and corrupts V8 state.
//
// Lifetimes.
//
//   - DelegateValueSerializer / DelegateValueDeserializer are closed
//     explicitly (Close); there are no finalizers. Close destroys the
//     engine object first, then the shim delegate, then unregisters the Go
//     delegate — after Close the delegate can no longer be invoked. A
//     serializer used after Release, and either type used after Close,
//     return errors (the crate panics; this module's documented
//     panic-to-error deviation). A second Release returns empty bytes and
//     no error, exactly like the crate's release() (fixture-pinned).
//   - Values handed to a hook (objects, SABs, the wasm module view) are
//     scope-local engine handles valid ONLY during the hook call; a hook
//     must not retain them.
//   - Values RETURNED by hooks (host objects, SABs from id) must be built
//     through the passed receiver's Scope (the hook's engine scope is
//     still open); the shim re-roots them before the engine consumes the
//     result.
//   - Deserializer input bytes are retained without copying until Close
//     (the engine's real contract, pinned by the oracle).
//
// Wasm transfer IDs are fully delegated in both directions. The original
// GetWasmModuleFromIDHook remains observation-only for compatibility;
// ResolveWasmModuleFromIDHook is the typed returning form and validates the
// target isolate, callback scope, lifetime, and Wasm type before FFI.
//
// Integers only: the registry stores Go values under int64 handles; the
// engine never observes a Go pointer.

// --- optional serializer hook interfaces -------------------------------------

// HasCustomHostObjectHook mirrors v8::ValueSerializerImpl::
// has_custom_host_object. Return true to have the engine consult
// IsHostObject for every new plain object; false keeps the embedder-field
// fallback (objects with internal fields are host objects). The engine
// consults it once per serializer, at construction.
type HasCustomHostObjectHook interface {
	HasCustomHostObject() bool
}

// IsHostObjectHook mirrors v8::ValueSerializerImpl::is_host_object. It
// fires for each new plain object when HasCustomHostObject returned true.
// answered=false maps to None: the write fails without an exception.
type IsHostObjectHook interface {
	IsHostObject(obj *Object) (isHost, answered bool)
}

// WriteHostObjectHook mirrors v8::ValueSerializerImpl::write_host_object.
// w is the serializer itself (the crate passes the ValueSerializer as its
// helper trait object): the hook writes its own bytes with w.WriteUint32 /
// WriteRawBytes / WriteDouble / WriteValue / .... The engine ignores the
// returned ok once no exception is pending (pinned release-build
// semantics); answered=false maps to None.
type WriteHostObjectHook interface {
	WriteHostObject(obj *Object, w *DelegateValueSerializer) (ok, answered bool)
}

// GetSharedArrayBufferIDHook mirrors v8::ValueSerializerImpl::
// get_shared_array_buffer_id. answered=false maps to None, which the
// pinned build rejects with V8's own data-clone error.
type GetSharedArrayBufferIDHook interface {
	GetSharedArrayBufferID(sab *SharedArrayBuffer) (id uint32, answered bool)
}

// GetWasmModuleTransferIDHook mirrors v8::ValueSerializerImpl::
// get_wasm_module_transfer_id. module is the generic value view of the
// WasmModuleObject. answered=false maps to None: the module silently
// disappears from the wire and the enclosing write succeeds.
type GetWasmModuleTransferIDHook interface {
	GetWasmModuleTransferID(module Value) (id uint32, answered bool)
}

// --- optional deserializer hook interfaces -----------------------------------

// ValueDeserializerDelegate is the (fully optional) analogue of the pinned
// crate's ValueDeserializerImpl: every hook below is optional, so the
// empty interface is a valid delegate reproducing the trait defaults. nil
// is accepted and means the same.
type ValueDeserializerDelegate interface{}

// ReadHostObjectHook mirrors v8::ValueDeserializerImpl::read_host_object.
// Build the object with r.Scope()/r.Context() and the usual constructors;
// consume the wire with r.ReadUint32 / r.ReadRawBytes / .... found=false
// maps to None: the engine throws "Unable to deserialize cloned data."
// (a silent None is never a clean read on this build).
type ReadHostObjectHook interface {
	ReadHostObject(r *DelegateValueDeserializer) (*Object, bool)
}

// GetSharedArrayBufferFromIDHook mirrors v8::ValueDeserializerImpl::
// get_shared_array_buffer_from_id. found=false maps to None and the same
// engine error. Note the pinned semantics: transfer_shared_array_buffer
// registrations are NEVER consulted for the SAB tag — this hook is always
// the only source.
type GetSharedArrayBufferFromIDHook interface {
	GetSharedArrayBufferFromID(id uint32) (*SharedArrayBuffer, bool)
}

// GetWasmModuleFromIDHook is the original observation-only shape for
// v8::ValueDeserializerImpl::get_wasm_module_from_id. Implementing it switches
// the read from the trait-default "not implemented" throw to the
// None-completion path. Use ResolveWasmModuleFromIDHook to return a module.
type GetWasmModuleFromIDHook interface {
	GetWasmModuleFromID(id uint32)
}

// ResolveWasmModuleFromIDHook is the typed completion of
// ValueDeserializerImpl::get_wasm_module_from_id. The returned module must
// be a live WasmModuleObject created in r.Scope() for the target isolate.
// found=false maps to None and V8's generic cloned-data error. The older
// GetWasmModuleFromIDHook remains supported as an observation-only hook.
type ResolveWasmModuleFromIDHook interface {
	ResolveWasmModuleFromID(r *DelegateValueDeserializer, id uint32) (module *WasmModuleObject, found bool)
}

// --- registry and trampoline --------------------------------------------------

// Hook op codes. Part of the shim ABI contract (keep in sync with the
// serialization_delegates.inc feature).
const (
	serOpThrowDataCloneError = iota + 1
	serOpHasCustomHostObject
	serOpIsHostObject
	serOpWriteHostObject
	serOpGetSABId
	serOpGetWasmTransferID
	serOpReadHostObject
	serOpGetSABFromID
	serOpGetWasmFromID
	serOpResolveWasmFromID
)

// Implemented-hook mask bits (shim ABI contract). Serializer and
// deserializer masks are distinct namespaces.
const (
	serHookHasCustomHostObject = 1 << iota
	serHookIsHostObject
	serHookWriteHostObject
	serHookGetSABID
	serHookGetWasmTransferID
)

const (
	deserHookReadHostObject = 1 << iota
	deserHookGetSABFromID
	deserHookGetWasmFromID
	deserHookResolveWasmFromID
)

type serDelEntry struct {
	iso *Isolate
	sc  *Scope
	ctx *Context
	// hook is the user delegate; owner is the wrapper whose methods hooks
	// may call back into. owner is filled before any owner-taking hook can
	// fire (only HasCustomHostObject fires during construction).
	hook  any
	owner any
}

var serDelRegistry = struct {
	mu      sync.Mutex
	next    int64
	entries map[int64]*serDelEntry
}{entries: make(map[int64]*serDelEntry)}

func serDelRegister(e *serDelEntry) (int64, error) {
	serDelRegistry.mu.Lock()
	defer serDelRegistry.mu.Unlock()
	if serDelRegistry.next < 0 || serDelRegistry.next == int64(^uint64(0)>>1) {
		return 0, errors.New("gov8: serializer delegate registry exhausted")
	}
	serDelRegistry.next++
	id := serDelRegistry.next
	if id == 0 {
		return 0, errors.New("gov8: serializer delegate registry exhausted")
	}
	serDelRegistry.entries[id] = e
	return id, nil
}

func serDelUnregister(id int64) {
	serDelRegistry.mu.Lock()
	delete(serDelRegistry.entries, id)
	serDelRegistry.mu.Unlock()
}

func serDelLookup(id int64) *serDelEntry {
	serDelRegistry.mu.Lock()
	e := serDelRegistry.entries[id]
	serDelRegistry.mu.Unlock()
	return e
}

var (
	serDelOnce sync.Once
	serDelErr  error
)

var goSerDelDispatch = syscall.NewCallback(func(id, op, a, b uintptr) uintptr {
	entry := serDelLookup(int64(id))
	if entry == nil {
		return 0
	}
	// A panic inside a hook must never unwind into the engine. Mirror the
	// native-callback boundary: print, fail-fast abort (0xC0000409),
	// unreachable repanic keeps the compiler honest about the defer.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "gov8: panic in serializer/deserializer delegate: %v\n", r)
			proc("gov8_host_panic_abort").Call()
			panic(r) // unreachable: the abort does not return
		}
	}()

	iso, sc := entry.iso, entry.sc
	switch int64(op) {
	case serOpThrowDataCloneError:
		d, ok := entry.hook.(ValueSerializerDelegate)
		if !ok {
			return 0
		}
		return boolWord(d.ThrowDataCloneError(copyCLenString(a, b)))
	case serOpHasCustomHostObject:
		d, ok := entry.hook.(HasCustomHostObjectHook)
		if !ok {
			return 0
		}
		return boolWord(d.HasCustomHostObject())
	case serOpIsHostObject:
		d, ok := entry.hook.(IsHostObjectHook)
		if !ok {
			return 2
		}
		isHost, answered := d.IsHostObject(&Object{Value: Value{iso: iso, sc: sc, h: a}})
		if !answered {
			return 2
		}
		return boolWord(isHost)
	case serOpWriteHostObject:
		d, ok := entry.hook.(WriteHostObjectHook)
		w, ok2 := entry.owner.(*DelegateValueSerializer)
		if !ok || !ok2 || w == nil {
			return 0
		}
		// The pinned release build ignores the result; the shim reports
		// Just(true) and the engine observes any pending exception itself.
		_, _ = d.WriteHostObject(&Object{Value: Value{iso: iso, sc: sc, h: a}}, w)
		return 1
	case serOpGetSABId:
		d, ok := entry.hook.(GetSharedArrayBufferIDHook)
		if !ok {
			return 0
		}
		sabID, answered := d.GetSharedArrayBufferID(&SharedArrayBuffer{Value: Value{iso: iso, sc: sc, h: a}})
		if !answered {
			return 0
		}
		return uintptr(sabID) + 1 // reserve 0 for None
	case serOpGetWasmTransferID:
		d, ok := entry.hook.(GetWasmModuleTransferIDHook)
		if !ok {
			return 0
		}
		wasmID, answered := d.GetWasmModuleTransferID(Value{iso: iso, sc: sc, h: a})
		if !answered {
			return 0
		}
		return uintptr(wasmID) + 1
	case serOpReadHostObject:
		d, ok := entry.hook.(ReadHostObjectHook)
		r, ok2 := entry.owner.(*DelegateValueDeserializer)
		if !ok || !ok2 || r == nil {
			return 0
		}
		obj, found := d.ReadHostObject(r)
		if !found || obj == nil {
			return 0
		}
		*(*uintptr)(abiWordToPtr(a)) = obj.h
		return 1
	case serOpGetSABFromID:
		d, ok := entry.hook.(GetSharedArrayBufferFromIDHook)
		if !ok {
			return 0
		}
		sab, found := d.GetSharedArrayBufferFromID(uint32(a))
		if !found || sab == nil {
			return 0
		}
		*(*uintptr)(abiWordToPtr(b)) = sab.h
		return 1
	case serOpGetWasmFromID:
		if d, ok := entry.hook.(GetWasmModuleFromIDHook); ok {
			d.GetWasmModuleFromID(uint32(a))
		}
		return 0 // preserved observation-only completion
	case serOpResolveWasmFromID:
		d, ok := entry.hook.(ResolveWasmModuleFromIDHook)
		r, ok2 := entry.owner.(*DelegateValueDeserializer)
		if !ok || !ok2 || r == nil {
			return 0
		}
		module, found := d.ResolveWasmModuleFromID(r, uint32(a))
		if !found || module == nil {
			return 0
		}
		if module.iso != iso || module.sc != sc {
			return rejectResolvedWasmModule(r, "resolved Wasm module must use the deserializer callback scope")
		}
		if err := module.check(); err != nil {
			return rejectResolvedWasmModule(r, "resolved Wasm module is no longer live")
		}
		isModule, err := module.Value.IsWasmModuleObject()
		if err != nil || !isModule {
			return rejectResolvedWasmModule(r, "resolved value is not a WasmModuleObject")
		}
		*(*uintptr)(abiWordToPtr(b)) = module.h
		return 1
	default:
		// Unknown op: report "did not handle" and keep the boundary inert.
		fmt.Fprintf(os.Stderr, "gov8: unknown serializer delegate op %d\n", int64(op))
		return 0
	}
})

func installSerDelEntry() error {
	serDelOnce.Do(func() {
		serDelErr = callErr("SerDelegateEntry",
			proc("gov8_ser_delegate_set_entry"), goSerDelDispatch)
	})
	return serDelErr
}

func rejectResolvedWasmModule(r *DelegateValueDeserializer, message string) uintptr {
	exception, err := r.NewError("gov8: " + message)
	if err == nil {
		_ = r.ThrowException(exception)
	}
	return 0
}

// serHookMask computes the implemented-hook bitmask for a serializer
// delegate (shim ABI contract: unimplemented hooks never cross the
// boundary, so their crate-default behavior runs un-instrumented).
func serHookMask(d ValueSerializerDelegate) uint32 {
	var mask uint32
	if _, ok := d.(HasCustomHostObjectHook); ok {
		mask |= serHookHasCustomHostObject
	}
	if _, ok := d.(IsHostObjectHook); ok {
		mask |= serHookIsHostObject
	}
	if _, ok := d.(WriteHostObjectHook); ok {
		mask |= serHookWriteHostObject
	}
	if _, ok := d.(GetSharedArrayBufferIDHook); ok {
		mask |= serHookGetSABID
	}
	if _, ok := d.(GetWasmModuleTransferIDHook); ok {
		mask |= serHookGetWasmTransferID
	}
	return mask
}

// deserHookMask is serHookMask for deserializer delegates (nil is the
// empty delegate: no hooks).
func deserHookMask(d ValueDeserializerDelegate) uint32 {
	if d == nil {
		return 0
	}
	var mask uint32
	if _, ok := d.(ReadHostObjectHook); ok {
		mask |= deserHookReadHostObject
	}
	if _, ok := d.(GetSharedArrayBufferFromIDHook); ok {
		mask |= deserHookGetSABFromID
	}
	if _, ok := d.(GetWasmModuleFromIDHook); ok {
		mask |= deserHookGetWasmFromID
	}
	if _, ok := d.(ResolveWasmModuleFromIDHook); ok {
		mask |= deserHookResolveWasmFromID
	}
	return mask
}

// --- DelegateValueSerializer ---------------------------------------------------

// DelegateValueSerializer is a ValueSerializer whose delegate can implement
// the full pinned crate hook surface (see the hook interfaces above). It is
// created by NewDelegateValueSerializer; plain wire production (WriteHeader,
// WriteValue, TransferArrayBuffer, Release) and direct helper writes
// (WriteUint32 / ...) mirror the crate's ValueSerializer and its helper
// trait. Close is explicit and required; there are no finalizers.
type DelegateValueSerializer struct {
	iso        *Isolate
	sc         *Scope
	ctx        *Context
	handle     uintptr
	delegateID int64
	closed     bool
	released   bool
}

// NewDelegateValueSerializer creates a delegate-completed serializer bound
// to the scope's isolate and the given context. d must implement
// ValueSerializerDelegate (throw_data_clone_error is the one required hook,
// as in the crate); the optional hook interfaces above are detected from d.
func NewDelegateValueSerializer(s *Scope, c *Context, d ValueSerializerDelegate) (*DelegateValueSerializer, error) {
	if d == nil {
		return nil, errors.New("gov8: nil value serializer delegate")
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
	if err := installSerDelEntry(); err != nil {
		return nil, err
	}
	entry := &serDelEntry{iso: s.iso, sc: s, ctx: c, hook: d}
	id, err := serDelRegister(entry)
	if err != nil {
		return nil, err
	}
	w := &DelegateValueSerializer{iso: s.iso, sc: s, ctx: c, delegateID: id}
	entry.owner = w // construction-time hooks (HasCustomHostObject) see it
	var out uintptr
	r1, _, _ := proc("gov8_value_serializer_new_del").Call(
		ih, c.handle, s.handle, uintptr(id), uintptr(serHookMask(d)),
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		serDelUnregister(id)
		return nil, shimError("NewDelegateValueSerializer", r1)
	}
	w.handle = out
	return w, nil
}

func (vs *DelegateValueSerializer) check() error {
	if err := vs.iso.check(); err != nil {
		return err
	}
	if vs.closed {
		return errors.New("gov8: delegate value serializer used after Close")
	}
	if vs.released {
		return errors.New("gov8: delegate value serializer used after Release")
	}
	return nil
}

// Scope returns the scope the serializer was created with; hooks build
// returned values through it (the engine scope is open during the hook).
func (vs *DelegateValueSerializer) Scope() *Scope { return vs.sc }

// Context returns the context the serializer writes against.
func (vs *DelegateValueSerializer) Context() *Context { return vs.ctx }

// WriteHeader writes the wire-format version header (version 16 bytes
// "ff 10" on this build).
func (vs *DelegateValueSerializer) WriteHeader() error {
	if err := vs.check(); err != nil {
		return err
	}
	return callErr("DelegateValueSerializer.WriteHeader",
		proc("gov8_value_serializer_write_header_del"), vs.handle)
}

// WriteValue serializes value into the wire buffer. ok is false when the
// value could not be serialized; when the write threw (delegated hook
// failure, the default host-object error, the engine's SAB rejection, ...)
// the returned error satisfies IsException and the details are in tc (nil
// uses a shim-internal fallback, in which case the exception is not
// observable).
func (vs *DelegateValueSerializer) WriteValue(c *Context, v Value, tc *TryCatch) (ok bool, err error) {
	if err := vs.check(); err != nil {
		return false, err
	}
	if err := v.check(); err != nil {
		return false, err
	}
	if c == nil || c.iso != vs.iso || v.iso != vs.iso {
		return false, foreignIsolate("context or value")
	}
	if err := c.check(); err != nil {
		return false, err
	}
	if tc != nil {
		if tc.iso != vs.iso {
			return false, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return false, err
		}
	}
	sh, err := vs.sc.checkedHandle()
	if err != nil {
		return false, err
	}
	var tcv uintptr
	if tc != nil {
		tcv = tc.handle
	}
	var outOK int32
	r1, _, _ := proc("gov8_value_serializer_write_value_del").Call(
		vs.handle, c.handle, tcv, sh, v.h, uintptr(unsafe.Pointer(&outOK)))
	if int64(r1) < 0 {
		return false, shimError("DelegateValueSerializer.WriteValue", r1)
	}
	return outOK == 1, nil
}

// TransferArrayBuffer marks ab as transferred out of band under the given
// id (writer-side maps are keyed by buffer: re-registering the same buffer
// replaces its id, last registration wins).
func (vs *DelegateValueSerializer) TransferArrayBuffer(id uint32, ab *ArrayBuffer) error {
	if err := vs.check(); err != nil {
		return err
	}
	if ab == nil {
		return errors.New("gov8: nil ArrayBuffer")
	}
	if err := ab.check(); err != nil {
		return err
	}
	if ab.iso != vs.iso {
		return foreignIsolate("ArrayBuffer")
	}
	sh, err := vs.sc.checkedHandle()
	if err != nil {
		return err
	}
	return callErr("DelegateValueSerializer.TransferArrayBuffer",
		proc("gov8_value_serializer_transfer_array_buffer_del"),
		vs.handle, sh, uintptr(id), ab.h)
}

// SetTreatArrayBufferViewsAsHostObjects routes ArrayBufferViews (typed
// arrays, DataViews) to WriteHostObject instead of the native view codec
// (v8::ValueSerializer::SetTreatArrayBufferViewsAsHostObjects).
func (vs *DelegateValueSerializer) SetTreatArrayBufferViewsAsHostObjects(mode bool) error {
	if err := vs.check(); err != nil {
		return err
	}
	m := int32(0)
	if mode {
		m = 1
	}
	return callErr("DelegateValueSerializer.SetTreatArrayBufferViewsAsHostObjects",
		proc("gov8_value_serializer_treat_views_as_host_objects_del"),
		vs.handle, uintptr(m))
}

// WriteUint32 appends value in base-128 varint form (the helper write used
// inside WriteHostObject; usable outside hooks too).
func (vs *DelegateValueSerializer) WriteUint32(value uint32) error {
	if err := vs.check(); err != nil {
		return err
	}
	return callErr("DelegateValueSerializer.WriteUint32",
		proc("gov8_value_serializer_write_uint32_del"), vs.handle, uintptr(value))
}

// WriteUint64 appends value in base-128 varint form.
func (vs *DelegateValueSerializer) WriteUint64(value uint64) error {
	if err := vs.check(); err != nil {
		return err
	}
	return callErr("DelegateValueSerializer.WriteUint64",
		proc("gov8_value_serializer_write_uint64_del"), vs.handle, uintptr(value))
}

// WriteDouble appends value as a little-endian 64-bit double.
func (vs *DelegateValueSerializer) WriteDouble(value float64) error {
	if err := vs.check(); err != nil {
		return err
	}
	return callErr("DelegateValueSerializer.WriteDouble",
		proc("gov8_value_serializer_write_double_del"),
		vs.handle, uintptr(unsafe.Pointer(&value)))
}

// WriteRawBytes appends the raw bytes with NO length prefix: framing is
// entirely the writer's job (v8::ValueSerializer::WriteRawBytes).
func (vs *DelegateValueSerializer) WriteRawBytes(data []byte) error {
	if err := vs.check(); err != nil {
		return err
	}
	var p uintptr
	if len(data) > 0 {
		p = uintptr(unsafe.Pointer(&data[0]))
	}
	return callErr("DelegateValueSerializer.WriteRawBytes",
		proc("gov8_value_serializer_write_raw_bytes_del"),
		vs.handle, p, uintptr(len(data)))
}

// Release returns the accumulated wire bytes and makes the serializer
// unusable. A second Release returns empty bytes and no error, exactly
// like the crate's release() (fixture-pinned). Close after Release is
// still required and still valid (the destructor path frees nothing then).
func (vs *DelegateValueSerializer) Release() ([]byte, error) {
	if err := vs.iso.check(); err != nil {
		return nil, err
	}
	if vs.closed {
		return nil, errors.New("gov8: delegate value serializer used after Close")
	}
	if vs.released {
		// The crate's release() is idempotent-empty after the first call.
		return []byte{}, nil
	}
	// First call may return kErrNoMemory with the required size parked in
	// the shim; retry once with a buffer of exactly that size.
	size := 256
	for attempt := 0; ; attempt++ {
		var outLen int64
		buf := make([]byte, max(size, 1))
		r1, _, _ := proc("gov8_value_serializer_release_del").Call(
			vs.handle, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
			uintptr(unsafe.Pointer(&outLen)))
		if int64(r1) == errNoMemory && attempt == 0 && outLen >= 0 {
			size = int(outLen)
			continue
		}
		if int64(r1) < 0 {
			return nil, shimError("DelegateValueSerializer.Release", r1)
		}
		if outLen < 0 || int(outLen) > len(buf) {
			return nil, errors.New("gov8: delegate value serializer release overflow")
		}
		vs.released = true
		return buf[:outLen], nil
	}
}

// NewError builds a JS Error object in the hook's scope (for hooks that
// throw their own failure).
func (vs *DelegateValueSerializer) NewError(message string) (Value, error) {
	if err := vs.check(); err != nil {
		return Value{}, err
	}
	sh, err := vs.sc.checkedHandle()
	if err != nil {
		return Value{}, err
	}
	msg, err := vs.sc.newStringAssumingCheck(message)
	if err != nil {
		return Value{}, err
	}
	h, err := callHandle("DelegateValueSerializer.NewError",
		proc("gov8_exception_error"), vs.iso.handleAssumingCheck(), sh, msg.h)
	if err != nil {
		return Value{}, err
	}
	return Value{iso: vs.iso, sc: vs.sc, h: h}, nil
}

// NewRangeError builds a JS RangeError object in the hook's scope
// (v8::Exception::RangeError).
func (vs *DelegateValueSerializer) NewRangeError(message string) (Value, error) {
	if err := vs.check(); err != nil {
		return Value{}, err
	}
	sh, err := vs.sc.checkedHandle()
	if err != nil {
		return Value{}, err
	}
	msg, err := vs.sc.newStringAssumingCheck(message)
	if err != nil {
		return Value{}, err
	}
	h, err := callHandle("DelegateValueSerializer.NewRangeError",
		proc("gov8_exception_range_error"), vs.iso.handleAssumingCheck(), sh, msg.h)
	if err != nil {
		return Value{}, err
	}
	return Value{iso: vs.iso, sc: vs.sc, h: h}, nil
}

// ThrowException schedules v to propagate out of the failing write (the
// delegate-drives-the-exception completion path). The write then fails
// with the exception pending; an enclosing TryCatch observes it verbatim.
func (vs *DelegateValueSerializer) ThrowException(v Value) error {
	if err := vs.check(); err != nil {
		return err
	}
	if err := v.check(); err != nil {
		return err
	}
	if v.iso != vs.iso {
		return foreignIsolate("exception")
	}
	return callErr("DelegateValueSerializer.ThrowException",
		proc("gov8_isolate_throw_exception"),
		vs.iso.handleAssumingCheck(), v.h)
}

// Close destroys the engine serializer and unregisters the Go delegate.
// The delegate can no longer be invoked afterwards. Close must be called
// on the owning thread before the scope closes. An un-released wire buffer
// is freed by the engine destructor through the shim delegate (the crate's
// drop path).
func (vs *DelegateValueSerializer) Close() error {
	if err := vs.iso.check(); err != nil {
		return err
	}
	if vs.closed {
		return errors.New("gov8: delegate value serializer already closed")
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	if err := callErr("DelegateValueSerializer.Close",
		proc("gov8_value_serializer_dispose_del"), vs.handle); err != nil {
		return err
	}
	vs.closed = true
	vs.handle = 0
	serDelUnregister(vs.delegateID)
	return nil
}

// --- DelegateValueDeserializer --------------------------------------------------

// DelegateValueDeserializer is a ValueDeserializer whose delegate can
// implement the pinned crate's deserializer hook surface. Created by
// NewDelegateValueDeserializer; a nil delegate reproduces the trait
// defaults (every read path throws the deterministic "not implemented"
// error).
type DelegateValueDeserializer struct {
	iso         *Isolate
	sc          *Scope
	ctx         *Context
	handle      uintptr
	data        []byte // keeps the engine's input pointer valid until Close
	delegateID  int64
	readStarted bool
	closed      bool
}

// NewDelegateValueDeserializer creates a delegate-completed deserializer
// over data (no copy) bound to the context's isolate. d may be nil (trait
// defaults). data must not be mutated while the deserializer is open.
func NewDelegateValueDeserializer(s *Scope, c *Context, data []byte, d ValueDeserializerDelegate) (*DelegateValueDeserializer, error) {
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
	if err := installSerDelEntry(); err != nil {
		return nil, err
	}
	entry := &serDelEntry{iso: s.iso, sc: s, ctx: c}
	if d != nil {
		entry.hook = d
	}
	id, err := serDelRegister(entry)
	if err != nil {
		return nil, err
	}
	w := &DelegateValueDeserializer{iso: s.iso, sc: s, ctx: c, delegateID: id, data: data}
	entry.owner = w
	var p uintptr
	if len(data) > 0 {
		p = uintptr(unsafe.Pointer(&data[0]))
	}
	h, err := callHandle("NewDelegateValueDeserializer",
		proc("gov8_value_deserializer_new_del"),
		ih, c.handle, s.handle, uintptr(id), uintptr(deserHookMask(d)), p,
		uintptr(len(data)))
	if err != nil {
		serDelUnregister(id)
		return nil, err
	}
	w.handle = h
	return w, nil
}

func (vd *DelegateValueDeserializer) check() error {
	if err := vd.iso.check(); err != nil {
		return err
	}
	if vd.closed {
		return errors.New("gov8: delegate value deserializer used after Close")
	}
	return nil
}

// Scope returns the scope the deserializer was created with; hooks build
// returned values through it.
func (vd *DelegateValueDeserializer) Scope() *Scope { return vd.sc }

// Context returns the context the deserializer reads against.
func (vd *DelegateValueDeserializer) Context() *Context { return vd.ctx }

// ReadValue deserializes the next value. A returned error satisfying
// IsException means the engine threw (invalid wire data, an unregistered
// or unanswered transfer id, a rejected host object, ...); the details are
// in tc (nil uses a shim-internal fallback), exactly like the crate's
// read_value returning None.
func (vd *DelegateValueDeserializer) ReadValue(c *Context, tc *TryCatch) (Value, error) {
	if err := vd.check(); err != nil {
		return Value{}, err
	}
	if c == nil || c.iso != vd.iso {
		return Value{}, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return Value{}, err
	}
	if tc != nil {
		if tc.iso != vd.iso {
			return Value{}, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return Value{}, err
		}
	}
	sh, err := vd.sc.checkedHandle()
	if err != nil {
		return Value{}, err
	}
	vd.readStarted = true
	var tcv uintptr
	if tc != nil {
		tcv = tc.handle
	}
	var out uintptr
	r1, _, _ := proc("gov8_value_deserializer_read_value_del").Call(
		vd.handle, c.handle, tcv, sh, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("DelegateValueDeserializer.ReadValue", r1)
	}
	if out == 0 {
		return Value{}, errors.New("gov8: value deserializer produced no value")
	}
	return Value{iso: vd.iso, sc: vd.sc, h: out}, nil
}

// ReadHeader reads and validates the wire-format header. ok is false when
// the header was absent (not an error); an invalid header throws
// (IsException, details in tc — nil uses a shim-internal fallback). Reads
// after ReadHeader report the header's wire format version through
// GetWireFormatVersion.
func (vd *DelegateValueDeserializer) ReadHeader(c *Context, tc *TryCatch) (ok bool, err error) {
	if err := vd.check(); err != nil {
		return false, err
	}
	if c == nil || c.iso != vd.iso {
		return false, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return false, err
	}
	if tc != nil {
		if tc.iso != vd.iso {
			return false, foreignIsolate("trycatch")
		}
		if err := tc.check(); err != nil {
			return false, err
		}
	}
	sh, err := vd.sc.checkedHandle()
	if err != nil {
		return false, err
	}
	vd.readStarted = true
	var tcv uintptr
	if tc != nil {
		tcv = tc.handle
	}
	var outOK int32
	r1, _, _ := proc("gov8_value_deserializer_read_header_del").Call(
		vd.handle, c.handle, tcv, sh, uintptr(unsafe.Pointer(&outOK)))
	if int64(r1) < 0 {
		return false, shimError("DelegateValueDeserializer.ReadHeader", r1)
	}
	return outOK == 1, nil
}

// TransferArrayBuffer registers the receiving buffer for transfer id
// (reader-side maps are keyed by id: re-registering an id replaces its
// target, last registration wins).
func (vd *DelegateValueDeserializer) TransferArrayBuffer(id uint32, ab *ArrayBuffer) error {
	if err := vd.check(); err != nil {
		return err
	}
	if ab == nil {
		return errors.New("gov8: nil ArrayBuffer")
	}
	if err := ab.check(); err != nil {
		return err
	}
	if ab.iso != vd.iso {
		return foreignIsolate("ArrayBuffer")
	}
	sh, err := vd.sc.checkedHandle()
	if err != nil {
		return err
	}
	return callErr("DelegateValueDeserializer.TransferArrayBuffer",
		proc("gov8_value_deserializer_transfer_array_buffer_del"),
		vd.handle, sh, uintptr(id), ab.h)
}

// TransferSharedArrayBuffer registers the receiving SAB for id
// (v8::ValueDeserializer::TransferSharedArrayBuffer). Pinned semantics
// note: SAB reads on this build never consult these registrations — the
// GetSharedArrayBufferFromID hook is always the source.
func (vd *DelegateValueDeserializer) TransferSharedArrayBuffer(id uint32, sab *SharedArrayBuffer) error {
	if err := vd.check(); err != nil {
		return err
	}
	if sab == nil {
		return errors.New("gov8: nil SharedArrayBuffer")
	}
	if err := sab.check(); err != nil {
		return err
	}
	if sab.iso != vd.iso {
		return foreignIsolate("SharedArrayBuffer")
	}
	sh, err := vd.sc.checkedHandle()
	if err != nil {
		return err
	}
	return callErr("DelegateValueDeserializer.TransferSharedArrayBuffer",
		proc("gov8_value_deserializer_transfer_shared_array_buffer_del"),
		vd.handle, sh, uintptr(id), sab.h)
}

// GetWireFormatVersion reports the wire format version of the data (0 for
// header-less/legacy data; must be called after ReadHeader for versioned
// data to be meaningful).
func (vd *DelegateValueDeserializer) GetWireFormatVersion() (uint32, error) {
	if err := vd.check(); err != nil {
		return 0, err
	}
	var out uint32
	if err := callErr("DelegateValueDeserializer.GetWireFormatVersion",
		proc("gov8_value_deserializer_wire_format_version_del"),
		vd.handle, uintptr(unsafe.Pointer(&out))); err != nil {
		return 0, err
	}
	return out, nil
}

// ReadUint32 reads one varint u32 (for use inside ReadHostObject). ok is
// false when the wire is exhausted.
func (vd *DelegateValueDeserializer) ReadUint32() (val uint32, ok bool, err error) {
	if err := vd.check(); err != nil {
		return 0, false, err
	}
	var out uint32
	var outOK int32
	if err := callErr("DelegateValueDeserializer.ReadUint32",
		proc("gov8_value_deserializer_read_uint32_del"),
		vd.handle, uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&outOK))); err != nil {
		return 0, false, err
	}
	return out, outOK == 1, nil
}

// ReadUint64 reads one varint u64 (for use inside ReadHostObject).
func (vd *DelegateValueDeserializer) ReadUint64() (val uint64, ok bool, err error) {
	if err := vd.check(); err != nil {
		return 0, false, err
	}
	var out uint64
	var outOK int32
	if err := callErr("DelegateValueDeserializer.ReadUint64",
		proc("gov8_value_deserializer_read_uint64_del"),
		vd.handle, uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&outOK))); err != nil {
		return 0, false, err
	}
	return out, outOK == 1, nil
}

// ReadDouble reads one little-endian f64 (for use inside ReadHostObject).
func (vd *DelegateValueDeserializer) ReadDouble() (val float64, ok bool, err error) {
	if err := vd.check(); err != nil {
		return 0, false, err
	}
	var out float64
	var outOK int32
	if err := callErr("DelegateValueDeserializer.ReadDouble",
		proc("gov8_value_deserializer_read_double_del"),
		vd.handle, uintptr(unsafe.Pointer(&out)), uintptr(unsafe.Pointer(&outOK))); err != nil {
		return 0, false, err
	}
	return out, outOK == 1, nil
}

// ReadRawBytes copies the next length bytes out of the wire (for use
// inside ReadHostObject). The bytes are copied at the boundary: Go owns
// the returned slice. ok is false when the wire is exhausted (V8 only
// advances its position on success).
func (vd *DelegateValueDeserializer) ReadRawBytes(length int) (data []byte, ok bool, err error) {
	if err := vd.check(); err != nil {
		return nil, false, err
	}
	if length < 0 {
		return nil, false, errors.New("gov8: negative raw byte length")
	}
	buf := make([]byte, length)
	var outLen int64
	var outOK int32
	var p uintptr
	if length > 0 {
		p = uintptr(unsafe.Pointer(&buf[0]))
	}
	if err := callErr("DelegateValueDeserializer.ReadRawBytes",
		proc("gov8_value_deserializer_read_raw_bytes_del"),
		vd.handle, uintptr(length), p, uintptr(len(buf)),
		uintptr(unsafe.Pointer(&outLen)), uintptr(unsafe.Pointer(&outOK))); err != nil {
		return nil, false, err
	}
	if outOK != 1 {
		return nil, false, nil
	}
	if outLen < 0 || int(outLen) > length {
		return nil, false, errors.New("gov8: raw byte read overflow")
	}
	return buf[:outLen], true, nil
}

// NewError builds a JS Error object in the hook's scope (for hooks that
// throw their own failure instead of answering None).
func (vd *DelegateValueDeserializer) NewError(message string) (Value, error) {
	if err := vd.check(); err != nil {
		return Value{}, err
	}
	sh, err := vd.sc.checkedHandle()
	if err != nil {
		return Value{}, err
	}
	msg, err := vd.sc.newStringAssumingCheck(message)
	if err != nil {
		return Value{}, err
	}
	h, err := callHandle("DelegateValueDeserializer.NewError",
		proc("gov8_exception_error"), vd.iso.handleAssumingCheck(), sh, msg.h)
	if err != nil {
		return Value{}, err
	}
	return Value{iso: vd.iso, sc: vd.sc, h: h}, nil
}

// ThrowException schedules v to propagate out of the failing read.
func (vd *DelegateValueDeserializer) ThrowException(v Value) error {
	if err := vd.check(); err != nil {
		return err
	}
	if err := v.check(); err != nil {
		return err
	}
	if v.iso != vd.iso {
		return foreignIsolate("exception")
	}
	return callErr("DelegateValueDeserializer.ThrowException",
		proc("gov8_isolate_throw_exception"),
		vd.iso.handleAssumingCheck(), v.h)
}

// Close destroys the engine deserializer (which releases its reference to
// the input bytes), then the shim delegate, then unregisters the Go
// delegate. Only after this call may the caller reuse or free the input
// slice.
func (vd *DelegateValueDeserializer) Close() error {
	if err := vd.iso.check(); err != nil {
		return err
	}
	if vd.closed {
		return errors.New("gov8: delegate value deserializer already closed")
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	if err := callErr("DelegateValueDeserializer.Close",
		proc("gov8_value_deserializer_dispose_del"), vd.handle); err != nil {
		return err
	}
	vd.closed = true
	vd.handle = 0
	vd.data = nil
	serDelUnregister(vd.delegateID)
	return nil
}
