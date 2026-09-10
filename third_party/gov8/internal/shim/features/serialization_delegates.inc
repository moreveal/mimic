// ===========================================================================
// serialization_delegates.inc -- ValueSerializer / ValueDeserializer delegate
// completion exports (feature slice).
//
// The base buffers slice (buffers_serialization.inc) ships a serializer and
// deserializer whose delegates reproduce the pinned crate's DEFAULT hook
// behaviors and forward only ThrowDataCloneError to Go. THIS slice completes
// the delegate surface: every hook of ValueSerializerImpl /
// ValueDeserializerImpl can be routed to a Go delegate through one
// process-wide trampoline, while the untouched default path stays
// byte-identical (the conformance-buffers fixture keeps passing).
//
// Rust surface mapped here (pinned crate v8 =152.2.0; the flat `_del`
// exports below are gov8's own C ABI, not artifact bindings):
//
//   v8::ValueSerializerImpl                        (optional Go interfaces)
//     has_custom_host_object(isolate) -> bool      -> HasCustomHostObjectHook
//     is_host_object(scope, object) -> Option<bool>-> IsHostObjectHook
//     write_host_object(scope, object, ser)        -> WriteHostObjectHook
//     get_shared_array_buffer_id(scope, sab)       -> GetSharedArrayBufferIDHook
//     get_wasm_module_transfer_id(scope, module)   -> GetWasmModuleTransferIDHook
//   v8::ValueDeserializerImpl
//     read_host_object(scope, deser)               -> ReadHostObjectHook
//     get_shared_array_buffer_from_id(id)          -> GetSharedArrayBufferFromIDHook
//     get_wasm_module_from_id(id)                  -> GetWasmModuleFromIDHook
//
// Semantics pinned by rust-oracle/src/bin/conformance-serializer-delegates.rs
// and reproduced here:
//
//   - A hook the Go delegate does NOT implement keeps the pinned crate's
//     trait default verbatim, WITHOUT calling into Go (the trait-default
//     body is un-instrumentable in the oracle, so its counters stay 0):
//     has_custom_host_object -> false; is_host_object / write_host_object /
//     get_wasm_module_transfer_id -> the deterministic "Deno serializer:
//     ... not implemented" Error throw + Nothing; the deserializer's three
//     read hooks -> the "Deno deserializer: ... not implemented" throws;
//     get_shared_array_buffer_id -> silent Nothing (the pinned build then
//     throws V8's own "#<SharedArrayBuffer> could not be cloned.").
//   - write_host_object's Just/Nothing RESULT is ignored on this build once
//     no exception is pending (release-build semantics): this shim returns
//     Just(true) after the hook and lets the engine observe a pending
//     exception itself, which aborts the write right after the 0x5c tag.
//   - A deserializer hook completing with None WITHOUT a pending exception
//     is NOT a clean failure on this build: the engine throws its own
//     "Unable to deserialize cloned data." Error. This shim returns an
//     empty MaybeLocal and does not throw.
//   - The legacy observation-only Wasm resolver stays source compatible.
//     A distinct typed resolver hook can now supply an actual, validated
//     WasmModuleObject; Go performs isolate/scope/type validation before a
//     handle reaches this native boundary.
//
// Delegate model. Exactly like the native-callback registries: delegates
// live in an integer registry on the Go side; the engine only ever sees an
// int64 id (packed with the implemented-hook bitmask at construction). One
// process-wide Go trampoline (syscall.NewCallback, pinned by the Go runtime)
// is registered once via gov8_ser_delegate_set_entry. No Go pointer ever
// crosses into the engine. A Go panic inside any hook is recovered and
// converted into the process fail-fast abort (gov8_host_panic_abort, exit
// code 0xC0000409 on Windows) after printing the message - the same
// observable boundary the pinned oracle characterizes for panics unwinding
// out of its extern "C" delegate trampolines.
//
// Ownership and lifetimes.
//
//   - SerDelWrap / DeserDelWrap carry their OWN magic words (distinct from
//     the base slices' kSerMagic/kDeserMagic): a wrap created here is
//     unusable with the base exports and vice versa, so cross-family
//     misuse fails loudly (kErrMagic) instead of corrupting state. The Go
//     wrapper types (DelegateValueSerializer / DelegateValueDeserializer)
//     only expose the matching `_del` exports and deterministic Close.
//   - The delegate object is placement-new'd into the wrap's storage and
//     destroyed before the engine serializer/deserializer; the engine
//     object is destroyed first (it owns the last delegate use), then the
//     delegate, then the wrap - reverse construction order.
//   - Output-buffer ownership: ReallocateBufferMemory / FreeBufferMemory
//     use stdlib realloc/free inside this TU, so release always copies out
//     and frees inside the shim (the delegate's AtomicUsize-capacity glue
//     of the crate is reproed by this pair). Drop-without-release frees
//     through FreeBufferMemory during dispose.
//   - Deserializer input: the engine stores the caller's data pointer
//     WITHOUT copying; the Go wrapper keeps the input slice referenced
//     until Close (same contract as the base slice).
//   - Handles handed to Go hooks (the object/sab/module arguments) are wire
//     words created by the engine inside the caller's open HandleScope;
//     they are valid only for the duration of the hook call. Handles
//     RETURNED by Go hooks (host object, SAB from id) are re-rooted with
//     v8::Local<T>::New before being handed back to the engine, so the
//     result survives regardless of which transient scope the Go-side
//     constructor used.
// ===========================================================================

