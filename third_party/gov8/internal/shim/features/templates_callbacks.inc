// templates_callbacks.inc -- template, callback, accessor, internal-field and
// External exports (feature slice).
//
// This file is textually included at the end of shim.cc, after the anonymous
// namespace with the shared helpers (ToWire/FromWire/AsCtx/AsScope/ScopeIs/
// OwnedBy/ValueText/BufferUtf8, the kErr* status codes and tls_error), so all
// of those are in scope here. Exports below re-open `extern "C"`.
//
// ---------------------------------------------------------------------------
// Native callback dispatch (Go-side callbacks)
//
// The engine invokes every Go callback through three small static C++
// trampolines defined here (one per engine callback shape):
//
//   FunctionCallback               void (*)(const FunctionCallbackInfo<Value>&)
//   AccessorNameGetterCallback     void (*)(Local<Name>, const PropertyCallbackInfo<Value>&)
//   AccessorNameSetterCallback     void (*)(Local<Name>, Local<Value>, const PropertyCallbackInfo<void>&)
//
// V8 stores exactly one host-visible value per callback: the `data` value
// given at template/function creation time, observable through
// `info.Data()`. That channel carries the dispatch handle, so it cannot also
// carry the embedder's own callback data verbatim. The design therefore is:
//
//   - At registration the shim heap-allocates a GoCallbackContext holding the
//     integer Go-registry handle and a v8::Global<v8::Value> copy of the
//     embedder data, wraps its address in a v8::External, and passes that
//     External as V8's `data`.
//   - The trampoline unwraps the context, re-materializes the *embedder*
//     data local (from the Global) plus the receiver / new.target /
//     arguments wires in its own HandleScope, packs everything into a
//     CallbackFrame on its stack, and calls the single Go dispatcher entry
//     point (one uintptr argument, registered by Go via
//     gov8_host_set_dispatcher).
//   - The Go dispatcher (root package callback.go) resolves the handle to
//     the Go closures, enforces thread affinity, borrows the trampoline's
//     HandleScope through a stack GoScope token, and runs the closure. It
//     recovers Go panics: a panic in a
//     native callback prints the panic message to stderr and terminates the
//     process through gov8_host_panic_abort (fail-fast abort, exit code
//     0xC0000409) -- the same observable process-fatal boundary the pinned
//     Rust oracle characterizes for panics unwinding out of its `extern "C"`
//     trampoline. Unwinding out of an engine callback frame in Go is
//     unsupported, so "recover + fail-fast abort" is the intentional,
//     documented equivalent.
//
// GoCallbackContext objects are owned by the Go layer, which releases them
// deterministically (gov8_host_contexts_release) before Isolate::Dispose
// (Global handles must be reset while the isolate lives). An embedder that
// never releases leaks one small heap object per created
// template/function/accessor until process exit; V8 itself reclaims the
// Global handle nodes when the isolate is destroyed, and the shim's
// live-set validation ensures a freed-or-abandoned context is never
// dispatched again.

#include <cstdlib>
#include <mutex>
#include <unordered_set>

namespace {

// Dispatch kinds packed into CallbackFrame::kind.
constexpr int32_t kCbKindFunction = 0;
constexpr int32_t kCbKindAccessorGet = 1;
constexpr int32_t kCbKindAccessorSet = 2;
// template-advanced interceptor kinds. Named kinds carry the key in
// property_wire; indexed kinds carry it in index. Enumerator kinds return
// their result array through out_wire; every other interceptor kind returns
// its Intercepted verdict through out_intercepted.
constexpr int32_t kCbKindNamedGet = 3;
constexpr int32_t kCbKindNamedSet = 4;
constexpr int32_t kCbKindNamedQuery = 5;
constexpr int32_t kCbKindNamedDelete = 6;
constexpr int32_t kCbKindNamedEnum = 7;
constexpr int32_t kCbKindNamedDefine = 8;
constexpr int32_t kCbKindNamedDescriptor = 9;
constexpr int32_t kCbKindIndexedGet = 10;
constexpr int32_t kCbKindIndexedSet = 11;
constexpr int32_t kCbKindIndexedQuery = 12;
constexpr int32_t kCbKindIndexedDelete = 13;
constexpr int32_t kCbKindIndexedEnum = 14;
constexpr int32_t kCbKindIndexedDefine = 15;
constexpr int32_t kCbKindIndexedDescriptor = 16;

// PropertyDescriptor presence bits packed into CallbackFrame::pd_flags
// (mirrored by the Go PropertyDescriptor view).
constexpr int32_t kPdFlagHasValue = 1;
constexpr int32_t kPdFlagHasWritable = 2;
constexpr int32_t kPdFlagHasEnumerable = 4;
constexpr int32_t kPdFlagHasConfigurable = 8;

// The single Go dispatcher entry point (from syscall.NewCallback on the Go
// side). Zero until gov8_host_set_dispatcher runs; it is set exactly once
// before any callback can be registered (synchronized by Go's sync.Once).
uintptr_t g_go_dispatch = 0;

// One GoCallbackContext per registered native callback (function template,
// One GoCallbackContext per registered native callback (function template,
// standalone function, or accessor pair). Contexts are handed to the Go side
// as opaque pointers at registration time; the Go layer groups them by
// isolate and releases them explicitly (gov8_host_contexts_release) before
// the isolate is disposed. The shim keeps only a live-set for validation,
// never a per-isolate map, so a disposed isolate whose embedder skipped the
// release can never leave dangling state that a later isolate (reusing the
// address) would trip over.
struct GoCallbackContext {
  uint64_t handle = 0;
  v8::Global<v8::Value> user_data;
};

std::mutex g_host_mu;
// Membership is the only ordering-independent operation on this collection:
// registration inserts, callback entry probes, and deterministic teardown
// erases. Keep the mutex so a successful probe still excludes reclamation,
// but avoid logarithmic tree walks as FunctionNewCall grows the live registry.
std::unordered_set<GoCallbackContext*> g_live_contexts;
// Direct-return IntegerValue uses a per-thread result slot because the full
// int64 domain leaves no in-band value for conversion failure. The export
// stores only after any nested JavaScript/Go re-entry has completed and returns
// the native slot's address; Go reads it immediately on the locked owner thread.
// Thus no pointer into the movable Go stack crosses a re-entrant engine call.
thread_local int64_t g_wctx_integer_value_result = 0;

GoCallbackContext* HostContextNew(v8::Isolate* iso, uint64_t handle,
                                  void* data_wire) {
  GoCallbackContext* ctx = new GoCallbackContext{};
  ctx->handle = handle;
  if (data_wire != nullptr) {
    ctx->user_data.Reset(iso, FromWire<v8::Value>(data_wire));
  }
  {
    std::lock_guard<std::mutex> lock(g_host_mu);
    g_live_contexts.insert(ctx);
  }
  return ctx;
}

// Frees one context: resets its Global handle and deletes it. Unknown
// pointers (already freed or never allocated) are ignored.
void HostContextFree(v8::Isolate* iso, GoCallbackContext* ctx) {
  {
    std::lock_guard<std::mutex> lock(g_host_mu);
    if (g_live_contexts.erase(ctx) == 0) {
      return;
    }
  }
  Gov8IsolateScope iso_scope(iso);
  ctx->user_data.Reset();
  delete ctx;
}

// Wraps the context in a v8::External local (created in the caller's open
// HandleScope) so V8 keeps the dispatch channel alive with the template.
void* HostContextExternalWire(v8::Isolate* iso, GoCallbackContext* ctx) {
  if (ctx == nullptr) {
    SetErr("callback context allocation failed");
    return nullptr;
  }
  return ToWire(v8::External::New(iso, ctx, v8::kExternalPointerTypeTagDefault));
}

GoCallbackContext* CallbackContextFromData(v8::Isolate* iso,
                                           v8::Local<v8::Value> data) {
  if (data.IsEmpty() || !data->IsExternal()) {
    return nullptr;
  }
  GoCallbackContext* ctx =
      static_cast<GoCallbackContext*>(
          data.As<v8::External>()->Value(v8::kExternalPointerTypeTagDefault));
  if (ctx == nullptr) {
    return nullptr;
  }
  std::lock_guard<std::mutex> lock(g_host_mu);
  if (g_live_contexts.count(ctx) == 0) {
    return nullptr;
  }
  return ctx;
}

// The frame passed to the Go dispatcher. All wires are slots in the
// trampoline's HandleScope, which stays open for the whole synchronous Go
// callback. argc/argv are valid only for kCbKindFunction. ctx_wire is the
// engine's current context captured at dispatch time (the context-bound
// operations in callbacks use it; there is deliberately no per-callback
// persistent context wrapper).
struct CallbackFrame {
  int32_t kind;
  int32_t flags;  // bit 0: IsConstructCall (function kind) /
                  //        ShouldThrowOnError (interceptor kinds)
  uint64_t handle;
  int64_t argc;
  void** argv;
  v8::Isolate* isolate;
  void* scope_wire;      // stack GoScope for the trampoline's HandleScope
  void* ctx_wire;        // Isolate::GetCurrentContext()
  void* data_wire;       // embedder callback data (unwrapped from the context)
  void* this_wire;       // This() / HolderV2()
  void* new_target_wire; // NewTarget() (function kind, else nullptr)
  void* property_wire;   // accessor name / named interceptor key
  void* value_wire;      // assigned value (accessor setter / interceptor setter)
  uintptr_t rv_word;     // ReturnValue raw word (getter/function kinds)
  // --- template-advanced extension (field-for-field mirrored by the Go
  // hostCallbackFrame; keep the two layouts in sync) ---
  uint32_t index;          // indexed interceptor key
  int32_t out_intercepted; // Go writes the Intercepted verdict (kYes=0/kNo=1)
  void* holder_wire;       // PropertyCallbackInfo::Holder()
  void* out_wire;          // Go writes the enumerator result array
  int32_t should_throw;    // ShouldThrowOnError() snapshot (duplicates flags)
  int32_t pd_flags;        // PropertyDescriptor presence bits
  int32_t pd_writable;
  int32_t pd_enumerable;
  int32_t pd_configurable;
  int32_t pd_pad;          // explicit alignment pad
  void* pd_value_wire;     // PropertyDescriptor value (when present)
};

#if defined(_WIN64)
static_assert(offsetof(CallbackFrame, scope_wire) == 40);
static_assert(sizeof(CallbackFrame) == 160);
#endif

// Owns the actual V8 HandleScope used by one callback dispatch and exposes a
// validation-only GoScope token for synchronous Go re-entry. hs_storage in the
// token is intentionally unused: ScopeIs reads only magic/iso, while handles
// owns the active V8 scope. The token is invalidated before handles closes.
class CallbackDispatchScope final {
 public:
  CallbackDispatchScope(v8::Isolate* iso, CallbackFrame* frame)
      : handles_(iso), token_{} {
    token_.magic = kScopeMagic;
    token_.iso = iso;
    frame->scope_wire = &token_;
  }

