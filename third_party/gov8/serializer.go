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

// Value serialization / deserialization (structured clone wire format).
//
// Rust surface mapped here (pinned crate v8 =152.2.0):
//
//	v8::ValueSerializer::new(scope, Box<D>)        -> NewValueSerializer(s, d)
//	    write_header                               -> (*ValueSerializer).WriteHeader
//	    write_value(context, value) -> Option<bool>-> (*ValueSerializer).WriteValue
//	    transfer_array_buffer(id, ab)              -> (*ValueSerializer).TransferArrayBuffer
//	    release() -> Vec<u8>                       -> (*ValueSerializer).Release
//	v8::ValueDeserializer::new(scope, D, data)     -> NewValueDeserializer(s, c, data)
//	    read_value(context) -> Option<Local>       -> (*ValueDeserializer).ReadValue
//	    read_header / transfer_array_buffer        -> the matching methods
//
// Delegate model. Rust passes a boxed ValueSerializerImpl trait object; Go
// cannot expose function pointers to the engine, so delegates live in an
// integer registry and the engine only ever sees an int64 id (the same
// pattern as the native-callback registries). One process-wide trampoline
// per callback shape (syscall.NewCallback, pinned by the Go runtime) is
// registered with the shim once.
//
//   - ValueSerializerDelegate.ThrowDataCloneError receives the engine's
//     message TEXT (already lossy-decoded, like to_rust_string_lossy) and
//     returns whether the shim should re-throw it as a JS Error — the
//     behavior of the oracle's DataCloneErrorReporter, which round-trips the
//     message through a fresh String and throws Exception::error. The hook
//     intentionally receives no live handles: nothing crosses the boundary
//     except bytes and an int.
//   - The remaining serializer hooks and all deserializer hooks are NOT
//     delegated to Go: the shim reproduces the pinned crate's DEFAULT
//     behaviors verbatim (Nothing from the SAB-id / wasm-transfer-id paths;
//     the deterministic "Deno serializer/deserializer: ... not implemented"
//     Error throws from the host-object paths). Custom host objects need
//     cross-boundary object marshaling and remain future scope; nothing here
//     silently succeeds where the crate fails.
//
// Wire-bytes ownership. Release copies the wire bytes into a Go slice and
// frees the engine buffer inside the shim; Go never holds a pointer into the
// serializer. The serializer is unusable after Release (mirroring the crate).
//
// Deserializer input lifetime. The engine stores the caller's data pointer
// WITHOUT copying — the pinned fixture's behavior depends on it (see the
// oracle's deser_describe! lifetime note). The Go wrapper therefore keeps
// the input slice referenced from NewValueDeserializer until Close; Close
// destroys the engine object first and only then drops the reference. Do not
// mutate the slice while the deserializer is open, and do not rely on it
// after Close.
type ValueSerializerDelegate interface {
	// ThrowDataCloneError is invoked when the engine reports a data-clone
	// error (for example serializing a function). Return true to re-throw
	// the message as a JS Error so an enclosing TryCatch observes the
	// failure (the structured-clone behavior); false leaves the engine
	// untouched and WriteValue simply reports failure.
	ThrowDataCloneError(message string) (rethrow bool)
}

// ValueSerializer produces structured-clone wire bytes for JS values.
type ValueSerializer struct {
	iso        *Isolate
	sc         *Scope
	handle     uintptr
	delegateID int64
	closed     bool
	released   bool
}

type serDelegateEntry struct {
	iso      *Isolate
	delegate ValueSerializerDelegate
}

var serDelegateRegistry = struct {
	mu      sync.Mutex
	next    int64
	entries map[int64]serDelegateEntry
}{entries: make(map[int64]serDelegateEntry)}

var (
	bufDelegateOnce sync.Once
	bufDelegateErr  error
)

