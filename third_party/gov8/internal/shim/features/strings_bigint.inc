// ===========================================================================
// Advanced string / external-string / BigInt feature slice.
//
// Characterizes and exports the observable contract pinned by
// rust-oracle/src/bin/conformance-strings-bigint.rs:
//
//   - String creation flavors: Empty, NewFromUtf8 / NewFromOneByte /
//     NewFromTwoByte with NewStringType (Normal / Internalized), the
//     kMaxLength constant, Concat, and the representation predicates
//     (IsOneByte / ContainsOnlyOneByte / IsExternal + typed variants).
//   - The Write / WriteOneByte / WriteUtf8 family (the crate's write_v2 /
//     write_one_byte_v2 / write_utf8_v2): caller-owned buffers, low-byte
//     truncation for one-byte writes of two-byte content, UTF-8 capacity
//     truncation that never splits a sequence, kNullTerminate (counts the
//     NUL in the return value) and kReplaceInvalidUtf8 (lone surrogates
//     become U+FFFD). Range validation happens BEFORE the engine is
//     touched: offset/length pairs that would read past the string are
//     rejected with kErrBadArg here and in the Go wrapper (the release
//     engine itself only DCHECKs the range - reaching it out of range is
//     undefined behavior and is never done anywhere in this project).
//   - String::ValueView: construct / describe (encoding + length) / copy
//     under the view's own no-GC scope / destruct. The C++ object is
//     heap-allocated by this shim and released ONLY by the explicit
//     gov8_sb_value_view_destruct call (deterministic Go Close).
//   - External strings, four ownership flavors:
//       * static  (new_external_onebyte/twobyte_static): this shim copies
//         the caller's bytes into shim-owned stable memory that is
//         intentionally never freed (static lifetime); the resource object
//         itself is engine-owned and deleted after finalization (default
//         Dispose), exactly like the pinned binding's static classes.
//       * const   (OneByteConst / new_from_onebyte_const): one shim-owned
//         resource with a no-op Dispose shared by any number of isolates
//         (process lifetime, mirroring the crate's &'static OneByteConst).
//       * owned   (new_external_onebyte/twobyte): shim copies; the copy is
//         freed by the resource destructor when V8 finalizes the string.
//       * raw     (new_external_onebyte/twobyte_raw with custom destructor):
//         shim copies prefixed with an int64 registry id; when V8 destroys
//         the string the fixed release function notifies the registered Go
//         trampoline (id, data pointer, length) exactly once and then frees
//         the copy. No Go pointer is ever retained by the engine: every
//         byte buffer handed across the boundary is copied first, and Go
//         only ever sees integers.
//   - Resource getters: base (with the C++ Encoding out param), generic
//     (two-byte-typed; null for one-byte externals in this pinned build)
//     and the one-byte getter (base + Encoding==ONE_BYTE cast, null
//     otherwise), each echoing the resource bytes through the virtual
//     data()/length() accessors for the one-byte flavor.
//   - BigInt: Uint64Value with lossless flag, WordCount, NewFromWords
//     (context-entered; over-i::BigInt::kMaxLength word counts return an
//     empty maybe with a pending JS RangeError, reported as kErrException
//     so the caller's TryCatch observes it) and ToWordsArray with the
//     engine's truncate-to-capacity semantics.
// ===========================================================================

#include <cstdint>
#include <cstdlib>
#include <cstring>