  ~CallbackDispatchScope() {
    token_.magic = 0;
    token_.iso = nullptr;
  }

  CallbackDispatchScope(const CallbackDispatchScope&) = delete;
  CallbackDispatchScope& operator=(const CallbackDispatchScope&) = delete;

 private:
  v8::HandleScope handles_;
  GoScope token_;
};

int64_t CallGoDispatcher(CallbackFrame* frame) {
  if (g_go_dispatch == 0) {
    // A callback exists but the Go side never registered its dispatcher:
    // hard programming error.
    std::abort();
  }
  using DispatchFn = uintptr_t (*)(uintptr_t);
  return static_cast<int64_t>(
      reinterpret_cast<DispatchFn>(g_go_dispatch)(reinterpret_cast<uintptr_t>(frame)));
}

int64_t CallGoDispatcherWithScope(CallbackFrame* frame) {
  CallbackDispatchScope callback_scope(frame->isolate, frame);
  return CallGoDispatcher(frame);
}

constexpr int32_t kHostFunctionDeferredRVInt32Magic = 0x47565231;  // "GVR1"
constexpr int32_t kHostFunctionDeferredRVNone = 0;
constexpr int32_t kHostFunctionDeferredRVInt32 = 1;
constexpr int32_t kHostFunctionInt32ArgsMagic = 0x47564131;  // "GVA1"
constexpr int32_t kHostFunctionInt32Arg0Valid = 1 << 0;
constexpr int32_t kHostFunctionInt32Arg1Valid = 1 << 1;

// Function callbacks may defer their overwhelmingly common terminal int32
// result into fields that are otherwise used only by interceptor callbacks.
// The capability marker keeps this bidirectionally compatible with pre-GVR1
// Go/DLL pairs: older Go leaves the state at zero, while newer Go falls back
// to the exported setter when an older DLL leaves the marker at zero.
int64_t CallGoFunctionDispatcherWithScope(
    CallbackFrame* frame, v8::ReturnValue<v8::Value> return_value) {
  CallbackDispatchScope callback_scope(frame->isolate, frame);
  frame->pd_pad = kHostFunctionDeferredRVInt32Magic;
  const int64_t status = CallGoDispatcher(frame);
  if (status != 0) {
    return status;
  }
  switch (frame->out_intercepted) {
    case kHostFunctionDeferredRVNone:
      return status;
    case kHostFunctionDeferredRVInt32: {
      static_assert(sizeof(frame->index) == sizeof(int32_t));
      int32_t value = 0;
      std::memcpy(&value, &frame->index, sizeof(value));
      return_value.Set(value);
      frame->out_intercepted = kHostFunctionDeferredRVNone;
      return status;
    }
    default:
      return kErrBadArg;
  }
}

void AbortBadCallbackStatus() {
  // The Go dispatcher reported a misuse it could not recover from (wrong
  // thread, stale registration). Like the oracle's fail-fast boundary this
  // must not return into the engine.
  std::abort();
}

// --- static engine-callback trampolines -------------------------------------

constexpr size_t kHostFunctionInlineArgc = 2;

void HostFunctionTrampoline(const v8::FunctionCallbackInfo<v8::Value>& info) {
  v8::Isolate* iso = info.GetIsolate();
  // The isolate is already entered: the engine only invokes API callbacks
  // while executing script/host work on the owning thread.
  CallbackFrame frame{};
  GoCallbackContext* ctx = CallbackContextFromData(iso, info.Data());
  if (ctx == nullptr) {
    std::abort();
  }
  v8::ReturnValue<v8::Value> rv = info.GetReturnValue();
  frame.kind = kCbKindFunction;
  frame.flags = info.IsConstructCall() ? 1 : 0;
  frame.handle = ctx->handle;
  frame.isolate = iso;
  frame.ctx_wire = ToWire(iso->GetCurrentContext());
  frame.data_wire = ToWire(ctx->user_data.Get(iso));
  frame.this_wire = ToWire(info.This());
  frame.new_target_wire = ToWire(info.NewTarget());
  frame.rv_word = *reinterpret_cast<uintptr_t*>(&rv);
  // Function callbacks do not use the property-descriptor extension fields.
  // Snapshot only positive IsInt32 results for the two common argument slots;
  // all other values remain uncached so their coercions happen lazily and on
  // every Go request. A separate marker keeps current-baseline ABI-42 DLLs on
  // the native fallback path without changing the frame layout or exports.
  frame.pd_configurable = kHostFunctionInt32ArgsMagic;
  const int argc = info.Length();
  frame.argc = argc;
  void* inline_argv[kHostFunctionInlineArgc];
  std::unique_ptr<void*[]> heap_argv;
  if (argc > 0) {
    void** argv = inline_argv;
    if (static_cast<size_t>(argc) > kHostFunctionInlineArgc) {
      heap_argv.reset(new void*[static_cast<size_t>(argc)]);
      argv = heap_argv.get();
    }
    for (int i = 0; i < argc; i++) {
      const v8::Local<v8::Value> argument = info[i];
      argv[i] = ToWire(argument);
      if (i < 2 && argument->IsInt32()) {
        const int32_t value = argument.As<v8::Int32>()->Value();
        if (i == 0) {
          frame.pd_flags |= kHostFunctionInt32Arg0Valid;
          frame.pd_writable = value;
        } else {
          frame.pd_flags |= kHostFunctionInt32Arg1Valid;
          frame.pd_enumerable = value;
        }
      }
    }
    frame.argv = argv;
  }
  if (CallGoFunctionDispatcherWithScope(&frame, rv) != 0) {
    AbortBadCallbackStatus();
  }
}

void HostAccessorGetTrampoline(v8::Local<v8::Name> property,
                               const v8::PropertyCallbackInfo<v8::Value>& info) {
  v8::Isolate* iso = info.GetIsolate();
  CallbackFrame frame{};
  GoCallbackContext* ctx = CallbackContextFromData(iso, info.Data());
  if (ctx == nullptr) {
    std::abort();
  }
  v8::ReturnValue<v8::Value> rv = info.GetReturnValue();
  frame.kind = kCbKindAccessorGet;
  frame.handle = ctx->handle;
  frame.isolate = iso;
  frame.ctx_wire = ToWire(iso->GetCurrentContext());
  frame.data_wire = ToWire(ctx->user_data.Get(iso));
  frame.this_wire = ToWire(info.HolderV2());
  frame.property_wire = ToWire(property);
  frame.rv_word = *reinterpret_cast<uintptr_t*>(&rv);
  if (CallGoDispatcherWithScope(&frame) != 0) {
    AbortBadCallbackStatus();
  }
}

void HostAccessorSetTrampoline(v8::Local<v8::Name> property,
                               v8::Local<v8::Value> value,
                               const v8::PropertyCallbackInfo<void>& info) {
  v8::Isolate* iso = info.GetIsolate();
  CallbackFrame frame{};
  GoCallbackContext* ctx = CallbackContextFromData(iso, info.Data());
  if (ctx == nullptr) {
    std::abort();
  }
  frame.kind = kCbKindAccessorSet;
  frame.handle = ctx->handle;
  frame.isolate = iso;
  frame.ctx_wire = ToWire(iso->GetCurrentContext());
  frame.data_wire = ToWire(ctx->user_data.Get(iso));
  frame.this_wire = ToWire(info.HolderV2());
  frame.property_wire = ToWire(property);
  frame.value_wire = ToWire(value);
  if (CallGoDispatcherWithScope(&frame) != 0) {
    AbortBadCallbackStatus();
  }
}

// --- interceptor trampolines (template-advanced slice) -----------------------
//
// The named/indexed property handler families dispatch through these static
// engine callbacks. They share the accessor/function trampoline protocol:
// one HandleScope, the dispatch External resolved from info.Data(), a stack
// CallbackFrame, and the single Go dispatcher call. The Intercepted verdict
// comes back through frame.out_intercepted; enumerator results come back
// through frame.out_wire.

// Fills the interceptor-independent part of the frame from a
// PropertyCallbackInfo and returns the resolved dispatch context.
template <class T>
GoCallbackContext* SetupInterceptorFrame(
    const v8::PropertyCallbackInfo<T>& info, CallbackFrame* frame,
    int32_t kind) {
  v8::Isolate* iso = info.GetIsolate();
  GoCallbackContext* ctx = CallbackContextFromData(iso, info.Data());
  if (ctx == nullptr) {
    std::abort();
  }
  const bool should_throw = info.ShouldThrowOnError();
  frame->kind = kind;
  frame->flags = should_throw ? 1 : 0;
  frame->handle = ctx->handle;
  frame->isolate = iso;
  frame->ctx_wire = ToWire(iso->GetCurrentContext());
  frame->data_wire = ToWire(ctx->user_data.Get(iso));
  // v8 152's PropertyCallbackInfo exposes only Holder() (the object in the
  // prototype chain carrying the interceptor); there is no separate This().
  // The holder is captured in both slots so the Go wrapper observes one
  // receiver consistently.
  frame->holder_wire = ToWire(info.Holder());
  frame->this_wire = frame->holder_wire;
  frame->new_target_wire = nullptr;
  frame->property_wire = nullptr;
  frame->value_wire = nullptr;
  frame->index = 0;
  frame->out_intercepted = 1;  // defensive default: Intercepted::kNo
  frame->out_wire = nullptr;
  frame->should_throw = should_throw ? 1 : 0;
  frame->pd_flags = 0;
  frame->pd_writable = 0;
  frame->pd_enumerable = 0;
  frame->pd_configurable = 0;
  frame->pd_pad = 0;
  frame->pd_value_wire = nullptr;
  // The ReturnValue word drives the Go-side typed setters, exactly like the
  // function/accessor trampolines.
  v8::ReturnValue<T> rv = info.GetReturnValue();
  frame->rv_word = *reinterpret_cast<uintptr_t*>(&rv);
  return ctx;
}

// Copies the PropertyDescriptor view into the frame for definer callbacks.
void FillDescriptorFrame(const v8::PropertyDescriptor& desc,
                         CallbackFrame* frame) {
  int32_t flags = 0;
  if (desc.has_value()) {
    flags |= kPdFlagHasValue;
    frame->pd_value_wire = ToWire(desc.value());
  }
  if (desc.has_writable()) {
    flags |= kPdFlagHasWritable;
    frame->pd_writable = desc.writable() ? 1 : 0;
  }
  if (desc.has_enumerable()) {
    flags |= kPdFlagHasEnumerable;
    frame->pd_enumerable = desc.enumerable() ? 1 : 0;
  }
  if (desc.has_configurable()) {
    flags |= kPdFlagHasConfigurable;
    frame->pd_configurable = desc.configurable() ? 1 : 0;
  }
  // Definer callbacks do not use setter value or constructor new-target.
  // Carry accessor handles in those kind-specific slots without changing ABI.
  if (desc.has_get()) { flags |= 16; frame->new_target_wire = ToWire(desc.get()); }
  if (desc.has_set()) { flags |= 32; frame->value_wire = ToWire(desc.set()); }
  frame->pd_flags = flags;
}

// Runs the Go dispatcher for an interceptor frame and returns the Intercepted
// verdict the Go callback reported.
v8::Intercepted DispatchInterceptor(CallbackFrame* frame) {
  if (CallGoDispatcherWithScope(frame) != 0) {
    AbortBadCallbackStatus();
  }
  return static_cast<v8::Intercepted>(frame->out_intercepted);
}

v8::Intercepted HostNamedGetTrampoline(v8::Local<v8::Name> property,
                                   const v8::PropertyCallbackInfo<v8::Value>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindNamedGet);
  frame.property_wire = ToWire(property);
  return DispatchInterceptor(&frame);
}

