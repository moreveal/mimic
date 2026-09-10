//go:build windows && amd64

package gov8

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// Array buffers, backing stores, typed arrays and DataViews.
//
// Rust surface mapped here (pinned crate v8 =152.2.0):
//
//	v8::ArrayBuffer::new / with_backing_store -> NewArrayBuffer /
//	NewArrayBufferWithBackingStore
//	v8::ArrayBuffer::{byte_length, is_detachable, was_detached, data,
//	get_backing_store, detach, set_detach_key} -> the matching methods
//	v8::ArrayBuffer::new_backing_store{,_from_vec,_from_boxed_slice,_from_ptr}
//	-> NewBackingStore / NewBackingStoreFromSlice / NewBackingStoreFromPtr
//	v8::SharedArrayBuffer::{new, with_backing_store, byte_length,
//	get_backing_store, new_backing_store,
//	new_backing_store_from_{vec,boxed_slice,ptr}} -> the
//	SharedArrayBuffer methods
//	v8::{Uint8Array,Float64Array,...}::new and ArrayBufferView geometry ->
//	NewUint8Array / NewFloat64Array / NewBigInt64Array and the view methods
//	v8::SharedRef<BackingStore>::{is_shared, byte_length,
//	is_resizable_by_user_javascript, assert_use_count_eq} -> BackingStore
//	methods (assert_use_count_eq becomes UseCount plus an explicit compare:
//	the crate polls and panics; Go returns the count)
//
// Ownership and lifetime rules:
//   - BackingStore is an explicit, counted reference. Close drops exactly one
//     reference; when the last reference dies (here or inside the engine) the
//     memory and any external deleter run synchronously. There are no
//     finalizers: a forgotten Close leaks one reference, never corrupts.
//   - ArrayBuffer / SharedArrayBuffer / views are scope-local values (they
//     embed Value) and follow the ordinary Value lifetime rules.
//   - GetBackingStore hands out a NEW counted reference each call; the caller
//     must Close it. Retaining it (forgetting Close) shows up as a higher
//     UseCount exactly like holding a SharedRef in Rust.
//   - NewBackingStoreFromPtr: the CALLER owns the memory and the deleter runs
//     once, on the owning thread, when the last reference dies. The raw data
//     pointer must stay valid until the deleter fires (runtime.KeepAlive the
//     Go object that backs it until then). Go function pointers cannot be
//     stored in engine memory, so the deleter is dispatched through an
//     integer registry: the engine only ever sees a registry handle, and the
//     Go callback receives exactly the (data, byteLength, deleterData) triple
//     it registered.
//   - The store read/write methods copy; no pointer into engine memory is
//     ever retained by Go, and no Go pointer is retained by the engine.
//
// Intentional deviations (documented, behavior-preserving where observable):
//   - Rust's was_detached() early-returns false whenever byte_length != 0 and
//     only consults the engine bit for zero-length buffers. WasDetached
//     reproduces that wrapper behavior exactly (the fixture pins it).
//   - Rust's detach(Some/None) returns Some(true) for non-detachable buffers
//     without calling the engine; Detach mirrors that and maps the
//     [[ArrayBufferDetachKey]] mismatch (None in Rust) to false.
//   - Typed-array / DataView construction with out-of-bounds or misaligned
//     geometry is a process-fatal V8 CHECK in Rust (characterized out of
//     process by rust-oracle/tests/buffers_negative.rs). A Go binding cannot
//     survive a V8 CHECK, so the shim prevalidates the geometry at the
//     boundary and construction returns an error instead. Every in-bounds
//     observation is unchanged.
//   - The pinned crate exposes no ArrayBuffer externalize method. Allocator
//     selection belongs to CreateParams; its unsafe custom-callback variant is
//     tracked explicitly in the Isolate parity row.

// Isolate.LowMemoryNotification requests a full garbage collection (the
// crate's isolate.low_memory_notification). Used to make engine-side drops of
// ArrayBuffer/backing-store references observable deterministically.
func (i *Isolate) LowMemoryNotification() error {
	ih, err := i.handleChecked()
	if err != nil {
		return err
	}
	if err := callErr("LowMemoryNotification", proc("gov8_isolate_low_memory_notification"), ih); err != nil {
		return err
	}
	return nil
}

// --- value predicates (buffers family) ---------------------------------------

// IsArrayBuffer reports whether the value is an ArrayBuffer.
func (v Value) IsArrayBuffer() (bool, error) { return v.predicate("gov8_is_array_buffer") }

// IsSharedArrayBuffer reports whether the value is a SharedArrayBuffer.
func (v Value) IsSharedArrayBuffer() (bool, error) {
	return v.predicate("gov8_is_shared_array_buffer")
}

// IsArrayBufferView reports whether the value is a view over a buffer (a
// typed array or a DataView).
func (v Value) IsArrayBufferView() (bool, error) {
	return v.predicate("gov8_is_array_buffer_view")
}

// IsTypedArray reports whether the value is a typed array.
func (v Value) IsTypedArray() (bool, error) { return v.predicate("gov8_is_typed_array") }

// IsDataView reports whether the value is a DataView.
func (v Value) IsDataView() (bool, error) { return v.predicate("gov8_is_data_view") }

// Same reports whether two values are the same object (v8::Local operator==,
// object identity rather than handle-slot identity).
func Same(a, b Value) (bool, error) {
	if err := a.check(); err != nil {
		return false, err
	}
	if err := b.check(); err != nil {
		return false, err
	}
	if a.iso != b.iso || a.sc.iso != b.sc.iso {
		return false, foreignIsolate("value")
	}
	ih, err := a.iso.handleChecked()
	if err != nil {
		return false, err
	}
	if _, err := a.sc.checkedHandle(); err != nil {
		return false, err
	}
	r1, _, _ := proc("gov8_value_same").Call(ih, a.h, b.h)
	if int64(r1) < 0 {
		return false, shimError("Same", r1)
	}
	return r1 == 1, nil
}

