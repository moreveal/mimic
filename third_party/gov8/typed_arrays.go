//go:build windows && amd64

package gov8

import (
	"fmt"
	"unsafe"
)

// Typed arrays: the complete 12-kind surface, per-kind predicates, view
// data/backing-store/contents access, and pinned per-kind limits.
//
// Rust surface mapped here (pinned crate v8 =152.2.0):
//
//	v8::{Int8,Uint8,Uint8Clamped,Int16,Uint16,Int32,Uint32,Float16,Float32,
//	     Float64,BigInt64,BigUint64}Array::new -> the per-kind New*Array
//	     constructors and NewTypedArrayOfKind
//	v8::Value::{is_int8_array,...,is_big_uint64_array} -> the 12 Is*Array
//	     predicates
//	v8::ArrayBufferView::{data, get_backing_store, get_contents,
//	     copy_contents, byte_offset, byte_length, has_buffer, buffer}
//	     -> TypedArray/DataView Data / GetBackingStore / GetContents /
//	     CopyContents / ByteOffset / ByteLength / HasBuffer / Buffer
//	v8::{Int8Array,...}::MAX_LENGTH and TypedArray::MAX_BYTE_LENGTH and
//	     v8::TYPED_ARRAY_MAX_SIZE_IN_HEAP -> TypedArrayKindLimitsQuery
//
// Ownership and lifetime rules (in addition to buffer.go):
//   - View.Data() reports the engine-side data pointer (crate data():
//     Buffer()->Data() + ByteOffset()). The pointer is engine-owned, valid
//     only while the view value is alive and its scope open, null after
//     detach, and must never be dereferenced or retained by Go. It exists so
//     Go can observe pointer relationships; bytes move through
//     CopyContents/GetContents or the backing store.
//   - GetContents returns the engine's LIVE contents description: Length is
//     the span's full length (for this build always the view byte length,
//     independent of the caller's storage size, engine api.cc GetContents
//     off-heap path) and Source the span's base (observation-only, same
//     rules as Data). The engine copies up to len(storage) bytes into
//     storage; with TYPED_ARRAY_MAX_SIZE_IN_HEAP = 0 nothing is ever on the
//     V8 heap, so "live contents" is observable by re-reading after JS
//     writes — Go never holds a live slice into engine memory (no Go slice
//     may alias engine memory: the GC could retain it beyond the view's
//     life). This is the one intentional shape change versus the crate's
//     unsafe MemorySpan return, documented per the parity standard.
//   - View.GetBackingStore hands out a NEW counted reference each call (the
//     caller must Close it), exactly like ArrayBuffer.GetBackingStore. It
//     works for SharedArrayBuffer-backed views too.
//   - NewTypedArrayOfKind prevalidates every process-fatal engine boundary
//     (per-kind max length ApiCheck, alignment CHECK, bounds CHECKs, in the
//     engine's fatal order) and returns an error instead of aborting. Every
//     in-bounds observation is unchanged; JS misuse keeps producing JS
//     RangeErrors, never these native-shape errors.
//
// Intentional deviations (documented, behavior-preserving where observable):
//   - copy_contents_uninit has no Go shape (Go has no MaybeUninit): every
//     CopyContents observes the same bytes regardless of destination
//     initialization state, so CopyContents covers both.
//   - Native SharedArrayBuffer view construction is not bound: the pinned
//     crate only binds the Local<ArrayBuffer> overload of TypedArray::New
//     (upstream binding gap pinned by the oracle); SAB-backed views are
//     created from JS and observed through this surface.
//   - Geometry that the engine answers with a process-fatal V8 CHECK or
//     ApiCheck becomes a Go error (see above); the pinned oracle
//     characterizes those boundaries out-of-process
//     (rust-oracle/tests/typed_arrays_negative.rs).

// viewFloat16 extends the shim kind numbering of buffer.go with the kind the
// buffers slice did not need: Float16Array (engine EACH_TYPED_ARRAY).
const viewFloat16 viewKind = 11

// TypedArrayKind identifies a typed-array element type at the shim boundary
// (the same wire values as the viewKind constants of buffer.go, which this
// alias keeps in lockstep with).
type TypedArrayKind = viewKind