v8::Intercepted HostNamedSetTrampoline(v8::Local<v8::Name> property,
                                   v8::Local<v8::Value> value,
                                   const v8::PropertyCallbackInfo<v8::Boolean>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindNamedSet);
  frame.property_wire = ToWire(property);
  frame.value_wire = ToWire(value);
  return DispatchInterceptor(&frame);
}

v8::Intercepted HostNamedQueryTrampoline(v8::Local<v8::Name> property,
                                     const v8::PropertyCallbackInfo<v8::Integer>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindNamedQuery);
  frame.property_wire = ToWire(property);
  return DispatchInterceptor(&frame);
}

v8::Intercepted HostNamedDeleteTrampoline(v8::Local<v8::Name> property,
                                      const v8::PropertyCallbackInfo<v8::Boolean>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindNamedDelete);
  frame.property_wire = ToWire(property);
  return DispatchInterceptor(&frame);
}

void HostNamedEnumTrampoline(const v8::PropertyCallbackInfo<v8::Array>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindNamedEnum);
  CallbackDispatchScope callback_scope(frame.isolate, &frame);
  if (CallGoDispatcher(&frame) != 0) {
    AbortBadCallbackStatus();
  }
  if (frame.out_wire != nullptr) {
    info.GetReturnValue().Set(FromWire<v8::Array>(frame.out_wire));
  }
}

v8::Intercepted HostNamedDefineTrampoline(v8::Local<v8::Name> property,
                                      const v8::PropertyDescriptor& desc,
                                      const v8::PropertyCallbackInfo<v8::Boolean>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindNamedDefine);
  frame.property_wire = ToWire(property);
  FillDescriptorFrame(desc, &frame);
  return DispatchInterceptor(&frame);
}

v8::Intercepted HostNamedDescriptorTrampoline(
    v8::Local<v8::Name> property, const v8::PropertyCallbackInfo<v8::Value>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindNamedDescriptor);
  frame.property_wire = ToWire(property);
  return DispatchInterceptor(&frame);
}

v8::Intercepted HostIndexedGetTrampoline(uint32_t index,
                                     const v8::PropertyCallbackInfo<v8::Value>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindIndexedGet);
  frame.index = index;
  return DispatchInterceptor(&frame);
}

v8::Intercepted HostIndexedSetTrampoline(uint32_t index, v8::Local<v8::Value> value,
                                     const v8::PropertyCallbackInfo<v8::Boolean>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindIndexedSet);
  frame.index = index;
  frame.value_wire = ToWire(value);
  return DispatchInterceptor(&frame);
}

v8::Intercepted HostIndexedQueryTrampoline(uint32_t index,
                                       const v8::PropertyCallbackInfo<v8::Integer>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindIndexedQuery);
  frame.index = index;
  return DispatchInterceptor(&frame);
}

