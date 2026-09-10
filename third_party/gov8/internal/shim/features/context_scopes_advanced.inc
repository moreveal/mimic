// ===========================================================================
// Advanced context construction, context-local hooks, microtask observations,
// and JavaScript execution guards. Engine calls use the pinned artifact's flat
// bindings so the ABI matches v8 crate 152.2.0 exactly.
// ===========================================================================

#include <cstdint>

extern "C" {
const v8::Context* v8__Context__New(v8::Isolate* isolate,
                                    const v8::ObjectTemplate* templ,
                                    const v8::Value* global_object,
                                    v8::MicrotaskQueue* microtask_queue);
const v8::Object* v8__Context__GetExtrasBindingObject(v8::Context& self);
void v8__Context__SetPromiseHooks(v8::Context& self,
                                  const v8::Function* init_hook,
                                  const v8::Function* before_hook,
                                  const v8::Function* after_hook,
                                  const v8::Function* resolve_hook);
void v8__Context__SetContinuationPreservedEmbedderData(
    v8::Isolate* isolate, const v8::Value* data);
const v8::Value* v8__Context__GetContinuationPreservedEmbedderData(
    v8::Isolate* isolate);
bool v8__MicrotaskQueue__IsRunningMicrotasks(v8::MicrotaskQueue* self);
int v8__MicrotaskQueue__GetMicrotasksScopeDepth(v8::MicrotaskQueue* self);

// The crate's raw scope wrappers are exactly two pointer-sized words, aligned
// as uintptr_t (scope/raw.rs). Keeping the storage opaque avoids compiling a
// second copy of the V8 class implementation against the vendored headers.
void v8__DisallowJavascriptExecutionScope__CONSTRUCT(
    void* buf, v8::Isolate* isolate, int32_t on_failure);
void v8__DisallowJavascriptExecutionScope__DESTRUCT(void* self);
void v8__AllowJavascriptExecutionScope__CONSTRUCT(void* buf,
                                                   v8::Isolate* isolate);
void v8__AllowJavascriptExecutionScope__DESTRUCT(void* self);
}

namespace gov8_csa_impl {

constexpr uint64_t kDisallowMagic = 0x674f563844495341ULL;  // "gOV8DISA"
constexpr uint64_t kAllowMagic = 0x674f5638414c4c57ULL;     // "gOV8ALLW"

struct ExecutionGuardWrap {
  uint64_t magic;
  v8::Isolate* iso;
  alignas(uintptr_t) unsigned char storage[sizeof(uintptr_t) * 2];
};

ExecutionGuardWrap* AsGuard(void* p, uint64_t magic) {
  if (p == nullptr) return nullptr;
  auto* w = static_cast<ExecutionGuardWrap*>(p);
  return w->magic == magic ? w : nullptr;
}

}  // namespace gov8_csa_impl