// The 12 typed-array kinds (v8::Int8Array ... v8::BigUint64Array).
const (
	KindUint8        TypedArrayKind = viewUint8
	KindInt8         TypedArrayKind = viewInt8
	KindUint16       TypedArrayKind = viewUint16
	KindInt16        TypedArrayKind = viewInt16
	KindUint32       TypedArrayKind = viewUint32
	KindInt32        TypedArrayKind = viewInt32
	KindFloat16      TypedArrayKind = viewFloat16
	KindFloat32      TypedArrayKind = viewFloat32
	KindFloat64      TypedArrayKind = viewFloat64
	KindBigInt64     TypedArrayKind = viewBigInt64
	KindBigUint64    TypedArrayKind = viewBigUint64
	KindUint8Clamped TypedArrayKind = viewUint8Clamped
)

// TypedArrayKinds lists all 12 kinds in the fixed oracle order (part of the
// observable contract of the typed-arrays fixture).
var TypedArrayKinds = []TypedArrayKind{
	KindInt8, KindUint8, KindUint8Clamped,
	KindInt16, KindUint16,
	KindInt32, KindUint32,
	KindFloat16, KindFloat32,
	KindFloat64,
	KindBigInt64, KindBigUint64,
}

// IsValid reports whether k is one of the 12 kinds.
func (k TypedArrayKind) IsValid() bool { return k >= 0 && k <= viewFloat16 }

// ElementSize returns the kind's element size in bytes (0 for an invalid
// kind). These are the engine's sizeof(element) values; the shim enforces
// the same alignment classes before entering V8.
func (k TypedArrayKind) ElementSize() int {
	switch k {
	case KindUint8, KindInt8, KindUint8Clamped:
		return 1
	case KindUint16, KindInt16, KindFloat16:
		return 2
	case KindUint32, KindInt32, KindFloat32:
		return 4
	case KindFloat64, KindBigInt64, KindBigUint64:
		return 8
	}
	return 0
}

// String returns the kind's JS constructor name (or "TypedArray(<n>)" for an
// invalid kind).
func (k TypedArrayKind) String() string {
	switch k {
	case KindUint8:
		return "Uint8Array"
	case KindInt8:
		return "Int8Array"
	case KindUint16:
		return "Uint16Array"
	case KindInt16:
		return "Int16Array"
	case KindUint32:
		return "Uint32Array"
	case KindInt32:
		return "Int32Array"
	case KindFloat16:
		return "Float16Array"
	case KindFloat32:
		return "Float32Array"
	case KindFloat64:
		return "Float64Array"
	case KindBigInt64:
		return "BigInt64Array"
	case KindBigUint64:
		return "BigUint64Array"
	case KindUint8Clamped:
		return "Uint8ClampedArray"
	}
	return fmt.Sprintf("TypedArray(%d)", int64(k))
}

// --- per-kind value predicates (v8::Value::is_*_array) ------------------------

// IsInt8Array reports whether the value is an Int8Array.
func (v Value) IsInt8Array() (bool, error) { return v.predicate("gov8_is_int8_array") }

// IsUint8Array reports whether the value is a Uint8Array.
func (v Value) IsUint8Array() (bool, error) { return v.predicate("gov8_is_uint8_array") }

// IsUint8ClampedArray reports whether the value is a Uint8ClampedArray.
func (v Value) IsUint8ClampedArray() (bool, error) {
	return v.predicate("gov8_is_uint8_clamped_array")
}

// IsInt16Array reports whether the value is an Int16Array.
func (v Value) IsInt16Array() (bool, error) { return v.predicate("gov8_is_int16_array") }

// IsUint16Array reports whether the value is a Uint16Array.
func (v Value) IsUint16Array() (bool, error) { return v.predicate("gov8_is_uint16_array") }

// IsInt32Array reports whether the value is an Int32Array.
func (v Value) IsInt32Array() (bool, error) { return v.predicate("gov8_is_int32_array") }

// IsUint32Array reports whether the value is a Uint32Array.
func (v Value) IsUint32Array() (bool, error) { return v.predicate("gov8_is_uint32_array") }

// IsFloat16Array reports whether the value is a Float16Array.
func (v Value) IsFloat16Array() (bool, error) { return v.predicate("gov8_is_float16_array") }

// IsFloat32Array reports whether the value is a Float32Array.
func (v Value) IsFloat32Array() (bool, error) { return v.predicate("gov8_is_float32_array") }

// IsFloat64Array reports whether the value is a Float64Array.
func (v Value) IsFloat64Array() (bool, error) { return v.predicate("gov8_is_float64_array") }

// IsBigInt64Array reports whether the value is a BigInt64Array.
func (v Value) IsBigInt64Array() (bool, error) { return v.predicate("gov8_is_big_int64_array") }

