// ===========================================================================
// buffers_serialization.inc -- ArrayBuffer, SharedArrayBuffer, BackingStore,
// typed arrays / DataView, and ValueSerializer / ValueDeserializer exports
// (feature slice).
//
// This file is textually included at the end of shim.cc, after the anonymous
// namespace with the shared helpers (ToWire/FromWire/AsCtx/AsScope/AsTc/
// ScopeIs/OwnedBy/TcOf/GlobalPlacementNew/FallbackTryCatch/BufferUtf8 and the
// kErr* status codes), so all of those are in scope here. Exports below
// re-open `extern "C"`.
//
// ---------------------------------------------------------------------------
// ABI notes
//
// The pinned artifact's V8 code is compiled by clang against its own libc++.
// Every engine API whose C++ signature contains a std:: type (shared_ptr /
// unique_ptr / pair) is therefore only callable through the artifact's flat
// extern "C" bindings (src/binding.cc of the crate, name-unmangled). This
// file uses those flat bindings for exactly that set:
//
//   - backing-store construction (ArrayBuffer / SharedArrayBuffer flavors,
//     embedder-pointer flavor),
//   - ArrayBuffer/SharedArrayBuffer <-> backing-store attachment and
//     GetBackingStore,
//   - shared_ptr<BackingStore> copy / get / reset / use_count,
//   - ValueSerializer construction, destruction and Release.
//
// A std::shared_ptr<v8::BackingStore> crosses that boundary as the crate's
// SharedPtrBase representation: two machine words passed by value
// (BsRep below). Everything else (plain v8::Locals, Maybe<bool>, primitives)
// goes through the ordinary C++ API from the vendored headers, exactly like
// the rest of the shim -- the layouts match because both sides compile the
// same v8.h.
//
// Virtual delegates remain ordinary C++ subclasses compiled HERE: the vtable
// layout is fully determined by v8.h, which both sides share, and no delegate
// method needs a std:: type. (The artifact's own Rust delegate plumbing is
// unreachable stubs in shim.cc; this slice never constructs it.)
//
// ---------------------------------------------------------------------------
// Design notes
//
// Backing stores
//   A v8::BackingStore is reference counted inside the engine. The shim wraps
//   one shared reference per Go BackingStore value in a heap BsWrap; Go's
//   explicit BackingStore.Close drops that reference (gov8 has no finalizers:
//   the last reference -- Go side or JS side -- deterministically owns the
//   memory). Store reads/writes copy through the boundary; no pointer into
//   engine memory ever escapes to Go, and no Go pointer is retained.
//
//   - gov8_backing_store_from_slice mallocs a copy of the Go bytes; the
//     store's deleter frees that buffer when the last reference dies. The Go
//     slice is not retained.
//   - gov8_backing_store_from_ptr forwards the embedder's raw pointer and
//     deleter exactly like the crate's new_backing_store_from_ptr: the
//     CALLER owns the memory and supplies the deleter. Go cannot hand over
//     function pointers, so the deleter callback is the process-wide
//     trampoline below and deleter_data carries an integer Go registry
//     handle (resolved on the Go side to the user callback and the user's
//     deleter_data word). The user callback observes exactly the
//     (data, byte_length, deleter_data) triple it registered.
//
// Typed arrays / DataView
//   v8's typed array / DataView constructors V8-CHECK-abort the process on
//   out-of-bounds or misaligned geometry (the pinned oracle characterizes
//   that in tests/buffers_negative.rs). A Go binding cannot recover from a
//   V8 CHECK, so the constructors below prevalidate the geometry at this
//   boundary and return kErrBadArg instead of entering the engine. This is
//   an intentional, documented deviation: the fatal Rust boundary becomes a
//   Go error; every in-bounds observation is unchanged.
//
// Serializer / deserializer
//   Go delegates are integer-registry backed: one process-wide Go trampoline
//   per callback shape is registered once (gov8_buf_delegate_set_entry /
//   gov8_buf_deleter_set_entry) and every instance carries only an int64
//   registry id -- no Go pointer is ever retained by the engine.
//
//   The shim serializer delegate reproduces the pinned crate's DEFAULT
//   delegate behaviors verbatim for the hooks Go does not customize (the
//   host-object / wasm / SAB-id paths), and forwards ThrowDataCloneError to
//   Go: the hook receives the message text and returns whether the shim
//   should re-throw it as a JS Error (the structured-clone reporting the
//   oracle's DataCloneErrorReporter implements).
//
//   The deserializer delegate reproduces the pinned crate's default
//   ValueDeserializerImpl: all three read paths throw the crate's
//   deterministic "Deno ... not implemented" errors and fail. This is what
//   the conformance fixture pins (header-less data reads as legacy version
//   0; version-16-tagged bytes reach the host-object path).
//
//   Serializer output memory is owned by this shim through the delegate's
//   ReallocateBufferMemory/FreeBufferMemory (stdlib realloc/free), so
//   gov8_value_serializer_release always copies out and frees inside the
//   shim.
//
//   The deserializer stores the caller's data pointer WITHOUT copying (the
//   engine's real contract, which the pinned fixture depends on): the Go
//   wrapper keeps the input slice referenced until
//   gov8_value_deserializer_dispose destroys the engine object, and only
//   then drops the reference.
//
// Value identity
//   gov8_value_same implements v8::Local operator== (object identity, not
//   handle-slot identity); two locals for the same object sit in different
//   slots.
// ===========================================================================

#include <algorithm>
#include <cstdlib>
#include <cstring>
#include <mutex>
#include <new>
#include <utility>

namespace gov8_buf_impl {

// The artifact's representation of std::shared_ptr<v8::BackingStore> (the
// crate's SharedPtrBase<T>): two machine words, passed by value across the
// flat bindings, manipulated only through them.
struct BsRep {
  void* ptr;
  void* ctrl;
};
static_assert(sizeof(BsRep) == 2 * sizeof(void*), "BsRep layout");

// Go trampoline for ValueSerializer::Delegate::ThrowDataCloneError:
// (registry id, op, message pointer, message length) -> 1 to re-throw the
// message as a JS Error, 0 to leave the engine untouched.
using BufDelegateEntry = uintptr_t (*)(int64_t id, int64_t op,
                                       const void* message, uintptr_t len);

constexpr int64_t kBufOpThrowDataCloneError = 1;

// Go trampoline for external backing-store deleters:
// (registry handle, data pointer, byte length) -> return value unused.
using BsDeleterEntry = uintptr_t (*)(int64_t handle, uintptr_t data,
                                     uintptr_t byte_length);

BufDelegateEntry g_buf_delegate_entry = nullptr;
BsDeleterEntry g_buf_deleter_entry = nullptr;

// Rebuilds a v8::Local from the raw object pointer the flat bindings use.
// With indirect (slot) handles, a Local's word is a SLOT ADDRESS, so the
// slot here is the caller's variable holding `ptr` (the same trick as the
// crate's ptr_to_local in src/support.h). The reference keeps location_
// pointing at the caller's live variable; escape the result into the parent
// (Go-owned) HandleScope before returning -- see Escapable below.
template <class T>
v8::Local<T> PtrToLocal(T* const& ptr) {
  return *reinterpret_cast<const v8::Local<T>*>(&ptr);
}

// --- shared-pointer wrapper for v8::BackingStore ----------------------------

struct BsWrap {
  uint64_t magic;
  v8::Isolate* iso;  // owning isolate for wrapper discipline (bookkeeping)
  // Heap-held shared reference; stable address handed to Go as an opaque
  // handle and released exactly once by gov8_backing_store_dispose.
  BsRep* rep;
  // Optional native owner retained for the lifetime of this wrapper. The
  // allocator-aware constructor uses it to keep the isolate's allocator alive
  // without a second Go-visible wrapper and two additional DLL crossings.
  void* retained_owner;
  void (*release_retained_owner)(void*) noexcept;
};

constexpr uint64_t kBsMagic = 0x674f563842535442ULL;  // "gOV8BSTB"

BsWrap* AsBs(void* p) {
  if (p == nullptr) return nullptr;
  BsWrap* w = static_cast<BsWrap*>(p);
  return w->magic == kBsMagic ? w : nullptr;
}

BsWrap* BsNew(v8::Isolate* iso, BsRep rep) {
  BsWrap* w = new BsWrap{};
  w->magic = kBsMagic;
  w->iso = iso;
  w->rep = new BsRep{};
  *w->rep = rep;
  return w;
}

// --- external deleter plumbing ----------------------------------------------

// Trampoline handed to the engine as the backing-store deleter for
// embedder-owned stores. deleter_data is the Go registry handle.
void Gov8BsDeleterTrampoline(void* data, size_t byte_length,
                             void* deleter_data) {
  BsDeleterEntry entry = g_buf_deleter_entry;
  if (entry == nullptr) {
    return;  // entry never registered: nothing safe to do here
  }
  entry(static_cast<int64_t>(reinterpret_cast<intptr_t>(deleter_data)),
        reinterpret_cast<uintptr_t>(data),
        static_cast<uintptr_t>(byte_length));
}

// Deleter for gov8_backing_store_from_slice stores: frees the shim-malloc'd
// copy when the last reference dies.
void Gov8OwnedSliceDeleter(void* data, size_t, void*) { free(data); }

// --- serializer --------------------------------------------------------------

class Gov8SerDelegate final : public v8::ValueSerializer::Delegate {
 public:
  Gov8SerDelegate(v8::Isolate* iso, int64_t id) : iso_(iso), id_(id) {}