var goBufDelegateDispatch = syscall.NewCallback(func(id, op, msgPtr, msgLen uintptr) uintptr {
	switch int64(op) {
	case 1: // ThrowDataCloneError
		serDelegateRegistry.mu.Lock()
		entry, ok := serDelegateRegistry.entries[int64(id)]
		serDelegateRegistry.mu.Unlock()
		if !ok || entry.delegate == nil {
			return 0
		}
		message := copyCLenString(msgPtr, msgLen)
		return boolWord(entry.delegate.ThrowDataCloneError(message))
	default:
		// Unknown op: report "did not handle" and keep the boundary inert.
		fmt.Fprintf(os.Stderr, "gov8: unknown buffer delegate op %d\n", int64(op))
		return 0
	}
})

func installBufDelegateEntry() error {
	bufDelegateOnce.Do(func() {
		bufDelegateErr = callErr("BufDelegateEntry",
			proc("gov8_buf_delegate_set_entry"), goBufDelegateDispatch)
	})
	return bufDelegateErr
}

// copyCLenString copies a (ptr,len) byte range handed to a trampoline into a
// Go string before returning; the source only lives for the call. The
// indirect round-trip mirrors abiWordToPtr and keeps vet clean.
func copyCLenString(ptr, length uintptr) string {
	if ptr == 0 || length == 0 {
		return ""
	}
	b := unsafe.Slice((*byte)(abiWordToPtr(ptr)), int(length))
	return string(b)
}