// IsBigUint64Array reports whether the value is a BigUint64Array.
func (v Value) IsBigUint64Array() (bool, error) {
	return v.predicate("gov8_is_big_uint64_array")
}

// IsTypedArrayOfKind reports whether the value is a typed array of exactly
// kind k (the per-kind predicates routed through the kind table).
func (k TypedArrayKind) IsTypedArrayOfKind(v Value) (bool, error) {
	ops := [...]string{
		KindUint8:        "gov8_is_uint8_array",
		KindInt8:         "gov8_is_int8_array",
		KindUint16:       "gov8_is_uint16_array",
		KindInt16:        "gov8_is_int16_array",
		KindUint32:       "gov8_is_uint32_array",
		KindInt32:        "gov8_is_int32_array",
		KindFloat16:      "gov8_is_float16_array",
		KindFloat32:      "gov8_is_float32_array",
		KindFloat64:      "gov8_is_float64_array",
		KindBigInt64:     "gov8_is_big_int64_array",
		KindBigUint64:    "gov8_is_big_uint64_array",
		KindUint8Clamped: "gov8_is_uint8_clamped_array",
	}
	if !k.IsValid() {
		return false, fmt.Errorf("gov8: unknown typed array kind %d", int64(k))
	}
	return v.predicate(ops[k])
}

// --- construction ---------------------------------------------------------------

// NewTypedArrayOfKind creates a typed array of the given kind over ab's
// bytes [byteOffset, byteOffset+length*elementSize) (X::new for every one of
// the 12 kinds). length counts elements. Geometry that the engine would
// answer with a process-fatal V8 CHECK/ApiCheck is prevalidated and returned
// as an error, in the engine's fatal order:
//
//  1. length > kind max length  ("...length exceeds max allowed value")
//  2. byteOffset not a multiple of the element size ("...not aligned...")
//  3. byteOffset/length out of the buffer ("...out of bounds")
func NewTypedArrayOfKind(s *Scope, c *Context, ab *ArrayBuffer, kind TypedArrayKind, byteOffset, length int) (*TypedArray, error) {
	if !kind.IsValid() {
		return nil, fmt.Errorf("gov8: unknown typed array kind %d", int64(kind))
	}
	if byteOffset < 0 || length < 0 {
		return nil, fmt.Errorf("gov8: negative view geometry (offset %d, length %d)", byteOffset, length)
	}
	ih, sh, err := scopeHandles(s, "NewTypedArrayOfKind")
	if err != nil {
		return nil, err
	}
	if ab == nil {
		return nil, fmt.Errorf("gov8: nil ArrayBuffer")
	}
	if err := ab.check(); err != nil {
		return nil, err
	}
	if err := contextHandles(s, c, "NewTypedArrayOfKind"); err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_typed_array_new_kind").Call(ih, c.handle, sh, ab.h,
		uintptr(int64(kind)), uintptr(int64(byteOffset)), uintptr(int64(length)),
		uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("NewTypedArrayOfKind", r1)
	}
	return &TypedArray{Value{iso: s.iso, sc: s, h: out}}, nil
}

// NewInt8Array creates an Int8Array over ab (v8::Int8Array::new).
func NewInt8Array(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*TypedArray, error) {
	return NewTypedArrayOfKind(s, c, ab, KindInt8, byteOffset, length)
}

// NewUint8ClampedArray creates a Uint8ClampedArray over ab
// (v8::Uint8ClampedArray::new).
func NewUint8ClampedArray(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*TypedArray, error) {
	return NewTypedArrayOfKind(s, c, ab, KindUint8Clamped, byteOffset, length)
}

// NewInt16Array creates an Int16Array over ab. byteOffset must be a multiple
// of 2 (v8::Int16Array::new).
func NewInt16Array(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*TypedArray, error) {
	return NewTypedArrayOfKind(s, c, ab, KindInt16, byteOffset, length)
}

// NewUint16Array creates a Uint16Array over ab. byteOffset must be a multiple
// of 2 (v8::Uint16Array::new).
func NewUint16Array(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*TypedArray, error) {
	return NewTypedArrayOfKind(s, c, ab, KindUint16, byteOffset, length)
}

// NewInt32Array creates an Int32Array over ab. byteOffset must be a multiple
// of 4 (v8::Int32Array::new).
func NewInt32Array(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*TypedArray, error) {
	return NewTypedArrayOfKind(s, c, ab, KindInt32, byteOffset, length)
}