v8::Intercepted HostIndexedDeleteTrampoline(uint32_t index,
                                        const v8::PropertyCallbackInfo<v8::Boolean>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindIndexedDelete);
  frame.index = index;
  return DispatchInterceptor(&frame);
}

void HostIndexedEnumTrampoline(const v8::PropertyCallbackInfo<v8::Array>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindIndexedEnum);
  CallbackDispatchScope callback_scope(frame.isolate, &frame);
  if (CallGoDispatcher(&frame) != 0) {
    AbortBadCallbackStatus();
  }
  if (frame.out_wire != nullptr) {
    info.GetReturnValue().Set(FromWire<v8::Array>(frame.out_wire));
  }
}

v8::Intercepted HostIndexedDefineTrampoline(uint32_t index,
                                         const v8::PropertyDescriptor& desc,
                                         const v8::PropertyCallbackInfo<v8::Boolean>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindIndexedDefine);
  frame.index = index;
  FillDescriptorFrame(desc, &frame);
  return DispatchInterceptor(&frame);
}

v8::Intercepted HostIndexedDescriptorTrampoline(
    uint32_t index, const v8::PropertyCallbackInfo<v8::Value>& info) {
  CallbackFrame frame{};
  SetupInterceptorFrame(info, &frame, kCbKindIndexedDescriptor);
  frame.index = index;
  return DispatchInterceptor(&frame);
}

// Shared registration helper: validates the Go-owned host context and builds
// its dispatch External, then hands the data value to `make`. Returns the
// wire of the engine object produced by make (or nullptr on failure).
template <class MakeFn>
void* RegisterNativeCallback(v8::Isolate* iso, GoCallbackContext* ctx,
                             MakeFn&& make) {
  void* data_external = HostContextExternalWire(iso, ctx);
  if (data_external == nullptr) {
    return nullptr;
  }
  return make(FromWire<v8::Value>(data_external));
}

// Status-based variant of RegisterNativeCallback for exports following the
// (int64 status, void** out) convention.
template <class MakeFn>
int64_t RegisterNativeCallbackStatus(v8::Isolate* iso, GoCallbackContext* ctx,
                                     MakeFn&& make, void** out) {
  void* data_external = HostContextExternalWire(iso, ctx);
  if (data_external == nullptr) {
    SetErr("callback context allocation failed");
    return kErrNoMemory;
  }
  return make(FromWire<v8::Value>(data_external), out);
}

}  // namespace

extern "C" {

// --- Go dispatcher registration / panic boundary / host contexts -------------

// Registers the single Go dispatcher entry point (a syscall.NewCallback
// pointer). Must be called before any native callback is created. Passing 0
// is rejected. Returns kOk.
__declspec(dllexport) int64_t gov8_host_set_dispatcher(uintptr_t dispatch) {
  ClearErr();
  if (dispatch == 0) {
    SetErr("null dispatcher");
    return kErrBadArg;
  }
  g_go_dispatch = dispatch;
  return kOk;
}

// Fail-fast abort used by the Go dispatcher after a recovered panic in a
// native callback. On Windows this terminates the process with the
// fail-fast code 0xC0000409, mirroring the pinned oracle's panic boundary.
__declspec(dllexport) void gov8_host_panic_abort(void) {
  std::abort();
}

// Allocates a native-callback dispatch context for the Go handle. The
// returned pointer is owned by the Go layer, which must eventually pass it
// to gov8_host_context_free or gov8_host_contexts_release on the isolate's
// owning thread while the isolate is still alive.
__declspec(dllexport) void* gov8_host_context_new(v8::Isolate* iso,
                                                  uint64_t handle,
                                                  void* data_wire) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return HostContextNew(iso, handle, data_wire);
  } catch (...) {
    SetErr("C++ exception in host_context_new");
    return nullptr;
  }
}

// Frees one dispatch context (error paths only; batch release through
// gov8_host_contexts_release otherwise).
__declspec(dllexport) int64_t gov8_host_context_free(v8::Isolate* iso,
                                                     void* ctx_ptr) {
  ClearErr();
  GoCallbackContext* ctx = static_cast<GoCallbackContext*>(ctx_ptr);
  if (iso == nullptr || ctx == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    HostContextFree(iso, ctx);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in host_context_free");
    return kErrCpp;
  }
}

// Releases a batch of dispatch contexts belonging to one isolate: resets
// their Global embedder-data handles and frees them. The contexts array is
// only read during this call.
__declspec(dllexport) int64_t gov8_host_contexts_release(
    v8::Isolate* iso, void** contexts, int64_t count) {
  ClearErr();
  if (iso == nullptr || (contexts == nullptr && count > 0)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    if (count > 0) {
      Gov8IsolateScope iso_scope(iso);
      for (int64_t i = 0; i < count; i++) {
        HostContextFree(iso, static_cast<GoCallbackContext*>(contexts[i]));
      }
    }
    return kOk;
  } catch (...) {
    SetErr("C++ exception in host_contexts_release");
    return kErrCpp;
  }
}

// --- function templates ------------------------------------------------------

// Creates a FunctionTemplate whose native callback dispatches through the
// Go-owned host context. `length` is the JS fn.length. Uses
// ConstructorBehavior::kAllow and SideEffectType::kHasSideEffect (the pinned
// oracle defaults).
__declspec(dllexport) void* gov8_function_template_new(
    v8::Isolate* iso, void* scope, void* host_ctx, int32_t length) {
  ClearErr();
  GoCallbackContext* ctx = static_cast<GoCallbackContext*>(host_ctx);
  if (iso == nullptr || !ScopeIs(iso, scope) || ctx == nullptr) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return RegisterNativeCallback(
        iso, ctx, [&](v8::Local<v8::Value> data) -> void* {
          v8::Local<v8::FunctionTemplate> tmpl = v8::FunctionTemplate::New(
              iso, HostFunctionTrampoline, data, v8::Local<v8::Signature>(),
              length, v8::ConstructorBehavior::kAllow,
              v8::SideEffectType::kHasSideEffect);
          if (tmpl.IsEmpty()) {
            SetErr("FunctionTemplate::New failed");
            return nullptr;
          }
          return ToWire(tmpl);
        });
  } catch (...) {
    SetErr("C++ exception in function_template_new");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_function_template_set_class_name(
    v8::Isolate* iso, void* scope, void* ft_wire, void* name_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::FunctionTemplate>(ft_wire)->SetClassName(
        FromWire<v8::String>(name_wire));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in function_template_set_class_name");
    return kErrCpp;
  }
}

// Instantiates the template's function in the context. Returns the function
// wire, or nullptr when instantiation threw (exception left in the active
// TryCatch, if any).
__declspec(dllexport) void* gov8_function_template_get_function(
    v8::Isolate* iso, void* ctxw, void* scope, void* ft_wire) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::FunctionTemplate> tmpl =
        FromWire<v8::FunctionTemplate>(ft_wire);
    v8::Local<v8::Function> fn;
    if (!tmpl->GetFunction(ctx).ToLocal(&fn)) {
      SetErr("FunctionTemplate::GetFunction failed");
      return nullptr;
    }
    return ToWire(fn);
  } catch (...) {
    SetErr("C++ exception in function_template_get_function");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_function_template_prototype_template(
    v8::Isolate* iso, void* scope, void* ft_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return ToWire(FromWire<v8::FunctionTemplate>(ft_wire)->PrototypeTemplate());
  } catch (...) {
    SetErr("C++ exception in function_template_prototype_template");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_function_template_instance_template(
    v8::Isolate* iso, void* scope, void* ft_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return ToWire(FromWire<v8::FunctionTemplate>(ft_wire)->InstanceTemplate());
  } catch (...) {
    SetErr("C++ exception in function_template_instance_template");
    return nullptr;
  }
}

// Installs a getter/setter function-template pair as an accessor property.
// A null setter wire installs a getter-only accessor.
__declspec(dllexport) int64_t gov8_function_template_set_accessor_property(
    v8::Isolate* iso, void* scope, void* ft_wire, void* key_wire,
    void* getter_wire, void* setter_wire, int32_t attr) {
  ClearErr();
  if (!ScopeIs(iso, scope) ||
      (getter_wire == nullptr && setter_wire == nullptr)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::FunctionTemplate>(ft_wire)
        ->SetAccessorProperty(
            FromWire<v8::Name>(key_wire),
            getter_wire == nullptr ? v8::Local<v8::FunctionTemplate>()
                                   : FromWire<v8::FunctionTemplate>(getter_wire),
            setter_wire == nullptr ? v8::Local<v8::FunctionTemplate>()
                                   : FromWire<v8::FunctionTemplate>(setter_wire),
            static_cast<v8::PropertyAttribute>(attr));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in function_template_set_accessor_property");
    return kErrCpp;
  }
}

// --- standalone native functions (Function::New / builder) --------------------

// Creates a function object in the context, backed by the Go-owned host
// context. Mirrors v8::Function::New (ConstructorBehavior::kAllow).
__declspec(dllexport) int64_t gov8_function_new(v8::Isolate* iso, void* ctxw,
                                                void* scope, void* host_ctx,
                                                int32_t length, void** out) {
  ClearErr();
  GoCallbackContext* cbctx = static_cast<GoCallbackContext*>(host_ctx);
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !ScopeIs(iso, scope) || out == nullptr ||
      cbctx == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return RegisterNativeCallbackStatus(
        iso, cbctx, [&](v8::Local<v8::Value> data,
                        void** out) -> int64_t {
          v8::Local<v8::Function> fn;
          if (!v8::Function::New(ctx, HostFunctionTrampoline, data, length,
                                 v8::ConstructorBehavior::kAllow,
                                 v8::SideEffectType::kHasSideEffect)
                   .ToLocal(&fn)) {
            SetErr("Function::New failed");
            return kErrGeneric;
          }
          *out = ToWire(fn);
          return kOk;
        }, out);
  } catch (...) {
    SetErr("C++ exception in function_new");
    return kErrCpp;
  }
}

