// ===========================================================================
// typed_arrays.inc — the complete 12-kind typed-array surface: per-kind value
// predicates, the kind-parameterized constructor with full CHECK-boundary
// prevalidation, the ArrayBufferView data / backing-store / live-contents
// exports, and the per-kind size-limit constants (feature slice).
//
// This file is textually included at the end of shim.cc, AFTER
// buffers_serialization.inc: every helper and wrapper type from shim.cc and
// namespace gov8_buf_impl (ToWire/FromWire/AsCtx/AsScope/ScopeIs/OwnedBy/
// ClearErr/SetErr, the kErr* status codes, BsWrap/BsNew/AsBs, ViewPtrOf,
// PtrToLocal, and the flat-binding declarations of buffers_serialization.inc)
// is already in scope. Exports below re-open `extern "C"`.
//
// ---------------------------------------------------------------------------
// What this slice adds over buffers_serialization.inc
//
// The buffers slice bound only the three per-kind constructors its checks
// needed (Uint8 / Float64 / BigInt64) and prevalidated alignment and bounds.
// This slice completes the kind matrix and closes the remaining fatal
// boundaries:
//
//   - all 12 kinds of engine EACH_TYPED_ARRAY (support.h), including
//     Float16Array (kind 11) and BigUint64Array, through ONE
//     kind-parameterized constructor export;
//   - per-kind MAX-length prevalidation: X::New runs Utils::ApiCheck
//     ("length exceeds max allowed value", engine src/api.cc TYPED_ARRAY_NEW)
//     BEFORE the factory CHECKs, so a length above X::kMaxLength is another
//     process-fatal boundary the Go binding must never reach;
//   - one value predicate export per kind (v8__Value__Is*Array of the pinned
//     artifact's binding.cc — including IsFloat16Array);
//   - the ArrayBufferView data/backing-store/contents surface of crate
//     src/array_buffer_view.rs: data() (= Buffer()->Data() + ByteOffset(),
//     null for detached views), get_backing_store() (a NEW shared reference
//     handed to Go; released by gov8_backing_store_dispose), and
//     get_contents() (engine api.cc ArrayBufferView::GetContents: off-heap
//     views return a live span over the backing store and IGNORE the caller's
//     storage size; with TYPED_ARRAY_MAX_SIZE_IN_HEAP = 0 in the pinned build
//     nothing is ever on the V8 heap).
//
// Kind wire values (same numbering as buffers_serialization.inc ViewKindOf,
// extended by kind 11): 0 Uint8, 1 Int8, 2 Uint16, 3 Int16, 4 Uint32, 5 Int32,
// 6 Float32, 7 Float64, 8 BigInt64, 9 BigUint64, 10 Uint8Clamped, 11 Float16.
//
// ---------------------------------------------------------------------------
// Prevalidation order (mirrors the engine's fatal order exactly)
//
//   1. ApiCheck: length > X::kMaxLength      -> "length exceeds max allowed
//                                               value" fragment preserved
//   2. Factory CHECK: byte_offset % size     -> alignment error
//   3. Factory CHECKs: byte_offset <= len,
//      length*size <= len - byte_offset      -> bounds error
//
// Each invalid geometry therefore becomes a kErrBadArg with a deterministic
// detail string instead of a V8 abort; the Go negative tests pin every case
// characterized out-of-process by rust-oracle/tests/typed_arrays_negative.rs.
// JS-side misuse keeps producing JS RangeErrors (never mapped onto these
// native-shape errors), exactly as the pinned fixture requires.
// ===========================================================================

#include <algorithm>
#include <cstring>

