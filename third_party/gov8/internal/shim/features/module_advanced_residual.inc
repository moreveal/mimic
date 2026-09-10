// Residual module APIs and host callbacks for V8 152.2.0.
// Include after modules.inc (uses ModuleWrap and resolver TLS helpers).

#include <mutex>
#include <unordered_map>

namespace gov8_module_residual_impl {

struct ArtifactStalledMessage {
  const v8::Module* module;
  const v8::Message* message;
};
extern "C" size_t v8__Module__GetStalledTopLevelAwaitMessage(
    const v8::Module& self, v8::Isolate* isolate,
    ArtifactStalledMessage* out, size_t length);
extern "C" const v8::Value* v8__Module__GetModuleNamespace2(
    const v8::Module& self, v8::ModuleImportPhase phase);
extern "C" const v8::Value* v8__Module__EvaluateForImportDefer(
    const v8::Module& self, const v8::Context& context);
extern "C" void v8__Isolate__SetHostInitializeImportMetaObjectCallback(
    v8::Isolate*, v8::HostInitializeImportMetaObjectCallback);
extern "C" void v8__Isolate__SetHostImportModuleDynamicallyCallback(
    v8::Isolate*, v8::HostImportModuleDynamicallyCallback);
extern "C" void v8__Isolate__SetHostImportModuleWithPhaseDynamicallyCallback(
    v8::Isolate*, v8::HostImportModuleWithPhaseDynamicallyCallback);
extern "C" void v8__Isolate__SetHostCreateShadowRealmContextCallback(
    v8::Isolate*, v8::HostCreateShadowRealmContextCallback);

using GoSourceResolve = int64_t (*)(uint64_t, void*, void*, void*, void*, void*, void**);
using GoImportMeta = void (*)(uint64_t, void*, void*, void*, void*, void*);
using GoDynamicImport = int64_t (*)(uint64_t, void*, void*, void*, void*, void*, void*, int32_t, void**);
using GoShadowRealm = int64_t (*)(uint64_t, void*, void*, void**);

GoSourceResolve go_source_resolve = nullptr;
GoImportMeta go_import_meta = nullptr;
GoDynamicImport go_dynamic_import = nullptr;
GoShadowRealm go_shadow_realm = nullptr;

struct HostIDs { uint64_t meta = 0, dynamic = 0, dynamic_phase = 0, shadow = 0; };
std::mutex host_mu;
std::unordered_map<v8::Isolate*, HostIDs> host_ids;
thread_local uint64_t source_resolve_id = 0;

HostIDs IDs(v8::Isolate* isolate) {
  std::lock_guard<std::mutex> lock(host_mu);
  auto it = host_ids.find(isolate);
  return it == host_ids.end() ? HostIDs{} : it->second;
}

struct CallbackScope {
  GoScope scope{};
  explicit CallbackScope(v8::Isolate* isolate) {
    scope.magic = kScopeMagic;
    scope.iso = isolate;
    GlobalPlacementNew<v8::HandleScope>(scope.hs_storage, isolate);
  }
  ~CallbackScope() { scope.hs()->~HandleScope(); scope.magic = 0; }
};

v8::MaybeLocal<v8::Object> ResolveSource(
    v8::Local<v8::Context> context, v8::Local<v8::String> specifier,
    v8::Local<v8::FixedArray> attributes, v8::Local<v8::Module> referrer) {
  if (go_source_resolve == nullptr || source_resolve_id == 0) return {};
  v8::Isolate* isolate = v8::Isolate::GetCurrent();
  ModuleWrap* referrer_wrap = FindModuleWrap(referrer);
  v8::Global<v8::Object> result;
  int64_t handled = 0;
  {
    CallbackScope callback_scope(isolate);
    void* result_wire = nullptr;
    handled = go_source_resolve(source_resolve_id, ToWire(context),
        &callback_scope.scope, ToWire(specifier), ToWire(attributes),
        referrer_wrap, &result_wire);
    if (handled == 1 && result_wire != nullptr) {
      v8::Local<v8::Value> value = FromWire<v8::Value>(result_wire);
      if (!value.IsEmpty() && value->IsObject()) result.Reset(isolate, value.As<v8::Object>());
    }
  }
  if (handled != 1 || result.IsEmpty()) return {};
  return result.Get(isolate);
}

void InitializeImportMeta(v8::Local<v8::Context> context,
                          v8::Local<v8::Module> module,
                          v8::Local<v8::Object> meta) {
  v8::Isolate* isolate = v8::Isolate::GetCurrent();
  HostIDs ids = IDs(isolate);
  if (ids.meta == 0 || go_import_meta == nullptr) return;
  CallbackScope callback_scope(isolate);
  go_import_meta(ids.meta, ToWire(context), &callback_scope.scope,
                 ToWire(module), ToWire(meta), isolate);
}

v8::MaybeLocal<v8::Promise> DynamicImportCommon(
    uint64_t id, v8::Local<v8::Context> context,
    v8::Local<v8::Data> host_options, v8::Local<v8::Value> resource,
    v8::Local<v8::String> specifier, v8::ModuleImportPhase phase,
    v8::Local<v8::FixedArray> attributes) {
  if (id == 0 || go_dynamic_import == nullptr) return {};
  v8::Isolate* isolate = v8::Isolate::GetCurrent();
  v8::Global<v8::Promise> result;
  int64_t handled = 0;
  {
    CallbackScope callback_scope(isolate);
    void* result_wire = nullptr;
    handled = go_dynamic_import(id, ToWire(context), &callback_scope.scope,
        ToWire(host_options), ToWire(resource), ToWire(specifier),
        ToWire(attributes), static_cast<int32_t>(phase), &result_wire);
    if (handled == 1 && result_wire != nullptr) {
      v8::Local<v8::Value> value = FromWire<v8::Value>(result_wire);
      if (!value.IsEmpty() && value->IsPromise()) result.Reset(isolate, value.As<v8::Promise>());
    }
  }
  if (handled != 1 || result.IsEmpty()) return {};
  return result.Get(isolate);
}

v8::MaybeLocal<v8::Promise> DynamicImport(
    v8::Local<v8::Context> context, v8::Local<v8::Data> host_options,
    v8::Local<v8::Value> resource, v8::Local<v8::String> specifier,
    v8::Local<v8::FixedArray> attributes) {
  return DynamicImportCommon(IDs(v8::Isolate::GetCurrent()).dynamic, context,
      host_options, resource, specifier, v8::ModuleImportPhase::kEvaluation,
      attributes);
}

v8::MaybeLocal<v8::Promise> DynamicImportWithPhase(
    v8::Local<v8::Context> context, v8::Local<v8::Data> host_options,
    v8::Local<v8::Value> resource, v8::Local<v8::String> specifier,
    v8::ModuleImportPhase phase, v8::Local<v8::FixedArray> attributes) {
  HostIDs ids = IDs(v8::Isolate::GetCurrent());
  // The pinned rusty_v8 surface continues to route evaluation imports through
  // the legacy callback after installing the experimental phase-aware hook.
  // Some header/library combinations invoke this hook for both phases, so
  // preserve the public contract explicitly here.
  uint64_t id = phase == v8::ModuleImportPhase::kEvaluation
                    ? ids.dynamic
                    : ids.dynamic_phase;
  return DynamicImportCommon(id, context, host_options, resource, specifier,
                             phase, attributes);
}

v8::MaybeLocal<v8::Context> CreateShadowRealm(v8::Local<v8::Context> initiator) {
  v8::Isolate* isolate = v8::Isolate::GetCurrent();
  HostIDs ids = IDs(isolate);
  if (ids.shadow == 0 || go_shadow_realm == nullptr) return {};
  void* context_wrap = nullptr;
  {
    CallbackScope callback_scope(isolate);
    if (go_shadow_realm(ids.shadow, ToWire(initiator), &callback_scope.scope,
                        &context_wrap) != 1) return {};
  }
  CtxWrap* wrap = AsCtx(context_wrap);
  if (wrap == nullptr || wrap->iso != isolate) return {};
  return wrap->ctx->Get(isolate);
}

}  // namespace gov8_module_residual_impl