  void ThrowDataCloneError(v8::Local<v8::String> message) override {
    BufDelegateEntry entry = g_buf_delegate_entry;
    if (entry == nullptr || iso_ == nullptr) {
      return;
    }
    // Copy the message to TLS bytes for the synchronous Go call; the Go
    // side copies again before returning.
    thread_local std::string tls_message;
    tls_message.clear();
    if (!message.IsEmpty()) {
      const size_t n = message->Utf8Length(iso_);
      if (n > 0) {
        tls_message.resize(n);
        const size_t written = message->WriteUtf8(
            iso_, tls_message.data(), n,
            v8::String::WriteFlags::kReplaceInvalidUtf8);
        tls_message.resize(written);
      }
    }
    const uintptr_t rethrow =
        entry(id_, kBufOpThrowDataCloneError, tls_message.data(),
              tls_message.size());
    if (rethrow == 1 && !tls_message.empty()) {
      v8::MaybeLocal<v8::String> rebuilt = v8::String::NewFromUtf8(
          iso_, tls_message.data(), v8::NewStringType::kNormal,
          static_cast<int>(tls_message.size()));
      v8::Local<v8::String> str;
      if (rebuilt.ToLocal(&str)) {
        iso_->ThrowException(v8::Exception::Error(str));
      }
    }
  }

  // --- pinned crate default behaviors for hooks Go does not customize ------

  bool HasCustomHostObject(v8::Isolate*) override { return false; }

  v8::Maybe<bool> IsHostObject(v8::Isolate*, v8::Local<v8::Object>) override {
    ThrowDenoDefault("Deno serializer: is_host_object not implemented");
    return v8::Nothing<bool>();
  }

  v8::Maybe<bool> WriteHostObject(v8::Isolate*,
                                  v8::Local<v8::Object>) override {
    ThrowDenoDefault("Deno serializer: write_host_object not implemented");
    return v8::Nothing<bool>();
  }

  v8::Maybe<uint32_t> GetSharedArrayBufferId(
      v8::Isolate*, v8::Local<v8::SharedArrayBuffer>) override {
    return v8::Nothing<uint32_t>();
  }

  v8::Maybe<uint32_t> GetWasmModuleTransferId(
      v8::Isolate*, v8::Local<v8::WasmModuleObject>) override {
    ThrowDenoDefault(
        "Deno serializer: get_wasm_module_transfer_id not implemented");
    return v8::Nothing<uint32_t>();
  }

  // Buffer memory is owned by this shim: allocated with stdlib realloc,
  // freed with stdlib free, both inside this TU (see release below).
  void* ReallocateBufferMemory(void* old_buffer, size_t size,
                               size_t* actual_size) override {
    *actual_size = size;
    return realloc(old_buffer, size);
  }

  void FreeBufferMemory(void* buffer) override { free(buffer); }

 private:
  void ThrowDenoDefault(const char* msg) {
    if (iso_ == nullptr) return;
    v8::MaybeLocal<v8::String> maybe =
        v8::String::NewFromUtf8(iso_, msg, v8::NewStringType::kNormal);
    v8::Local<v8::String> str;
    if (maybe.ToLocal(&str)) {
      iso_->ThrowException(v8::Exception::Error(str));
    }
  }

  v8::Isolate* iso_;
  int64_t id_;
};

// v8::ValueSerializer is one pointer wide (PrivateData*) and constructed by
// the artifact's flat CONSTRUCT binding; the delegate is placement-new'd
// into the same heap wrap so both share one lifetime.
struct SerWrap {
  uint64_t magic;
  v8::Isolate* iso;
  alignas(Gov8SerDelegate) unsigned char delegate_storage
     [sizeof(Gov8SerDelegate)];
  v8::ValueSerializer* ser = nullptr;
  bool built = false;
  // Parked released buffer when the first release call had no room to copy.
  uint8_t* released_ptr = nullptr;
  size_t released_size = 0;
  bool released = false;
};

constexpr uint64_t kSerMagic = 0x674f563853455257ULL;  // "gOV8SERW"

SerWrap* AsSer(void* p) {
  if (p == nullptr) return nullptr;
  SerWrap* w = static_cast<SerWrap*>(p);
  return w->magic == kSerMagic ? w : nullptr;
}

// --- deserializer -------------------------------------------------------------

// Reproduces the pinned crate's default ValueDeserializerImpl behaviors: all
// three delegate read paths throw the crate's deterministic error text and
// fail. This is what the conformance fixture pins.
class Gov8DeserDelegate final : public v8::ValueDeserializer::Delegate {
 public:
  v8::MaybeLocal<v8::Object> ReadHostObject(v8::Isolate* isolate) override {
    ThrowDenoDefault(isolate,
                     "Deno deserializer: read_host_object not implemented");
    return v8::MaybeLocal<v8::Object>();
  }

  v8::MaybeLocal<v8::SharedArrayBuffer> GetSharedArrayBufferFromId(
      v8::Isolate* isolate, uint32_t) override {
    ThrowDenoDefault(
        isolate,
        "Deno deserializer: get_shared_array_buffer_from_id not implemented");
    return v8::MaybeLocal<v8::SharedArrayBuffer>();
  }

  v8::MaybeLocal<v8::WasmModuleObject> GetWasmModuleFromId(v8::Isolate* isolate,
                                                           uint32_t) override {
    ThrowDenoDefault(
        isolate, "Deno deserializer: get_wasm_module_from_id not implemented");
    return v8::MaybeLocal<v8::WasmModuleObject>();
  }

 private:
  static void ThrowDenoDefault(v8::Isolate* iso, const char* msg) {
    v8::MaybeLocal<v8::String> maybe =
        v8::String::NewFromUtf8(iso, msg, v8::NewStringType::kNormal);
    v8::Local<v8::String> str;
    if (maybe.ToLocal(&str)) {
      iso->ThrowException(v8::Exception::Error(str));
    }
  }
};

struct DeserWrap {
  uint64_t magic;
  v8::Isolate* iso;
  alignas(Gov8DeserDelegate) unsigned char delegate_storage
     [sizeof(Gov8DeserDelegate)];
  v8::ValueDeserializer* deser = nullptr;
  bool built = false;
};

constexpr uint64_t kDeserMagic = 0x674f563844455257ULL;  // "gOV8DERW"

DeserWrap* AsDeser(void* p) {
  if (p == nullptr) return nullptr;
  DeserWrap* w = static_cast<DeserWrap*>(p);
  return w->magic == kDeserMagic ? w : nullptr;
}

// --- typed-array geometry kinds -------------------------------------------------

struct ViewKindInfo {
  size_t element_size;
  bool require_alignment;
};

ViewKindInfo ViewKindOf(int64_t kind) {
  // Element sizes per kind (see the Go viewKind constants).
  switch (kind) {
    case 0:  // Uint8
    case 1:  // Int8
    case 10:  // Uint8Clamped
      return {1, false};
    case 2:  // Uint16
    case 3:  // Int16
      return {2, true};
    case 4:  // Uint32
    case 5:  // Int32
    case 6:  // Float32
      return {4, true};
    case 7:  // Float64
    case 8:  // BigInt64
    case 9:  // BigUint64
      return {8, true};
    default:
      return {0, false};
  }
}

}  // namespace gov8_buf_impl