// --- backing stores -----------------------------------------------------------
//
// BackingStore is one counted reference to engine-owned (or embedder-owned,
// for FromPtr stores) raw memory. Like rusty_v8's SharedRef<BackingStore>, it
// may outlive its creating isolate and its copied queries/read/write/Close
// operations are safe from any thread. Creating an engine object from it
// remains isolate- and thread-affine. These copied native shared-reference
// operations also remain valid after Dispose and DisposePlatform: they do not
// enter V8 or use its platform, and Close must remain available to release the
// final allocator/storage reference after process shutdown.

// BackingStoreDeleter is invoked exactly once when the last reference to a
// FromPtr backing store dies: (data, byteLength, deleterData) -- the same
// triple v8's BackingStoreDeleterCallback observes.
type BackingStoreDeleter func(data unsafe.Pointer, byteLength int, deleterData uintptr)

type BackingStore struct {
	mu                            sync.Mutex
	iso                           *Isolate
	handle                        uintptr
	arrayBufferAllocatorReference uintptr
	closed                        bool
	closing                       bool
	active                        int
}

var (
	backingStoreHotProcsOnce         sync.Once
	backingStoreNewAddr              uintptr
	backingStoreNewWithAllocatorAddr uintptr
	backingStoreDisposeAddr          uintptr
	allocatorCloneAddr               uintptr
	allocatorDisposeAddr             uintptr
)

func ensureBackingStoreHotProcs() {
	backingStoreHotProcsOnce.Do(func() {
		backingStoreNewAddr = proc("gov8_backing_store_new").Addr()
		backingStoreNewWithAllocatorAddr = proc("gov8_aba_backing_store_new").Addr()
		backingStoreDisposeAddr = proc("gov8_backing_store_dispose").Addr()
		allocatorCloneAddr = proc("gov8_aba_clone").Addr()
		allocatorDisposeAddr = proc("gov8_aba_dispose").Addr()
	})
}

func cloneArrayBufferAllocatorReference(reference uintptr) (uintptr, error) {
	if reference == 0 {
		return 0, nil
	}
	ensureBackingStoreHotProcs()
	handle, _, _ := syscall.Syscall(allocatorCloneAddr, 1, reference, 0, 0)
	if handle == 0 {
		return 0, shimError("ArrayBufferAllocator.backing-store.clone", 0)
	}
	return handle, nil
}

func (i *Isolate) backingStore(handle uintptr) (*BackingStore, error) {
	reference, err := cloneArrayBufferAllocatorReference(i.arrayBufferAllocatorReference)
	if err != nil {
		ensureBackingStoreHotProcs()
		_, _, _ = syscall.Syscall(backingStoreDisposeAddr, 1, handle, 0, 0)
		return nil, err
	}
	return &BackingStore{iso: i, handle: handle, arrayBufferAllocatorReference: reference}, nil
}

// deleterRegistry maps integer handles handed to the engine to Go deleter
// callbacks plus the caller's deleterData word. Entries self-remove after the
// single invocation (a store's deleter fires at most once).
var deleterRegistry = struct {
	mu      sync.Mutex
	next    int64
	entries map[int64]BackingStoreDeleterEntry
}{entries: make(map[int64]BackingStoreDeleterEntry)}

type BackingStoreDeleterEntry struct {
	data        unsafe.Pointer
	byteLength  int
	deleterData uintptr
	fn          BackingStoreDeleter
}

var (
	deleterEntryOnce sync.Once
	deleterEntryErr  error
)

// goBsDeleterDispatch is the single trampoline the engine retains; entries
// created by syscall.NewCallback are pinned by the Go runtime for the
// process lifetime.
var goBsDeleterDispatch = syscall.NewCallback(func(handle, data, byteLength uintptr) uintptr {
	deleterRegistry.mu.Lock()
	entry, ok := deleterRegistry.entries[int64(handle)]
	if ok {
		delete(deleterRegistry.entries, int64(handle))
	}
	deleterRegistry.mu.Unlock()
	if ok && entry.fn != nil {
		entry.fn(entry.data, entry.byteLength, entry.deleterData)
	}
	return 1
})

func installDeleterEntry() error {
	deleterEntryOnce.Do(func() {
		deleterEntryErr = callErr("BufDeleterEntry",
			proc("gov8_buf_deleter_set_entry"), goBsDeleterDispatch)
	})
	return deleterEntryErr
}

// NewBackingStore allocates a zero-initialized, isolate-owned backing store
// (the crate's ArrayBuffer::new_backing_store).
func (i *Isolate) NewBackingStore(byteLength int) (*BackingStore, error) {
	if byteLength < 0 {
		return nil, fmt.Errorf("gov8: negative backing store length %d", byteLength)
	}
	if err := i.check(); err != nil {
		return nil, err
	}
	ensureBackingStoreHotProcs()
	var h uintptr
	if i.arrayBufferAllocatorReference != 0 {
		h, _, _ = syscall.Syscall(backingStoreNewWithAllocatorAddr, 3,
			i.handle, uintptr(byteLength), i.arrayBufferAllocatorReference)
	} else {
		h, _, _ = syscall.Syscall(backingStoreNewAddr, 2,
			i.handle, uintptr(byteLength), 0)
	}
	if h == 0 {
		return nil, shimError("NewBackingStore", 0)
	}
	if i.arrayBufferAllocatorReference != 0 {
		return &BackingStore{iso: i, handle: h}, nil
	}
	return i.backingStore(h)
}