#include <cstdlib>
#include <cstring>

namespace gov8_serdel_impl {

// ToWire / FromWire are shim.cc's anonymous-namespace helpers, directly
// visible in this TU.

// Go trampoline for every delegated hook:
// (registry id, op, payload a, payload b) -> op-specific return word.
using SerDelEntry = uintptr_t (*)(int64_t id, int64_t op, uintptr_t a,
                                 uintptr_t b);

SerDelEntry g_serdel_entry = nullptr;

// Hook op codes. These are part of the Go ABI contract (keep in sync with
// serializer_delegates.go).
constexpr int64_t kOpThrowDataCloneError = 1;  // a=msg ptr b=msg len -> 1 rethrow
constexpr int64_t kOpHasCustomHostObject = 2;  // -> 0/1
constexpr int64_t kOpIsHostObject = 3;         // a=obj wire -> 0/1=Some, 2=None
constexpr int64_t kOpWriteHostObject = 4;      // a=obj wire -> 0/1 (result unused)
constexpr int64_t kOpGetSABId = 5;             // a=sab wire -> id+1, 0=None
constexpr int64_t kOpGetWasmTransferId = 6;    // a=module wire -> id+1, 0=None
constexpr int64_t kOpReadHostObject = 7;       // a=&out wire -> 1=Some, 0=None
constexpr int64_t kOpGetSABFromId = 8;         // a=id b=&out wire -> 1/0
constexpr int64_t kOpGetWasmFromId = 9;        // a=id -> always 0 (observe only)
constexpr int64_t kOpResolveWasmFromId = 10;   // a=id b=&out module -> 1/0

// Implemented-hook bitmask bits (Go ABI contract; keep in sync with
// serializer_delegates.go). Serializer and deserializer masks are distinct
// namespaces.
constexpr uint32_t kHookHasCustomHostObject = 1u << 0;
constexpr uint32_t kHookIsHostObject = 1u << 1;
constexpr uint32_t kHookWriteHostObject = 1u << 2;
constexpr uint32_t kHookGetSharedArrayBufferId = 1u << 3;
constexpr uint32_t kHookGetWasmModuleTransferId = 1u << 4;
constexpr uint32_t kHookReadHostObject = 1u << 0;
constexpr uint32_t kHookGetSharedArrayBufferFromId = 1u << 1;
constexpr uint32_t kHookGetWasmModuleFromId = 1u << 2;
constexpr uint32_t kHookResolveWasmModuleFromId = 1u << 3;

void ThrowDenoDefaultSer(v8::Isolate* iso, const char* msg) {
  if (iso == nullptr) return;
  v8::MaybeLocal<v8::String> maybe =
      v8::String::NewFromUtf8(iso, msg, v8::NewStringType::kNormal);
  v8::Local<v8::String> str;
  if (maybe.ToLocal(&str)) {
    iso->ThrowException(v8::Exception::Error(str));
  }
}

// --- extended serializer delegate -------------------------------------------

// Layout-compatible with the base slice's delegate storage needs: this class
// lives in this slice's own wrap, so its size is free; the static_assert
// documents the shape (vptr + isolate + id + hooks).
class Gov8SerDelX final : public v8::ValueSerializer::Delegate {
 public:
  Gov8SerDelX(v8::Isolate* iso, int64_t id, uint32_t hooks)
      : iso_(iso), id_(id), hooks_(hooks) {}