namespace gov8_sb_impl {

// The single Go trampoline notified when a raw external string's buffer is
// released (registered once via gov8_sb_external_deleter_set_entry).
using GoSbDeleterEntry = void (*)(int64_t id, uintptr_t data, uintptr_t len);
GoSbDeleterEntry g_sb_deleter_entry = nullptr;

// Releases an id-prefixed raw copy: notifies Go exactly once with the
// payload pointer and length, then frees the whole allocation. The int64
// id header sits immediately before the payload (malloc on x64 returns
// 16-aligned blocks, so both the header and a uint16 payload stay
// naturally aligned).
void Gov8SbRawRelease(const void* data, size_t len) {
  unsigned char* base =
      reinterpret_cast<unsigned char*>(const_cast<void*>(data)) -
      sizeof(int64_t);
  const int64_t id = *reinterpret_cast<const int64_t*>(base);
  if (g_sb_deleter_entry != nullptr) {
    g_sb_deleter_entry(id, reinterpret_cast<uintptr_t>(data),
                       static_cast<uintptr_t>(len));
  }
  free(base);
}

// Allocates an id-prefixed copy of src (8-byte header + len*unit bytes).
// Returns the payload pointer (base + 8) or nullptr on allocation failure.
void* Gov8SbRawCopy(const void* src, int64_t len, int64_t id, size_t unit) {
  if (len < 0 || (src == nullptr && len > 0)) {
    return nullptr;
  }
  const size_t payload = static_cast<size_t>(len) * unit;
  unsigned char* base =
      static_cast<unsigned char*>(malloc(sizeof(int64_t) + payload));
  if (base == nullptr) {
    return nullptr;
  }
  *reinterpret_cast<int64_t*>(base) = id;
  if (payload > 0) {
    memcpy(base + sizeof(int64_t), src, payload);
  }
  return base + sizeof(int64_t);
}

// Copies the caller's bytes into shim-owned memory that intentionally
// outlives every isolate (static external strings). Returns the copy or
// nullptr.
void* Gov8SbStaticCopy(const void* src, int64_t len, size_t unit) {
  if (len < 0 || (src == nullptr && len > 0)) {
    return nullptr;
  }
  const size_t bytes = static_cast<size_t>(len) * unit;
  void* copy = malloc(bytes ? bytes : 1);
  if (copy == nullptr) {
    return nullptr;
  }
  if (bytes > 0) {
    memcpy(copy, src, bytes);
  }
  return copy;
}

// Owned one-byte resource: the copy is freed when V8 finalizes the string
// (destructor runs from the default Dispose => delete this path).
class Gov8OwnedOneByteResource
    : public v8::String::ExternalOneByteStringResource {
 public:
  Gov8OwnedOneByteResource(char* data, size_t length, v8::Isolate* isolate)
      : data_(data), length_(length), isolate_(isolate) {
    isolate_->AdjustAmountOfExternalAllocatedMemory(
        static_cast<int64_t>(length_));
  }
  ~Gov8OwnedOneByteResource() override {
    free(data_);
    isolate_->AdjustAmountOfExternalAllocatedMemory(
        -static_cast<int64_t>(length_));
  }
  const char* data() const override { return data_; }
  size_t length() const override { return length_; }

 private:
  char* data_;
  const size_t length_;
  v8::Isolate* isolate_;
};

// Owned two-byte counterpart.
class Gov8OwnedTwoByteResource : public v8::String::ExternalStringResource {
 public:
  Gov8OwnedTwoByteResource(uint16_t* data, size_t length,
                           v8::Isolate* isolate)
      : data_(data), length_(length), isolate_(isolate) {
    isolate_->AdjustAmountOfExternalAllocatedMemory(
        static_cast<int64_t>(length_) * 2);
  }
  ~Gov8OwnedTwoByteResource() override {
    free(data_);
    isolate_->AdjustAmountOfExternalAllocatedMemory(
        -static_cast<int64_t>(length_) * 2);
  }
  const uint16_t* data() const override { return data_; }
  size_t length() const override { return length_; }

 private:
  uint16_t* data_;
  const size_t length_;
  v8::Isolate* isolate_;
};

// Raw one-byte resource: destructor notifies Go exactly once (via the
// registered trampoline) and frees the id-prefixed copy.
class Gov8RawOneByteResource
    : public v8::String::ExternalOneByteStringResource {
 public:
  Gov8RawOneByteResource(char* data, size_t length, v8::Isolate* isolate)
      : data_(data), length_(length), isolate_(isolate) {
    isolate_->AdjustAmountOfExternalAllocatedMemory(
        static_cast<int64_t>(length_));
  }
  ~Gov8RawOneByteResource() override {
    Gov8SbRawRelease(data_, length_);
    isolate_->AdjustAmountOfExternalAllocatedMemory(
        -static_cast<int64_t>(length_));
  }
  const char* data() const override { return data_; }
  size_t length() const override { return length_; }

 private:
  char* data_;
  const size_t length_;
  v8::Isolate* isolate_;
};

// Raw two-byte counterpart.
class Gov8RawTwoByteResource : public v8::String::ExternalStringResource {
 public:
  Gov8RawTwoByteResource(uint16_t* data, size_t length, v8::Isolate* isolate)
      : data_(data), length_(length), isolate_(isolate) {
    isolate_->AdjustAmountOfExternalAllocatedMemory(
        static_cast<int64_t>(length_) * 2);
  }
  ~Gov8RawTwoByteResource() override {
    Gov8SbRawRelease(data_, length_);
    isolate_->AdjustAmountOfExternalAllocatedMemory(
        -static_cast<int64_t>(length_) * 2);
  }
  const uint16_t* data() const override { return data_; }
  size_t length() const override { return length_; }