// NewBackingStoreFromSlice creates a backing store that owns a copy of data
// (the crate's new_backing_store_from_vec / from_boxed_slice). The Go slice
// is copied at construction and never retained; the copy is freed by the
// store's deleter when the last reference dies.
func (i *Isolate) NewBackingStoreFromSlice(data []byte) (*BackingStore, error) {
	if err := i.check(); err != nil {
		return nil, err
	}
	var p uintptr
	if len(data) > 0 {
		p = uintptr(unsafe.Pointer(&data[0]))
	}
	h, err := callHandle("NewBackingStoreFromSlice",
		proc("gov8_backing_store_from_slice"), i.handle, p, uintptr(len(data)))
	if err != nil {
		return nil, err
	}
	return &BackingStore{iso: i, handle: h}, nil
}

// NewBackingStoreFromPtr creates a backing store over CALLER-owned memory
// (the crate's new_backing_store_from_ptr). The engine reads through data
// until the deleter runs; the caller must keep the memory valid until then
// and must not free it beforehand. The deleter fires exactly once, after the
// last reference dies, with the registered triple.
func (i *Isolate) NewBackingStoreFromPtr(data unsafe.Pointer, byteLength int, fn BackingStoreDeleter, deleterData uintptr) (*BackingStore, error) {
	if byteLength < 0 {
		return nil, fmt.Errorf("gov8: negative backing store length %d", byteLength)
	}
	if data == nil && byteLength > 0 {
		return nil, fmt.Errorf("gov8: nil data pointer with positive length")
	}
	if fn == nil {
		return nil, fmt.Errorf("gov8: nil backing store deleter")
	}
	if err := i.check(); err != nil {
		return nil, err
	}
	if err := installDeleterEntry(); err != nil {
		return nil, err
	}
	deleterRegistry.mu.Lock()
	deleterRegistry.next++
	handle := deleterRegistry.next
	deleterRegistry.entries[handle] = BackingStoreDeleterEntry{
		data:        data,
		byteLength:  byteLength,
		deleterData: deleterData,
		fn:          fn,
	}
	deleterRegistry.mu.Unlock()
	h, err := callHandle("NewBackingStoreFromPtr",
		proc("gov8_backing_store_from_ptr"), i.handle, uintptr(data),
		uintptr(byteLength), uintptr(handle))
	if err != nil {
		deleterRegistry.mu.Lock()
		delete(deleterRegistry.entries, handle)
		deleterRegistry.mu.Unlock()
		return nil, err
	}
	return &BackingStore{iso: i, handle: h}, nil
}

func (bs *BackingStore) beginUse() (uintptr, error) {
	if bs == nil {
		return 0, fmt.Errorf("gov8: nil backing store")
	}
	if err := loadShim(); err != nil {
		return 0, err
	}
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if bs.closed || bs.handle == 0 {
		return 0, fmt.Errorf("gov8: backing store used after Close")
	}
	if bs.closing {
		return 0, fmt.Errorf("gov8: backing store Close is active")
	}
	bs.active++
	return bs.handle, nil
}

func (bs *BackingStore) endUse() {
	bs.mu.Lock()
	bs.active--
	bs.mu.Unlock()
}

// ByteLength returns the length in bytes of the store's memory.
func (bs *BackingStore) ByteLength() (int, error) {
	handle, err := bs.beginUse()
	if err != nil {
		return 0, err
	}
	defer bs.endUse()
	r1, _, _ := proc("gov8_backing_store_byte_length").Call(handle)
	if int64(r1) < 0 {
		return 0, shimError("BackingStore.ByteLength", r1)
	}
	return int(r1), nil
}

// IsShared reports whether the store was created for a SharedArrayBuffer.
func (bs *BackingStore) IsShared() (bool, error) {
	handle, err := bs.beginUse()
	if err != nil {
		return false, err
	}
	defer bs.endUse()
	r1, _, _ := proc("gov8_backing_store_is_shared").Call(handle)
	if int64(r1) < 0 {
		return false, shimError("BackingStore.IsShared", r1)
	}
	return r1 == 1, nil
}

// IsResizableByUserJavaScript reports whether the store belongs to a
// resizable ArrayBuffer (or growable SharedArrayBuffer).
func (bs *BackingStore) IsResizableByUserJavaScript() (bool, error) {
	handle, err := bs.beginUse()
	if err != nil {
		return false, err
	}
	defer bs.endUse()
	r1, _, _ := proc("gov8_backing_store_is_resizable").Call(handle)
	if int64(r1) < 0 {
		return false, shimError("BackingStore.IsResizableByUserJavaScript", r1)
	}
	return r1 == 1, nil
}

// UseCount returns the store's live reference count: 1 while standalone, +1
// for every engine object (ArrayBuffer/SharedArrayBuffer) currently aliasing
// it. This is the readable form of the crate's assert_use_count_eq polling
// assertion.
func (bs *BackingStore) UseCount() (int, error) {
	handle, err := bs.beginUse()
	if err != nil {
		return 0, err
	}
	defer bs.endUse()
	r1, _, _ := proc("gov8_backing_store_use_count").Call(handle)
	if int64(r1) < 0 {
		return 0, shimError("BackingStore.UseCount", r1)
	}
	return int(r1), nil
}

// ReadAt copies up to len(buf) bytes from the store at byte offset off into
// buf and returns the number of bytes copied. Out-of-range reads are errors
// (Rust would panic on the slice bound).
func (bs *BackingStore) ReadAt(buf []byte, off int) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("gov8: negative backing store read offset")
	}
	handle, err := bs.beginUse()
	if err != nil {
		return 0, err
	}
	defer bs.endUse()
	var p uintptr
	if len(buf) > 0 {
		p = uintptr(unsafe.Pointer(&buf[0]))
	}
	r1, _, _ := proc("gov8_backing_store_read_at").Call(
		handle, uintptr(off), p, uintptr(len(buf)))
	if int64(r1) < 0 {
		return 0, shimError("BackingStore.ReadAt", r1)
	}
	return int(r1), nil
}