// NewUint32Array creates a Uint32Array over ab. byteOffset must be a multiple
// of 4 (v8::Uint32Array::new).
func NewUint32Array(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*TypedArray, error) {
	return NewTypedArrayOfKind(s, c, ab, KindUint32, byteOffset, length)
}

// NewFloat16Array creates a Float16Array over ab (IEEE binary16 elements).
// byteOffset must be a multiple of 2 (v8::Float16Array::new; the pinned build
// ships js_float16array on).
func NewFloat16Array(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*TypedArray, error) {
	return NewTypedArrayOfKind(s, c, ab, KindFloat16, byteOffset, length)
}

// NewFloat32Array creates a Float32Array over ab. byteOffset must be a
// multiple of 4 (v8::Float32Array::new).
func NewFloat32Array(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*TypedArray, error) {
	return NewTypedArrayOfKind(s, c, ab, KindFloat32, byteOffset, length)
}

// NewBigUint64Array creates a BigUint64Array over ab. byteOffset must be a
// multiple of 8 (v8::BigUint64Array::new).
func NewBigUint64Array(s *Scope, c *Context, ab *ArrayBuffer, byteOffset, length int) (*TypedArray, error) {
	return NewTypedArrayOfKind(s, c, ab, KindBigUint64, byteOffset, length)
}

// --- ArrayBufferView data / backing-store / live contents -------------------------

// viewDatum is the shared implementation of TypedArray.Data / DataView.Data.
func viewDatum(v Value) (uintptr, bool, error) {
	if err := v.check(); err != nil {
		return 0, false, err
	}
	ih, sh, err := scopeHandles(v.sc, "view.Data")
	if err != nil {
		return 0, false, err
	}
	r1, _, _ := proc("gov8_view_data").Call(ih, sh, v.h)
	if int64(r1) < 0 {
		return 0, false, shimError("view.Data", r1)
	}
	return uintptr(r1), r1 != 0, nil
}

// Data returns the view's engine-side data pointer (crate data():
// buffer data + byte offset). The second result reports whether the pointer
// is non-null (it is null for detached views). OBSERVATION ONLY: the pointer
// is valid only while the view is alive, must never be dereferenced or
// retained by Go, and becomes invalid when the scope closes.
func (ta *TypedArray) Data() (uintptr, bool, error) { return viewDatum(ta.Value) }

// Data returns the view's engine-side data pointer; see TypedArray.Data.
func (dv *DataView) Data() (uintptr, bool, error) { return viewDatum(dv.Value) }

// viewBackingStore is the shared implementation of TypedArray /
// DataView.GetBackingStore.
func viewBackingStore(v Value) (*BackingStore, error) {
	if err := v.check(); err != nil {
		return nil, err
	}
	ih, sh, err := scopeHandles(v.sc, "view.GetBackingStore")
	if err != nil {
		return nil, err
	}
	var out uintptr
	r1, _, _ := proc("gov8_view_get_backing_store").Call(ih, sh, v.h, uintptr(unsafe.Pointer(&out)))
	if int64(r1) < 0 {
		return nil, shimError("view.GetBackingStore", r1)
	}
	return v.iso.backingStore(out)
}

// GetBackingStore returns a NEW counted reference to the view buffer's
// backing store (crate get_backing_store). The caller must Close it. Works
// for SharedArrayBuffer-backed views as well (IsShared reports true there).
func (ta *TypedArray) GetBackingStore() (*BackingStore, error) {
	return viewBackingStore(ta.Value)
}

// GetBackingStore returns a NEW counted reference to the DataView buffer's
// backing store; see TypedArray.GetBackingStore.
func (dv *DataView) GetBackingStore() (*BackingStore, error) {
	return viewBackingStore(dv.Value)
}

// ViewContents describes the engine's live contents span of a view
// (ArrayBufferView::GetContents).
type ViewContents struct {
	// Length is the span's full length in bytes. For this build it is always
	// the view's byte length and independent of the caller's storage size
	// (the off-heap GetContents path ignores the storage argument).
	Length int
	// Source is the span's base address. OBSERVATION ONLY (same rules as
	// TypedArray.Data): never dereference or retain it.
	Source uintptr
}

// SourceIsData reports whether the live span's base is the view's data
// pointer (the off-heap aliasing contract pinned by the oracle). A null
// source (detached view) never matches.
func (vc ViewContents) SourceIsData(data uintptr) bool {
	return vc.Source != 0 && vc.Source == data
}