// Calls a function (Function::Call). Returns kOk with *out set, or
// kErrException when the call threw (the exception is recorded by the active
// TryCatch). argv is an array of `argc` value wires; it is only read during
// this call.
__declspec(dllexport) int64_t gov8_function_call_ctx(
    v8::Isolate* iso, void* ctxw, void* scope, void* fn_wire, void* recv_wire,
    int32_t argc, void** argv, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !ScopeIs(iso, scope) || out == nullptr ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value>* args =
        reinterpret_cast<v8::Local<v8::Value>*>(argv);
    v8::Local<v8::Value> result;
    if (!FromWire<v8::Function>(fn_wire)
             ->Call(ctx, FromWire<v8::Value>(recv_wire), argc, args)
             .ToLocal(&result)) {
      SetErr("Function::Call failed");
      return kErrException;
    }
    *out = ToWire(result);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in function_call_ctx");
    return kErrCpp;
  }
}

// Function::Call with a context WIRE (the engine's current context captured
// by the callback trampoline). Used for native re-entry into JavaScript from
// inside a running callback.
__declspec(dllexport) int64_t gov8_function_call_wctx(
    v8::Isolate* iso, void* ctx_wire, void* scope, void* fn_wire,
    void* recv_wire, int32_t argc, void** argv, void** out) {
  ClearErr();
  if (!ScopeIs(iso, scope) || ctx_wire == nullptr || out == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = FromWire<v8::Context>(ctx_wire);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value>* args =
        reinterpret_cast<v8::Local<v8::Value>*>(argv);
    v8::Local<v8::Value> result;
    if (!FromWire<v8::Function>(fn_wire)
             ->Call(ctx, FromWire<v8::Value>(recv_wire), argc, args)
             .ToLocal(&result)) {
      SetErr("Function::Call failed");
      return kErrException;
    }
    *out = ToWire(result);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in function_call_wctx");
    return kErrCpp;
  }
}

// Function::NewInstance (construct call from the host).
__declspec(dllexport) int64_t gov8_function_new_instance(
    v8::Isolate* iso, void* ctxw, void* scope, void* fn_wire, int32_t argc,
    void** argv, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !ScopeIs(iso, scope) || out == nullptr ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value>* args =
        reinterpret_cast<v8::Local<v8::Value>*>(argv);
    v8::Local<v8::Object> instance;
    if (!FromWire<v8::Function>(fn_wire)
             ->NewInstance(ctx, argc, args)
             .ToLocal(&instance)) {
      SetErr("Function::NewInstance failed");
      return kErrException;
    }
    *out = ToWire(instance);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in function_new_instance");
    return kErrCpp;
  }
}

__declspec(dllexport) void* gov8_function_get_name(v8::Isolate* iso,
                                                   void* scope,
                                                   void* fn_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return ToWire(FromWire<v8::Function>(fn_wire)->GetName());
  } catch (...) {
    SetErr("C++ exception in function_get_name");
    return nullptr;
  }
}

// --- object templates ---------------------------------------------------------

// ObjectTemplate::New. When `ft_wire` is non-null the template is derived
// from that function template (ObjectTemplate::new_from_template in the
// oracle): its instances inherit the function template's prototype.
__declspec(dllexport) void* gov8_object_template_new(v8::Isolate* iso,
                                                     void* scope,
                                                     void* ft_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::ObjectTemplate> tmpl =
        ft_wire == nullptr
            ? v8::ObjectTemplate::New(iso)
            : v8::ObjectTemplate::New(
                  iso, FromWire<v8::FunctionTemplate>(ft_wire));
    if (tmpl.IsEmpty()) {
      SetErr("ObjectTemplate::New failed");
      return nullptr;
    }
    return ToWire(tmpl);
  } catch (...) {
    SetErr("C++ exception in object_template_new");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_object_template_new_instance(
    v8::Isolate* iso, void* ctxw, void* scope, void* ot_wire, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !ScopeIs(iso, scope) || out == nullptr ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Object> instance;
    if (!FromWire<v8::ObjectTemplate>(ot_wire)
             ->NewInstance(ctx)
             .ToLocal(&instance)) {
      SetErr("ObjectTemplate::NewInstance failed");
      return kErrException;
    }
    *out = ToWire(instance);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in object_template_new_instance");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_template_set(v8::Isolate* iso, void* scope,
                                                void* tmpl_wire, void* key_wire,
                                                void* val_wire, int32_t attr) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::Template>(tmpl_wire)
        ->Set(FromWire<v8::Name>(key_wire), FromWire<v8::Data>(val_wire),
              static_cast<v8::PropertyAttribute>(attr));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in template_set");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_object_template_set_internal_field_count(
    v8::Isolate* iso, void* scope, void* ot_wire, int32_t count) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::ObjectTemplate>(ot_wire)->SetInternalFieldCount(count);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in object_template_set_internal_field_count");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_object_template_internal_field_count(
    v8::Isolate* iso, void* scope, void* ot_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return FromWire<v8::ObjectTemplate>(ot_wire)->InternalFieldCount();
  } catch (...) {
    SetErr("C++ exception in object_template_internal_field_count");
    return kErrCpp;
  }
}

// Installs a native data property with Go getter (and optionally setter)
// callbacks dispatched through the Go-owned host context. When
// `with_setter` is 0 the property is getter-only.
__declspec(dllexport) int64_t gov8_object_template_set_native_data_property(
    v8::Isolate* iso, void* scope, void* ot_wire, void* key_wire,
    void* host_ctx, int32_t with_setter, int32_t attr) {
  ClearErr();
  GoCallbackContext* cbctx = static_cast<GoCallbackContext*>(host_ctx);
  if (!ScopeIs(iso, scope) || cbctx == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    void* data_external = HostContextExternalWire(iso, cbctx);
    if (data_external == nullptr) {
      return kErrNoMemory;
    }
    v8::AccessorNameGetterCallback getter = HostAccessorGetTrampoline;
    v8::AccessorNameSetterCallback setter =
        with_setter != 0 ? HostAccessorSetTrampoline : nullptr;
    FromWire<v8::ObjectTemplate>(ot_wire)
        ->SetNativeDataProperty(
            FromWire<v8::Name>(key_wire), getter, setter,
            FromWire<v8::Value>(data_external),
            static_cast<v8::PropertyAttribute>(attr));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in object_template_set_native_data_property");
    return kErrCpp;
  }
}

// --- object internal fields ----------------------------------------------------

__declspec(dllexport) int64_t gov8_object_internal_field_count(
    v8::Isolate* iso, void* scope, void* obj_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return FromWire<v8::Object>(obj_wire)->InternalFieldCount();
  } catch (...) {
    SetErr("C++ exception in object_internal_field_count");
    return kErrCpp;
  }
}

// Bounds-checked by the Go wrapper before this call; the engine release
// build performs no bounds checking, so the wrapper is the safety boundary.
__declspec(dllexport) int64_t gov8_object_set_internal_field(
    v8::Isolate* iso, void* scope, void* obj_wire, int32_t index,
    void* data_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::Object>(obj_wire)
        ->SetInternalField(index, FromWire<v8::Data>(data_wire));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in object_set_internal_field");
    return kErrCpp;
  }
}