// WriteAt writes data into the store at byte offset off and returns the
// number of bytes written. The store is interior-mutable, so writes are
// visible through every ArrayBuffer and view aliasing it. Out-of-range
// writes are errors.
func (bs *BackingStore) WriteAt(data []byte, off int) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("gov8: negative backing store write offset")
	}
	handle, err := bs.beginUse()
	if err != nil {
		return 0, err
	}
	defer bs.endUse()
	var p uintptr
	if len(data) > 0 {
		p = uintptr(unsafe.Pointer(&data[0]))
	}
	r1, _, _ := proc("gov8_backing_store_write_at").Call(
		handle, uintptr(off), p, uintptr(len(data)))
	if int64(r1) < 0 {
		return 0, shimError("BackingStore.WriteAt", r1)
	}
	return int(r1), nil
}

// Close drops this reference to the store. When it is the last one, the
// store's memory is freed (and an external deleter runs) synchronously on
// the calling thread. Close is idempotent-guarded: a second call is an error.
func (bs *BackingStore) Close() error {
	if bs == nil {
		return fmt.Errorf("gov8: nil backing store")
	}
	if err := loadShim(); err != nil {
		return err
	}
	bs.mu.Lock()
	if bs.closed || bs.handle == 0 {
		bs.mu.Unlock()
		return fmt.Errorf("gov8: backing store already closed")
	}
	if bs.closing || bs.active != 0 {
		bs.mu.Unlock()
		return fmt.Errorf("gov8: backing store is active")
	}
	bs.closing = true
	handle := bs.handle
	bs.mu.Unlock()
	ensureBackingStoreHotProcs()
	status, _, _ := syscall.Syscall(backingStoreDisposeAddr, 1, handle, 0, 0)
	if int64(status) < 0 {
		bs.mu.Lock()
		bs.closing = false
		bs.mu.Unlock()
		return shimError("BackingStore.Close", status)
	}
	bs.mu.Lock()
	bs.closed = true
	bs.handle = 0
	allocatorReference := bs.arrayBufferAllocatorReference
	bs.arrayBufferAllocatorReference = 0
	bs.closing = false
	bs.mu.Unlock()
	if allocatorReference != 0 {
		status, _, _ := syscall.Syscall(allocatorDisposeAddr, 1, allocatorReference, 0, 0)
		if int64(status) < 0 {
			return shimError("ArrayBufferAllocator.backing-store.Close", status)
		}
	}
	return nil
}

// --- ArrayBuffer ---------------------------------------------------------------

// ArrayBuffer is a scope-local v8::ArrayBuffer.
type ArrayBuffer struct {
	Value
}

// NewArrayBuffer allocates a new zero-initialized ArrayBuffer of byteLength
// bytes (v8::ArrayBuffer::new). The context supplies the instance map the
// engine allocates against (its native context must be the caller's).
// Absurd sizes fail inside the engine exactly as in the pinned oracle: as a
// process-fatal OOM, not a Go error.
func NewArrayBuffer(s *Scope, c *Context, byteLength int) (*ArrayBuffer, error) {
	if byteLength < 0 {
		return nil, fmt.Errorf("gov8: negative ArrayBuffer length %d", byteLength)
	}
	if err := contextHandles(s, c, "NewArrayBuffer"); err != nil {
		return nil, err
	}
	ih, sh, err := scopeHandles(s, "NewArrayBuffer")
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_array_buffer_new").Call(ih, c.handle, sh, uintptr(byteLength), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewArrayBuffer", r1)
	}
	return &ArrayBuffer{Value{iso: s.iso, sc: s, h: out}}, nil
}

// NewArrayBufferWithBackingStore creates an ArrayBuffer aliasing the store
// (v8::ArrayBuffer::with_backing_store). The store gains one reference while
// the engine object lives; the Go BackingStore wrapper is unaffected.
func NewArrayBufferWithBackingStore(s *Scope, c *Context, bs *BackingStore) (*ArrayBuffer, error) {
	if bs == nil {
		return nil, fmt.Errorf("gov8: nil backing store")
	}
	if err := contextHandles(s, c, "NewArrayBufferWithBackingStore"); err != nil {
		return nil, err
	}
	ih, sh, err := scopeHandles(s, "NewArrayBufferWithBackingStore")
	if err != nil {
		return nil, err
	}
	if s.iso != bs.iso {
		return nil, foreignIsolate("backing store")
	}
	backingStoreHandle, err := bs.beginUse()
	if err != nil {
		return nil, err
	}
	defer bs.endUse()
	var out uintptr
	r1, _, _ := proc("gov8_array_buffer_new_with_backing_store").Call(ih, c.handle, sh, backingStoreHandle, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewArrayBufferWithBackingStore", r1)
	}
	return &ArrayBuffer{Value{iso: s.iso, sc: s, h: out}}, nil
}

// AsArrayBuffer converts a generic value into an ArrayBuffer view of it.
func AsArrayBuffer(v Value) (*ArrayBuffer, error) {
	is, err := v.IsArrayBuffer()
	if err != nil {
		return nil, err
	}
	if !is {
		return nil, fmt.Errorf("gov8: value is not an ArrayBuffer")
	}
	return &ArrayBuffer{Value: v}, nil
}

// ByteLength returns the buffer's length in bytes (0 after detach).
func (ab *ArrayBuffer) ByteLength() (int, error) {
	if err := ab.check(); err != nil {
		return 0, err
	}
	ih, sh, err := scopeHandles(ab.sc, "ArrayBuffer.ByteLength")
	if err != nil {
		return 0, err
	}
	var out int64
	r1, _, _ := proc("gov8_array_buffer_byte_length").Call(ih, sh, ab.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("ArrayBuffer.ByteLength", r1)
	}
	return int(out), nil
}