// --- flat bindings exported by the pinned artifact ----------------------------
//
// Declared exactly as the crate declares them (v8-152.2.0/src/*.rs, compiled
// by src/binding.cc). extern "C": no name mangling, plain pointer ABI.
extern "C" {
// BackingStore construction (returns a released unique_ptr as a raw pointer).
v8::BackingStore* v8__ArrayBuffer__NewBackingStore__with_byte_length(
    v8::Isolate* isolate, size_t byte_length);
v8::BackingStore* v8__ArrayBuffer__NewBackingStore__with_data(
    void* data, size_t byte_length,
    void (*deleter)(void* data, size_t length, void* deleter_data),
    void* deleter_data);
v8::BackingStore* v8__SharedArrayBuffer__NewBackingStore__with_byte_length(
    v8::Isolate* isolate, size_t byte_length);
v8::BackingStore* v8__SharedArrayBuffer__NewBackingStore__with_data(
    void* data, size_t byte_length,
    void (*deleter)(void* data, size_t length, void* deleter_data),
    void* deleter_data);

// ArrayBuffer / SharedArrayBuffer construction and access.
const v8::ArrayBuffer* v8__ArrayBuffer__New__with_byte_length(
    v8::Isolate* isolate, size_t byte_length);
const v8::ArrayBuffer* v8__ArrayBuffer__New__with_backing_store(
    v8::Isolate* isolate, const gov8_buf_impl::BsRep* backing_store);
size_t v8__ArrayBuffer__ByteLength(const v8::ArrayBuffer* self);
bool v8__ArrayBuffer__IsDetachable(const v8::ArrayBuffer* self);
bool v8__ArrayBuffer__WasDetached(const v8::ArrayBuffer* self);
void* v8__ArrayBuffer__Data(const v8::ArrayBuffer* self);
int v8__ArrayBuffer__Detach(const v8::ArrayBuffer* self, const v8::Value* key);
void v8__ArrayBuffer__SetDetachKey(const v8::ArrayBuffer* self,
                                   const v8::Value* key);
gov8_buf_impl::BsRep v8__ArrayBuffer__GetBackingStore(
    const v8::ArrayBuffer* self);
const v8::SharedArrayBuffer* v8__SharedArrayBuffer__New__with_byte_length(
    v8::Isolate* isolate, size_t byte_length);
const v8::SharedArrayBuffer* v8__SharedArrayBuffer__New__with_backing_store(
    v8::Isolate* isolate, const gov8_buf_impl::BsRep* backing_store);
size_t v8__SharedArrayBuffer__ByteLength(const v8::SharedArrayBuffer* self);
gov8_buf_impl::BsRep v8__SharedArrayBuffer__GetBackingStore(
    const v8::SharedArrayBuffer* self);

// Typed array / DataView / view access. All bindings are declared with
// pointer parameters (extern "C": references and pointers are
// ABI-identical, and v8::Local's operator* yields T*).
const v8::DataView* v8__DataView__New(const v8::ArrayBuffer* buf,
                                      size_t byte_offset, size_t length);
#define GOV8_BUF_TA_DECL(NAME)                                               \
  const v8::NAME* v8__##NAME##__New(const v8::ArrayBuffer* buf,              \
                                    size_t byte_offset, size_t length);
GOV8_BUF_TA_DECL(Uint8Array)
GOV8_BUF_TA_DECL(Int8Array)
GOV8_BUF_TA_DECL(Uint16Array)
GOV8_BUF_TA_DECL(Int16Array)
GOV8_BUF_TA_DECL(Uint32Array)
GOV8_BUF_TA_DECL(Int32Array)
GOV8_BUF_TA_DECL(Float32Array)
GOV8_BUF_TA_DECL(Float64Array)
GOV8_BUF_TA_DECL(BigInt64Array)
GOV8_BUF_TA_DECL(BigUint64Array)
GOV8_BUF_TA_DECL(Uint8ClampedArray)
#undef GOV8_BUF_TA_DECL
size_t v8__TypedArray__Length(const v8::TypedArray* self);
const v8::ArrayBuffer* v8__ArrayBufferView__Buffer(
    const v8::ArrayBufferView* self);
size_t v8__ArrayBufferView__ByteLength(const v8::ArrayBufferView* self);
size_t v8__ArrayBufferView__ByteOffset(const v8::ArrayBufferView* self);
bool v8__ArrayBufferView__HasBuffer(const v8::ArrayBufferView* self);
size_t v8__ArrayBufferView__CopyContents(const v8::ArrayBufferView* self,
                                         void* dest, int byte_length);

// Value identity (Local operator==) and buffers-family predicates.
bool v8__Data__EQ(const v8::Data* self, const v8::Data* other);
bool v8__Value__IsArrayBuffer(const v8::Value* self);
bool v8__Value__IsArrayBufferView(const v8::Value* self);
bool v8__Value__IsTypedArray(const v8::Value* self);
bool v8__Value__IsDataView(const v8::Value* self);
bool v8__Value__IsSharedArrayBuffer(const v8::Value* self);

// std::shared_ptr<v8::BackingStore> plumbing (libc++ side of the artifact).
gov8_buf_impl::BsRep std__shared_ptr__v8__BackingStore__COPY(
    const gov8_buf_impl::BsRep* self);
gov8_buf_impl::BsRep std__shared_ptr__v8__BackingStore__CONVERT__std__unique_ptr(
    v8::BackingStore* unique);
v8::BackingStore* std__shared_ptr__v8__BackingStore__get(
    const gov8_buf_impl::BsRep* self);
void std__shared_ptr__v8__BackingStore__reset(gov8_buf_impl::BsRep* self);
long std__shared_ptr__v8__BackingStore__use_count(
    const gov8_buf_impl::BsRep* self);

// BackingStore queries (flat; avoid the C++ methods for symmetry).
void* v8__BackingStore__Data(const v8::BackingStore* self);
size_t v8__BackingStore__ByteLength(const v8::BackingStore* self);
bool v8__BackingStore__IsShared(const v8::BackingStore* self);
bool v8__BackingStore__IsResizableByUserJavaScript(
    const v8::BackingStore* self);

// ValueSerializer construction / destruction / release (std::-free shapes).
void v8__ValueSerializer__CONSTRUCT(v8::ValueSerializer* buf,
                                    v8::Isolate* isolate,
                                    v8::ValueSerializer::Delegate* delegate);
void v8__ValueSerializer__DESTRUCT(v8::ValueSerializer* self);
void v8__ValueSerializer__Release(v8::ValueSerializer* self, uint8_t** ptr,
                                  size_t* size);
}

extern "C" {

// --- isolate extension ---------------------------------------------------------

// Requests a full GC (Isolate::LowMemoryNotification), making engine-side
// drops of ArrayBuffer/backing-store references observable deterministically.
__declspec(dllexport) int64_t gov8_isolate_low_memory_notification(
    v8::Isolate* iso) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    iso->LowMemoryNotification();
    return kOk;
  } catch (...) {
    SetErr("C++ exception in isolate_low_memory_notification");
    return kErrCpp;
  }
}

// --- callback registry entry points -----------------------------------------

__declspec(dllexport) int64_t gov8_buf_delegate_set_entry(uintptr_t entry) {
  ClearErr();
  gov8_buf_impl::g_buf_delegate_entry =
      reinterpret_cast<gov8_buf_impl::BufDelegateEntry>(entry);
  return kOk;
}

__declspec(dllexport) int64_t gov8_buf_deleter_set_entry(uintptr_t entry) {
  ClearErr();
  gov8_buf_impl::g_buf_deleter_entry =
      reinterpret_cast<gov8_buf_impl::BsDeleterEntry>(entry);
  return kOk;
}

// --- backing stores -----------------------------------------------------------

// Creates a backing store owning a malloc'd copy of the given bytes (the
// crate's new_backing_store_from_vec / from_boxed_slice). The engine call is
// isolate-free there, so no Isolate::Scope is entered; iso is recorded on the
// wrapper only.
__declspec(dllexport) void* gov8_backing_store_from_slice(v8::Isolate* iso,
                                                          const char* data,
                                                          int64_t len) {
  ClearErr();
  if (len < 0 || (data == nullptr && len > 0)) {
    SetErr("invalid slice");
    return nullptr;
  }
  try {
    void* copy = nullptr;
    if (len > 0) {
      copy = malloc(static_cast<size_t>(len));
      if (copy == nullptr) {
        SetErr("backing store allocation failed");
        return nullptr;
      }
      memcpy(copy, data, static_cast<size_t>(len));
    }
    v8::BackingStore* unique = v8__ArrayBuffer__NewBackingStore__with_data(
        copy, static_cast<size_t>(len), gov8_buf_impl::Gov8OwnedSliceDeleter,
        nullptr);
    if (unique == nullptr) {
      free(copy);
      SetErr("NewBackingStore failed");
      return nullptr;
    }
    return gov8_buf_impl::BsNew(
        iso, std__shared_ptr__v8__BackingStore__CONVERT__std__unique_ptr(unique));
  } catch (...) {
    SetErr("C++ exception in backing_store_from_slice");
    return nullptr;
  }
}