  void ThrowDataCloneError(v8::Local<v8::String> message) override {
    SerDelEntry entry = g_serdel_entry;
    if (entry == nullptr || iso_ == nullptr) {
      return;
    }
    // Copy the message to TLS bytes for the synchronous Go call; the Go
    // side copies again before returning (same discipline as the base
    // slice's delegate).
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
        entry(id_, kOpThrowDataCloneError,
              reinterpret_cast<uintptr_t>(tls_message.data()),
              static_cast<uintptr_t>(tls_message.size()));
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

  bool HasCustomHostObject(v8::Isolate*) override {
    if (!(hooks_ & kHookHasCustomHostObject)) {
      return false;  // pinned trait default, un-instrumented
    }
    SerDelEntry entry = g_serdel_entry;
    if (entry == nullptr) {
      return false;
    }
    return entry(id_, kOpHasCustomHostObject, 0, 0) == 1;
  }

  v8::Maybe<bool> IsHostObject(v8::Isolate*,
                               v8::Local<v8::Object> object) override {
    if (!(hooks_ & kHookIsHostObject)) {
      ThrowDenoDefaultSer(iso_, "Deno serializer: is_host_object not implemented");
      return v8::Nothing<bool>();
    }
    SerDelEntry entry = g_serdel_entry;
    if (entry == nullptr) {
      return v8::Nothing<bool>();
    }
    const uintptr_t r = entry(id_, kOpIsHostObject,
                              reinterpret_cast<uintptr_t>(ToWire(object)),
                              0);
    if (r == 2) {
      // Delegate answered None: the write fails without an exception
      // (V8 propagates the Nothing; the embedder decides what it means).
      return v8::Nothing<bool>();
    }
    return v8::Just(r == 1);
  }

  v8::Maybe<bool> WriteHostObject(v8::Isolate*,
                                  v8::Local<v8::Object> object) override {
    if (!(hooks_ & kHookWriteHostObject)) {
      ThrowDenoDefaultSer(iso_,
                          "Deno serializer: write_host_object not implemented");
      return v8::Nothing<bool>();
    }
    SerDelEntry entry = g_serdel_entry;
    if (entry == nullptr) {
      return v8::Nothing<bool>();
    }
    // The pinned release build ignores the delegate's Just/Nothing result -
    // only a pending exception aborts the write, and the ENGINE checks for
    // it right after this call. Always reporting Just(true) reproduces both
    // pinned behaviors: "Some(false) without throwing still succeeds" and
    // "a thrown exception stops the write after the 0x5c tag".
    entry(id_, kOpWriteHostObject,
          reinterpret_cast<uintptr_t>(ToWire(object)), 0);
    return v8::Just(true);
  }

  v8::Maybe<uint32_t> GetSharedArrayBufferId(
      v8::Isolate*, v8::Local<v8::SharedArrayBuffer> sab) override {
    SerDelEntry entry = g_serdel_entry;
    uint32_t id = 0;
    if ((hooks_ & kHookGetSharedArrayBufferId) && entry != nullptr) {
      const uintptr_t r = entry(id_, kOpGetSABId,
                                reinterpret_cast<uintptr_t>(ToWire(sab)), 0);
      if (r != 0) {
        id = static_cast<uint32_t>(r - 1);
        return v8::Just(id);
      }
    }
    // None (unimplemented trait default is a silent None; a Go hook may
    // also answer None). The pinned crate's delegate glue forwards None to
    // the base Delegate::GetSharedArrayBufferId, which throws the engine's
    // kDataCloneError with the SAB interpolated and returns Nothing. (V8's
    // WriteJSArrayBuffer would FromJust a Nothing and abort if the glue
    // hadn't thrown - the throw is mandatory on this build.)
    ThrowDenoDefaultSer(iso_, "#<SharedArrayBuffer> could not be cloned.");
    return v8::Nothing<uint32_t>();
  }

  v8::Maybe<uint32_t> GetWasmModuleTransferId(
      v8::Isolate*, v8::Local<v8::WasmModuleObject> module) override {
    if (!(hooks_ & kHookGetWasmModuleTransferId)) {
      ThrowDenoDefaultSer(
          iso_, "Deno serializer: get_wasm_module_transfer_id not implemented");
      return v8::Nothing<uint32_t>();
    }
    SerDelEntry entry = g_serdel_entry;
    if (entry == nullptr) {
      return v8::Nothing<uint32_t>();
    }
    const uintptr_t r =
        entry(id_, kOpGetWasmTransferId,
              reinterpret_cast<uintptr_t>(ToWire(module)), 0);
    if (r == 0) {
      // None without throwing: the module silently disappears from the
      // wire while the enclosing write succeeds (pinned asymmetry with the
      // SAB path).
      return v8::Nothing<uint32_t>();
    }
    return v8::Just(static_cast<uint32_t>(r - 1));
  }

  // Buffer memory is owned by this shim: allocated with stdlib realloc,
  // freed with stdlib free, both inside this TU.
  void* ReallocateBufferMemory(void* old_buffer, size_t size,
                               size_t* actual_size) override {
    *actual_size = size;
    return realloc(old_buffer, size);
  }

  void FreeBufferMemory(void* buffer) override { free(buffer); }

 private:
  v8::Isolate* iso_;
  int64_t id_;
  uint32_t hooks_;
};

static_assert(sizeof(Gov8SerDelX) > sizeof(void*) * 2,
              "delegate carries isolate, id and hook mask");

// --- extended deserializer delegate -------------------------------------------

class Gov8DeserDelX final : public v8::ValueDeserializer::Delegate {
 public:
  Gov8DeserDelX(v8::Isolate* iso, int64_t id, uint32_t hooks)
      : iso_(iso), id_(id), hooks_(hooks) {}