// IsDetachable reports whether the buffer may be detached.
func (ab *ArrayBuffer) IsDetachable() (bool, error) {
	if err := ab.check(); err != nil {
		return false, err
	}
	ih, sh, err := scopeHandles(ab.sc, "ArrayBuffer.IsDetachable")
	if err != nil {
		return false, err
	}
	var out int32
	r1, _, _ := proc("gov8_array_buffer_is_detachable").Call(ih, sh, ab.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return false, shimError("ArrayBuffer.IsDetachable", r1)
	}
	return out == 1, nil
}

// WasDetached reports whether the buffer has been detached. It reproduces
// the pinned crate's wrapper exactly: a non-zero-length buffer reports false
// without consulting the engine (only zero-length buffers read the real
// WasDetached bit).
func (ab *ArrayBuffer) WasDetached() (bool, error) {
	length, err := ab.ByteLength()
	if err != nil {
		return false, err
	}
	if length != 0 {
		return false, nil
	}
	if err := ab.check(); err != nil {
		return false, err
	}
	ih, sh, err := scopeHandles(ab.sc, "ArrayBuffer.WasDetached")
	if err != nil {
		return false, err
	}
	var out int32
	r1, _, _ := proc("gov8_array_buffer_was_detached_raw").Call(ih, sh, ab.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return false, shimError("ArrayBuffer.WasDetached", r1)
	}
	return out == 1, nil
}

// Data returns the buffer's raw data pointer (0 when there is none: a
// zero-length or detached buffer). The pointer is engine-owned, valid only
// while the buffer is alive, and must never be dereferenced or retained by
// Go; use a BackingStore (ReadAt/WriteAt) or views to touch the bytes.
func (ab *ArrayBuffer) Data() (uintptr, bool, error) {
	if err := ab.check(); err != nil {
		return 0, false, err
	}
	ih, sh, err := scopeHandles(ab.sc, "ArrayBuffer.Data")
	if err != nil {
		return 0, false, err
	}
	r1, _, _ := proc("gov8_array_buffer_data").Call(ih, sh, ab.h)
	if int64(r1) < 0 {
		return 0, false, shimError("ArrayBuffer.Data", r1)
	}
	return uintptr(r1), r1 != 0, nil
}

// GetBackingStore returns a NEW counted reference to the buffer's backing
// store. The caller must Close it; while it is open the store outlives the
// buffer, exactly like holding a SharedRef in the pinned crate.
func (ab *ArrayBuffer) GetBackingStore() (*BackingStore, error) {
	if err := ab.check(); err != nil {
		return nil, err
	}
	ih, sh, err := scopeHandles(ab.sc, "ArrayBuffer.GetBackingStore")
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_array_buffer_get_backing_store").Call(ih, sh, ab.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("ArrayBuffer.GetBackingStore", r1)
	}
	return ab.iso.backingStore(out)
}

// Detach detaches the buffer and all its views. c is the context used for
// engine-internal bookkeeping (a key mismatch allocates a TypeError from
// it); key is the detach key to present (an empty Value{} means "no key",
// Rust's detach(None)). The bool result is false only when the stored
// [[ArrayBufferDetachKey]] rejected the request (Rust's None, with the
// engine's TypeError captured by a shim-internal TryCatch); non-detachable
// buffers report true without touching the engine, mirroring the crate's
// wrapper.
func (ab *ArrayBuffer) Detach(c *Context, key Value) (bool, error) {
	detachable, err := ab.IsDetachable()
	if err != nil {
		return false, err
	}
	if !detachable {
		return true, nil
	}
	if err := ab.check(); err != nil {
		return false, err
	}
	if c == nil || c.iso != ab.iso {
		return false, foreignIsolate("context")
	}
	if err := c.check(); err != nil {
		return false, err
	}
	ih, sh, err := scopeHandles(ab.sc, "ArrayBuffer.Detach")
	if err != nil {
		return false, err
	}
	var out int32
	r1, _, _ := proc("gov8_array_buffer_detach").Call(ih, c.handle, 0, sh, ab.h, key.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return false, shimError("ArrayBuffer.Detach", r1)
	}
	return out == 1, nil
}

// SetDetachKey stores the [[ArrayBufferDetachKey]]: after this, only a
// Detach presenting an equal key succeeds.
func (ab *ArrayBuffer) SetDetachKey(key Value) error {
	if err := ab.check(); err != nil {
		return err
	}
	if err := key.check(); err != nil {
		return err
	}
	ih, sh, err := scopeHandles(ab.sc, "ArrayBuffer.SetDetachKey")
	if err != nil {
		return err
	}
	return callErr("ArrayBuffer.SetDetachKey", proc("gov8_array_buffer_set_detach_key"),
		ih, sh, ab.h, key.h)
}

// --- SharedArrayBuffer ----------------------------------------------------------

// SharedArrayBuffer is a scope-local v8::SharedArrayBuffer.
type SharedArrayBuffer struct {
	Value
}

// NewSharedArrayBuffer allocates a new zero-initialized SharedArrayBuffer.
func NewSharedArrayBuffer(s *Scope, c *Context, byteLength int) (*SharedArrayBuffer, error) {
	if byteLength < 0 {
		return nil, fmt.Errorf("gov8: negative SharedArrayBuffer length %d", byteLength)
	}
	if err := contextHandles(s, c, "NewSharedArrayBuffer"); err != nil {
		return nil, err
	}
	ih, sh, err := scopeHandles(s, "NewSharedArrayBuffer")
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_sab_new").Call(ih, c.handle, sh, uintptr(byteLength), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewSharedArrayBuffer", r1)
	}
	return &SharedArrayBuffer{Value{iso: s.iso, sc: s, h: out}}, nil
}