// Returns the data wire, or 0 when the field is absent or the index is out
// of bounds (the empty-local wire encoding).
__declspec(dllexport) void* gov8_object_get_internal_field(
    v8::Isolate* iso, void* scope, void* obj_wire, int32_t index) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return ToWire(FromWire<v8::Object>(obj_wire)->GetInternalField(index));
  } catch (...) {
    SetErr("C++ exception in object_get_internal_field");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_object_set_aligned_pointer_in_internal_field(
    v8::Isolate* iso, void* scope, void* obj_wire, int32_t index, void* ptr,
    int32_t tag) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::Object>(obj_wire)
        ->SetAlignedPointerInInternalField(
            index, ptr, static_cast<v8::EmbedderDataTypeTag>(tag));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in object_set_aligned_pointer_in_internal_field");
    return kErrCpp;
  }
}

__declspec(dllexport) void* gov8_object_get_aligned_pointer_from_internal_field(
    v8::Isolate* iso, void* scope, void* obj_wire, int32_t index,
    int32_t tag) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return FromWire<v8::Object>(obj_wire)
        ->GetAlignedPointerFromInternalField(
            index, static_cast<v8::EmbedderDataTypeTag>(tag));
  } catch (...) {
    SetErr("C++ exception in object_get_aligned_pointer_from_internal_field");
    return nullptr;
  }
}

// --- external values -------------------------------------------------------------

__declspec(dllexport) void* gov8_external_new(v8::Isolate* iso, void* scope,
                                              void* value) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return ToWire(
        v8::External::New(iso, value, v8::kExternalPointerTypeTagDefault));
  } catch (...) {
    SetErr("C++ exception in external_new");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_external_value(v8::Isolate* iso, void* scope,
                                                void* wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return FromWire<v8::External>(wire)->Value(v8::kExternalPointerTypeTagDefault);
  } catch (...) {
    SetErr("C++ exception in external_value");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_value_is_external(v8::Isolate* iso,
                                                     void* v) {
  ClearErr();
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    return FromWire<v8::Value>(v)->IsExternal() ? 1 : 0;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

// --- exceptions scheduled from native code ----------------------------------------

__declspec(dllexport) void* gov8_exception_error(v8::Isolate* iso, void* scope,
                                                 void* message_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return ToWire(
        v8::Exception::Error(FromWire<v8::String>(message_wire)));
  } catch (...) {
    SetErr("C++ exception in exception_error");
    return nullptr;
  }
}

// Schedules an exception to propagate to the JS caller when the active
// native callback returns (Scope::throw_exception in the oracle).
__declspec(dllexport) int64_t gov8_isolate_throw_exception(v8::Isolate* iso,
                                                           void* exc_wire) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    if (FromWire<v8::Value>(exc_wire).IsEmpty()) {
      SetErr("empty exception value");
      return kErrBadArg;
    }
    if (iso->ThrowException(FromWire<v8::Value>(exc_wire)).IsEmpty()) {
      SetErr("ThrowException failed");
      return kErrGeneric;
    }
    return kOk;
  } catch (...) {
    SetErr("C++ exception in isolate_throw_exception");
    return kErrCpp;
  }
}

// --- callback-path context-bound operations -----------------------------------------
//
// Inside a native callback the engine's *current* context is available as a
// wire captured by the trampoline; there is no Go-side persistent context
// wrapper for it (materializing one per invocation would add a Global
// handle churn to every callback). These exports take that wire directly
// and mirror the context-bound Value operations.

__declspec(dllexport) int64_t gov8_wctx_to_string_utf8(v8::Isolate* iso,
                                                       void* ctx_wire,
                                                       void* scope, void* v,
                                                       char* buf, int64_t cap,
                                                       int64_t* out_len) {
  ClearErr();
  if (!ScopeIs(iso, scope) || ctx_wire == nullptr || out_len == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = FromWire<v8::Context>(ctx_wire);
    v8::Context::Scope ctx_scope(ctx);
    std::string text;
    if (!ValueText(iso, ctx, FromWire<v8::Value>(v), &text)) {
      return kErrException;
    }
    return BufferUtf8(text, buf, cap, out_len);
  } catch (...) {
    SetErr("C++ exception in wctx_to_string_utf8");
    return kErrCpp;
  }
}