  v8::MaybeLocal<v8::Object> ReadHostObject(v8::Isolate*) override {
    if (!(hooks_ & kHookReadHostObject)) {
      ThrowDenoDefaultSer(
          iso_, "Deno deserializer: read_host_object not implemented");
      return v8::MaybeLocal<v8::Object>();
    }
    void* out = nullptr;
    if (!CallReturnWire(kOpReadHostObject,
                        reinterpret_cast<uintptr_t>(&out), 0) ||
        out == nullptr) {
      // None without throwing: the pinned build surfaces the engine's own
      // "Unable to deserialize cloned data." Error; do NOT throw here.
      return v8::MaybeLocal<v8::Object>();
    }
    return ReRoot<v8::Object>(out);
  }

  v8::MaybeLocal<v8::SharedArrayBuffer> GetSharedArrayBufferFromId(
      v8::Isolate*, uint32_t transfer_id) override {
    if (!(hooks_ & kHookGetSharedArrayBufferFromId)) {
      ThrowDenoDefaultSer(
          iso_,
          "Deno deserializer: get_shared_array_buffer_from_id not implemented");
      return v8::MaybeLocal<v8::SharedArrayBuffer>();
    }
    void* out = nullptr;
    if (!CallReturnWire(kOpGetSABFromId, static_cast<uintptr_t>(transfer_id),
                        reinterpret_cast<uintptr_t>(&out)) ||
        out == nullptr) {
      return v8::MaybeLocal<v8::SharedArrayBuffer>();
    }
    return ReRoot<v8::SharedArrayBuffer>(out);
  }

  v8::MaybeLocal<v8::WasmModuleObject> GetWasmModuleFromId(
      v8::Isolate*, uint32_t transfer_id) override {
    if (!(hooks_ & (kHookGetWasmModuleFromId |
                    kHookResolveWasmModuleFromId))) {
      ThrowDenoDefaultSer(
          iso_, "Deno deserializer: get_wasm_module_from_id not implemented");
      return v8::MaybeLocal<v8::WasmModuleObject>();
    }
    if (hooks_ & kHookResolveWasmModuleFromId) {
      void* out = nullptr;
      if (!CallReturnWire(kOpResolveWasmFromId,
                          static_cast<uintptr_t>(transfer_id),
                          reinterpret_cast<uintptr_t>(&out)) ||
          out == nullptr) {
        return v8::MaybeLocal<v8::WasmModuleObject>();
      }
      v8::Local<v8::Value> value = FromWire<v8::Value>(out);
      if (value.IsEmpty() || !value->IsWasmModuleObject()) {
        ThrowDenoDefaultSer(iso_, "gov8: resolved value is not a WasmModuleObject");
        return v8::MaybeLocal<v8::WasmModuleObject>();
      }
      return value.As<v8::WasmModuleObject>();
    }
    // Preserve the original observation-only hook exactly.
    SerDelEntry entry = g_serdel_entry;
    if (entry != nullptr) {
      entry(id_, kOpGetWasmFromId, static_cast<uintptr_t>(transfer_id), 0);
    }
    return v8::MaybeLocal<v8::WasmModuleObject>();
  }

 private:
  // Calls the trampoline for a hook that RETURNS a wire word through the
  // out-slot pointer. Returns whether the delegate produced a value.
  bool CallReturnWire(int64_t op, uintptr_t a, uintptr_t b) {
    SerDelEntry entry = g_serdel_entry;
    if (entry == nullptr) {
      return false;
    }
    return entry(id_, op, a, b) == 1;
  }