 private:
  uint16_t* data_;
  const size_t length_;
  v8::Isolate* isolate_;
};

// Static one-byte resource wrapping shim-owned stable data. The resource
// object is deleted by V8 after finalization (default Dispose); the data
// copy is intentionally process-lifetime.
class Gov8StaticOneByteResource
    : public v8::String::ExternalOneByteStringResource {
 public:
  Gov8StaticOneByteResource(const char* data, size_t length)
      : data_(data), length_(length) {}
  const char* data() const override { return data_; }
  size_t length() const override { return length_; }

 private:
  const char* data_;
  const size_t length_;
};

// Static two-byte counterpart.
class Gov8StaticTwoByteResource : public v8::String::ExternalStringResource {
 public:
  Gov8StaticTwoByteResource(const uint16_t* data, size_t length)
      : data_(data), length_(length) {}
  const uint16_t* data() const override { return data_; }
  size_t length() const override { return length_; }

 private:
  const uint16_t* data_;
  const size_t length_;
};

// Const one-byte resource (the Go analog of the crate's &'static
// OneByteConst): created once, shared by any number of isolates, never
// disposed and never deleted (Dispose is a no-op so V8 cannot destroy a
// resource another isolate's external string still references).
class Gov8ConstOneByteResource
    : public v8::String::ExternalOneByteStringResource {
 public:
  Gov8ConstOneByteResource(const char* data, size_t length)
      : data_(data), length_(length) {}
  const char* data() const override { return data_; }
  size_t length() const override { return length_; }
  void Dispose() override {}

 private:
  const char* data_;
  const size_t length_;
};

// Validates that the wire holds a JS string before any String-typed
// dereference (a mis-typed wire must fail with kErrBadArg, not UB).
bool WireIsString(v8::Isolate* iso, void* wire) {
  v8::HandleScope hs(iso);
  return FromWire<v8::Value>(wire)->IsString();
}

}  // namespace gov8_sb_impl