extern "C" {
using namespace gov8_module_residual_impl;

__declspec(dllexport) int64_t gov8_mar_set_dispatchers(
    void* source, void* meta, void* dynamic, void* shadow) {
  if (!source || !meta || !dynamic || !shadow) return kErrBadArg;
  go_source_resolve = reinterpret_cast<GoSourceResolve>(source);
  go_import_meta = reinterpret_cast<GoImportMeta>(meta);
  go_dynamic_import = reinterpret_cast<GoDynamicImport>(dynamic);
  go_shadow_realm = reinterpret_cast<GoShadowRealm>(shadow);
  return kOk;
}

__declspec(dllexport) int64_t gov8_mar_set_host_callback(
    v8::Isolate* isolate, int32_t kind, uint64_t id) {
  ClearErr();
  if (!isolate || kind < 0 || kind > 3) return kErrBadArg;
  try {
    std::lock_guard<std::mutex> lock(host_mu);
    HostIDs& ids = host_ids[isolate];
    switch (kind) {
      case 0: ids.meta = id; v8__Isolate__SetHostInitializeImportMetaObjectCallback(isolate, InitializeImportMeta); break;
      case 1: ids.dynamic = id; v8__Isolate__SetHostImportModuleDynamicallyCallback(isolate, DynamicImport); break;
      case 2: ids.dynamic_phase = id; v8__Isolate__SetHostImportModuleWithPhaseDynamicallyCallback(isolate, DynamicImportWithPhase); break;
      case 3: ids.shadow = id; v8__Isolate__SetHostCreateShadowRealmContextCallback(isolate, CreateShadowRealm); break;
    }
    return kOk;
  } catch (...) { SetErr("C++ exception setting module host callback"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_mar_release(v8::Isolate* isolate) {
  std::lock_guard<std::mutex> lock(host_mu);
  host_ids.erase(isolate);
  return kOk;
}

__declspec(dllexport) int64_t gov8_mar_instantiate2(
    v8::Isolate* iso, void* ctxw, void* scope, void* modulew, void* tcw,
    uint64_t resolve_id, uint64_t source_id, void** modules,
    int64_t module_count, int32_t* out) {
  ClearErr();
  CtxWrap* context = AsCtx(ctxw); ModuleWrap* root = AsModule(modulew);
  if (!context || !root || !out || !ScopeIs(iso, scope) ||
      context->iso != iso || root->iso != iso || root->ctx != context ||
      (!modules && module_count > 0)) return kErrBadArg;
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::Context> ctx = context->ctx->Get(iso);
    v8::Context::Scope context_scope(ctx);
    FallbackTryCatch fallback; v8::TryCatch* tc = nullptr;
    if (tcw) { TcWrap* tw = AsTc(tcw); if (!tw || tw->iso != iso) return kErrBadArg; tc = TcOf(tw); }
    if (!tc) tc = fallback.Init(iso);
    std::vector<ModuleWrap*> known;
    for (int64_t n = 0; n < module_count; ++n) {
      ModuleWrap* candidate = AsModule(modules[n]);
      if (candidate && candidate->iso == iso) known.push_back(candidate);
    }
    if (std::find(known.begin(), known.end(), root) == known.end()) known.push_back(root);
    int64_t old_id = tls_module_resolve_id; v8::Isolate* old_iso = tls_module_resolve_iso;
    auto* old_modules = tls_module_resolve_modules; uint64_t old_source = source_resolve_id;
    tls_module_resolve_id = static_cast<int64_t>(resolve_id); tls_module_resolve_iso = iso;
    tls_module_resolve_modules = &known; source_resolve_id = source_id;
    v8::Maybe<bool> result = root->module->Get(iso)->InstantiateModule(
        ctx, Gov8ResolveModule, ResolveSource);
    tls_module_resolve_id = old_id; tls_module_resolve_iso = old_iso;
    tls_module_resolve_modules = old_modules; source_resolve_id = old_source;
    if (result.IsNothing()) return kErrException;
    *out = result.FromJust() ? 1 : 0; return kOk;
  } catch (...) {
    tls_module_resolve_id = 0; tls_module_resolve_iso = nullptr;
    tls_module_resolve_modules = nullptr; source_resolve_id = 0;
    SetErr("C++ exception instantiating module"); return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_mar_namespace_phase(
    v8::Isolate* iso, void* scope, void* modulew, int32_t phase, void** out) {
  ModuleWrap* module = AsModule(modulew);
  if (!module || !out || !ScopeIs(iso, scope) || module->iso != iso || phase < 0 || phase > 2) return kErrBadArg;
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::Context> context = module->ctx->ctx->Get(iso);
    v8::Context::Scope entered(context);
    *out = const_cast<v8::Value*>(v8__Module__GetModuleNamespace2(
        gov8_snap_impl::DerefLocal(module->module->Get(iso)),
        static_cast<v8::ModuleImportPhase>(phase)));
    return kOk;
  } catch (...) { SetErr("C++ exception getting module namespace"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_mar_evaluate_defer(
    v8::Isolate* iso, void* ctxw, void* scope, void* modulew, void** out) {
  CtxWrap* ctx = AsCtx(ctxw); ModuleWrap* module = AsModule(modulew);
  if (!ctx || !module || !out || !ScopeIs(iso, scope) || ctx->iso != iso || module->iso != iso) return kErrBadArg;
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::Context> local = ctx->ctx->Get(iso); v8::Context::Scope entered(local);
    const v8::Value* value = v8__Module__EvaluateForImportDefer(
        gov8_snap_impl::DerefLocal(module->module->Get(iso)),
        gov8_snap_impl::DerefLocal(local));
    if (!value) return kErrException;
    *out = const_cast<v8::Value*>(value); return kOk;
  } catch (...) { SetErr("C++ exception evaluating deferred import"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_mar_stalled(
    v8::Isolate* iso, void* scope, void* modulew, void** module_wires,
    void** message_wires, int64_t cap, int64_t* count) {
  ModuleWrap* module = AsModule(modulew);
  if (!module || !count || !ScopeIs(iso, scope) || module->iso != iso || cap < 0) return kErrBadArg;
  try {
    Gov8IsolateScope isolate_scope(iso);
    ArtifactStalledMessage values[16]{};
    *count = static_cast<int64_t>(v8__Module__GetStalledTopLevelAwaitMessage(
        gov8_snap_impl::DerefLocal(module->module->Get(iso)), iso, values, 16));
    if (cap < *count) return kErrNoMemory;
    for (int64_t n = 0; n < *count; ++n) {
      module_wires[n] = const_cast<v8::Module*>(values[n].module);
      message_wires[n] = const_cast<v8::Message*>(values[n].message);
    }
    return kOk;
  } catch (...) { SetErr("C++ exception reading stalled top-level await"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_mar_module_matches(void* modulew, void* local_wire) {
  ModuleWrap* module = AsModule(modulew);
  if (!module || !local_wire) return kErrBadArg;
  try {
    Gov8IsolateScope isolate_scope(module->iso);
    v8::HandleScope handle_scope(module->iso);
    return module->module->Get(module->iso) == FromWire<v8::Module>(local_wire) ? 1 : 0;
  }
  catch (...) { return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_mar_callback_promise_new(
    v8::Isolate* iso, void* context_wire, void* scope, void** resolver, void** promise) {
  if (!iso || !context_wire || !resolver || !promise || !ScopeIs(iso, scope)) return kErrBadArg;
  try {
    v8::Local<v8::Context> context = FromWire<v8::Context>(context_wire);
    v8::Local<v8::Promise::Resolver> r;
    if (!v8::Promise::Resolver::New(context).ToLocal(&r)) return kErrException;
    *resolver = ToWire(r); *promise = ToWire(r->GetPromise()); return kOk;
  } catch (...) { return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_mar_callback_promise_settle(
    v8::Isolate* iso, void* context_wire, void* resolver_wire,
    void* value_wire, int32_t reject, int32_t* ok) {
  if (!iso || !context_wire || !resolver_wire || !value_wire || !ok) return kErrBadArg;
  try {
    v8::Local<v8::Context> context = FromWire<v8::Context>(context_wire);
    auto resolver = FromWire<v8::Promise::Resolver>(resolver_wire);
    v8::Maybe<bool> result = reject ? resolver->Reject(context, FromWire<v8::Value>(value_wire))
                                    : resolver->Resolve(context, FromWire<v8::Value>(value_wire));
    if (result.IsNothing()) return kErrException;
    *ok = result.FromJust() ? 1 : 0; return kOk;
  } catch (...) { return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_mar_callback_type_error(
    v8::Isolate* iso, void* context_wire, void* scope,
    const char* message, int64_t length, void** out) {
  if (!iso || !context_wire || !ScopeIs(iso, scope) || !out || length < 0 ||
      (!message && length > 0) || length > INT_MAX) return kErrBadArg;
  try {
    v8::Local<v8::Context> context = FromWire<v8::Context>(context_wire);
    v8::Context::Scope entered(context);
    v8::Local<v8::String> text;
    if (!v8::String::NewFromUtf8(iso, message ? message : "",
        v8::NewStringType::kNormal, static_cast<int>(length)).ToLocal(&text)) return kErrGeneric;
    *out = ToWire(v8::Exception::TypeError(text)); return kOk;
  } catch (...) { return kErrCpp; }
}

}  // extern "C"