  // Re-roots a wire word produced inside a Go hook before handing it back
  // to the engine. Go-created wires are stable Go-scope slots; the copy
  // also normalizes the local representation for the engine's synchronous
  // consumption (the engine wraps the result in its own handles before any
  // further Go allocation can occur).
  template <class T>
  v8::Local<T> ReRoot(void* wire) {
    return FromWire<T>(wire);
  }

  v8::Isolate* iso_;
  int64_t id_;
  uint32_t hooks_;
};

// --- wraps (own magic words: loud cross-family misuse) -----------------------

struct SerDelWrap {
  uint64_t magic;
  v8::Isolate* iso;
  alignas(Gov8SerDelX) unsigned char delegate_storage[sizeof(Gov8SerDelX)];
  v8::ValueSerializer* ser = nullptr;
  bool built = false;
  // Parked released buffer when a release call had no room to copy.
  uint8_t* released_ptr = nullptr;
  size_t released_size = 0;
  bool released = false;
};

constexpr uint64_t kSerDelMagic = 0x674f563853454458ULL;  // "gOV8SEDX"

SerDelWrap* AsSerDel(void* p) {
  if (p == nullptr) return nullptr;
  SerDelWrap* w = static_cast<SerDelWrap*>(p);
  return w->magic == kSerDelMagic ? w : nullptr;
}

struct DeserDelWrap {
  uint64_t magic;
  v8::Isolate* iso;
  alignas(Gov8DeserDelX) unsigned char delegate_storage[sizeof(Gov8DeserDelX)];
  v8::ValueDeserializer* deser = nullptr;
  bool built = false;
};

constexpr uint64_t kDeserDelMagic = 0x674f563844455258ULL;  // "gOV8DERX"

DeserDelWrap* AsDeserDel(void* p) {
  if (p == nullptr) return nullptr;
  DeserDelWrap* w = static_cast<DeserDelWrap*>(p);
  return w->magic == kDeserDelMagic ? w : nullptr;
}

}  // namespace gov8_serdel_impl

// --- exports -----------------------------------------------------------------
// (v8__ValueSerializer__CONSTRUCT / __DESTRUCT / __Release are the buffers
// slice's flat artifact bindings, already declared in buffers_serialization.inc
// with identical signatures and visible in this TU.)
extern "C" {

// Registers the process-wide Go trampoline once (syscall.NewCallback on the
// Go side pins the function pointer for the process lifetime).
__declspec(dllexport) int64_t gov8_ser_delegate_set_entry(uintptr_t entry) {
  gov8_serdel_impl::g_serdel_entry =
      reinterpret_cast<gov8_serdel_impl::SerDelEntry>(entry);
  return kOk;
}

// --- serializer --------------------------------------------------------------

// Creates a delegate-completed serializer. hooks carries the
// implemented-hook bitmask; delegate_id is the Go registry handle.
__declspec(dllexport) int64_t gov8_value_serializer_new_del(
    v8::Isolate* iso, void* ctxw, void* scope, int64_t delegate_id,
    uint32_t hooks, void** out) {
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
    gov8_serdel_impl::SerDelWrap* sw = new gov8_serdel_impl::SerDelWrap{};
    sw->magic = gov8_serdel_impl::kSerDelMagic;
    sw->iso = iso;
    // The delegate must outlive the serializer; dispose destroys the
    // engine object first and the delegate last (reverse order).
    GlobalPlacementNew<gov8_serdel_impl::Gov8SerDelX>(sw->delegate_storage,
                                                      iso, delegate_id, hooks);
    sw->ser = static_cast<v8::ValueSerializer*>(
        ::operator new(sizeof(v8::ValueSerializer)));
    v8__ValueSerializer__CONSTRUCT(sw->ser, iso,
                                   reinterpret_cast<gov8_serdel_impl::Gov8SerDelX*>(
                                       sw->delegate_storage));
    sw->built = true;
    *out = sw;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_serializer_new_del");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_serializer_write_header_del(
    void* serw) {
  ClearErr();
  gov8_serdel_impl::SerDelWrap* sw = gov8_serdel_impl::AsSerDel(serw);
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
    SetErr("C++ exception in value_serializer_write_header_del");
    return kErrCpp;
  }
}

// WriteValue with Go-delegated hooks. *out_ok is 1 for Just(true), 0 for
// Just(false); a Nothing result (the delegate threw, or the pinned build
// rejected the write) returns kErrException so the Go wrapper surfaces it
// through IsException like the script/run path.
__declspec(dllexport) int64_t gov8_value_serializer_write_value_del(
    void* serw, void* ctxw, void* tcw, void* scope, void* value_wire,
    int32_t* out_ok) {
  ClearErr();
  gov8_serdel_impl::SerDelWrap* sw = gov8_serdel_impl::AsSerDel(serw);
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
    v8::Maybe<bool> ok =
        sw->ser->WriteValue(ctx, FromWire<v8::Value>(value_wire));
    if (ok.IsNothing()) {
      return kErrException;
    }
    *out_ok = ok.FromJust() ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_serializer_write_value_del");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_serializer_transfer_array_buffer_del(
    void* serw, void* scope, uint32_t transfer_id, void* ab_wire) {
  ClearErr();
  gov8_serdel_impl::SerDelWrap* sw = gov8_serdel_impl::AsSerDel(serw);
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
    SetErr("C++ exception in value_serializer_transfer_array_buffer_del");
    return kErrCpp;
  }
}