// Creates a backing store over the CALLER-owned pointer with a Go-registry
// deleter (the crate's new_backing_store_from_ptr). The caller must keep the
// memory valid until the deleter runs.
__declspec(dllexport) void* gov8_backing_store_from_ptr(
    v8::Isolate* iso, void* data, int64_t byte_length, int64_t go_handle) {
  ClearErr();
  if (byte_length < 0 || (data == nullptr && byte_length > 0)) {
    SetErr("invalid pointer range");
    return nullptr;
  }
  if (go_handle <= 0) {
    SetErr("invalid deleter registry handle");
    return nullptr;
  }
  try {
    v8::BackingStore* unique = v8__ArrayBuffer__NewBackingStore__with_data(
        data, static_cast<size_t>(byte_length),
        gov8_buf_impl::Gov8BsDeleterTrampoline,
        reinterpret_cast<void*>(static_cast<intptr_t>(go_handle)));
    if (unique == nullptr) {
      SetErr("NewBackingStore failed");
      return nullptr;
    }
    return gov8_buf_impl::BsNew(
        iso, std__shared_ptr__v8__BackingStore__CONVERT__std__unique_ptr(unique));
  } catch (...) {
    SetErr("C++ exception in backing_store_from_ptr");
    return nullptr;
  }
}

// Standalone zero-initialized store allocated by the isolate's array buffer
// allocator (ArrayBuffer::new_backing_store).
__declspec(dllexport) void* gov8_backing_store_new(v8::Isolate* iso,
                                                   int64_t byte_length) {
  ClearErr();
  if (iso == nullptr || byte_length < 0) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::BackingStore* unique = v8__ArrayBuffer__NewBackingStore__with_byte_length(
        iso, static_cast<size_t>(byte_length));
    if (unique == nullptr) {
      SetErr("NewBackingStore failed");
      return nullptr;
    }
    return gov8_buf_impl::BsNew(
        iso, std__shared_ptr__v8__BackingStore__CONVERT__std__unique_ptr(unique));
  } catch (...) {
    SetErr("C++ exception in backing_store_new");
    return nullptr;
  }
}

// Standalone store for a SharedArrayBuffer (is_shared() == true).
__declspec(dllexport) void* gov8_sab_backing_store_new(v8::Isolate* iso,
                                                       int64_t byte_length) {
  ClearErr();
  if (iso == nullptr || byte_length < 0) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::BackingStore* unique =
        v8__SharedArrayBuffer__NewBackingStore__with_byte_length(
            iso, static_cast<size_t>(byte_length));
    if (unique == nullptr) {
      SetErr("SharedArrayBuffer::NewBackingStore failed");
      return nullptr;
    }
    return gov8_buf_impl::BsNew(
        iso, std__shared_ptr__v8__BackingStore__CONVERT__std__unique_ptr(unique));
  } catch (...) {
    SetErr("C++ exception in sab_backing_store_new");
    return nullptr;
  }
}

// Creates a shared backing store owning a malloc'd copy of the given bytes
// (SharedArrayBuffer::new_backing_store_from_vec / from_boxed_slice).
__declspec(dllexport) void* gov8_sab_backing_store_from_slice(
    v8::Isolate* iso, const char* data, int64_t len) {
  ClearErr();
  if (iso == nullptr || len < 0 || (data == nullptr && len > 0)) {
    SetErr("invalid slice");
    return nullptr;
  }
  try {
    void* copy = nullptr;
    if (len > 0) {
      copy = malloc(static_cast<size_t>(len));
      if (copy == nullptr) {
        SetErr("shared backing store allocation failed");
        return nullptr;
      }
      memcpy(copy, data, static_cast<size_t>(len));
    }
    v8::BackingStore* unique =
        v8__SharedArrayBuffer__NewBackingStore__with_data(
            copy, static_cast<size_t>(len),
            gov8_buf_impl::Gov8OwnedSliceDeleter, nullptr);
    if (unique == nullptr) {
      free(copy);
      SetErr("SharedArrayBuffer::NewBackingStore failed");
      return nullptr;
    }
    return gov8_buf_impl::BsNew(
        iso, std__shared_ptr__v8__BackingStore__CONVERT__std__unique_ptr(unique));
  } catch (...) {
    SetErr("C++ exception in sab_backing_store_from_slice");
    return nullptr;
  }
}

// Creates a shared backing store over caller-owned memory with the same
// registry-backed deleter contract as gov8_backing_store_from_ptr.
__declspec(dllexport) void* gov8_sab_backing_store_from_ptr(
    v8::Isolate* iso, void* data, int64_t byte_length, int64_t go_handle) {
  ClearErr();
  if (iso == nullptr || byte_length < 0 ||
      (data == nullptr && byte_length > 0)) {
    SetErr("invalid pointer range");
    return nullptr;
  }
  if (go_handle <= 0) {
    SetErr("invalid deleter registry handle");
    return nullptr;
  }
  try {
    v8::BackingStore* unique =
        v8__SharedArrayBuffer__NewBackingStore__with_data(
            data, static_cast<size_t>(byte_length),
            gov8_buf_impl::Gov8BsDeleterTrampoline,
            reinterpret_cast<void*>(static_cast<intptr_t>(go_handle)));
    if (unique == nullptr) {
      SetErr("SharedArrayBuffer::NewBackingStore failed");
      return nullptr;
    }
    return gov8_buf_impl::BsNew(
        iso, std__shared_ptr__v8__BackingStore__CONVERT__std__unique_ptr(unique));
  } catch (...) {
    SetErr("C++ exception in sab_backing_store_from_ptr");
    return nullptr;
  }
}