// NewSharedArrayBufferWithBackingStore creates a SharedArrayBuffer aliasing
// the (shared) store (v8::SharedArrayBuffer::with_backing_store).
func NewSharedArrayBufferWithBackingStore(s *Scope, c *Context, bs *BackingStore) (*SharedArrayBuffer, error) {
	if bs == nil {
		return nil, fmt.Errorf("gov8: nil backing store")
	}
	if err := contextHandles(s, c, "NewSharedArrayBufferWithBackingStore"); err != nil {
		return nil, err
	}
	ih, sh, err := scopeHandles(s, "NewSharedArrayBufferWithBackingStore")
	if err != nil {
		return nil, err
	}
	if s.iso != bs.iso {
		return nil, foreignIsolate("backing store")
	}
	backingStoreHandle, err := bs.beginUse()
	if err != nil {
		return nil, err
	}
	defer bs.endUse()
	var out uintptr
	r1, _, _ := proc("gov8_sab_new_with_backing_store").Call(ih, c.handle, sh, backingStoreHandle, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewSharedArrayBufferWithBackingStore", r1)
	}
	return &SharedArrayBuffer{Value{iso: s.iso, sc: s, h: out}}, nil
}

// AsSharedArrayBuffer converts a generic value into a SharedArrayBuffer.
func AsSharedArrayBuffer(v Value) (*SharedArrayBuffer, error) {
	is, err := v.IsSharedArrayBuffer()
	if err != nil {
		return nil, err
	}
	if !is {
		return nil, fmt.Errorf("gov8: value is not a SharedArrayBuffer")
	}
	return &SharedArrayBuffer{Value: v}, nil
}

// ByteLength returns the buffer's length in bytes.
func (sab *SharedArrayBuffer) ByteLength() (int, error) {
	if err := sab.check(); err != nil {
		return 0, err
	}
	ih, sh, err := scopeHandles(sab.sc, "SharedArrayBuffer.ByteLength")
	if err != nil {
		return 0, err
	}
	var out int64
	r1, _, _ := proc("gov8_sab_byte_length").Call(ih, sh, sab.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("SharedArrayBuffer.ByteLength", r1)
	}
	return int(out), nil
}

// GetBackingStore returns a NEW counted reference to the buffer's backing
// store; the caller must Close it.
func (sab *SharedArrayBuffer) GetBackingStore() (*BackingStore, error) {
	if err := sab.check(); err != nil {
		return nil, err
	}
	ih, sh, err := scopeHandles(sab.sc, "SharedArrayBuffer.GetBackingStore")
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_sab_get_backing_store").Call(ih, sh, sab.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("SharedArrayBuffer.GetBackingStore", r1)
	}
	return sab.iso.backingStore(out)
}

// NewSharedArrayBufferBackingStore allocates a standalone shared backing
// store (the crate's SharedArrayBuffer::new_backing_store); IsShared reports
// true for it and it can back SharedArrayBuffers.
func (i *Isolate) NewSharedArrayBufferBackingStore(byteLength int) (*BackingStore, error) {
	if byteLength < 0 {
		return nil, fmt.Errorf("gov8: negative backing store length %d", byteLength)
	}
	if err := i.check(); err != nil {
		return nil, err
	}
	h, err := callHandle("NewSharedArrayBufferBackingStore",
		proc("gov8_sab_backing_store_new"), i.handle, uintptr(byteLength))
	if err != nil {
		return nil, err
	}
	return i.backingStore(h)
}

// NewSharedArrayBufferBackingStoreFromSlice creates a shared backing store
// that owns a copy of data (the crate's new_backing_store_from_vec /
// new_backing_store_from_boxed_slice). The Go slice is never retained.
func (i *Isolate) NewSharedArrayBufferBackingStoreFromSlice(data []byte) (*BackingStore, error) {
	if err := i.check(); err != nil {
		return nil, err
	}
	var p uintptr
	if len(data) > 0 {
		p = uintptr(unsafe.Pointer(&data[0]))
	}
	h, err := callHandle("NewSharedArrayBufferBackingStoreFromSlice",
		proc("gov8_sab_backing_store_from_slice"), i.handle, p, uintptr(len(data)))
	if err != nil {
		return nil, err
	}
	return &BackingStore{iso: i, handle: h}, nil
}

// NewSharedArrayBufferBackingStoreFromPtr creates a shared backing store over
// caller-owned memory. The memory and deleter have the same lifetime contract
// as NewBackingStoreFromPtr.
func (i *Isolate) NewSharedArrayBufferBackingStoreFromPtr(data unsafe.Pointer, byteLength int, fn BackingStoreDeleter, deleterData uintptr) (*BackingStore, error) {
	if byteLength < 0 {
		return nil, fmt.Errorf("gov8: negative backing store length %d", byteLength)
	}
	if data == nil && byteLength > 0 {
		return nil, fmt.Errorf("gov8: nil data pointer with positive length")
	}
	if fn == nil {
		return nil, fmt.Errorf("gov8: nil backing store deleter")
	}
	if err := i.check(); err != nil {
		return nil, err
	}
	if err := installDeleterEntry(); err != nil {
		return nil, err
	}
	deleterRegistry.mu.Lock()
	deleterRegistry.next++
	handle := deleterRegistry.next
	deleterRegistry.entries[handle] = BackingStoreDeleterEntry{
		data: data, byteLength: byteLength, deleterData: deleterData, fn: fn,
	}
	deleterRegistry.mu.Unlock()
	h, err := callHandle("NewSharedArrayBufferBackingStoreFromPtr",
		proc("gov8_sab_backing_store_from_ptr"), i.handle, uintptr(data),
		uintptr(byteLength), uintptr(handle))
	if err != nil {
		deleterRegistry.mu.Lock()
		delete(deleterRegistry.entries, handle)
		deleterRegistry.mu.Unlock()
		return nil, err
	}
	return &BackingStore{iso: i, handle: h}, nil
}

// --- typed arrays / DataView -----------------------------------------------------

// viewKind identifies the typed-array element type at the shim boundary.
// Keep in sync with ViewKindOf in
// internal/shim/features/buffers_serialization.inc.
type viewKind int64