namespace gov8_ta_impl {

// The artifact's memory_span_t (crate src/support.h): two machine words,
// passed and returned by value across the flat binding.
struct MemorySpan {
  uint8_t* data;
  size_t size;
};
static_assert(sizeof(MemorySpan) == 2 * sizeof(void*), "MemorySpan layout");

// Per-kind X::kMaxLength of the pinned headers (v8-typed-array.h:
// TypedArray::kMaxByteLength / sizeof(element)). Returns 0 for unknown kinds.
int64_t TypedArrayKindMaxLength(int64_t kind) {
  switch (kind) {
    case 0: return static_cast<int64_t>(v8::Uint8Array::kMaxLength);
    case 1: return static_cast<int64_t>(v8::Int8Array::kMaxLength);
    case 2: return static_cast<int64_t>(v8::Uint16Array::kMaxLength);
    case 3: return static_cast<int64_t>(v8::Int16Array::kMaxLength);
    case 4: return static_cast<int64_t>(v8::Uint32Array::kMaxLength);
    case 5: return static_cast<int64_t>(v8::Int32Array::kMaxLength);
    case 6: return static_cast<int64_t>(v8::Float32Array::kMaxLength);
    case 7: return static_cast<int64_t>(v8::Float64Array::kMaxLength);
    case 8: return static_cast<int64_t>(v8::BigInt64Array::kMaxLength);
    case 9: return static_cast<int64_t>(v8::BigUint64Array::kMaxLength);
    case 10: return static_cast<int64_t>(v8::Uint8ClampedArray::kMaxLength);
    case 11: return static_cast<int64_t>(v8::Float16Array::kMaxLength);
    default: return 0;
  }
}

}  // namespace gov8_ta_impl

// --- flat bindings exported by the pinned artifact ---------------------------
//
// Declared exactly as the crate declares them (v8-152.2.0/src/binding.cc,
// generated over EACH_TYPED_ARRAY; src/value.rs, src/array_buffer_view.rs).
extern "C" {
// Per-kind value predicates (12 of the 12 kinds; Float16Array included).
bool v8__Value__IsInt8Array(const v8::Value* self);
bool v8__Value__IsUint8Array(const v8::Value* self);
bool v8__Value__IsUint8ClampedArray(const v8::Value* self);
bool v8__Value__IsInt16Array(const v8::Value* self);
bool v8__Value__IsUint16Array(const v8::Value* self);
bool v8__Value__IsInt32Array(const v8::Value* self);
bool v8__Value__IsUint32Array(const v8::Value* self);
bool v8__Value__IsFloat16Array(const v8::Value* self);
bool v8__Value__IsFloat32Array(const v8::Value* self);
bool v8__Value__IsFloat64Array(const v8::Value* self);
bool v8__Value__IsBigInt64Array(const v8::Value* self);
bool v8__Value__IsBigUint64Array(const v8::Value* self);

// Float16Array construction (the other 11 constructors are already declared
// by buffers_serialization.inc; redundant redeclaration is well-formed).
const v8::Float16Array* v8__Float16Array__New(const v8::ArrayBuffer* buf,
                                              size_t byte_offset,
                                              size_t length);

// ArrayBufferView data/contents (crate array_buffer_view.rs). data() is
// Buffer()->Data() + ByteOffset(); GetContents takes the caller's storage
// span and returns the live contents span.
const void* v8__ArrayBufferView__Buffer__Data(const v8::ArrayBufferView* self);
gov8_ta_impl::MemorySpan v8__ArrayBufferView__GetContents(
    const v8::ArrayBufferView* self, gov8_ta_impl::MemorySpan storage);
}