#define GOV8_BS_QUERY(name, expr)                                            \
  __declspec(dllexport) int64_t name(void* bsw) {                            \
    ClearErr();                                                              \
    gov8_buf_impl::BsWrap* w = gov8_buf_impl::AsBs(bsw);                     \
    if (w == nullptr) {                                                      \
      SetErr("invalid backing store handle");                                \
      return kErrMagic;                                                      \
    }                                                                        \
    try {                                                                    \
      return static_cast<int64_t>(expr);                                     \
    } catch (...) {                                                          \
      SetErr("C++ exception in " #name);                                     \
      return kErrCpp;                                                        \
    }                                                                        \
  }

GOV8_BS_QUERY(gov8_backing_store_byte_length,
              v8__BackingStore__ByteLength(
                  std__shared_ptr__v8__BackingStore__get(w->rep)))
GOV8_BS_QUERY(gov8_backing_store_is_shared,
              v8__BackingStore__IsShared(
                  std__shared_ptr__v8__BackingStore__get(w->rep))
                  ? 1
                  : 0)
GOV8_BS_QUERY(gov8_backing_store_is_resizable,
              v8__BackingStore__IsResizableByUserJavaScript(
                  std__shared_ptr__v8__BackingStore__get(w->rep))
                  ? 1
                  : 0)
GOV8_BS_QUERY(gov8_backing_store_use_count,
              std__shared_ptr__v8__BackingStore__use_count(w->rep))

#undef GOV8_BS_QUERY

// Copies exactly min(cap, byte_length - off) bytes from the store at off;
// out-of-range requests are rejected at the boundary.
__declspec(dllexport) int64_t gov8_backing_store_read_at(void* bsw,
                                                         int64_t off,
                                                         char* buf,
                                                         int64_t cap) {
  ClearErr();
  gov8_buf_impl::BsWrap* w = gov8_buf_impl::AsBs(bsw);
  if (w == nullptr) {
    SetErr("invalid backing store handle");
    return kErrMagic;
  }
  if (buf == nullptr || cap < 0 || off < 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::BackingStore* bs = std__shared_ptr__v8__BackingStore__get(w->rep);
    const size_t store_len = v8__BackingStore__ByteLength(bs);
    if (static_cast<size_t>(off) > store_len ||
        static_cast<size_t>(cap) > store_len - static_cast<size_t>(off)) {
      SetErr("read out of range");
      return kErrBadArg;
    }
    const void* src = v8__BackingStore__Data(bs);
    if (cap > 0 && src != nullptr) {
      memcpy(buf, static_cast<const uint8_t*>(src) + off,
             static_cast<size_t>(cap));
    }
    return cap;
  } catch (...) {
    SetErr("C++ exception in backing_store_read_at");
    return kErrCpp;
  }
}

// Writes data into the store at off. The store is interior-mutable in the
// engine, so writes are observed through every buffer and view aliasing it.
__declspec(dllexport) int64_t gov8_backing_store_write_at(void* bsw,
                                                          int64_t off,
                                                          const char* data,
                                                          int64_t len) {
  ClearErr();
  gov8_buf_impl::BsWrap* w = gov8_buf_impl::AsBs(bsw);
  if (w == nullptr) {
    SetErr("invalid backing store handle");
    return kErrMagic;
  }
  if (len < 0 || (data == nullptr && len > 0) || off < 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::BackingStore* bs = std__shared_ptr__v8__BackingStore__get(w->rep);
    const size_t store_len = v8__BackingStore__ByteLength(bs);
    if (static_cast<size_t>(off) > store_len ||
        static_cast<size_t>(len) > store_len - static_cast<size_t>(off)) {
      SetErr("write out of range");
      return kErrBadArg;
    }
    void* dst = v8__BackingStore__Data(bs);
    if (len > 0 && dst != nullptr) {
      memcpy(static_cast<uint8_t*>(dst) + off, data, static_cast<size_t>(len));
    }
    return len;
  } catch (...) {
    SetErr("C++ exception in backing_store_write_at");
    return kErrCpp;
  }
}

// Drops the Go-side reference. If this is the last reference the store (and
// any external deleter) is destroyed synchronously inside this call.
__declspec(dllexport) int64_t gov8_backing_store_dispose(void* bsw) {
  ClearErr();
  gov8_buf_impl::BsWrap* w = gov8_buf_impl::AsBs(bsw);
  if (w == nullptr) {
    SetErr("invalid backing store handle");
    return kErrMagic;
  }
  try {
    // BackingStore is independently ref-counted and may outlive its creating
    // isolate. Resetting the shared_ptr neither requires nor may enter that
    // possibly already-disposed isolate.
    std__shared_ptr__v8__BackingStore__reset(w->rep);
    delete w->rep;
    if (w->release_retained_owner != nullptr)
      w->release_retained_owner(w->retained_owner);
    delete w;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in backing_store_dispose");
    return kErrCpp;
  }
}

// --- ArrayBuffer ---------------------------------------------------------------
//
// Constructors that create engine objects (ArrayBuffer, SharedArrayBuffer,
// typed arrays, DataView) require the CALLER's context to be entered: their
// instance maps come from the native context (Factory::NewJSArrayBuffer...
// reads isolate()->native_context()). These exports therefore take the
// context wrapper and enter a Context::Scope, like the promise exports.

__declspec(dllexport) int64_t gov8_array_buffer_new(v8::Isolate* iso,
                                                    void* ctxw, void* scope,
                                                    int64_t byte_length,
                                                    void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (out == nullptr || byte_length < 0 || w == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    // The wire must point at a slot in the CALLER'S (Go-owned) scope.
    v8::EscapableHandleScope escapable(iso);
    // Allocation failure is a process-fatal OOM inside the engine (the pinned
    // oracle characterizes the same boundary); null results are not expected.
    const v8::ArrayBuffer* ab =
        v8__ArrayBuffer__New__with_byte_length(iso, static_cast<size_t>(byte_length));
    if (ab == nullptr) {
      SetErr("ArrayBuffer::New failed");
      return kErrGeneric;
    }
    *out = ToWire(escapable.Escape(gov8_buf_impl::PtrToLocal(ab)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in array_buffer_new");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_array_buffer_new_with_backing_store(
    v8::Isolate* iso, void* ctxw, void* scope, void* bsw, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  gov8_buf_impl::BsWrap* b = gov8_buf_impl::AsBs(bsw);
  if (out == nullptr || b == nullptr || w == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso) || !OwnedBy(iso, b->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::EscapableHandleScope escapable(iso);
    const v8::ArrayBuffer* ab =
        v8__ArrayBuffer__New__with_backing_store(iso, b->rep);
    if (ab == nullptr) {
      SetErr("ArrayBuffer::New failed");
      return kErrGeneric;
    }
    *out = ToWire(escapable.Escape(gov8_buf_impl::PtrToLocal(ab)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in array_buffer_new_with_backing_store");
    return kErrCpp;
  }
}

#define GOV8_AB_QUERY(name, out_type, expr)                                  \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* scope,          \
                                     void* wire, out_type out) {             \
    ClearErr();                                                              \
    if (out == nullptr || wire == nullptr || !ScopeIs(iso, scope)) {         \
      SetErr("invalid argument");                                            \
      return kErrBadArg;                                                     \
    }                                                                        \
    try {                                                                    \
      Gov8IsolateScope iso_scope(iso);                                     \
      v8::HandleScope hs(iso);                                               \
      v8::Local<v8::ArrayBuffer> local = FromWire<v8::ArrayBuffer>(wire);    \
      if (!v8__Value__IsArrayBuffer(*local)) {                               \
        SetErr("not an ArrayBuffer");                                        \
        return kErrBadArg;                                                   \
      }                                                                      \
      const v8::ArrayBuffer* ab = *local;                                    \
      *out = (expr);                                                         \
      return kOk;                                                            \
    } catch (...) {                                                          \
      SetErr("C++ exception in " #name);                                     \
      return kErrCpp;                                                        \
    }                                                                        \
  }

GOV8_AB_QUERY(gov8_array_buffer_byte_length, int64_t*,
              static_cast<int64_t>(v8__ArrayBuffer__ByteLength(ab)))
GOV8_AB_QUERY(gov8_array_buffer_is_detachable, int32_t*,
              v8__ArrayBuffer__IsDetachable(ab) ? 1 : 0)
GOV8_AB_QUERY(gov8_array_buffer_was_detached_raw, int32_t*,
              v8__ArrayBuffer__WasDetached(ab) ? 1 : 0)

#undef GOV8_AB_QUERY

// Raw ArrayBuffer::Data() (null for detached or zero-length buffers). The
// pointer is engine-owned and only valid while the buffer is alive; Go must
// not dereference or retain it.
__declspec(dllexport) int64_t gov8_array_buffer_data(v8::Isolate* iso,
                                                     void* scope, void* wire) {
  ClearErr();
  if (wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::ArrayBuffer> local = FromWire<v8::ArrayBuffer>(wire);
    if (!v8__Value__IsArrayBuffer(*local)) {
      SetErr("not an ArrayBuffer");
      return kErrBadArg;
    }
    return reinterpret_cast<int64_t>(v8__ArrayBuffer__Data(*local));
  } catch (...) {
    SetErr("C++ exception in array_buffer_data");
    return kErrCpp;
  }
}

// Returns a NEW shared reference to the buffer's backing store, wrapped for
// Go. The Go side must release it with gov8_backing_store_dispose.
__declspec(dllexport) int64_t gov8_array_buffer_get_backing_store(
    v8::Isolate* iso, void* scope, void* wire, void** out) {
  ClearErr();
  if (out == nullptr || wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::ArrayBuffer> local = FromWire<v8::ArrayBuffer>(wire);
    if (!v8__Value__IsArrayBuffer(*local)) {
      SetErr("not an ArrayBuffer");
      return kErrBadArg;
    }
    gov8_buf_impl::BsRep rep = v8__ArrayBuffer__GetBackingStore(*local);
    *out = gov8_buf_impl::BsNew(iso, rep);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in array_buffer_get_backing_store");
    return kErrCpp;
  }
}

// ArrayBuffer::Detach(key). key_wire == null means "no key". *out_ok is 1 on
// success and 0 when the [[ArrayBufferDetachKey]] check rejected the request
// (the engine throws a TypeError in that case; a shim-internal fallback
// TryCatch -- or the caller's when supplied -- captures it so the pending
// exception never leaks into later calls).
__declspec(dllexport) int64_t gov8_array_buffer_detach(v8::Isolate* iso,
                                                       void* ctxw, void* tcw,
                                                       void* scope, void* wire,
                                                       void* key_wire,
                                                       int32_t* out_ok) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (out_ok == nullptr || wire == nullptr || w == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    // The key-mismatch path allocates a TypeError from the native context.
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
        SetErr("trycatch belongs to a different isolate");
        return kErrBadArg;
      }
      tc = TcOf(tw);
    }
    if (tc == nullptr) tc = fallback.Init(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::ArrayBuffer> local = FromWire<v8::ArrayBuffer>(wire);
    if (!v8__Value__IsArrayBuffer(*local)) {
      SetErr("not an ArrayBuffer");
      return kErrBadArg;
    }
    // key_wire may be null: an empty local means "no key" (the crate's
    // detach(None)).
    v8::Local<v8::Value> key = FromWire<v8::Value>(key_wire);
    const int mb =
        v8__ArrayBuffer__Detach(*local, key.IsEmpty() ? nullptr : *key);
    switch (mb) {
      case 1:  // MaybeBool::JustTrue
        *out_ok = 1;
        return kOk;
      case 0:   // MaybeBool::JustFalse
      default:  // MaybeBool::Nothing: key check failed
        *out_ok = 0;
        return kOk;
    }
  } catch (...) {
    SetErr("C++ exception in array_buffer_detach");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_array_buffer_set_detach_key(
    v8::Isolate* iso, void* scope, void* wire, void* key_wire) {
  ClearErr();
  if (key_wire == nullptr || wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::ArrayBuffer> local = FromWire<v8::ArrayBuffer>(wire);
    if (!v8__Value__IsArrayBuffer(*local)) {
      SetErr("not an ArrayBuffer");
      return kErrBadArg;
    }
    v8__ArrayBuffer__SetDetachKey(*local, *FromWire<v8::Value>(key_wire));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in array_buffer_set_detach_key");
    return kErrCpp;
  }
}

// --- SharedArrayBuffer ------------------------------------------------------

__declspec(dllexport) int64_t gov8_sab_new(v8::Isolate* iso, void* ctxw,
                                           void* scope, int64_t byte_length,
                                           void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (out == nullptr || byte_length < 0 || w == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::EscapableHandleScope escapable(iso);
    const v8::SharedArrayBuffer* sab =
        v8__SharedArrayBuffer__New__with_byte_length(iso,
                                                     static_cast<size_t>(byte_length));
    if (sab == nullptr) {
      SetErr("SharedArrayBuffer::New failed");
      return kErrGeneric;
    }
    *out = ToWire(escapable.Escape(gov8_buf_impl::PtrToLocal(sab)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in sab_new");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_sab_new_with_backing_store(
    v8::Isolate* iso, void* ctxw, void* scope, void* bsw, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  gov8_buf_impl::BsWrap* b = gov8_buf_impl::AsBs(bsw);
  if (out == nullptr || b == nullptr || w == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso) || !OwnedBy(iso, b->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::EscapableHandleScope escapable(iso);
    const v8::SharedArrayBuffer* sab =
        v8__SharedArrayBuffer__New__with_backing_store(iso, b->rep);
    if (sab == nullptr) {
      SetErr("SharedArrayBuffer::New failed");
      return kErrGeneric;
    }
    *out = ToWire(escapable.Escape(gov8_buf_impl::PtrToLocal(sab)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in sab_new_with_backing_store");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_sab_byte_length(v8::Isolate* iso,
                                                   void* scope, void* wire,
                                                   int64_t* out) {
  ClearErr();
  if (out == nullptr || wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::SharedArrayBuffer> local =
        FromWire<v8::SharedArrayBuffer>(wire);
    if (!v8__Value__IsSharedArrayBuffer(*local)) {
      SetErr("not a SharedArrayBuffer");
      return kErrBadArg;
    }
    *out = static_cast<int64_t>(v8__SharedArrayBuffer__ByteLength(*local));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in sab_byte_length");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_sab_get_backing_store(v8::Isolate* iso,
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
    v8::Local<v8::SharedArrayBuffer> local =
        FromWire<v8::SharedArrayBuffer>(wire);
    if (!v8__Value__IsSharedArrayBuffer(*local)) {
      SetErr("not a SharedArrayBuffer");
      return kErrBadArg;
    }
    gov8_buf_impl::BsRep rep = v8__SharedArrayBuffer__GetBackingStore(*local);
    *out = gov8_buf_impl::BsNew(iso, rep);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in sab_get_backing_store");
    return kErrCpp;
  }
}

// --- value predicates (buffers family) ----------------------------------------

#define GOV8_BUF_PREDICATE(name, expr)                                       \
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

GOV8_BUF_PREDICATE(gov8_is_array_buffer, v8__Value__IsArrayBuffer(*val))
GOV8_BUF_PREDICATE(gov8_is_shared_array_buffer, v8__Value__IsSharedArrayBuffer(*val))
GOV8_BUF_PREDICATE(gov8_is_array_buffer_view, v8__Value__IsArrayBufferView(*val))
GOV8_BUF_PREDICATE(gov8_is_typed_array, v8__Value__IsTypedArray(*val))
GOV8_BUF_PREDICATE(gov8_is_data_view, v8__Value__IsDataView(*val))

#undef GOV8_BUF_PREDICATE

// v8::Local operator== : object identity (not handle-slot identity).
__declspec(dllexport) int64_t gov8_value_same(v8::Isolate* iso, void* a,
                                              void* b) {
  ClearErr();
  if (a == nullptr || b == nullptr) {
    SetErr("null value wire");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> va = FromWire<v8::Value>(a);
    v8::Local<v8::Value> vb = FromWire<v8::Value>(b);
    return v8__Data__EQ(*va, *vb) ? 1 : 0;
  } catch (...) {
    SetErr("C++ exception in value_same");
    return kErrCpp;
  }
}

// --- typed arrays / DataView (with boundary prevalidation) ----------------------
//
// v8's own constructors V8-CHECK-abort on out-of-bounds or misaligned
// geometry. The checks below turn those fatal boundaries into kErrBadArg
// before the engine is entered; the engine call itself follows unchanged.

// kind values (Go viewKind constants): 0 Uint8, 1 Int8, 2 Uint16, 3 Int16,
// 4 Uint32, 5 Int32, 6 Float32, 7 Float64, 8 BigInt64, 9 BigUint64,
// 10 Uint8Clamped.
__declspec(dllexport) int64_t gov8_typed_array_new(v8::Isolate* iso,
                                                   void* ctxw, void* scope,
                                                   void* ab_wire, int64_t kind,
                                                   int64_t byte_offset,
                                                   int64_t length,
                                                   void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (out == nullptr || ab_wire == nullptr || w == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  const gov8_buf_impl::ViewKindInfo info = gov8_buf_impl::ViewKindOf(kind);
  if (info.element_size == 0) {
    SetErr("unknown typed array kind");
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
    if (byte_offset < 0 || length < 0) {
      SetErr("negative view geometry");
      return kErrBadArg;
    }
    const size_t off = static_cast<size_t>(byte_offset);
    const size_t elems = static_cast<size_t>(length);
    if (info.require_alignment && off % info.element_size != 0) {
      // V8 CHECK: "0 == byte_offset % element_size".
      SetErr("typed array byte_offset not aligned to element size");
      return kErrBadArg;
    }
    if (off > store_len ||
        elems > (store_len - off) / info.element_size) {
      // V8 CHECK: offset + length*element_size <= ByteLength.
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
    SetErr("C++ exception in typed_array_new");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_data_view_new(v8::Isolate* iso,
                                                 void* ctxw, void* scope,
                                                 void* ab_wire,
                                                 int64_t byte_offset,
                                                 int64_t length, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (out == nullptr || ab_wire == nullptr || w == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
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
    if (byte_offset < 0 || length < 0 ||
        static_cast<size_t>(byte_offset) > store_len ||
        static_cast<size_t>(length) >
            store_len - static_cast<size_t>(byte_offset)) {
      // V8 CHECK inside DataView::New for out-of-bounds geometry.
      SetErr("data view out of bounds");
      return kErrBadArg;
    }
    const v8::DataView* dv = v8__DataView__New(
        *ab, static_cast<size_t>(byte_offset), static_cast<size_t>(length));
    if (dv == nullptr) {
      SetErr("DataView::New failed");
      return kErrGeneric;
    }
    *out = ToWire(escapable.Escape(gov8_buf_impl::PtrToLocal(dv)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in data_view_new");
    return kErrCpp;
  }
}

// View-pointer helper: validates the wire is an ArrayBufferView and returns
// the raw view pointer for the flat bindings (or null after setting an
// error). Runs inside an Isolate::Scope + HandleScope opened by the caller.
const v8::ArrayBufferView* ViewPtrOf(v8::Local<v8::Value> val) {
  if (!v8__Value__IsArrayBufferView(*val)) {
    SetErr("not an ArrayBufferView");
    return nullptr;
  }
  return *val.As<v8::ArrayBufferView>();
}

#define GOV8_VIEW_QUERY(name, out_type, expr)                                \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* scope,          \
                                     void* wire, out_type out) {             \
    ClearErr();                                                              \
    if (out == nullptr || wire == nullptr || !ScopeIs(iso, scope)) {         \
      SetErr("invalid argument");                                            \
      return kErrBadArg;                                                     \
    }                                                                        \
    try {                                                                    \
      Gov8IsolateScope iso_scope(iso);                                     \
      v8::HandleScope hs(iso);                                               \
      v8::Local<v8::Value> val = FromWire<v8::Value>(wire);                  \
      const v8::ArrayBufferView* view = ViewPtrOf(val);                      \
      if (view == nullptr) {                                                 \
        return kErrBadArg;                                                   \
      }                                                                      \
      *out = (expr);                                                         \
      return kOk;                                                            \
    } catch (...) {                                                          \
      SetErr("C++ exception in " #name);                                     \
      return kErrCpp;                                                        \
    }                                                                        \
  }

GOV8_VIEW_QUERY(gov8_view_byte_offset, int64_t*,
                static_cast<int64_t>(v8__ArrayBufferView__ByteOffset(view)))
GOV8_VIEW_QUERY(gov8_view_byte_length, int64_t*,
                static_cast<int64_t>(v8__ArrayBufferView__ByteLength(view)))

#undef GOV8_VIEW_QUERY

// TypedArray::Length (element count). Only valid on typed arrays.
__declspec(dllexport) int64_t gov8_typed_array_length(v8::Isolate* iso,
                                                      void* scope, void* wire,
                                                      int64_t* out) {
  ClearErr();
  if (out == nullptr || wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> val = FromWire<v8::Value>(wire);
    if (!v8__Value__IsTypedArray(*val)) {
      SetErr("not a TypedArray");
      return kErrBadArg;
    }
    *out = static_cast<int64_t>(
        v8__TypedArray__Length(*val.As<v8::TypedArray>()));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in typed_array_length");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_view_has_buffer(v8::Isolate* iso,
                                                   void* scope, void* wire,
                                                   int32_t* out) {
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
    *out = v8__ArrayBufferView__HasBuffer(view) ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in view_has_buffer");
    return kErrCpp;
  }
}

// The view's underlying ArrayBuffer as a new local in the caller's scope.
__declspec(dllexport) int64_t gov8_view_buffer(v8::Isolate* iso, void* scope,
                                               void* wire, void** out) {
  ClearErr();
  if (out == nullptr || wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::EscapableHandleScope escapable(iso);
    v8::Local<v8::Value> val = FromWire<v8::Value>(wire);
    const v8::ArrayBufferView* view = ViewPtrOf(val);
    if (view == nullptr) {
      return kErrBadArg;
    }
    *out = ToWire(escapable.Escape(gov8_buf_impl::PtrToLocal(v8__ArrayBufferView__Buffer(view))));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in view_buffer");
    return kErrCpp;
  }
}

// ArrayBufferView::CopyContents into the caller buffer; returns the number
// of bytes written (at most min(cap, view byte length)).
__declspec(dllexport) int64_t gov8_view_copy_contents(v8::Isolate* iso,
                                                      void* scope, void* wire,
                                                      char* buf, int64_t cap) {
  ClearErr();
  if (wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (buf == nullptr || cap < 0) {
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
    const size_t written =
        v8__ArrayBufferView__CopyContents(view, buf, static_cast<int>(cap));
    return static_cast<int64_t>(written);
  } catch (...) {
    SetErr("C++ exception in view_copy_contents");
    return kErrCpp;
  }
}

// --- pinned size limits ---------------------------------------------------------

// Compile-time limits of the pinned headers (v8::TypedArray::kMaxByteLength
// and the per-type kMaxLength constants) plus the pinned artifact's on-heap
// typed-array size (GN arg v8_typed_array_max_size_in_heap = 0 for the
// prebuilt library; the oracle fixture pins the same value).
__declspec(dllexport) int64_t gov8_typed_array_limits(
    int64_t* max_byte_length, int64_t* uint8_max_length,
    int64_t* float64_max_length, int64_t* bigint64_max_length,
    int64_t* max_size_in_heap) {
  ClearErr();
  if (max_byte_length == nullptr || uint8_max_length == nullptr ||
      float64_max_length == nullptr || bigint64_max_length == nullptr ||
      max_size_in_heap == nullptr) {
    SetErr("null out params");
    return kErrBadArg;
  }
  *max_byte_length = static_cast<int64_t>(v8::TypedArray::kMaxByteLength);
  *uint8_max_length = static_cast<int64_t>(v8::Uint8Array::kMaxLength);
  *float64_max_length = static_cast<int64_t>(v8::Float64Array::kMaxLength);
  *bigint64_max_length = static_cast<int64_t>(v8::BigInt64Array::kMaxLength);
  *max_size_in_heap = 0;
  return kOk;
}

// --- ValueSerializer -----------------------------------------------------------

__declspec(dllexport) int64_t gov8_value_serializer_new(v8::Isolate* iso,
                                                        void* ctxw,
                                                        void* scope,
                                                        int64_t delegate_id,
                                                        void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (out == nullptr || w == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (delegate_id <= 0) {
    SetErr("invalid delegate registry id");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    gov8_buf_impl::SerWrap* sw = new gov8_buf_impl::SerWrap{};
    sw->magic = gov8_buf_impl::kSerMagic;
    sw->iso = iso;
    // The delegate must outlive the serializer; destroy the serializer first
    // and the delegate last (reverse order in dispose). GlobalPlacementNew
    // lives in shim.cc's anonymous namespace (visible from this include).
    gov8_buf_impl::Gov8SerDelegate* delegate =
        GlobalPlacementNew<gov8_buf_impl::Gov8SerDelegate>(
            sw->delegate_storage, iso, delegate_id);
    sw->ser = static_cast<v8::ValueSerializer*>(
        ::operator new(sizeof(v8::ValueSerializer)));
    v8__ValueSerializer__CONSTRUCT(sw->ser, iso, delegate);
    sw->built = true;
    *out = sw;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_serializer_new");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_serializer_write_header(void* serw) {
  ClearErr();
  gov8_buf_impl::SerWrap* sw = gov8_buf_impl::AsSer(serw);
  if (sw == nullptr) {
    SetErr("invalid serializer handle");
    return kErrMagic;
  }
  if (sw->released) {
    SetErr("serializer already released");
    return kErrState;
  }
  try {
    Gov8IsolateScope iso_scope(sw->iso);
    sw->ser->WriteHeader();
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_serializer_write_header");
    return kErrCpp;
  }
}

// WriteValue. *out_ok is 1 for Just(true), 0 for Just(false); a Nothing
// result (the delegate threw) returns kErrException so the Go wrapper
// surfaces it through IsException like the script/run path.
__declspec(dllexport) int64_t gov8_value_serializer_write_value(
    void* serw, void* ctxw, void* tcw, void* scope, void* value_wire,
    int32_t* out_ok) {
  ClearErr();
  gov8_buf_impl::SerWrap* sw = gov8_buf_impl::AsSer(serw);
  CtxWrap* w = AsCtx(ctxw);
  if (sw == nullptr || w == nullptr || out_ok == nullptr ||
      value_wire == nullptr || !ScopeIs(sw->iso, scope) ||
      !OwnedBy(sw->iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (sw->released) {
    SetErr("serializer already released");
    return kErrState;
  }
  v8::Isolate* iso = sw->iso;
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
        SetErr("trycatch belongs to a different isolate");
        return kErrBadArg;
      }
      tc = TcOf(tw);
    }
    if (tc == nullptr) tc = fallback.Init(iso);
    v8::HandleScope hs(iso);
    v8::Maybe<bool> ok = sw->ser->WriteValue(ctx, FromWire<v8::Value>(value_wire));
    if (ok.IsNothing()) {
      return kErrException;
    }
    *out_ok = ok.FromJust() ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_serializer_write_value");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_serializer_transfer_array_buffer(
    void* serw, void* scope, uint32_t transfer_id, void* ab_wire) {
  ClearErr();
  gov8_buf_impl::SerWrap* sw = gov8_buf_impl::AsSer(serw);
  if (sw == nullptr || ab_wire == nullptr || !ScopeIs(sw->iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (sw->released) {
    SetErr("serializer already released");
    return kErrState;
  }
  try {
    Gov8IsolateScope iso_scope(sw->iso);
    v8::HandleScope hs(sw->iso);
    sw->ser->TransferArrayBuffer(transfer_id,
                                 FromWire<v8::ArrayBuffer>(ab_wire));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_serializer_transfer_array_buffer");
    return kErrCpp;
  }
}

// Releases the wire bytes: copies them into (buf, cap) and frees the engine
// buffer inside the shim. When the buffer does not fit, the bytes are parked
// in the wrapper, *out_len reports the required size and kErrNoMemory is
// returned; the next call with a large enough buffer copies and frees.
__declspec(dllexport) int64_t gov8_value_serializer_release(void* serw,
                                                            char* buf,
                                                            int64_t cap,
                                                            int64_t* out_len) {
  ClearErr();
  gov8_buf_impl::SerWrap* sw = gov8_buf_impl::AsSer(serw);
  if (sw == nullptr) {
    SetErr("invalid serializer handle");
    return kErrMagic;
  }
  if (out_len == nullptr || buf == nullptr || cap < 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(sw->iso);
    if (!sw->released) {
      uint8_t* ptr = nullptr;
      size_t size = 0;
      v8__ValueSerializer__Release(sw->ser, &ptr, &size);
      sw->released = true;
      sw->released_ptr = ptr;
      sw->released_size = size;
    }
    *out_len = static_cast<int64_t>(sw->released_size);
    if (sw->released_size > static_cast<size_t>(cap)) {
      SetErr("output buffer too small");
      return kErrNoMemory;
    }
    if (sw->released_size > 0) {
      memcpy(buf, sw->released_ptr, sw->released_size);
    }
    free(sw->released_ptr);
    sw->released_ptr = nullptr;
    sw->released_size = 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_serializer_release");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_serializer_dispose(void* serw) {
  ClearErr();
  gov8_buf_impl::SerWrap* sw = gov8_buf_impl::AsSer(serw);
  if (sw == nullptr) {
    SetErr("invalid serializer handle");
    return kErrMagic;
  }
  try {
    Gov8IsolateScope iso_scope(sw->iso);
    if (sw->built) {
      v8__ValueSerializer__DESTRUCT(sw->ser);
      ::operator delete(sw->ser);
      sw->ser = nullptr;
      reinterpret_cast<gov8_buf_impl::Gov8SerDelegate*>(sw->delegate_storage)
          ->~Gov8SerDelegate();
      sw->built = false;
    }
    free(sw->released_ptr);
    sw->released_ptr = nullptr;
    delete sw;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_serializer_dispose");
    return kErrCpp;
  }
}

// --- ValueDeserializer -----------------------------------------------------------

// Creates a deserializer over (data, len). The engine stores the pointer
// WITHOUT copying; the Go wrapper must keep the input slice referenced until
// gov8_value_deserializer_dispose destroys the engine object. This mirrors
// the pinned crate exactly (its fixture depends on the no-copy contract).
__declspec(dllexport) void* gov8_value_deserializer_new(v8::Isolate* iso,
                                                        void* ctxw,
                                                        void* scope,
                                                        const char* data,
                                                        int64_t len) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return nullptr;
  }
  if (len < 0 || (data == nullptr && len > 0)) {
    SetErr("invalid data range");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    gov8_buf_impl::DeserWrap* dw = new gov8_buf_impl::DeserWrap{};
    dw->magic = gov8_buf_impl::kDeserMagic;
    dw->iso = iso;
    gov8_buf_impl::Gov8DeserDelegate* delegate =
        GlobalPlacementNew<gov8_buf_impl::Gov8DeserDelegate>(
            dw->delegate_storage);
    dw->deser = static_cast<v8::ValueDeserializer*>(
        ::operator new(sizeof(v8::ValueDeserializer)));
    GlobalPlacementNew<v8::ValueDeserializer>(
        dw->deser, iso, reinterpret_cast<const uint8_t*>(data),
        static_cast<size_t>(len), delegate);
    dw->built = true;
    return dw;
  } catch (...) {
    SetErr("C++ exception in value_deserializer_new");
    return nullptr;
  }
}

// ReadValue. An empty result (deserialization failure; the engine has thrown)
// returns kErrException so Go routes it through IsException like the script
// run path. The value local is created in the caller's scope.
__declspec(dllexport) int64_t gov8_value_deserializer_read_value(void* deserw,
                                                                 void* ctxw,
                                                                 void* tcw,
                                                                 void* scope,
                                                                 void** out) {
  ClearErr();
  gov8_buf_impl::DeserWrap* dw = gov8_buf_impl::AsDeser(deserw);
  CtxWrap* w = AsCtx(ctxw);
  if (dw == nullptr || w == nullptr || out == nullptr ||
      !ScopeIs(dw->iso, scope) || !OwnedBy(dw->iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  v8::Isolate* iso = dw->iso;
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
        SetErr("trycatch belongs to a different isolate");
        return kErrBadArg;
      }
      tc = TcOf(tw);
    }
    if (tc == nullptr) tc = fallback.Init(iso);
    v8::MaybeLocal<v8::Value> value = dw->deser->ReadValue(ctx);
    v8::Local<v8::Value> local;
    if (!value.ToLocal(&local)) {
      return kErrException;
    }
    *out = ToWire(local);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_deserializer_read_value");
    return kErrCpp;
  }
}

// ReadHeader for explicit wire probing. *out_ok reports the Maybe; a Nothing
// result (invalid header; the engine has thrown) returns kErrException.
__declspec(dllexport) int64_t gov8_value_deserializer_read_header(
    void* deserw, void* ctxw, void* tcw, void* scope, int32_t* out_ok) {
  ClearErr();
  gov8_buf_impl::DeserWrap* dw = gov8_buf_impl::AsDeser(deserw);
  CtxWrap* w = AsCtx(ctxw);
  if (dw == nullptr || w == nullptr || out_ok == nullptr ||
      !ScopeIs(dw->iso, scope) || !OwnedBy(dw->iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  v8::Isolate* iso = dw->iso;
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (tcw != nullptr) {
      TcWrap* tw = AsTc(tcw);
      if (tw == nullptr || !OwnedBy(iso, tw->iso)) {
        SetErr("invalid trycatch handle");
        return kErrMagic;
      }
      tc = TcOf(tw);
    }
    if (tc == nullptr) tc = fallback.Init(iso);
    v8::Maybe<bool> ok = dw->deser->ReadHeader(ctx);
    if (ok.IsNothing()) {
      return kErrException;
    }
    *out_ok = ok.FromJust() ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_deserializer_read_header");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_deserializer_transfer_array_buffer(
    void* deserw, void* scope, uint32_t transfer_id, void* ab_wire) {
  ClearErr();
  gov8_buf_impl::DeserWrap* dw = gov8_buf_impl::AsDeser(deserw);
  if (dw == nullptr || ab_wire == nullptr || !ScopeIs(dw->iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(dw->iso);
    v8::HandleScope hs(dw->iso);
    dw->deser->TransferArrayBuffer(transfer_id,
                                   FromWire<v8::ArrayBuffer>(ab_wire));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_deserializer_transfer_array_buffer");
    return kErrCpp;
  }
}

// Destroys the engine deserializer (which releases the caller's data
// pointer). The Go wrapper may drop its input slice reference only after
// this call succeeds.
__declspec(dllexport) int64_t gov8_value_deserializer_dispose(void* deserw) {
  ClearErr();
  gov8_buf_impl::DeserWrap* dw = gov8_buf_impl::AsDeser(deserw);
  if (dw == nullptr) {
    SetErr("invalid deserializer handle");
    return kErrMagic;
  }
  try {
    Gov8IsolateScope iso_scope(dw->iso);
    if (dw->built) {
      dw->deser->~ValueDeserializer();
      ::operator delete(dw->deser);
      dw->deser = nullptr;
      reinterpret_cast<gov8_buf_impl::Gov8DeserDelegate*>(dw->delegate_storage)
          ->~Gov8DeserDelegate();
      dw->built = false;
    }
    delete dw;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_deserializer_dispose");
    return kErrCpp;
  }
}

}  // extern "C"