// viewGetContents is the shared implementation of TypedArray /
// DataView.GetContents. The engine fills up to len(storage) bytes of storage
// and reports the full live span.
func viewGetContents(v Value, storage []byte) (ViewContents, error) {
	if err := v.check(); err != nil {
		return ViewContents{}, err
	}
	ih, sh, err := scopeHandles(v.sc, "view.GetContents")
	if err != nil {
		return ViewContents{}, err
	}
	var p uintptr
	if len(storage) > 0 {
		p = uintptr(unsafe.Pointer(&storage[0]))
	}
	var fullLen int64
	var src uintptr
	r1, _, _ := proc("gov8_view_get_contents").Call(ih, sh, v.h, p,
		uintptr(len(storage)), uintptr(unsafe.Pointer(&fullLen)),
		uintptr(unsafe.Pointer(&src)))
	if int64(r1) < 0 {
		return ViewContents{}, shimError("view.GetContents", r1)
	}
	return ViewContents{Length: int(fullLen), Source: src}, nil
}

// GetContents copies up to len(storage) bytes of the view's live contents
// into storage and describes the full span (ArrayBufferView::GetContents).
// The reported Length is independent of len(storage): for this build the
// contents are always off-heap and the engine ignores the storage size. The
// bytes in storage reflect the live backing store at call time — re-read
// after JS writes to observe the writes.
func (ta *TypedArray) GetContents(storage []byte) (ViewContents, error) {
	return viewGetContents(ta.Value, storage)
}

// GetContents copies up to len(storage) bytes of the DataView's live
// contents into storage and describes the full span; see TypedArray.GetContents.
func (dv *DataView) GetContents(storage []byte) (ViewContents, error) {
	return viewGetContents(dv.Value, storage)
}

// CopyContents copies at most len(dst) bytes of the DataView's contents into
// dst (ArrayBufferView::CopyContents) and returns the number of bytes
// written. The copy includes the byte offset: a DataView at offset 3 copies
// buffer bytes 3..3+min(len(dst), byteLength).
func (dv *DataView) CopyContents(dst []byte) (int, error) {
	if err := dv.check(); err != nil {
		return 0, err
	}
	ih, sh, err := scopeHandles(dv.sc, "DataView.CopyContents")
	if err != nil {
		return 0, err
	}
	var p uintptr
	if len(dst) > 0 {
		p = uintptr(unsafe.Pointer(&dst[0]))
	}
	r1, _, _ := proc("gov8_view_copy_contents").Call(ih, sh, dv.h, p, uintptr(len(dst)))
	if int64(r1) < 0 {
		return 0, shimError("DataView.CopyContents", r1)
	}
	return int(r1), nil
}

// --- pinned per-kind limits -----------------------------------------------------

// TypedArrayKindLimits carries the pinned build's typed-array size limits
// for every kind (X::MAX_LENGTH in the crate).
type TypedArrayKindLimits struct {
	// MaxLengths maps each kind to its maximum element count
	// (TypedArray::kMaxByteLength / element size, truncated).
	MaxLengths map[TypedArrayKind]int64
	// MaxByteLength is the largest supported typed-array byte size
	// (2^53-1 for this build).
	MaxByteLength int64
	// MaxSizeInHeap is the pinned artifact's on-heap typed-array size
	// threshold (0: this build never stores typed arrays on the JS heap).
	MaxSizeInHeap int64
}

// TypedArrayKindLimitsQuery reads the pinned build's limits for all 12 kinds
// from the engine shim (v8-typed-array.h kMaxLength constants,
// TypedArray::kMaxByteLength and the GN-arg-pinned heap threshold).
func TypedArrayKindLimitsQuery() (TypedArrayKindLimits, error) {
	var out [12]int64
	var maxByteLength, maxSizeInHeap int64
	r1, _, _ := proc("gov8_typed_array_limits_all").Call(
		uintptr(unsafe.Pointer(&out[0])),
		uintptr(unsafe.Pointer(&maxByteLength)),
		uintptr(unsafe.Pointer(&maxSizeInHeap)))
	if int64(r1) < 0 {
		return TypedArrayKindLimits{}, shimError("TypedArrayKindLimits", r1)
	}
	limits := TypedArrayKindLimits{
		MaxLengths:    make(map[TypedArrayKind]int64, 12),
		MaxByteLength: maxByteLength,
		MaxSizeInHeap: maxSizeInHeap,
	}
	for idx, kind := range TypedArrayKinds {
		limits.MaxLengths[kind] = out[idx]
	}
	return limits, nil
}