func boolWord(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

// NewValueSerializer creates a serializer bound to the scope's isolate and
// the given context (the crate captures the scope's current context; gov8
// passes the context explicitly). d must be non-nil; a data-clone error
// without a delegate would otherwise leave the engine without a reporting
// path (the Rust API requires a boxed impl too).
func NewValueSerializer(s *Scope, c *Context, d ValueSerializerDelegate) (*ValueSerializer, error) {
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
	if err := installBufDelegateEntry(); err != nil {
		return nil, err
	}
	serDelegateRegistry.mu.Lock()
	serDelegateRegistry.next++
	id := serDelegateRegistry.next
	serDelegateRegistry.entries[id] = serDelegateEntry{iso: s.iso, delegate: d}
	serDelegateRegistry.mu.Unlock()
	var out uintptr
	r1, _, _ := proc("gov8_value_serializer_new").Call(ih, c.handle, s.handle, uintptr(id), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		serDelegateRegistry.mu.Lock()
		delete(serDelegateRegistry.entries, id)
		serDelegateRegistry.mu.Unlock()
		return nil, shimError("NewValueSerializer", r1)
	}
	return &ValueSerializer{iso: s.iso, sc: s, handle: out, delegateID: id}, nil
}

func (vs *ValueSerializer) check() error {
	if err := vs.iso.check(); err != nil {
		return err
	}
	if vs.closed {
		return errors.New("gov8: value serializer used after Close")
	}
	if vs.released {
		return errors.New("gov8: value serializer used after Release")
	}
	return nil
}

// WriteHeader writes the wire-format version header (v8's
// WriteHeader; this build emits version 16 bytes "ff 10"). write_value alone
// emits NO header bytes — an explicit WriteHeader is the canonical embedder
// flow and changes the bytes observable in the wire.
func (vs *ValueSerializer) WriteHeader() error {
	if err := vs.check(); err != nil {
		return err
	}
	return callErr("ValueSerializer.WriteHeader",
		proc("gov8_value_serializer_write_header"), vs.handle)
}

// WriteValue serializes value into the wire buffer. ok is false when the
// value could not be serialized; when a delegate threw (the normal
// data-clone path) the returned error satisfies IsException and the details
// are in tc (the caller's TryCatch; nil uses a shim-internal fallback, in
// which case the exception is not observable).
func (vs *ValueSerializer) WriteValue(c *Context, v Value, tc *TryCatch) (ok bool, err error) {
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
	r1, _, _ := proc("gov8_value_serializer_write_value").Call(
		vs.handle, c.handle, tcv, sh, v.h, uintptr(unsafe.Pointer(&outOK)))
	if int64(r1) < 0 {
		return false, shimError("ValueSerializer.WriteValue", r1)
	}
	return outOK == 1, nil
}

// TransferArrayBuffer marks ab as transferred out of band under the given id
// (v8::ValueSerializer::transfer_array_buffer). The receiving side must
// register the same id on its deserializer or deserialization fails
// deterministically. This build does NOT detach the source at write time.
func (vs *ValueSerializer) TransferArrayBuffer(id uint32, ab *ArrayBuffer) error {
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
	return callErr("ValueSerializer.TransferArrayBuffer",
		proc("gov8_value_serializer_transfer_array_buffer"),
		vs.handle, sh, uintptr(id), ab.h)
}

// Release returns the accumulated wire bytes and makes the serializer
// unusable (the engine hands buffer ownership to the shim, which copies into
// Go memory and frees the original). The contents are whatever was written
// before a failed write — the crate behaves the same way.
func (vs *ValueSerializer) Release() ([]byte, error) {
	if err := vs.iso.check(); err != nil {
		return nil, err
	}
	if vs.closed {
		return nil, errors.New("gov8: value serializer used after Close")
	}
	if vs.released {
		return nil, errors.New("gov8: value serializer already released")
	}
	// First call may return kErrNoMemory with the required size parked in
	// the shim; retry once with a buffer of exactly that size. Start small:
	// most wires are far below 4 KiB and the retry costs one extra call.
	size := 256
	for attempt := 0; ; attempt++ {
		var outLen int64
		buf := make([]byte, max(size, 1))
		r1, _, _ := proc("gov8_value_serializer_release").Call(
			vs.handle, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
			uintptr(unsafe.Pointer(&outLen)))
		if int64(r1) == errNoMemory && attempt == 0 && outLen >= 0 {
			size = int(outLen)
			continue
		}
		if int64(r1) < 0 {
			return nil, shimError("ValueSerializer.Release", r1)
		}
		if outLen < 0 || int(outLen) > len(buf) {
			return nil, errors.New("gov8: value serializer release overflow")
		}
		vs.released = true
		return buf[:outLen], nil
	}
}

// Close destroys the engine serializer and unregisters the Go delegate. The
// delegate can no longer be invoked afterwards. Close must be called on the
// owning thread before the scope closes.
func (vs *ValueSerializer) Close() error {
	if err := vs.iso.check(); err != nil {
		return err
	}
	if vs.closed {
		return errors.New("gov8: value serializer already closed")
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	if err := callErr("ValueSerializer.Close", proc("gov8_value_serializer_dispose"), vs.handle); err != nil {
		return err
	}
	vs.closed = true
	vs.handle = 0
	serDelegateRegistry.mu.Lock()
	delete(serDelegateRegistry.entries, vs.delegateID)
	serDelegateRegistry.mu.Unlock()
	return nil
}

// AsObject converts a generic value into an object view of it. Deserialized
// plain objects are inspected through it (property access), mirroring the
// oracle's try_cast::<v8::Object>.
func AsObject(v Value) (*Object, error) {
	is, err := v.IsObject()
	if err != nil {
		return nil, err
	}
	if !is {
		return nil, errors.New("gov8: value is not an Object")
	}
	return &Object{Value: v}, nil
}

// ValueDeserializer reads JS values from structured-clone wire bytes. The
// input slice is retained (uncopied) until Close: see the lifetime notes at
// the top of this file.
type ValueDeserializer struct {
	iso    *Isolate
	sc     *Scope
	handle uintptr
	data   []byte // keeps the engine's input pointer valid until Close
	// readStarted makes SetSupportsLegacyWireFormat a deterministic Go error
	// once ReadHeader or ReadValue has entered V8. The native precondition is
	// stricter than an ordinary recoverable API failure.
	readStarted bool
	closed      bool
}

// NewValueDeserializer creates a deserializer over data (no copy) bound to
// the context's isolate. The delegate behavior is the pinned crate's
// default: reads of host objects, transferred shared array buffers and wasm
// modules throw the deterministic "Deno deserializer: ... not implemented"
// errors. data must not be mutated while the deserializer is open.
func NewValueDeserializer(s *Scope, c *Context, data []byte) (*ValueDeserializer, error) {
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
	var p uintptr
	if len(data) > 0 {
		p = uintptr(unsafe.Pointer(&data[0]))
	}
	h, err := callHandle("NewValueDeserializer", proc("gov8_value_deserializer_new"),
		ih, c.handle, s.handle, p, uintptr(len(data)))
	if err != nil {
		return nil, err
	}
	return &ValueDeserializer{iso: s.iso, sc: s, handle: h, data: data}, nil
}

func (vd *ValueDeserializer) check() error {
	if err := vd.iso.check(); err != nil {
		return err
	}
	if vd.closed {
		return errors.New("gov8: value deserializer used after Close")
	}
	return nil
}

// ReadValue deserializes the next value. A returned error satisfying
// IsException means the engine threw (invalid wire data, an unregistered
// transfer id, or a rejected host object); the details are in tc (nil uses a
// shim-internal fallback), exactly like the crate's read_value returning
// None.
func (vd *ValueDeserializer) ReadValue(c *Context, tc *TryCatch) (Value, error) {
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
	r1, _, _ := proc("gov8_value_deserializer_read_value").Call(
		vd.handle, c.handle, tcv, sh, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return Value{}, shimError("ValueDeserializer.ReadValue", r1)
	}
	if out == 0 {
		return Value{}, errors.New("gov8: value deserializer produced no value")
	}
	return Value{iso: vd.iso, sc: vd.sc, h: out}, nil
}

// ReadHeader reads and validates the wire-format header. Call it before
// ReadValue. Header-less data is classified as legacy version 0 and, unless
// legacy support is explicitly enabled with SetSupportsLegacyWireFormat, is
// rejected by this pinned engine. Missing, truncated, and unsupported headers
// therefore return an error satisfying IsException; current versioned data
// returns true.
func (vd *ValueDeserializer) ReadHeader(c *Context) (ok bool, err error) {
	if err := vd.check(); err != nil {
		return false, err
	}
	if c == nil || c.iso != vd.iso {
		return false, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return false, err
	}
	sh, err := vd.sc.checkedHandle()
	if err != nil {
		return false, err
	}
	vd.readStarted = true
	var outOK int32
	r1, _, _ := proc("gov8_value_deserializer_read_header").Call(
		vd.handle, c.handle, 0, sh, uintptr(unsafe.Pointer(&outOK)))
	if int64(r1) < 0 {
		return false, shimError("ValueDeserializer.ReadHeader", r1)
	}
	return outOK == 1, nil
}

// TransferArrayBuffer registers the receiving buffer for transfer id
// (v8::ValueDeserializer::transfer_array_buffer). The registered buffer's
// own store is reused as the transferred contents.
func (vd *ValueDeserializer) TransferArrayBuffer(id uint32, ab *ArrayBuffer) error {
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
	return callErr("ValueDeserializer.TransferArrayBuffer",
		proc("gov8_value_deserializer_transfer_array_buffer"),
		vd.handle, sh, uintptr(id), ab.h)
}

// Close destroys the engine deserializer, which releases its reference to
// the input bytes; only after this call may the caller reuse or free the
// slice. The wrapper's own reference is dropped here too.
func (vd *ValueDeserializer) Close() error {
	if err := vd.iso.check(); err != nil {
		return err
	}
	if vd.closed {
		return errors.New("gov8: value deserializer already closed")
	}
	if err := requireInitialized(); err != nil {
		return err
	}
	if err := callErr("ValueDeserializer.Close", proc("gov8_value_deserializer_dispose"), vd.handle); err != nil {
		return err
	}
	vd.closed = true
	vd.handle = 0
	vd.data = nil
	return nil
}