#define GOV8_WCTX_CONV(name, ctype, call)                                    \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* ctx_wire,      \
                                     void* v, ctype* out, int32_t* ok) {    \
    ClearErr();                                                             \
    if (ctx_wire == nullptr || out == nullptr || ok == nullptr) {           \
      SetErr("invalid argument");                                           \
      return kErrBadArg;                                                    \
    }                                                                       \
    try {                                                                   \
      Gov8IsolateScope iso_scope(iso);                                    \
      v8::Local<v8::Context> ctx = FromWire<v8::Context>(ctx_wire);         \
      v8::Context::Scope ctx_scope(ctx);                                    \
      ctype value;                                                          \
      if ((call).To(&value)) {                                              \
        *out = value;                                                       \
        *ok = 1;                                                            \
      } else {                                                              \
        *ok = 0;                                                            \
      }                                                                     \
      return kOk;                                                           \
    } catch (...) {                                                         \
      SetErr("C++ exception in " #name);                                    \
      return kErrCpp;                                                       \
    }                                                                       \
  }

GOV8_WCTX_CONV(gov8_wctx_number_value, double,
               FromWire<v8::Value>(v)->NumberValue(ctx))
GOV8_WCTX_CONV(gov8_wctx_integer_value, int64_t,
               FromWire<v8::Value>(v)->IntegerValue(ctx))

#undef GOV8_WCTX_CONV

__declspec(dllexport) int64_t gov8_wctx_integer_value_direct(
    v8::Isolate* iso, void* ctx_wire, void* v) {
  ClearErr();
  if (ctx_wire == nullptr || v == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = FromWire<v8::Context>(ctx_wire);
    v8::Context::Scope ctx_scope(ctx);
    int64_t value;
    if (!FromWire<v8::Value>(v)->IntegerValue(ctx).To(&value)) return 0;
    g_wctx_integer_value_result = value;
    return static_cast<int64_t>(
        reinterpret_cast<intptr_t>(&g_wctx_integer_value_result));
  } catch (...) {
    SetErr("C++ exception in gov8_wctx_integer_value_direct");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wctx_object_get(v8::Isolate* iso,
                                                   void* ctx_wire, void* scope,
                                                   void* obj, void* key,
                                                   void** out, int32_t* ok) {
  ClearErr();
  if (!ScopeIs(iso, scope) || ctx_wire == nullptr || out == nullptr ||
      ok == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = FromWire<v8::Context>(ctx_wire);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> result;
    if (FromWire<v8::Object>(obj)
            ->Get(ctx, FromWire<v8::Value>(key))
            .ToLocal(&result)) {
      *out = ToWire(result);
      *ok = 1;
    } else {
      *ok = 0;
    }
    return kOk;
  } catch (...) {
    SetErr("C++ exception in wctx_object_get");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wctx_object_set(v8::Isolate* iso,
                                                   void* ctx_wire, void* scope,
                                                   void* obj, void* key,
                                                   void* val, int32_t* ok) {
  ClearErr();
  if (!ScopeIs(iso, scope) || ctx_wire == nullptr || ok == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = FromWire<v8::Context>(ctx_wire);
    v8::Context::Scope ctx_scope(ctx);
    v8::Maybe<bool> result = FromWire<v8::Object>(obj)->Set(
        ctx, FromWire<v8::Value>(key), FromWire<v8::Value>(val));
    if (result.IsNothing()) {
      *ok = 0;
    } else {
      *ok = result.FromJust() ? 1 : 0;
    }
    return kOk;
  } catch (...) {
    SetErr("C++ exception in wctx_object_set");
    return kErrCpp;
  }
}

// --- value identity ------------------------------------------------------------------

__declspec(dllexport) int64_t gov8_value_strict_equals(v8::Isolate* iso,
                                                       void* a, void* b) {
  ClearErr();
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    return FromWire<v8::Value>(a)->StrictEquals(FromWire<v8::Value>(b)) ? 1 : 0;
  } catch (...) {
    SetErr("C++ exception in value_strict_equals");
    return kErrCpp;
  }
}

// --- return values -------------------------------------------------------------------
//
// The Go side holds the raw ReturnValue word captured by the trampoline and
// drives typed setters through these exports. The word is the ReturnValue
// object's single internal pointer (see v8__FunctionCallbackInfo__GetParts
// in the pinned binding layer); the static_assert guards that layout.

#define GOV8_RV(name, args_decl, args_use, body)                             \
  __declspec(dllexport) int64_t name args_decl {                             \
    ClearErr();                                                              \
    try {                                                                    \
      static_assert(sizeof(v8::ReturnValue<v8::Value>) == sizeof(uintptr_t), \
                    "unexpected ReturnValue layout");                        \
      body;                                                                  \
      return kOk;                                                            \
    } catch (...) {                                                          \
      SetErr("C++ exception in " #name);                                     \
      return kErrCpp;                                                        \
    }                                                                        \
  }

namespace {
// Reconstructs a ReturnValue view over the raw word. ReturnValue has no
// default constructor, so the word is copied into aligned raw storage and
// reinterpreted; the storage never escapes this call.
inline v8::ReturnValue<v8::Value>& RvFromWord(uintptr_t word) {
  alignas(v8::ReturnValue<v8::Value>) static thread_local unsigned char
      storage[sizeof(v8::ReturnValue<v8::Value>)];
  std::memcpy(storage, &word, sizeof(word));
  return *reinterpret_cast<v8::ReturnValue<v8::Value>*>(storage);
}
}  // namespace

GOV8_RV(gov8_rv_set, (uintptr_t rv_word, void* val_wire),
        (rv_word, val_wire),
        RvFromWord(rv_word).Set(FromWire<v8::Value>(val_wire)))
GOV8_RV(gov8_rv_set_int32, (uintptr_t rv_word, int32_t v), (rv_word, v),
        RvFromWord(rv_word).Set(v))
GOV8_RV(gov8_rv_set_uint32, (uintptr_t rv_word, uint32_t v), (rv_word, v),
        RvFromWord(rv_word).Set(v))
GOV8_RV(gov8_rv_set_double, (uintptr_t rv_word, double v), (rv_word, v),
        RvFromWord(rv_word).Set(v))
GOV8_RV(gov8_rv_set_bool, (uintptr_t rv_word, int32_t v), (rv_word, v),
        RvFromWord(rv_word).Set(v != 0))
GOV8_RV(gov8_rv_set_null, (uintptr_t rv_word), (rv_word),
        RvFromWord(rv_word).SetNull())
GOV8_RV(gov8_rv_set_undefined, (uintptr_t rv_word), (rv_word),
        RvFromWord(rv_word).SetUndefined())
GOV8_RV(gov8_rv_set_empty_string, (uintptr_t rv_word), (rv_word),
        RvFromWord(rv_word).SetEmptyString())

#undef GOV8_RV

// Reads back the value currently held by the callback's ReturnValue
// (undefined when nothing was set -- the engine pre-seeds the slot).
__declspec(dllexport) void* gov8_rv_get(uintptr_t rv_word) {
  ClearErr();
  try {
    static_assert(sizeof(v8::ReturnValue<v8::Value>) == sizeof(uintptr_t),
                  "unexpected ReturnValue layout");
    return ToWire(RvFromWord(rv_word).Get());
  } catch (...) {
    SetErr("C++ exception in gov8_rv_get");
    return nullptr;
  }
}

// --- template-advanced: function template extras -----------------------------

// v8::Signature::New over a function template.
__declspec(dllexport) void* gov8_signature_new(v8::Isolate* iso, void* scope,
                                               void* ft_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return ToWire(
        v8::Signature::New(iso, FromWire<v8::FunctionTemplate>(ft_wire)));
  } catch (...) {
    SetErr("C++ exception in signature_new");
    return nullptr;
  }
}

// FunctionTemplate::New with the full builder surface the advanced slice
// needs: signature wire (nullable) and constructor behavior (Go maps its
// Default/Allow to kAllow, Throw to kThrow).
__declspec(dllexport) void* gov8_function_template_new_full(
    v8::Isolate* iso, void* scope, void* host_ctx, int32_t length,
    void* sig_wire, int32_t throw_behavior) {
  ClearErr();
  GoCallbackContext* ctx = static_cast<GoCallbackContext*>(host_ctx);
  if (iso == nullptr || !ScopeIs(iso, scope) || ctx == nullptr) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return RegisterNativeCallback(
        iso, ctx, [&](v8::Local<v8::Value> data) -> void* {
          v8::Local<v8::Signature> sig =
              sig_wire == nullptr ? v8::Local<v8::Signature>()
                                  : FromWire<v8::Signature>(sig_wire);
          const v8::ConstructorBehavior behavior =
              throw_behavior != 0 ? v8::ConstructorBehavior::kThrow
                                  : v8::ConstructorBehavior::kAllow;
          v8::Local<v8::FunctionTemplate> tmpl = v8::FunctionTemplate::New(
              iso, HostFunctionTrampoline, data, sig, length, behavior,
              v8::SideEffectType::kHasSideEffect);
          if (tmpl.IsEmpty()) {
            SetErr("FunctionTemplate::New failed");
            return nullptr;
          }
          return ToWire(tmpl);
        });
  } catch (...) {
    SetErr("C++ exception in function_template_new_full");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_function_template_inherit(
    v8::Isolate* iso, void* scope, void* ft_wire, void* parent_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::FunctionTemplate>(ft_wire)
        ->Inherit(FromWire<v8::FunctionTemplate>(parent_wire));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in function_template_inherit");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_function_template_read_only_prototype(
    v8::Isolate* iso, void* scope, void* ft_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::FunctionTemplate>(ft_wire)->ReadOnlyPrototype();
    return kOk;
  } catch (...) {
    SetErr("C++ exception in function_template_read_only_prototype");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_function_template_remove_prototype(
    v8::Isolate* iso, void* scope, void* ft_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::FunctionTemplate>(ft_wire)->RemovePrototype();
    return kOk;
  } catch (...) {
    SetErr("C++ exception in function_template_remove_prototype");
    return kErrCpp;
  }
}

// Template::SetIntrinsicDataProperty -- binds a context intrinsic object as a
// data property at instantiation. Accepts any template wire (the method is
// declared on the Template base).
__declspec(dllexport) int64_t gov8_template_set_intrinsic_data_property(
    v8::Isolate* iso, void* scope, void* tmpl_wire, void* key_wire,
    int32_t intrinsic, int32_t attr) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::Template>(tmpl_wire)
        ->SetIntrinsicDataProperty(FromWire<v8::Name>(key_wire),
                                   static_cast<v8::Intrinsic>(intrinsic),
                                   static_cast<v8::PropertyAttribute>(attr));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in template_set_intrinsic_data_property");
    return kErrCpp;
  }
}

// --- template-advanced: object template extras --------------------------------

// Template::SetAccessorProperty on an object template: installs an
// accessor-SHAPED property (getter/setter function templates), unlike the
// native data property of SetNativeDataProperty.
__declspec(dllexport) int64_t gov8_object_template_set_accessor_property(
    v8::Isolate* iso, void* scope, void* ot_wire, void* key_wire,
    void* getter_wire, void* setter_wire, int32_t attr) {
  ClearErr();
  if (!ScopeIs(iso, scope) ||
      (getter_wire == nullptr && setter_wire == nullptr)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::ObjectTemplate>(ot_wire)
        ->SetAccessorProperty(
            FromWire<v8::Name>(key_wire),
            getter_wire == nullptr ? v8::Local<v8::FunctionTemplate>()
                                   : FromWire<v8::FunctionTemplate>(getter_wire),
            setter_wire == nullptr ? v8::Local<v8::FunctionTemplate>()
                                   : FromWire<v8::FunctionTemplate>(setter_wire),
            static_cast<v8::PropertyAttribute>(attr));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in object_template_set_accessor_property");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_object_template_set_immutable_proto(
    v8::Isolate* iso, void* scope, void* ot_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::ObjectTemplate>(ot_wire)->SetImmutableProto();
    return kOk;
  } catch (...) {
    SetErr("C++ exception in object_template_set_immutable_proto");
    return kErrCpp;
  }
}

// ObjectTemplate::SetCallAsFunctionHandler. The handler is the shared
// function trampoline; the dispatch External carries the Go registry handle
// and the embedder data travels through the context's Global as everywhere
// else.
__declspec(dllexport) int64_t gov8_object_template_set_call_as_function_handler(
    v8::Isolate* iso, void* scope, void* ot_wire, void* host_ctx) {
  ClearErr();
  GoCallbackContext* cbctx = static_cast<GoCallbackContext*>(host_ctx);
  if (!ScopeIs(iso, scope) || cbctx == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    void* data_external = HostContextExternalWire(iso, cbctx);
    if (data_external == nullptr) {
      return kErrNoMemory;
    }
    FromWire<v8::ObjectTemplate>(ot_wire)
        ->SetCallAsFunctionHandler(HostFunctionTrampoline,
                                   FromWire<v8::Value>(data_external));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in object_template_set_call_as_function_handler");
    return kErrCpp;
  }
}

// Callback presence masks for the property handler installers below.
constexpr int32_t kHandlerBitGetter = 1;
constexpr int32_t kHandlerBitSetter = 2;
constexpr int32_t kHandlerBitQuery = 4;
constexpr int32_t kHandlerBitDeleter = 8;
constexpr int32_t kHandlerBitEnumerator = 16;
constexpr int32_t kHandlerBitDefiner = 32;
constexpr int32_t kHandlerBitDescriptor = 64;

// ObjectTemplate::SetHandler with a NamedPropertyHandlerConfiguration. The
// static trampolines above fill the enabled slots; flags are forwarded
// verbatim (the pinned crate's binding passes them through unmodified).
__declspec(dllexport) int64_t gov8_object_template_set_named_property_handler(
    v8::Isolate* iso, void* scope, void* ot_wire, void* host_ctx,
    int32_t cb_mask, int32_t flags) {
  ClearErr();
  GoCallbackContext* cbctx = static_cast<GoCallbackContext*>(host_ctx);
  if (!ScopeIs(iso, scope) || cbctx == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    void* data_external = HostContextExternalWire(iso, cbctx);
    if (data_external == nullptr) {
      return kErrNoMemory;
    }
    FromWire<v8::ObjectTemplate>(ot_wire)
        ->SetHandler(v8::NamedPropertyHandlerConfiguration(
            (cb_mask & kHandlerBitGetter) ? HostNamedGetTrampoline : nullptr,
            (cb_mask & kHandlerBitSetter) ? HostNamedSetTrampoline : nullptr,
            (cb_mask & kHandlerBitQuery) ? HostNamedQueryTrampoline : nullptr,
            (cb_mask & kHandlerBitDeleter) ? HostNamedDeleteTrampoline : nullptr,
            (cb_mask & kHandlerBitEnumerator) ? HostNamedEnumTrampoline : nullptr,
            (cb_mask & kHandlerBitDefiner) ? HostNamedDefineTrampoline : nullptr,
            (cb_mask & kHandlerBitDescriptor) ? HostNamedDescriptorTrampoline
                                              : nullptr,
            FromWire<v8::Value>(data_external),
            static_cast<v8::PropertyHandlerFlags>(flags)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in object_template_set_named_property_handler");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_object_template_set_indexed_property_handler(
    v8::Isolate* iso, void* scope, void* ot_wire, void* host_ctx,
    int32_t cb_mask, int32_t flags) {
  ClearErr();
  GoCallbackContext* cbctx = static_cast<GoCallbackContext*>(host_ctx);
  if (!ScopeIs(iso, scope) || cbctx == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    void* data_external = HostContextExternalWire(iso, cbctx);
    if (data_external == nullptr) {
      return kErrNoMemory;
    }
    FromWire<v8::ObjectTemplate>(ot_wire)
        ->SetHandler(v8::IndexedPropertyHandlerConfiguration(
            (cb_mask & kHandlerBitGetter) ? HostIndexedGetTrampoline : nullptr,
            (cb_mask & kHandlerBitSetter) ? HostIndexedSetTrampoline : nullptr,
            (cb_mask & kHandlerBitQuery) ? HostIndexedQueryTrampoline : nullptr,
            (cb_mask & kHandlerBitDeleter) ? HostIndexedDeleteTrampoline : nullptr,
            (cb_mask & kHandlerBitEnumerator) ? HostIndexedEnumTrampoline : nullptr,
            (cb_mask & kHandlerBitDefiner) ? HostIndexedDefineTrampoline : nullptr,
            (cb_mask & kHandlerBitDescriptor) ? HostIndexedDescriptorTrampoline
                                              : nullptr,
            FromWire<v8::Value>(data_external),
            static_cast<v8::PropertyHandlerFlags>(flags)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in object_template_set_indexed_property_handler");
    return kErrCpp;
  }
}

// --- template-advanced: value identity ----------------------------------------

// v8::Value::GetHash -- the identity hash used by the oracle's object-identity
// observations (stable per object within one isolate).
__declspec(dllexport) int64_t gov8_value_get_hash(v8::Isolate* iso,
                                                  void* wire) {
  ClearErr();
  if (iso == nullptr || wire == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    return static_cast<int64_t>(FromWire<v8::Value>(wire)->GetHash());
  } catch (...) {
    SetErr("C++ exception in gov8_value_get_hash");
    return kErrCpp;
  }
}

// --- template-advanced: context security tokens -------------------------------

__declspec(dllexport) void* gov8_context_get_security_token(
    v8::Isolate* iso, void* ctxw, void* scope) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return ToWire(w->ctx->Get(iso)->GetSecurityToken());
  } catch (...) {
    SetErr("C++ exception in context_get_security_token");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_context_set_security_token(
    v8::Isolate* iso, void* ctxw, void* token_wire) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    w->ctx->Get(iso)->SetSecurityToken(FromWire<v8::Value>(token_wire));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_set_security_token");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_context_use_default_security_token(
    v8::Isolate* iso, void* ctxw) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    w->ctx->Get(iso)->UseDefaultSecurityToken();
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_use_default_security_token");
    return kErrCpp;
  }
}

// --- template-advanced: callback-scope construction helpers --------------------
//
// Inside callbacks only the engine's current context wire is available, so
// these mirror the runtime-values slice's constructors with a raw context
// wire instead of a CtxWrap. The artifact's flat bindings are reused verbatim
// (declared by runtime_values.inc too, which is included after this file, so
// this slice declares its own references).

extern "C" {
const v8::Object* v8__Object__New(v8::Isolate* isolate);
const v8::Array* v8__Array__New_with_elements(
    v8::Isolate* isolate, const v8::Value* const elements[], size_t length);
}

__declspec(dllexport) void* gov8_wctx_object_new(v8::Isolate* iso,
                                                 void* ctx_wire, void* scope) {
  ClearErr();
  if (!ScopeIs(iso, scope) || ctx_wire == nullptr) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    // No Context::Scope here: v8::Object::New only needs the isolate
    // entered, and the nested enter/exit would unbalance V8's API-level
    // entered-context stack while running inside an interceptor callback
    // (the engine may have pushed its own context frames around the
    // dispatch). The caller's context is current either way.
    (void)ctx_wire;
    return const_cast<void*>(static_cast<const void*>(v8__Object__New(iso)));
  } catch (...) {
    SetErr("C++ exception in wctx_object_new");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_wctx_array_new_with_elements(
    v8::Isolate* iso, void* ctx_wire, void* scope, void* wires, int64_t n,
    void** out) {
  ClearErr();
  if (!ScopeIs(iso, scope) || ctx_wire == nullptr || out == nullptr || n < 0 ||
      (wires == nullptr && n > 0)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    // No Context::Scope: v8::Array::New(isolate, elements, length) needs
    // only the entered isolate, and nesting a Context::Scope here trips
    // Context::Exit's last-entered CHECK under interceptor dispatch.
    (void)ctx_wire;
    *out = const_cast<void*>(static_cast<const void*>(
        v8__Array__New_with_elements(iso,
                                     static_cast<const v8::Value* const*>(wires),
                                     static_cast<size_t>(n))));
    return *out == nullptr ? kErrGeneric : kOk;
  } catch (...) {
    SetErr("C++ exception in wctx_array_new_with_elements");
    return kErrCpp;
  }
}

}  // extern "C"

extern "C" __declspec(dllexport) int64_t gov8_object_template_mark_as_undetectable(v8::Isolate* iso, void* scope, void* ot_wire) {
 ClearErr();
 if (!ScopeIs(iso, scope)) { SetErr("invalid scope handle for this isolate"); return kErrBadArg; }
 try { Gov8IsolateScope iso_scope(iso); FromWire<v8::ObjectTemplate>(ot_wire)->MarkAsUndetectable(); return kOk; }
 catch (...) { SetErr("C++ exception in object_template_mark_as_undetectable"); return kErrCpp; }
}