const (
	viewUint8 viewKind = iota
	viewInt8
	viewUint16
	viewInt16
	viewUint32
	viewInt32
	viewFloat32
	viewFloat64
	viewBigInt64
	viewBigUint64
	viewUint8Clamped
)

// viewElementSize is the per-type element size used for Go-side errors and
// documentation; the shim enforces the same values before entering V8.
var viewElementSize = map[viewKind]int{
	viewUint8: 1, viewInt8: 1, viewUint8Clamped: 1,
	viewUint16: 2, viewInt16: 2,
	viewUint32: 4, viewInt32: 4, viewFloat32: 4,
	viewFloat64: 8, viewBigInt64: 8, viewBigUint64: 8,
}

// TypedArray is a scope-local typed array (v8::TypedArray surface).
type TypedArray struct {
	Value
}

// newTypedArray constructs a typed array over an ArrayBuffer after the shim
// has prevalidated the geometry (out-of-bounds and misaligned geometry are
// rejected as errors instead of the engine's process-fatal CHECK).
func newTypedArray(s *Scope, c *Context, ab *ArrayBuffer, kind viewKind, byteOffset, length int) (*TypedArray, error) {
	if err := contextHandles(s, c, "NewTypedArray"); err != nil {
		return nil, err
	}
	ih, sh, err := scopeHandles(s, "NewTypedArray")
	if err != nil {
		return nil, err
	}
	if ab == nil {
		return nil, fmt.Errorf("gov8: nil ArrayBuffer")
	}
	if err := ab.check(); err != nil {
		return nil, err
	}
	if ab.iso != s.iso {
		return nil, foreignIsolate("ArrayBuffer")
	}
	var out uintptr
	r1, _, _ := proc("gov8_typed_array_new").Call(ih, c.handle, sh, ab.h, uintptr(kind),
		uintptr(byteOffset), uintptr(length), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewTypedArray", r1)
	}
	return &TypedArray{Value{iso: s.iso, sc: s, h: out}}, nil
}

// NewUint8Array creates a Uint8Array over ab's bytes [byteOffset,
// byteOffset+length) (v8::Uint8Array::new). length counts elements.
func NewUint8Array(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*TypedArray, error) {
	return newTypedArray(s, c, ab, viewUint8, byteOffset, length)
}

// NewFloat64Array creates a Float64Array over ab. byteOffset must be a
// multiple of 8 and byteOffset+8*length must not exceed the buffer.
func NewFloat64Array(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*TypedArray, error) {
	return newTypedArray(s, c, ab, viewFloat64, byteOffset, length)
}

// NewBigInt64Array creates a BigInt64Array over ab. byteOffset must be a
// multiple of 8 and byteOffset+8*length must not exceed the buffer.
func NewBigInt64Array(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*TypedArray, error) {
	return newTypedArray(s, c, ab, viewBigInt64, byteOffset, length)
}

// AsTypedArray converts a generic value into a typed-array view of it.
func AsTypedArray(v Value) (*TypedArray, error) {
	is, err := v.IsTypedArray()
	if err != nil {
		return nil, err
	}
	if !is {
		return nil, fmt.Errorf("gov8: value is not a TypedArray")
	}
	return &TypedArray{Value: v}, nil
}

// Length returns the element count of the typed array.
func (ta *TypedArray) Length() (int, error) {
	if err := ta.check(); err != nil {
		return 0, err
	}
	ih, sh, err := scopeHandles(ta.sc, "TypedArray.Length")
	if err != nil {
		return 0, err
	}
	var out int64
	r1, _, _ := proc("gov8_typed_array_length").Call(ih, sh, ta.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError("TypedArray.Length", r1)
	}
	return int(out), nil
}

// DataView is a scope-local DataView.
type DataView struct {
	Value
}

// NewDataView creates a DataView over ab's bytes [byteOffset,
// byteOffset+length). Out-of-bounds geometry is an error (the engine would
// CHECK-abort; the shim prevalidates).
func NewDataView(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*DataView, error) {
	if err := contextHandles(s, c, "NewDataView"); err != nil {
		return nil, err
	}
	ih, sh, err := scopeHandles(s, "NewDataView")
	if err != nil {
		return nil, err
	}
	if ab == nil {
		return nil, fmt.Errorf("gov8: nil ArrayBuffer")
	}
	if err := ab.check(); err != nil {
		return nil, err
	}
	if ab.iso != s.iso {
		return nil, foreignIsolate("ArrayBuffer")
	}
	var out uintptr
	r1, _, _ := proc("gov8_data_view_new").Call(ih, c.handle, sh, ab.h,
		uintptr(byteOffset), uintptr(length), uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewDataView", r1)
	}
	return &DataView{Value{iso: s.iso, sc: s, h: out}}, nil
}

// AsDataView converts a generic value into a DataView view of it.
func AsDataView(v Value) (*DataView, error) {
	is, err := v.IsDataView()
	if err != nil {
		return nil, err
	}
	if !is {
		return nil, fmt.Errorf("gov8: value is not a DataView")
	}
	return &DataView{Value: v}, nil
}

// byteOffsetOf reads ArrayBufferView::ByteOffset for any view value.
func byteOffsetOf(op string, v Value) (int, error) {
	if err := v.check(); err != nil {
		return 0, err
	}
	ih, sh, err := scopeHandles(v.sc, op)
	if err != nil {
		return 0, err
	}
	var out int64
	r1, _, _ := proc(op).Call(ih, sh, v.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError(op, r1)
	}
	return int(out), nil
}

// ByteOffset returns the view's offset into its buffer.
func (ta *TypedArray) ByteOffset() (int, error) {
	return byteOffsetOf("gov8_view_byte_offset", ta.Value)
}

// ByteOffset returns the view's offset into its buffer.
func (dv *DataView) ByteOffset() (int, error) {
	return byteOffsetOf("gov8_view_byte_offset", dv.Value)
}