// v8::ValueSerializer::SetTreatArrayBufferViewsAsHostObjects.
__declspec(dllexport) int64_t
gov8_value_serializer_treat_views_as_host_objects_del(void* serw,
                                                      int32_t mode) {
  ClearErr();
  gov8_serdel_impl::SerDelWrap* sw = gov8_serdel_impl::AsSerDel(serw);
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
    sw->ser->SetTreatArrayBufferViewsAsHostObjects(mode != 0);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in treat_views_as_host_objects_del");
    return kErrCpp;
  }
}

// Direct helper writes (usable outside hooks too - the crate's
// ValueSerializer itself implements its helper trait). All append bytes
// only; no engine state is touched.
__declspec(dllexport) int64_t gov8_value_serializer_write_uint32_del(
    void* serw, uint32_t value) {
  ClearErr();
  gov8_serdel_impl::SerDelWrap* sw = gov8_serdel_impl::AsSerDel(serw);
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
    sw->ser->WriteUint32(value);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in write_uint32_del");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_serializer_write_uint64_del(
    void* serw, uint64_t value) {
  ClearErr();
  gov8_serdel_impl::SerDelWrap* sw = gov8_serdel_impl::AsSerDel(serw);
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
    sw->ser->WriteUint64(value);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in write_uint64_del");
    return kErrCpp;
  }
}

// Doubles cross the boundary through a pointer: the x64 calling convention
// passes floating-point arguments in XMM registers, which SyscallN words
// never populate.
__declspec(dllexport) int64_t gov8_value_serializer_write_double_del(
    void* serw, const double* value) {
  ClearErr();
  gov8_serdel_impl::SerDelWrap* sw = gov8_serdel_impl::AsSerDel(serw);
  if (sw == nullptr || value == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (sw->released) {
    SetErr("serializer already released");
    return kErrState;
  }
  try {
    Gov8IsolateScope iso_scope(sw->iso);
    sw->ser->WriteDouble(*value);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in write_double_del");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_serializer_write_raw_bytes_del(
    void* serw, const char* data, int64_t len) {
  ClearErr();
  gov8_serdel_impl::SerDelWrap* sw = gov8_serdel_impl::AsSerDel(serw);
  if (sw == nullptr) {
    SetErr("invalid serializer handle");
    return kErrMagic;
  }
  if (sw->released) {
    SetErr("serializer already released");
    return kErrState;
  }
  if (len < 0 || (data == nullptr && len > 0)) {
    SetErr("invalid byte range");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(sw->iso);
    sw->ser->WriteRawBytes(data, static_cast<size_t>(len));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in write_raw_bytes_del");
    return kErrCpp;
  }
}

// Releases the wire bytes (same two-phase protocol as the base slice: a
// too-small buffer parks the bytes in the wrap and reports the required
// size through *out_len with kErrNoMemory).
__declspec(dllexport) int64_t gov8_value_serializer_release_del(
    void* serw, char* buf, int64_t cap, int64_t* out_len) {
  ClearErr();
  gov8_serdel_impl::SerDelWrap* sw = gov8_serdel_impl::AsSerDel(serw);
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
    SetErr("C++ exception in value_serializer_release_del");
    return kErrCpp;
  }
}

// Destroys the engine serializer, then the delegate, then the wrap. An
// un-released buffer is freed through the delegate's FreeBufferMemory (the
// pinned crate's destructor path).
__declspec(dllexport) int64_t gov8_value_serializer_dispose_del(void* serw) {
  ClearErr();
  gov8_serdel_impl::SerDelWrap* sw = gov8_serdel_impl::AsSerDel(serw);
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
      reinterpret_cast<gov8_serdel_impl::Gov8SerDelX*>(sw->delegate_storage)
          ->~Gov8SerDelX();
      sw->built = false;
    }
    free(sw->released_ptr);
    sw->released_ptr = nullptr;
    delete sw;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_serializer_dispose_del");
    return kErrCpp;
  }
}