extern "C" {

__declspec(dllexport) void* gov8_csa_context_new_options(
    v8::Isolate* iso, void* scope, void* templ_wire, void* global_wire,
    void* mqw) {
  ClearErr();
  GoScope* s = AsScope(scope);
  MqWrap* mq = mqw == nullptr ? nullptr : AsMq(mqw);
  if (iso == nullptr || s == nullptr || !OwnedBy(iso, s->iso) ||
      (mqw != nullptr && mq == nullptr) ||
      (mq != nullptr && !OwnedBy(iso, mq->iso))) {
    SetErr("invalid context options");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    const v8::Context* context = v8__Context__New(
        iso, static_cast<const v8::ObjectTemplate*>(templ_wire),
        static_cast<const v8::Value*>(global_wire),
        mq == nullptr ? nullptr : mq->queue);
    if (context == nullptr) {
      SetErr("Context::New returned empty");
      return nullptr;
    }
    v8::Local<v8::Context> local =
        FromWire<v8::Context>(const_cast<v8::Context*>(context));
    auto* w = new CtxWrap{};
    w->magic = kCtxMagic;
    w->iso = iso;
    w->ctx = new v8::Global<v8::Context>(iso, local);
    return w;
  } catch (...) {
    SetErr("C++ exception in context_new_options");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_csa_context_extras_binding_object(
    v8::Isolate* iso, void* ctxw, void* scope, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> context = w->ctx->Get(iso);
    v8::Context::Scope context_scope(context);
    const v8::Object* object = v8__Context__GetExtrasBindingObject(
        gov8_snap_impl::DerefLocal(context));
    if (object == nullptr) return kErrException;
    *out = const_cast<v8::Object*>(object);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_extras_binding_object");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_csa_set_continuation_preserved_data(
    v8::Isolate* iso, void* scope, void* value_wire) {
  ClearErr();
  if (value_wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8__Context__SetContinuationPreservedEmbedderData(
        iso, static_cast<const v8::Value*>(value_wire));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in set_continuation_preserved_data");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_csa_get_continuation_preserved_data(
    v8::Isolate* iso, void* scope, void** out) {
  ClearErr();
  if (out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    const v8::Value* value =
        v8__Context__GetContinuationPreservedEmbedderData(iso);
    if (value == nullptr) {
      SetErr("continuation-preserved data returned empty");
      return kErrException;
    }
    *out = const_cast<v8::Value*>(value);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in get_continuation_preserved_data");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_csa_context_set_promise_hooks(
    v8::Isolate* iso, void* ctxw, void* init_hook, void* before_hook,
    void* after_hook, void* resolve_hook) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid context");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> context = w->ctx->Get(iso);
    v8::Context::Scope context_scope(context);
    v8__Context__SetPromiseHooks(
        gov8_snap_impl::DerefLocal(context),
        static_cast<const v8::Function*>(init_hook),
        static_cast<const v8::Function*>(before_hook),
        static_cast<const v8::Function*>(after_hook),
        static_cast<const v8::Function*>(resolve_hook));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_set_promise_hooks");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_csa_microtask_queue_is_running(void* mqw,
                                                                  int32_t* out) {
  ClearErr();
  MqWrap* w = AsMq(mqw);
  if (w == nullptr || out == nullptr) {
    SetErr("invalid microtask queue");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    *out = v8__MicrotaskQueue__IsRunningMicrotasks(w->queue) ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in microtask_queue_is_running");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_csa_microtask_queue_scope_depth(
    void* mqw, int32_t* out) {
  ClearErr();
  MqWrap* w = AsMq(mqw);
  if (w == nullptr || out == nullptr) {
    SetErr("invalid microtask queue");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    *out = static_cast<int32_t>(
        v8__MicrotaskQueue__GetMicrotasksScopeDepth(w->queue));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in microtask_queue_scope_depth");
    return kErrCpp;
  }
}

__declspec(dllexport) void* gov8_csa_disallow_javascript_new(
    v8::Isolate* iso, int32_t on_failure) {
  ClearErr();
  if (iso == nullptr || on_failure < 0 || on_failure > 2) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    auto* w = new gov8_csa_impl::ExecutionGuardWrap{};
    w->magic = gov8_csa_impl::kDisallowMagic;
    w->iso = iso;
    v8__DisallowJavascriptExecutionScope__CONSTRUCT(w->storage, iso,
                                                    on_failure);
    return w;
  } catch (...) {
    SetErr("C++ exception in disallow_javascript_new");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_csa_allow_javascript_new(v8::Isolate* iso) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("invalid isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    auto* w = new gov8_csa_impl::ExecutionGuardWrap{};
    w->magic = gov8_csa_impl::kAllowMagic;
    w->iso = iso;
    v8__AllowJavascriptExecutionScope__CONSTRUCT(w->storage, iso);
    return w;
  } catch (...) {
    SetErr("C++ exception in allow_javascript_new");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_csa_disallow_javascript_dispose(void* p) {
  ClearErr();
  auto* w = gov8_csa_impl::AsGuard(p, gov8_csa_impl::kDisallowMagic);
  if (w == nullptr) {
    SetErr("invalid disallow guard");
    return kErrMagic;
  }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    v8__DisallowJavascriptExecutionScope__DESTRUCT(w->storage);
    delete w;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in disallow_javascript_dispose");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_csa_allow_javascript_dispose(void* p) {
  ClearErr();
  auto* w = gov8_csa_impl::AsGuard(p, gov8_csa_impl::kAllowMagic);
  if (w == nullptr) {
    SetErr("invalid allow guard");
    return kErrMagic;
  }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    v8__AllowJavascriptExecutionScope__DESTRUCT(w->storage);
    delete w;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in allow_javascript_dispose");
    return kErrCpp;
  }
}

}  // extern "C"