extern "C" {

// --- constants -----------------------------------------------------------------

__declspec(dllexport) int64_t gov8_sb_string_max_length(void) {
  ClearErr();
  return v8::String::kMaxLength;
}

// --- creation ------------------------------------------------------------------

__declspec(dllexport) void* gov8_sb_string_empty(v8::Isolate* iso,
                                                 void* scope) {
  ClearErr();
  if (iso == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return ToWire(v8::String::Empty(iso));
  } catch (...) {
    SetErr("C++ exception");
    return nullptr;
  }
}

// Shared body for the three creation entry points. new_type: 0 = kNormal,
// 1 = kInternalized. An empty maybe (input longer than kMaxLength) is a
// recoverable kErrGeneric with no pending exception (the C++ API rejects
// over-long inputs before allocating and does not throw).
#define GOV8_SB_STRING_NEW(name, ctype, v8call)                               \
  __declspec(dllexport) int64_t name(                                         \
      v8::Isolate * iso, void* scope, const ctype* data, int64_t len,         \
      int32_t new_type, void** out) {                                         \
    ClearErr();                                                               \
    if (iso == nullptr || out == nullptr || !ScopeIs(iso, scope) ||           \
        len < 0 || (data == nullptr && len > 0)) {                            \
      SetErr("invalid argument");                                             \
      return kErrBadArg;                                                      \
    }                                                                         \
    if (new_type != 0 && new_type != 1) {                                     \
      SetErr("invalid NewStringType");                                        \
      return kErrBadArg;                                                      \
    }                                                                         \
    try {                                                                     \
      Gov8IsolateScope iso_scope(iso);                                        \
      const v8::NewStringType type =                                          \
          new_type == 1 ? v8::NewStringType::kInternalized                     \
                        : v8::NewStringType::kNormal;                         \
      v8::MaybeLocal<v8::String> maybe =                                      \
          v8call(iso, data, type, static_cast<int>(len));                     \
      v8::Local<v8::String> s;                                               \
      if (!maybe.ToLocal(&s)) {                                              \
        SetErr("String creation failed (input longer than kMaxLength?)");     \
        return kErrGeneric;                                                   \
      }                                                                       \
      *out = ToWire(s);                                                       \
      return kOk;                                                             \
    } catch (...) {                                                           \
      SetErr("C++ exception");                                                \
      return kErrCpp;                                                         \
    }                                                                         \
  }

GOV8_SB_STRING_NEW(gov8_sb_string_new_from_utf8, char,
                   v8::String::NewFromUtf8)
GOV8_SB_STRING_NEW(gov8_sb_string_new_from_one_byte, uint8_t,
                   v8::String::NewFromOneByte)
GOV8_SB_STRING_NEW(gov8_sb_string_new_from_two_byte, uint16_t,
                   v8::String::NewFromTwoByte)

#undef GOV8_SB_STRING_NEW

__declspec(dllexport) int64_t gov8_sb_string_concat(v8::Isolate* iso,
                                                    void* scope, void* left,
                                                    void* right, void** out) {
  ClearErr();
  if (iso == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !gov8_sb_impl::WireIsString(iso, left) ||
      !gov8_sb_impl::WireIsString(iso, right)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::String> result = v8::String::Concat(
        iso, FromWire<v8::String>(left), FromWire<v8::String>(right));
    *out = ToWire(result);
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

// --- predicates (0/1) -------------------------------------------------------------

#define GOV8_SB_SPRED(name, expr)                                            \
  __declspec(dllexport) int64_t name(v8::Isolate * iso, void* v) {           \
    ClearErr();                                                              \
    if (iso == nullptr || !gov8_sb_impl::WireIsString(iso, v)) {              \
      SetErr("value is not a string");                                       \
      return kErrBadArg;                                                     \
    }                                                                        \
    try {                                                                    \
      Gov8IsolateScope iso_scope(iso);                                      \
      v8::HandleScope hs(iso);                                               \
      v8::Local<v8::String> s = FromWire<v8::String>(v);                     \
      return (expr) ? 1 : 0;                                                 \
    } catch (...) {                                                          \
      SetErr("C++ exception");                                               \
      return kErrCpp;                                                        \
    }                                                                        \
  }

GOV8_SB_SPRED(gov8_sb_string_is_one_byte, s->IsOneByte())
GOV8_SB_SPRED(gov8_sb_string_contains_only_one_byte, s->ContainsOnlyOneByte())
GOV8_SB_SPRED(gov8_sb_string_is_external_onebyte, s->IsExternalOneByte())
GOV8_SB_SPRED(gov8_sb_string_is_external_twobyte, s->IsExternalTwoByte())
GOV8_SB_SPRED(gov8_sb_string_is_external,
              s->IsExternalOneByte() || s->IsExternalTwoByte())

#undef GOV8_SB_SPRED

// --- writes ------------------------------------------------------------------------

// Range-checked Write: offset + length must stay within the string, and
// the buffer must hold the units plus the NUL when kNullTerminate is set
// (the release engine only DCHECKs the string range and never checks the
// buffer; out-of-range is UB and never reached from Go).
#define GOV8_SB_WRITE(name, ctype, method)                                   \
  __declspec(dllexport) int64_t name(                                        \
      v8::Isolate * iso, void* v, uint32_t offset, uint32_t length,          \
      ctype* buf, int64_t bufcap, int32_t flags) {                           \
    ClearErr();                                                              \
    if (iso == nullptr || (buf == nullptr && bufcap > 0) || bufcap < 0 ||    \
        !gov8_sb_impl::WireIsString(iso, v)) {                                \
      SetErr("invalid argument");                                            \
      return kErrBadArg;                                                     \
    }                                                                        \
    if ((flags & ~1) != 0) {                                                 \
      SetErr("invalid WriteFlags");                                          \
      return kErrBadArg;                                                     \
    }                                                                        \
    try {                                                                    \
      Gov8IsolateScope iso_scope(iso);                                      \
      v8::HandleScope hs(iso);                                               \
      v8::Local<v8::String> s = FromWire<v8::String>(v);                     \
      const uint32_t len = static_cast<uint32_t>(s->Length());               \
      if (offset > len || length > len - offset) {                            \
        SetErr("write range out of bounds");                                  \
        return kErrBadArg;                                                    \
      }                                                                       \
      const int64_t need =                                                    \
          static_cast<int64_t>(length) + ((flags & 1) != 0 ? 1 : 0);          \
      if (need > bufcap) {                                                    \
        SetErr("write buffer too small (null terminator included)");          \
        return kErrBadArg;                                                    \
      }                                                                       \
      s->method(iso, offset, length, buf, static_cast<int>(flags));           \
      return kOk;                                                             \
    } catch (...) {                                                          \
      SetErr("C++ exception");                                               \
      return kErrCpp;                                                        \
    }                                                                        \
  }

GOV8_SB_WRITE(gov8_sb_string_write, uint16_t, Write)
GOV8_SB_WRITE(gov8_sb_string_write_one_byte, uint8_t, WriteOneByte)

#undef GOV8_SB_WRITE

// WriteUtf8 with the full flag set. Returns the byte count written (>= 0).
// kNullTerminate requires capacity >= 1 (the engine's NUL write has no
// capacity check; capacity 0 is an out-of-bounds write, so it is rejected
// here before the engine is touched).
__declspec(dllexport) int64_t gov8_sb_string_write_utf8(
    v8::Isolate* iso, void* v, char* buf, int64_t cap, int32_t flags,
    int64_t* processed) {
  ClearErr();
  if (iso == nullptr || (buf == nullptr && cap > 0) || cap < 0 ||
      processed == nullptr || !gov8_sb_impl::WireIsString(iso, v)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if ((flags & 1) != 0 && cap < 1) {
    SetErr("kNullTerminate requires capacity >= 1");
    return kErrBadArg;
  }
  if (flags & ~3) {
    SetErr("invalid WriteFlags");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::String> s = FromWire<v8::String>(v);
    size_t processed_count = 0;
    const size_t written =
        s->WriteUtf8(iso, buf, static_cast<size_t>(cap),
                     static_cast<int>(flags), &processed_count);
    *processed = static_cast<int64_t>(processed_count);
    return static_cast<int64_t>(written);
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

// --- String::ValueView -------------------------------------------------------------

__declspec(dllexport) void* gov8_sb_value_view_construct(v8::Isolate* iso,
                                                         void* v) {
  ClearErr();
  if (iso == nullptr || !gov8_sb_impl::WireIsString(iso, v)) {
    SetErr("value is not a string");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    // No shim-local HandleScope here: the view stores the string's Local,
    // whose slot lives in the CALLER's open Go scope. This matches the
    // pinned binding exactly and is why the view must be closed before the
    // scope that created it (the Go wrapper documents and enforces the
    // use-after-close direction; the scope-close order is the embedder's
    // contract, identical to the crate's PinScope lifetime rule).
    return new v8::String::ValueView(iso, FromWire<v8::String>(v));
  } catch (...) {
    SetErr("C++ exception");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_sb_value_view_info(void* view,
                                                      int32_t* one_byte,
                                                      int64_t* length) {
  ClearErr();
  if (view == nullptr || one_byte == nullptr || length == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    const v8::String::ValueView* vv =
        static_cast<v8::String::ValueView*>(view);
    *one_byte = vv->is_one_byte() ? 1 : 0;
    *length = static_cast<int64_t>(vv->length());
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

// Copies min(length, cap/unitSize) content units into buf under the view's
// own no-GC scope. Returns the byte count copied (>= 0).
__declspec(dllexport) int64_t gov8_sb_value_view_copy(void* view,
                                                      uint8_t* buf,
                                                      int64_t cap) {
  ClearErr();
  if (view == nullptr || (buf == nullptr && cap > 0) || cap < 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    const v8::String::ValueView* vv =
        static_cast<v8::String::ValueView*>(view);
    const size_t unit = vv->is_one_byte() ? 1 : 2;
    size_t units = static_cast<size_t>(cap) / unit;
    if (units > vv->length()) {
      units = vv->length();
    }
    const size_t bytes = units * unit;
    if (bytes > 0) {
      memcpy(buf, vv->is_one_byte()
                      ? static_cast<const void*>(vv->data8())
                      : static_cast<const void*>(vv->data16()),
             bytes);
    }
    return static_cast<int64_t>(bytes);
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_sb_value_view_destruct(void* view) {
  ClearErr();
  if (view == nullptr) {
    SetErr("invalid view handle");
    return kErrMagic;
  }
  try {
    delete static_cast<v8::String::ValueView*>(view);
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

// --- external strings: static and const ----------------------------------------------

__declspec(dllexport) void* gov8_sb_external_onebyte_static_new(
    v8::Isolate* iso, void* scope, const uint8_t* data, int64_t len) {
  ClearErr();
  if (iso == nullptr || !ScopeIs(iso, scope) || len < 0 ||
      (data == nullptr && len > 0)) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    // The copy is intentionally never freed: static external data outlives
    // every isolate (the resource object itself is engine-owned).
    char* copy = static_cast<char*>(
        gov8_sb_impl::Gov8SbStaticCopy(data, len, 1));
    if (copy == nullptr && len > 0) {
      SetErr("external string allocation failed");
      return nullptr;
    }
    v8::MaybeLocal<v8::String> maybe = v8::String::NewExternalOneByte(
        iso, new gov8_sb_impl::Gov8StaticOneByteResource(
                 copy, static_cast<size_t>(len)));
    v8::Local<v8::String> s;
    if (!maybe.ToLocal(&s)) {
      SetErr("NewExternalOneByte failed");
      return nullptr;
    }
    return ToWire(s);
  } catch (...) {
    SetErr("C++ exception");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_sb_external_twobyte_static_new(
    v8::Isolate* iso, void* scope, const uint16_t* data, int64_t len) {
  ClearErr();
  if (iso == nullptr || !ScopeIs(iso, scope) || len < 0 ||
      (data == nullptr && len > 0)) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    uint16_t* copy = static_cast<uint16_t*>(
        gov8_sb_impl::Gov8SbStaticCopy(data, len, 2));
    if (copy == nullptr && len > 0) {
      SetErr("external string allocation failed");
      return nullptr;
    }
    v8::MaybeLocal<v8::String> maybe = v8::String::NewExternalTwoByte(
        iso, new gov8_sb_impl::Gov8StaticTwoByteResource(
                 copy, static_cast<size_t>(len)));
    v8::Local<v8::String> s;
    if (!maybe.ToLocal(&s)) {
      SetErr("NewExternalTwoByte failed");
      return nullptr;
    }
    return ToWire(s);
  } catch (...) {
    SetErr("C++ exception");
    return nullptr;
  }
}

// Creates the shared const resource (process lifetime, no-op Dispose).
__declspec(dllexport) void* gov8_sb_external_onebyte_const_create(
    const uint8_t* data, int64_t len) {
  ClearErr();
  if (len < 0 || (data == nullptr && len > 0)) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    char* copy =
        static_cast<char*>(gov8_sb_impl::Gov8SbStaticCopy(data, len, 1));
    if (copy == nullptr && len > 0) {
      SetErr("const resource allocation failed");
      return nullptr;
    }
    return new gov8_sb_impl::Gov8ConstOneByteResource(
        copy, static_cast<size_t>(len));
  } catch (...) {
    SetErr("C++ exception");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_sb_external_onebyte_const_string_new(
    v8::Isolate* iso, void* scope, void* resource) {
  ClearErr();
  if (iso == nullptr || !ScopeIs(iso, scope) || resource == nullptr) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::MaybeLocal<v8::String> maybe = v8::String::NewExternalOneByte(
        iso, static_cast<gov8_sb_impl::Gov8ConstOneByteResource*>(resource));
    v8::Local<v8::String> s;
    if (!maybe.ToLocal(&s)) {
      SetErr("NewExternalOneByte failed");
      return nullptr;
    }
    return ToWire(s);
  } catch (...) {
    SetErr("C++ exception");
    return nullptr;
  }
}

// Reads the const resource's data (the as_str analog) with the growth
// protocol: kErrNoMemory reports the required size through *out_len.
__declspec(dllexport) int64_t gov8_sb_external_onebyte_const_data(
    void* resource, char* buf, int64_t cap, int64_t* out_len) {
  ClearErr();
  if (resource == nullptr || out_len == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    const auto* res =
        static_cast<gov8_sb_impl::Gov8ConstOneByteResource*>(resource);
    const std::string text(res->data(), res->length());
    return BufferUtf8(text, buf, cap, out_len);
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

// --- external strings: resource getters ----------------------------------------------

// Base getter: *base is 0 for plain strings; *encoding always carries the
// C++ Encoding value (0x0 two-byte, 0x1 unknown, 0x8 one-byte).
__declspec(dllexport) int64_t gov8_sb_external_string_resource_base(
    v8::Isolate* iso, void* v, uintptr_t* base, int32_t* encoding) {
  ClearErr();
  if (iso == nullptr || base == nullptr || encoding == nullptr ||
      !gov8_sb_impl::WireIsString(iso, v)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::String::Encoding enc = v8::String::Encoding::UNKNOWN_ENCODING;
    v8::String::ExternalStringResourceBase* res =
        FromWire<v8::String>(v)->GetExternalStringResourceBase(&enc);
    *base = reinterpret_cast<uintptr_t>(res);
    *encoding = static_cast<int32_t>(enc);
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

// Generic (two-byte-typed) getter: null for one-byte externals and plain
// strings in this pinned build.
__declspec(dllexport) int64_t gov8_sb_external_string_resource_generic(
    v8::Isolate* iso, void* v, uintptr_t* res) {
  ClearErr();
  if (iso == nullptr || res == nullptr ||
      !gov8_sb_impl::WireIsString(iso, v)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    *res = reinterpret_cast<uintptr_t>(
        FromWire<v8::String>(v)->GetExternalStringResource());
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

// One-byte getter (base + Encoding==ONE_BYTE cast): *res is 0 unless the
// string is an external one-byte string; the resource bytes are echoed
// through the virtual data()/length() accessors with the growth protocol.
__declspec(dllexport) int64_t gov8_sb_external_onebyte_resource(
    v8::Isolate* iso, void* v, uintptr_t* res, char* buf, int64_t cap,
    int64_t* out_len) {
  ClearErr();
  if (iso == nullptr || res == nullptr || out_len == nullptr ||
      !gov8_sb_impl::WireIsString(iso, v)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::String::Encoding enc = v8::String::Encoding::UNKNOWN_ENCODING;
    v8::String::ExternalStringResourceBase* base =
        FromWire<v8::String>(v)->GetExternalStringResourceBase(&enc);
    if (base == nullptr || enc != v8::String::Encoding::ONE_BYTE_ENCODING) {
      *res = 0;
      *out_len = 0;
      return kOk;
    }
    *res = reinterpret_cast<uintptr_t>(base);
    const auto* one =
        static_cast<const v8::String::ExternalOneByteStringResource*>(base);
    const std::string text(one->data(), one->length());
    return BufferUtf8(text, buf, cap, out_len);
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

// --- external strings: owned and raw ---------------------------------------------------

__declspec(dllexport) void* gov8_sb_external_onebyte_owned_new(
    v8::Isolate* iso, void* scope, const uint8_t* data, int64_t len) {
  ClearErr();
  if (iso == nullptr || !ScopeIs(iso, scope) || len < 0 ||
      (data == nullptr && len > 0)) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    char* copy = static_cast<char*>(malloc(len > 0 ? static_cast<size_t>(len)
                                                   : 1));
    if (copy == nullptr) {
      SetErr("external string allocation failed");
      return nullptr;
    }
    if (len > 0) {
      memcpy(copy, data, static_cast<size_t>(len));
    }
    v8::MaybeLocal<v8::String> maybe = v8::String::NewExternalOneByte(
        iso, new gov8_sb_impl::Gov8OwnedOneByteResource(
                 copy, static_cast<size_t>(len), iso));
    v8::Local<v8::String> s;
    if (!maybe.ToLocal(&s)) {
      SetErr("NewExternalOneByte failed");
      return nullptr;
    }
    return ToWire(s);
  } catch (...) {
    SetErr("C++ exception");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_sb_external_twobyte_owned_new(
    v8::Isolate* iso, void* scope, const uint16_t* data, int64_t len) {
  ClearErr();
  if (iso == nullptr || !ScopeIs(iso, scope) || len < 0 ||
      (data == nullptr && len > 0)) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    uint16_t* copy = static_cast<uint16_t*>(
        malloc(len > 0 ? static_cast<size_t>(len) * 2 : 2));
    if (copy == nullptr) {
      SetErr("external string allocation failed");
      return nullptr;
    }
    if (len > 0) {
      memcpy(copy, data, static_cast<size_t>(len) * 2);
    }
    v8::MaybeLocal<v8::String> maybe = v8::String::NewExternalTwoByte(
        iso, new gov8_sb_impl::Gov8OwnedTwoByteResource(
                 copy, static_cast<size_t>(len), iso));
    v8::Local<v8::String> s;
    if (!maybe.ToLocal(&s)) {
      SetErr("NewExternalTwoByte failed");
      return nullptr;
    }
    return ToWire(s);
  } catch (...) {
    SetErr("C++ exception");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_sb_external_deleter_set_entry(
    uintptr_t entry) {
  ClearErr();
  gov8_sb_impl::g_sb_deleter_entry =
      reinterpret_cast<gov8_sb_impl::GoSbDeleterEntry>(entry);
  return kOk;
}

// Raw creations: the id-prefixed copy's payload address is reported
// through *out_data so the caller can echo it against the deleter's
// argument (the pinned pointer-identity observation).
__declspec(dllexport) void* gov8_sb_external_onebyte_raw_new(
    v8::Isolate* iso, void* scope, const uint8_t* data, int64_t len,
    int64_t id, uintptr_t* out_data) {
  ClearErr();
  if (iso == nullptr || !ScopeIs(iso, scope) || len < 0 ||
      (data == nullptr && len > 0) || id == 0 || out_data == nullptr) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    char* payload = static_cast<char*>(
        gov8_sb_impl::Gov8SbRawCopy(data, len, id, 1));
    if (payload == nullptr) {
      SetErr("external string allocation failed");
      return nullptr;
    }
    *out_data = reinterpret_cast<uintptr_t>(payload);
    v8::MaybeLocal<v8::String> maybe = v8::String::NewExternalOneByte(
        iso, new gov8_sb_impl::Gov8RawOneByteResource(
                 payload, static_cast<size_t>(len), iso));
    v8::Local<v8::String> s;
    if (!maybe.ToLocal(&s)) {
      SetErr("NewExternalOneByte failed");
      return nullptr;
    }
    return ToWire(s);
  } catch (...) {
    SetErr("C++ exception");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_sb_external_twobyte_raw_new(
    v8::Isolate* iso, void* scope, const uint16_t* data, int64_t len,
    int64_t id, uintptr_t* out_data) {
  ClearErr();
  if (iso == nullptr || !ScopeIs(iso, scope) || len < 0 ||
      (data == nullptr && len > 0) || id == 0 || out_data == nullptr) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    uint16_t* payload = static_cast<uint16_t*>(
        gov8_sb_impl::Gov8SbRawCopy(data, len, id, 2));
    if (payload == nullptr) {
      SetErr("external string allocation failed");
      return nullptr;
    }
    *out_data = reinterpret_cast<uintptr_t>(payload);
    v8::MaybeLocal<v8::String> maybe = v8::String::NewExternalTwoByte(
        iso, new gov8_sb_impl::Gov8RawTwoByteResource(
                 payload, static_cast<size_t>(len), iso));
    v8::Local<v8::String> s;
    if (!maybe.ToLocal(&s)) {
      SetErr("NewExternalTwoByte failed");
      return nullptr;
    }
    return ToWire(s);
  } catch (...) {
    SetErr("C++ exception");
    return nullptr;
  }
}

// --- BigInt ------------------------------------------------------------------------------

__declspec(dllexport) int64_t gov8_sb_bigint_u64_value(v8::Isolate* iso,
                                                       void* v, uint64_t* out,
                                                       int32_t* lossless) {
  ClearErr();
  if (out == nullptr || lossless == nullptr) {
    SetErr("null out params");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> val = FromWire<v8::Value>(v);
    if (!val->IsBigInt()) {
      SetErr("not a BigInt");
      return kErrBadArg;
    }
    bool is_lossless = false;
    *out = val.As<v8::BigInt>()->Uint64Value(&is_lossless);
    *lossless = is_lossless ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_sb_bigint_word_count(v8::Isolate* iso,
                                                        void* v) {
  ClearErr();
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> val = FromWire<v8::Value>(v);
    if (!val->IsBigInt()) {
      SetErr("not a BigInt");
      return kErrBadArg;
    }
    return val.As<v8::BigInt>()->WordCount();
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

// NewFromWords with the caller's context and optional TryCatch. More than
// i::BigInt::kMaxLength words empties the maybe with a pending JS
// RangeError (BigInt::FromWords64 -> ThrowBigIntTooBig); that is reported
// as kErrException so the caller's TryCatch observes it. Without a caller
// TryCatch a fallback observes (and stops) the exception.
__declspec(dllexport) int64_t gov8_sb_bigint_new_from_words(
    v8::Isolate* iso, void* ctxw, void* scope, void* tcw, int32_t sign,
    const uint64_t* words, int64_t count, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso) || (sign != 0 && sign != 1) || count < 0 ||
      (words == nullptr && count > 0)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (count > 0x7FFFFFFF) {
    SetErr("word count exceeds int32 range");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (tcw != nullptr) {
      TcWrap* tw = AsTc(tcw);
      if (tw == nullptr) {
        SetErr("invalid trycatch handle");
        return kErrMagic;
      }
      if (!OwnedBy(iso, tw->iso)) {
        return kErrBadArg;
      }
      tc = TcOf(tw);
    }
    if (tc == nullptr) tc = fallback.Init(iso);
    v8::MaybeLocal<v8::BigInt> maybe = v8::BigInt::NewFromWords(
        ctx, sign, static_cast<int>(count), words);
    v8::Local<v8::BigInt> big;
    if (!maybe.ToLocal(&big)) {
      SetErr("BigInt::NewFromWords failed");
      return kErrException;
    }
    *out = ToWire(big);
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

// ToWordsArray: *sign receives the sign bit; the caller's capacity goes in
// through cap and the number of words written comes back in *written (the
// engine truncates to capacity and reports what fits).
__declspec(dllexport) int64_t gov8_sb_bigint_to_words_array(
    v8::Isolate* iso, void* v, uint64_t* buf, int64_t cap, int32_t* sign,
    int64_t* written) {
  ClearErr();
  if (sign == nullptr || written == nullptr || cap < 0 ||
      (buf == nullptr && cap > 0)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (cap > 0x7FFFFFFF) {
    SetErr("capacity exceeds int32 range");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> val = FromWire<v8::Value>(v);
    if (!val->IsBigInt()) {
      SetErr("not a BigInt");
      return kErrBadArg;
    }
    int sign_bit = 0;
    int count = static_cast<int>(cap);
    val.As<v8::BigInt>()->ToWordsArray(&sign_bit, &count, buf);
    *sign = sign_bit;
    *written = count;
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

}  // extern "C"