// byteLengthOf reads ArrayBufferView::ByteLength for any view value.
func byteLengthOf(op string, v Value) (int, error) {
	if err := v.check(); err != nil {
		return 0, err
	}
	ih, sh, err := scopeHandles(v.sc, op)
	if err != nil {
		return 0, err
	}
	var out int64
	r1, _, _ := proc(op).Call(ih, sh, v.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return 0, shimError(op, r1)
	}
	return int(out), nil
}

// ByteLength returns the view's size in bytes.
func (ta *TypedArray) ByteLength() (int, error) {
	return byteLengthOf("gov8_view_byte_length", ta.Value)
}

// ByteLength returns the view's size in bytes.
func (dv *DataView) ByteLength() (int, error) {
	return byteLengthOf("gov8_view_byte_length", dv.Value)
}

// HasBuffer reports whether the view's backing ArrayBuffer is allocated.
func hasBuffer(v Value) (bool, error) {
	if err := v.check(); err != nil {
		return false, err
	}
	ih, sh, err := scopeHandles(v.sc, "view.HasBuffer")
	if err != nil {
		return false, err
	}
	var out int32
	r1, _, _ := proc("gov8_view_has_buffer").Call(ih, sh, v.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return false, shimError("view.HasBuffer", r1)
	}
	return out == 1, nil
}

// HasBuffer reports whether the typed array's backing buffer is allocated.
func (ta *TypedArray) HasBuffer() (bool, error) { return hasBuffer(ta.Value) }

// Buffer returns the view's underlying ArrayBuffer as a fresh scope-local
// value (object identity: Same(view.Buffer(), ab) is true for the ab the
// view was created over).
func bufferOf(v Value) (*ArrayBuffer, error) {
	if err := v.check(); err != nil {
		return nil, err
	}
	ih, sh, err := scopeHandles(v.sc, "view.Buffer")
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_view_buffer").Call(ih, sh, v.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("view.Buffer", r1)
	}
	return &ArrayBuffer{Value{iso: v.iso, sc: v.sc, h: out}}, nil
}

// Buffer returns the typed array's underlying ArrayBuffer.
func (ta *TypedArray) Buffer() (*ArrayBuffer, error) { return bufferOf(ta.Value) }

// Buffer returns the DataView's underlying ArrayBuffer.
func (dv *DataView) Buffer() (*ArrayBuffer, error) { return bufferOf(dv.Value) }

// copyContents copies at most len(dst) bytes of the view's contents into dst
// (ArrayBufferView::CopyContents) and returns the number of bytes written.
func (ta *TypedArray) CopyContents(dst []byte) (int, error) {
	if err := ta.check(); err != nil {
		return 0, err
	}
	ih, sh, err := scopeHandles(ta.sc, "TypedArray.CopyContents")
	if err != nil {
		return 0, err
	}
	var p uintptr
	if len(dst) > 0 {
		p = uintptr(unsafe.Pointer(&dst[0]))
	}
	r1, _, _ := proc("gov8_view_copy_contents").Call(ih, sh, ta.h, p, uintptr(len(dst)))
	if int64(r1) < 0 {
		return 0, shimError("TypedArray.CopyContents", r1)
	}
	return int(r1), nil
}

// TypedArrayLimits carries the pinned build's typed-array size limits.
type TypedArrayLimits struct {
	// MaxByteLength is the largest supported typed-array byte size
	// (2^53-1 for this build).
	MaxByteLength int64
	// Uint8MaxLength / Float64MaxLength / BigInt64MaxLength are the
	// per-type maximum element counts accepted by the constructors.
	Uint8MaxLength    int64
	Float64MaxLength  int64
	BigInt64MaxLength int64
	// MaxSizeInHeap is the pinned artifact's on-heap typed-array size
	// threshold (0: this build never stores typed arrays on the JS heap).
	MaxSizeInHeap int64
}

// TypedArrayLimits reads the pinned build's limits from the engine shim
// (v8::TypedArray::kMaxByteLength and the per-type kMaxLength constants of
// the pinned headers).
func TypedArrayLimitsQuery() (TypedArrayLimits, error) {
	var out TypedArrayLimits
	r1, _, _ := proc("gov8_typed_array_limits").Call(
		uintptr(unsafe.Pointer(&out.MaxByteLength)),
		uintptr(unsafe.Pointer(&out.Uint8MaxLength)),
		uintptr(unsafe.Pointer(&out.Float64MaxLength)),
		uintptr(unsafe.Pointer(&out.BigInt64MaxLength)),
		uintptr(unsafe.Pointer(&out.MaxSizeInHeap)))
	if int64(r1) < 0 {
		return TypedArrayLimits{}, shimError("TypedArrayLimits", r1)
	}
	return out, nil
}

// contextHandles validates the explicit live Context that the shim enters
// before allocation. Rust's context-scope type makes this precondition
// implicit; calling upstream ArrayBuffer::New without an entered context is
// an access violation. Go requires the Context argument and rejects missing,
// closed, or cross-isolate contexts before the ABI.
func contextHandles(s *Scope, c *Context, op string) error {
	if s == nil {
		return fmt.Errorf("gov8: %s requires a scope", op)
	}
	if err := s.check(); err != nil {
		return err
	}
	if c == nil {
		return fmt.Errorf("gov8: %s requires an entered context", op)
	}
	if c.iso != s.iso {
		return foreignIsolate("context")
	}
	return c.check()
}

// scopeHandles validates scope state and returns (isolate, scope) shim
// handles for an operation.
func scopeHandles(s *Scope, op string) (uintptr, uintptr, error) {
	if err := s.check(); err != nil {
		return 0, 0, err
	}
	ih, err := s.iso.handleChecked()
	if err != nil {
		return 0, 0, err
	}
	return ih, s.handle, nil
}