extern "C" {

// --- per-kind value predicates -------------------------------------------------

#define GOV8_TA_PREDICATE(name, expr)                                        \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* v) {            \
    ClearErr();                                                              \
    if (v == nullptr) {                                                      \
      SetErr("null value wire");                                             \
      return kErrBadArg;                                                     \
    }                                                                        \
    try {                                                                    \
      Gov8IsolateScope iso_scope(iso);                                     \
      v8::HandleScope hs(iso);                                               \
      v8::Local<v8::Value> val = FromWire<v8::Value>(v);                     \
      return (expr) ? 1 : 0;                                                 \
    } catch (...) {                                                          \
      SetErr("C++ exception in " #name);                                     \
      return kErrCpp;                                                        \
    }                                                                        \
  }

GOV8_TA_PREDICATE(gov8_is_int8_array, v8__Value__IsInt8Array(*val))
GOV8_TA_PREDICATE(gov8_is_uint8_array, v8__Value__IsUint8Array(*val))
GOV8_TA_PREDICATE(gov8_is_uint8_clamped_array,
                  v8__Value__IsUint8ClampedArray(*val))
GOV8_TA_PREDICATE(gov8_is_int16_array, v8__Value__IsInt16Array(*val))
GOV8_TA_PREDICATE(gov8_is_uint16_array, v8__Value__IsUint16Array(*val))
GOV8_TA_PREDICATE(gov8_is_int32_array, v8__Value__IsInt32Array(*val))
GOV8_TA_PREDICATE(gov8_is_uint32_array, v8__Value__IsUint32Array(*val))
GOV8_TA_PREDICATE(gov8_is_float16_array, v8__Value__IsFloat16Array(*val))
GOV8_TA_PREDICATE(gov8_is_float32_array, v8__Value__IsFloat32Array(*val))
GOV8_TA_PREDICATE(gov8_is_float64_array, v8__Value__IsFloat64Array(*val))
GOV8_TA_PREDICATE(gov8_is_big_int64_array, v8__Value__IsBigInt64Array(*val))
GOV8_TA_PREDICATE(gov8_is_big_uint64_array, v8__Value__IsBigUint64Array(*val))

#undef GOV8_TA_PREDICATE

// --- kind-parameterized constructor with full prevalidation ---------------------

// Kind element sizes (bytes) and their alignment classes (element sizes > 1
// require byte_offset % element_size == 0; 1-byte kinds and DataView do not).
size_t TaElementSizeOfKind(int64_t kind) {
  switch (kind) {
    case 0:  // Uint8
    case 1:  // Int8
    case 10: // Uint8Clamped
      return 1;
    case 2:  // Uint16
    case 3:  // Int16
    case 11: // Float16
      return 2;
    case 4:  // Uint32
    case 5:  // Int32
    case 6:  // Float32
      return 4;
    case 7:  // Float64
    case 8:  // BigInt64
    case 9:  // BigUint64
      return 8;
    default:
      return 0;
  }
}

// Constructs a typed array of `kind` over ab's bytes
// [byte_offset, byte_offset + length * element_size). Every geometry that the
// engine would answer with a process-fatal V8 CHECK / ApiCheck is rejected
// here as kErrBadArg with a deterministic detail (see the file header for the
// order, which mirrors the engine's fatal order).
__declspec(dllexport) int64_t gov8_typed_array_new_kind(
    v8::Isolate* iso, void* ctxw, void* scope, void* ab_wire, int64_t kind,
    int64_t byte_offset, int64_t length, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (out == nullptr || ab_wire == nullptr || w == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  const size_t element_size = TaElementSizeOfKind(kind);
  if (element_size == 0) {
    SetErr("unknown typed array kind");
    return kErrBadArg;
  }
  if (byte_offset < 0 || length < 0) {
    SetErr("negative view geometry");
    return kErrBadArg;
  }
  // 1. ApiCheck (engine api.cc TYPED_ARRAY_NEW, before any factory CHECK):
  //    length must not exceed the kind's kMaxLength. The comparison is
  //    overflow-free (both operands non-negative int64).
  if (static_cast<int64_t>(length) > gov8_ta_impl::TypedArrayKindMaxLength(kind)) {
    SetErr("typed array length exceeds max allowed value");
    return kErrBadArg;
  }
  // 2. Factory CHECK (Factory::NewJSTypedArray): byte_offset must be a
  //    multiple of the element size. Zero-length views are NOT exempt.
  if (element_size > 1 &&
      static_cast<size_t>(byte_offset) % element_size != 0) {
    SetErr("typed array byte_offset not aligned to element size");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::EscapableHandleScope escapable(iso);
    v8::Local<v8::ArrayBuffer> ab = FromWire<v8::ArrayBuffer>(ab_wire);
    if (!v8__Value__IsArrayBuffer(*ab)) {
      SetErr("not an ArrayBuffer");
      return kErrBadArg;
    }
    const size_t store_len = v8__ArrayBuffer__ByteLength(*ab);
    const size_t off = static_cast<size_t>(byte_offset);
    const size_t elems = static_cast<size_t>(length);
    // 3. Factory CHECKs (Factory::NewJSArrayBufferView), overflow-free form:
    //    off <= store_len && elems * element_size <= store_len - off.
    if (off > store_len || elems > (store_len - off) / element_size) {
      SetErr("typed array view out of bounds");
      return kErrBadArg;
    }
    const void* result = nullptr;
    switch (kind) {
      case 0: result = v8__Uint8Array__New(*ab, off, elems); break;
      case 1: result = v8__Int8Array__New(*ab, off, elems); break;
      case 2: result = v8__Uint16Array__New(*ab, off, elems); break;
      case 3: result = v8__Int16Array__New(*ab, off, elems); break;
      case 4: result = v8__Uint32Array__New(*ab, off, elems); break;
      case 5: result = v8__Int32Array__New(*ab, off, elems); break;
      case 6: result = v8__Float32Array__New(*ab, off, elems); break;
      case 7: result = v8__Float64Array__New(*ab, off, elems); break;
      case 8: result = v8__BigInt64Array__New(*ab, off, elems); break;
      case 9: result = v8__BigUint64Array__New(*ab, off, elems); break;
      case 10: result = v8__Uint8ClampedArray__New(*ab, off, elems); break;
      case 11: result = v8__Float16Array__New(*ab, off, elems); break;
      default:
        SetErr("unknown typed array kind");
        return kErrBadArg;
    }
    if (result == nullptr) {
      SetErr("typed array construction failed");
      return kErrGeneric;
    }
    *out = ToWire(escapable.Escape(
        gov8_buf_impl::PtrToLocal(static_cast<const v8::Value*>(result))));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in typed_array_new_kind");
    return kErrCpp;
  }
}

// --- ArrayBufferView data / backing-store / live contents ------------------------

// The view's data pointer (crate ArrayBufferView::data: Buffer()->Data() +
// ByteOffset()). Null for detached views. The pointer is engine-owned, valid
// only while the view is alive, and must never be dereferenced or retained by
// Go; it exists so Go can observe pointer relationships (e.g. that
// GetContents hands back the data pointer) without copying.
__declspec(dllexport) int64_t gov8_view_data(v8::Isolate* iso, void* scope,
                                             void* wire) {
  ClearErr();
  if (wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> val = FromWire<v8::Value>(wire);
    const v8::ArrayBufferView* view = ViewPtrOf(val);
    if (view == nullptr) {
      return kErrBadArg;
    }
    const void* base = v8__ArrayBufferView__Buffer__Data(view);
    const size_t off = v8__ArrayBufferView__ByteOffset(view);
    return reinterpret_cast<int64_t>(static_cast<const uint8_t*>(base) + off);
  } catch (...) {
    SetErr("C++ exception in view_data");
    return kErrCpp;
  }
}

// A NEW shared reference to the view buffer's backing store (crate
// ArrayBufferView::get_backing_store: buffer()->get_backing_store()). Works
// for SharedArrayBuffer-backed views too (the masquerading Local<ArrayBuffer>
// of a SAB view reaches the same engine path). Go must release the wrapper
// with gov8_backing_store_dispose.
__declspec(dllexport) int64_t gov8_view_get_backing_store(v8::Isolate* iso,
                                                          void* scope,
                                                          void* wire,
                                                          void** out) {
  ClearErr();
  if (out == nullptr || wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> val = FromWire<v8::Value>(wire);
    const v8::ArrayBufferView* view = ViewPtrOf(val);
    if (view == nullptr) {
      return kErrBadArg;
    }
    const v8::ArrayBuffer* buf = v8__ArrayBufferView__Buffer(view);
    if (buf == nullptr) {
      SetErr("view has no buffer");
      return kErrBadArg;
    }
    gov8_buf_impl::BsRep rep = v8__ArrayBuffer__GetBackingStore(buf);
    *out = gov8_buf_impl::BsNew(iso, rep);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in view_get_backing_store");
    return kErrCpp;
  }
}

// ArrayBufferView::GetContents (engine api.cc): returns the live contents
// span (data pointer + length). Off-heap views — the only kind this build can
// produce, TYPED_ARRAY_MAX_SIZE_IN_HEAP = 0 — return a span over the backing
// store and ignore the caller's storage size entirely; on-heap views (if a
// future build allows them) are copied by the engine into storage.
//
// This export copies min(cap, span size) bytes into the caller's buffer so no
// engine pointer escapes to Go, then reports the LIVE span: its full length
// in *out_len (independent of cap) and its base in *out_src (observation
// only: never dereference or retain it).
__declspec(dllexport) int64_t gov8_view_get_contents(v8::Isolate* iso,
                                                     void* scope, void* wire,
                                                     char* buf, int64_t cap,
                                                     int64_t* out_len,
                                                     void** out_src) {
  ClearErr();
  if (out_len == nullptr || out_src == nullptr || wire == nullptr ||
      !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (cap < 0 || (buf == nullptr && cap > 0)) {
    SetErr("invalid storage buffer");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> val = FromWire<v8::Value>(wire);
    const v8::ArrayBufferView* view = ViewPtrOf(val);
    if (view == nullptr) {
      return kErrBadArg;
    }
    gov8_ta_impl::MemorySpan storage{reinterpret_cast<uint8_t*>(buf),
                                     static_cast<size_t>(cap)};
    gov8_ta_impl::MemorySpan span =
        v8__ArrayBufferView__GetContents(view, storage);
    // Off-heap path: the span points into the backing store, storage was not
    // filled — copy the observable prefix out. On-heap path (span == storage,
    // or null span): nothing to copy.
    if (buf != nullptr && span.data != nullptr &&
        span.data != reinterpret_cast<uint8_t*>(buf)) {
      const size_t n = std::min(span.size, static_cast<size_t>(cap));
      if (n > 0) {
        std::memmove(buf, span.data, n);
      }
    }
    *out_len = static_cast<int64_t>(span.size);
    *out_src = span.data;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in view_get_contents");
    return kErrCpp;
  }
}

// --- pinned per-kind size limits ------------------------------------------------

// All 12 per-kind kMaxLength constants in the fixture's kind order
// (int8, uint8, uint8_clamped, int16, uint16, int32, uint32, float16,
// float32, float64, bigint64, biguint64), plus TypedArray::kMaxByteLength
// (= ArrayBuffer::kMaxByteLength = 2^53-1 with the sandbox off) and the
// pinned artifact's on-heap typed-array threshold (GN arg
// v8_typed_array_max_size_in_heap = 0; mirrors gov8_typed_array_limits).
__declspec(dllexport) int64_t gov8_typed_array_limits_all(
    int64_t* out12, int64_t* max_byte_length, int64_t* max_size_in_heap) {
  ClearErr();
  if (out12 == nullptr || max_byte_length == nullptr ||
      max_size_in_heap == nullptr) {
    SetErr("null out params");
    return kErrBadArg;
  }
  out12[0] = static_cast<int64_t>(v8::Int8Array::kMaxLength);
  out12[1] = static_cast<int64_t>(v8::Uint8Array::kMaxLength);
  out12[2] = static_cast<int64_t>(v8::Uint8ClampedArray::kMaxLength);
  out12[3] = static_cast<int64_t>(v8::Int16Array::kMaxLength);
  out12[4] = static_cast<int64_t>(v8::Uint16Array::kMaxLength);
  out12[5] = static_cast<int64_t>(v8::Int32Array::kMaxLength);
  out12[6] = static_cast<int64_t>(v8::Uint32Array::kMaxLength);
  out12[7] = static_cast<int64_t>(v8::Float16Array::kMaxLength);
  out12[8] = static_cast<int64_t>(v8::Float32Array::kMaxLength);
  out12[9] = static_cast<int64_t>(v8::Float64Array::kMaxLength);
  out12[10] = static_cast<int64_t>(v8::BigInt64Array::kMaxLength);
  out12[11] = static_cast<int64_t>(v8::BigUint64Array::kMaxLength);
  *max_byte_length = static_cast<int64_t>(v8::TypedArray::kMaxByteLength);
  *max_size_in_heap = 0;
  return kOk;
}

}  // extern "C"