// --- deserializer --------------------------------------------------------------

// Creates a delegate-completed deserializer over (data, len). The engine
// stores the pointer WITHOUT copying; the Go wrapper keeps the input slice
// referenced until dispose.
__declspec(dllexport) void* gov8_value_deserializer_new_del(
    v8::Isolate* iso, void* ctxw, void* scope, int64_t delegate_id,
    uint32_t hooks, const char* data, int64_t len) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return nullptr;
  }
  if (delegate_id <= 0) {
    SetErr("invalid delegate registry id");
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
    gov8_serdel_impl::DeserDelWrap* dw = new gov8_serdel_impl::DeserDelWrap{};
    dw->magic = gov8_serdel_impl::kDeserDelMagic;
    dw->iso = iso;
    GlobalPlacementNew<gov8_serdel_impl::Gov8DeserDelX>(dw->delegate_storage,
                                                        iso, delegate_id,
                                                        hooks);
    dw->deser = static_cast<v8::ValueDeserializer*>(
        ::operator new(sizeof(v8::ValueDeserializer)));
    GlobalPlacementNew<v8::ValueDeserializer>(
        dw->deser, iso, reinterpret_cast<const uint8_t*>(data),
        static_cast<size_t>(len),
        reinterpret_cast<gov8_serdel_impl::Gov8DeserDelX*>(
            dw->delegate_storage));
    dw->built = true;
    return dw;
  } catch (...) {
    SetErr("C++ exception in value_deserializer_new_del");
    return nullptr;
  }
}

// ReadValue with Go-delegated hooks. An empty engine result returns
// kErrException (the engine has thrown, or the pinned build rejected the
// read); the value local is created in the caller's scope.
__declspec(dllexport) int64_t gov8_value_deserializer_read_value_del(
    void* deserw, void* ctxw, void* tcw, void* scope, void** out) {
  ClearErr();
  gov8_serdel_impl::DeserDelWrap* dw = gov8_serdel_impl::AsDeserDel(deserw);
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
    v8::Local<v8::Value> local;
    {
      v8::HandleScope hs(iso);
      v8::MaybeLocal<v8::Value> value = dw->deser->ReadValue(ctx);
      v8::Local<v8::Value> localInner;
      if (!value.ToLocal(&localInner)) {
        return kErrException;
      }
      local = localInner;
    }
    // hs has closed here: the engine's transient handle slots (including
    // any escape slot ReadValue handed us) are now free-reuse region. The
    // wire handed to Go must NOT live there - re-root it into the Go
    // scope's own region (no shim-local HandleScope is open, so this
    // allocation advances the Go scope's next permanently, exactly like
    // the value-producing exports of the base slice).
    v8::Local<v8::Value> rooted = v8::Local<v8::Value>::New(iso, local);
    *out = ToWire(rooted);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_deserializer_read_value_del");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_deserializer_read_header_del(
    void* deserw, void* ctxw, void* tcw, void* scope, int32_t* out_ok) {
  ClearErr();
  gov8_serdel_impl::DeserDelWrap* dw = gov8_serdel_impl::AsDeserDel(deserw);
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
    SetErr("C++ exception in value_deserializer_read_header_del");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t
gov8_value_deserializer_transfer_array_buffer_del(void* deserw, void* scope,
                                                  uint32_t transfer_id,
                                                  void* ab_wire) {
  ClearErr();
  gov8_serdel_impl::DeserDelWrap* dw = gov8_serdel_impl::AsDeserDel(deserw);
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
    SetErr("C++ exception in transfer_array_buffer_del");
    return kErrCpp;
  }
}

// v8::ValueDeserializer::TransferSharedArrayBuffer. The pinned oracle pins
// that SAB READS never consult these registrations (the delegate hook is
// always used); the registration exists for wire-format completeness.
__declspec(dllexport) int64_t
gov8_value_deserializer_transfer_shared_array_buffer_del(void* deserw,
                                                         void* scope,
                                                         uint32_t id,
                                                         void* sab_wire) {
  ClearErr();
  gov8_serdel_impl::DeserDelWrap* dw = gov8_serdel_impl::AsDeserDel(deserw);
  if (dw == nullptr || sab_wire == nullptr || !ScopeIs(dw->iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(dw->iso);
    v8::HandleScope hs(dw->iso);
    dw->deser->TransferSharedArrayBuffer(id,
                                         FromWire<v8::SharedArrayBuffer>(
                                             sab_wire));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in transfer_shared_array_buffer_del");
    return kErrCpp;
  }
}

// Helper reads for use inside (or after) ReadHostObject. *out_ok reports
// the V8_WARN_UNUSED_RESULT bool; these never throw.
__declspec(dllexport) int64_t gov8_value_deserializer_read_uint32_del(
    void* deserw, uint32_t* out, int32_t* out_ok) {
  ClearErr();
  gov8_serdel_impl::DeserDelWrap* dw = gov8_serdel_impl::AsDeserDel(deserw);
  if (dw == nullptr || out == nullptr || out_ok == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(dw->iso);
    *out_ok = dw->deser->ReadUint32(out) ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in read_uint32_del");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_deserializer_read_uint64_del(
    void* deserw, uint64_t* out, int32_t* out_ok) {
  ClearErr();
  gov8_serdel_impl::DeserDelWrap* dw = gov8_serdel_impl::AsDeserDel(deserw);
  if (dw == nullptr || out == nullptr || out_ok == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(dw->iso);
    *out_ok = dw->deser->ReadUint64(out) ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in read_uint64_del");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_deserializer_read_double_del(
    void* deserw, double* out, int32_t* out_ok) {
  ClearErr();
  gov8_serdel_impl::DeserDelWrap* dw = gov8_serdel_impl::AsDeserDel(deserw);
  if (dw == nullptr || out == nullptr || out_ok == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(dw->iso);
    *out_ok = dw->deser->ReadDouble(out) ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in read_double_del");
    return kErrCpp;
  }
}

// ReadRawBytes copies into (buf, cap) - the engine's pointer points into
// the caller's input slice, but the copy keeps the Go helper's contract
// ("bytes are owned by Go") and survives any later input reuse. cap < len
// fails with kErrNoMemory without consuming the bytes (V8 only advanced
// its position on success).
__declspec(dllexport) int64_t gov8_value_deserializer_read_raw_bytes_del(
    void* deserw, int64_t length, char* buf, int64_t cap, int64_t* out_len,
    int32_t* out_ok) {
  ClearErr();
  gov8_serdel_impl::DeserDelWrap* dw = gov8_serdel_impl::AsDeserDel(deserw);
  if (dw == nullptr || out_len == nullptr || out_ok == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (length < 0 || cap < 0 || (buf == nullptr && cap > 0)) {
    SetErr("invalid byte range");
    return kErrBadArg;
  }
  if (length > cap) {
    SetErr("output buffer too small");
    return kErrNoMemory;
  }
  try {
    Gov8IsolateScope iso_scope(dw->iso);
    const void* data = nullptr;
    if (!dw->deser->ReadRawBytes(static_cast<size_t>(length), &data)) {
      *out_ok = 0;
      *out_len = 0;
      return kOk;
    }
    if (length > 0) {
      memcpy(buf, data, static_cast<size_t>(length));
    }
    *out_len = length;
    *out_ok = 1;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in read_raw_bytes_del");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_value_deserializer_wire_format_version_del(
    void* deserw, uint32_t* out) {
  ClearErr();
  gov8_serdel_impl::DeserDelWrap* dw = gov8_serdel_impl::AsDeserDel(deserw);
  if (dw == nullptr || out == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(dw->iso);
    *out = dw->deser->GetWireFormatVersion();
    return kOk;
  } catch (...) {
    SetErr("C++ exception in wire_format_version_del");
    return kErrCpp;
  }
}

// Destroys the engine deserializer (releasing the caller's data pointer),
// then the delegate, then the wrap.
__declspec(dllexport) int64_t gov8_value_deserializer_dispose_del(
    void* deserw) {
  ClearErr();
  gov8_serdel_impl::DeserDelWrap* dw = gov8_serdel_impl::AsDeserDel(deserw);
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
      reinterpret_cast<gov8_serdel_impl::Gov8DeserDelX*>(dw->delegate_storage)
          ->~Gov8DeserDelX();
      dw->built = false;
    }
    delete dw;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in value_deserializer_dispose_del");
    return kErrCpp;
  }
}

// v8::Exception::RangeError for delegates that throw their own error from
// inside a hook (the companion of the buffers slice's gov8_exception_error).
__declspec(dllexport) void* gov8_exception_range_error(v8::Isolate* iso,
                                                       void* scope,
                                                       void* message_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return ToWire(
        v8::Exception::RangeError(FromWire<v8::String>(message_wire)));
  } catch (...) {
    SetErr("C++ exception in exception_range_error");
    return nullptr;
  }
}

}  // extern "C"
