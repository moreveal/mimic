// modules.inc -- SourceTextModule bindings for rusty_v8 / V8 152.2.0.
// Included after the core shim has closed its extern "C" block.

#include <algorithm>
#include <vector>

namespace {

constexpr uint64_t kModuleMagic = 0x674f56384d4f4455ULL;  // "gOV8MODU"

struct ModuleWrap {
  uint64_t magic;
  v8::Isolate* iso;
  CtxWrap* ctx;
  v8::Global<v8::Module>* module;
};

ModuleWrap* AsModule(void* p) {
  if (p == nullptr) return nullptr;
  ModuleWrap* w = static_cast<ModuleWrap*>(p);
  return w->magic == kModuleMagic ? w : nullptr;
}


using GoModuleResolveEntry = int64_t (*)(int64_t, void*, void*, void*, void**);
GoModuleResolveEntry g_module_resolve_entry = nullptr;
thread_local int64_t tls_module_resolve_id = 0;
thread_local v8::Isolate* tls_module_resolve_iso = nullptr;
thread_local std::vector<ModuleWrap*>* tls_module_resolve_modules = nullptr;

ModuleWrap* FindModuleWrap(v8::Local<v8::Module> module) {
  if (tls_module_resolve_modules == nullptr) return nullptr;
  for (ModuleWrap* w : *tls_module_resolve_modules) {
    if (w != nullptr && w->module->Get(w->iso) == module) return w;
  }
  return nullptr;
}

v8::MaybeLocal<v8::Module> Gov8ResolveModule(
    v8::Local<v8::Context>, v8::Local<v8::String> specifier,
    v8::Local<v8::FixedArray> import_attributes,
    v8::Local<v8::Module> referrer) {
  if (g_module_resolve_entry == nullptr || tls_module_resolve_id == 0 ||
      tls_module_resolve_iso == nullptr) {
    return v8::MaybeLocal<v8::Module>();
  }
  ModuleWrap* referrer_wrap = FindModuleWrap(referrer);
  void* resolved_raw = nullptr;
  if (g_module_resolve_entry(tls_module_resolve_id, ToWire(specifier),
                             ToWire(import_attributes), referrer_wrap,
                             &resolved_raw) != 1) {
    return v8::MaybeLocal<v8::Module>();
  }
  ModuleWrap* resolved = AsModule(resolved_raw);
  if (resolved == nullptr || resolved->iso != tls_module_resolve_iso) {
    return v8::MaybeLocal<v8::Module>();
  }
  if (tls_module_resolve_modules != nullptr &&
      std::find(tls_module_resolve_modules->begin(),
                tls_module_resolve_modules->end(), resolved) ==
          tls_module_resolve_modules->end()) {
    tls_module_resolve_modules->push_back(resolved);
  }
  return resolved->module->Get(resolved->iso);
}

}  // namespace

extern "C" {

__declspec(dllexport) void gov8_module_resolve_set_entry(void* entry) {
  g_module_resolve_entry = reinterpret_cast<GoModuleResolveEntry>(entry);
}

__declspec(dllexport) int64_t gov8_module_compile(
    v8::Isolate* iso, void* ctxw, void* scope, void* tcw, const char* src,
    int64_t src_len, const char* resource, int64_t resource_len,
    int32_t line_offset, int32_t column_offset, void** out_module) {
  ClearErr();
  CtxWrap* context = AsCtx(ctxw);
  if (context == nullptr || out_module == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, context->iso) || (src == nullptr && src_len > 0) ||
      (resource == nullptr && resource_len > 0)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = context->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (tcw != nullptr) {
      TcWrap* tw = AsTc(tcw);
      if (tw == nullptr) { SetErr("invalid trycatch handle"); return kErrMagic; }
      if (!OwnedBy(iso, tw->iso)) return kErrBadArg;
      tc = TcOf(tw);
    }
    if (tc == nullptr) tc = fallback.Init(iso);
    v8::Local<v8::String> source;
    v8::Local<v8::String> name;
    if (!v8::String::NewFromUtf8(iso, src, v8::NewStringType::kNormal,
                                 static_cast<int>(src_len)).ToLocal(&source) ||
        !v8::String::NewFromUtf8(iso, resource, v8::NewStringType::kNormal,
                                 static_cast<int>(resource_len)).ToLocal(&name)) {
      SetErr("String::NewFromUtf8 failed");
      return kErrGeneric;
    }
    v8::ScriptOrigin origin(name, line_offset, column_offset, false, -1,
                            v8::Local<v8::Value>(), false, false, true);
    v8::ScriptCompiler::Source compiler_source(source, origin);
    v8::Local<v8::Module> module;
    if (!v8::ScriptCompiler::CompileModule(iso, &compiler_source).ToLocal(&module)) {
      SetErr("module compile failed");
      return kErrException;
    }
    ModuleWrap* mw = new ModuleWrap{};
    mw->magic = kModuleMagic;
    mw->iso = iso;
    mw->ctx = context;
    mw->module = new v8::Global<v8::Module>(iso, module);
    *out_module = mw;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in module_compile");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_module_dispose(void* modulew) {
  ClearErr();
  ModuleWrap* w = AsModule(modulew);
  if (w == nullptr) { SetErr("invalid module handle"); return kErrMagic; }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    w->module->Reset();
    delete w->module;
    w->magic = 0;
    delete w;
    return kOk;
  } catch (...) { SetErr("C++ exception in module_dispose"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_module_status(void* modulew) {
  ClearErr();
  ModuleWrap* w = AsModule(modulew);
  if (w == nullptr) { SetErr("invalid module handle"); return kErrMagic; }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    v8::HandleScope hs(w->iso);
    return static_cast<int64_t>(w->module->Get(w->iso)->GetStatus());
  } catch (...) { SetErr("C++ exception in module_status"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_module_identity_hash(void* modulew,
                                                        int32_t* out) {
  ClearErr();
  ModuleWrap* w = AsModule(modulew);
  if (w == nullptr || out == nullptr) { SetErr("invalid argument"); return kErrBadArg; }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    v8::HandleScope hs(w->iso);
    *out = w->module->Get(w->iso)->GetIdentityHash();
    return kOk;
  } catch (...) { SetErr("C++ exception in module_identity_hash"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_module_script_id(void* modulew,
                                                    int32_t* out) {
  ClearErr();
  ModuleWrap* w = AsModule(modulew);
  if (w == nullptr || out == nullptr) { SetErr("invalid argument"); return kErrBadArg; }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    v8::HandleScope hs(w->iso);
    v8::Local<v8::Module> module = w->module->Get(w->iso);
    if (!module->IsSourceTextModule() || module->GetStatus() == v8::Module::kErrored) {
      SetErr("script id unavailable");
      return kErrState;
    }
    *out = module->ScriptId();
    return kOk;
  } catch (...) { SetErr("C++ exception in module_script_id"); return kErrCpp; }
}

#define GOV8_MODULE_PREDICATE(name, expression)                              \
  __declspec(dllexport) int64_t name(void* modulew) {                        \
    ClearErr();                                                               \
    ModuleWrap* w = AsModule(modulew);                                        \
    if (w == nullptr) { SetErr("invalid module handle"); return kErrMagic; } \
    try {                                                                     \
      Gov8IsolateScope iso_scope(w->iso);                                     \
      v8::HandleScope hs(w->iso);                                             \
      v8::Local<v8::Module> module = w->module->Get(w->iso);                  \
      return (expression) ? 1 : 0;                                            \
    } catch (...) { SetErr("C++ exception"); return kErrCpp; }               \
  }

GOV8_MODULE_PREDICATE(gov8_module_is_source_text, module->IsSourceTextModule())
GOV8_MODULE_PREDICATE(gov8_module_is_synthetic, module->IsSyntheticModule())
GOV8_MODULE_PREDICATE(gov8_module_has_top_level_await, module->HasTopLevelAwait())
GOV8_MODULE_PREDICATE(gov8_module_is_graph_async, module->IsGraphAsync())

#undef GOV8_MODULE_PREDICATE

__declspec(dllexport) int64_t gov8_module_resolve_throw(
    v8::Isolate* iso, const char* text, int64_t len) {
  ClearErr();
  if (iso == nullptr || (text == nullptr && len > 0)) { SetErr("invalid argument"); return kErrBadArg; }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::String> message;
    if (!v8::String::NewFromUtf8(iso, text, v8::NewStringType::kNormal,
                                 static_cast<int>(len)).ToLocal(&message)) {
      SetErr("String::NewFromUtf8 failed"); return kErrGeneric;
    }
    iso->ThrowException(message);
    return kOk;
  } catch (...) { SetErr("C++ exception in module_resolve_throw"); return kErrCpp; }
}

}  // extern "C"

namespace {

v8::Local<v8::ModuleRequest> ModuleRequestAt(ModuleWrap* w, int32_t index) {
  v8::Local<v8::FixedArray> requests = w->module->Get(w->iso)->GetModuleRequests();
  if (index < 0 || index >= requests->Length()) return v8::Local<v8::ModuleRequest>();
  return requests->Get(index).As<v8::ModuleRequest>();
}

}  // namespace

extern "C" {

__declspec(dllexport) int64_t gov8_module_resolve_specifier(
    v8::Isolate* iso, void* specifier, char* output, int64_t capacity,
    int64_t* out_length) {
  ClearErr();
  if (iso == nullptr || specifier == nullptr || out_length == nullptr ||
      capacity < 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::String::Utf8Value text(iso, FromWire<v8::String>(specifier));
    *out_length = text.length();
    if (output == nullptr || capacity < text.length()) {
      SetErr("output buffer too small");
      return kErrNoMemory;
    }
    if (text.length() > 0) {
      std::memcpy(output, *text, static_cast<size_t>(text.length()));
    }
    return kOk;
  } catch (...) {
    SetErr("C++ exception in module_resolve_specifier");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_module_resolve_attribute_count(
    v8::Isolate* iso, void* attributes) {
  ClearErr();
  if (iso == nullptr || attributes == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    return FromWire<v8::FixedArray>(attributes)->Length() / 3;
  } catch (...) {
    SetErr("C++ exception in module_resolve_attribute_count");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_module_resolve_attribute(
    v8::Isolate* iso, void* attributes, int32_t attribute_index,
    char* key, int64_t key_cap, int64_t* key_len, char* value,
    int64_t value_cap, int64_t* value_len, int32_t* source_offset) {
  ClearErr();
  if (iso == nullptr || attributes == nullptr || key_len == nullptr ||
      value_len == nullptr || source_offset == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::FixedArray> attrs = FromWire<v8::FixedArray>(attributes);
    int32_t base = attribute_index * 3;
    if (attribute_index < 0 || base + 2 >= attrs->Length()) {
      SetErr("module attribute index out of range");
      return kErrBadArg;
    }
    v8::String::Utf8Value key_text(iso, attrs->Get(base).As<v8::String>());
    v8::String::Utf8Value value_text(iso, attrs->Get(base + 1).As<v8::String>());
    std::string key_string(*key_text == nullptr ? "" : *key_text,
                           key_text.length());
    std::string value_string(*value_text == nullptr ? "" : *value_text,
                             value_text.length());
    *key_len = static_cast<int64_t>(key_string.size());
    *value_len = static_cast<int64_t>(value_string.size());
    *source_offset = attrs->Get(base + 2).As<v8::Int32>()->Value();
    if (key == nullptr || value == nullptr || *key_len > key_cap ||
        *value_len > value_cap) {
      SetErr("output buffer too small");
      return kErrNoMemory;
    }
    std::memcpy(key, key_string.data(), key_string.size());
    std::memcpy(value, value_string.data(), value_string.size());
    return kOk;
  } catch (...) {
    SetErr("C++ exception in module_resolve_attribute");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_module_request_count(void* modulew) {
  ClearErr();
  ModuleWrap* w = AsModule(modulew);
  if (w == nullptr) { SetErr("invalid module handle"); return kErrMagic; }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    v8::HandleScope hs(w->iso);
    return w->module->Get(w->iso)->GetModuleRequests()->Length();
  } catch (...) { SetErr("C++ exception in module_request_count"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_module_request_info(
    void* modulew, int32_t index, int32_t* phase, int32_t* source_offset,
    int32_t* attribute_count, char* specifier, int64_t cap,
    int64_t* out_len) {
  ClearErr();
  ModuleWrap* w = AsModule(modulew);
  if (w == nullptr || phase == nullptr || source_offset == nullptr ||
      attribute_count == nullptr || out_len == nullptr) {
    SetErr("invalid argument"); return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    v8::HandleScope hs(w->iso);
    v8::Local<v8::ModuleRequest> request = ModuleRequestAt(w, index);
    if (request.IsEmpty()) { SetErr("module request index out of range"); return kErrBadArg; }
    *phase = static_cast<int32_t>(request->GetPhase());
    *source_offset = request->GetSourceOffset();
    *attribute_count = request->GetImportAttributes()->Length() / 3;
    v8::String::Utf8Value text(w->iso, request->GetSpecifier());
    std::string value(*text == nullptr ? "" : *text, text.length());
    return BufferUtf8(value, specifier, cap, out_len);
  } catch (...) { SetErr("C++ exception in module_request_info"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_module_request_attribute(
    void* modulew, int32_t request_index, int32_t attribute_index,
    char* key, int64_t key_cap, int64_t* key_len, char* value,
    int64_t value_cap, int64_t* value_len, int32_t* source_offset) {
  ClearErr();
  ModuleWrap* w = AsModule(modulew);
  if (w == nullptr || key_len == nullptr || value_len == nullptr ||
      source_offset == nullptr) { SetErr("invalid argument"); return kErrBadArg; }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    v8::HandleScope hs(w->iso);
    v8::Local<v8::ModuleRequest> request = ModuleRequestAt(w, request_index);
    if (request.IsEmpty()) { SetErr("module request index out of range"); return kErrBadArg; }
    v8::Local<v8::FixedArray> attrs = request->GetImportAttributes();
    int32_t base = attribute_index * 3;
    if (attribute_index < 0 || base + 2 >= attrs->Length()) {
      SetErr("module attribute index out of range"); return kErrBadArg;
    }
    v8::String::Utf8Value key_text(w->iso, attrs->Get(base).As<v8::String>());
    v8::String::Utf8Value value_text(w->iso, attrs->Get(base + 1).As<v8::String>());
    std::string key_string(*key_text == nullptr ? "" : *key_text, key_text.length());
    std::string value_string(*value_text == nullptr ? "" : *value_text, value_text.length());
    *key_len = static_cast<int64_t>(key_string.size());
    *value_len = static_cast<int64_t>(value_string.size());
    *source_offset = attrs->Get(base + 2).As<v8::Int32>()->Value();
    if (key == nullptr || value == nullptr || *key_len > key_cap ||
        *value_len > value_cap) { SetErr("output buffer too small"); return kErrNoMemory; }
    std::memcpy(key, key_string.data(), key_string.size());
    std::memcpy(value, value_string.data(), value_string.size());
    return kOk;
  } catch (...) { SetErr("C++ exception in module_request_attribute"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_module_source_location(
    void* modulew, int32_t offset, int32_t* line, int32_t* column) {
  ClearErr();
  ModuleWrap* w = AsModule(modulew);
  if (w == nullptr || line == nullptr || column == nullptr) { SetErr("invalid argument"); return kErrBadArg; }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    v8::HandleScope hs(w->iso);
    v8::Location loc = w->module->Get(w->iso)->SourceOffsetToLocation(offset);
    *line = loc.GetLineNumber();
    *column = loc.GetColumnNumber();
    return kOk;
  } catch (...) { SetErr("C++ exception in module_source_location"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_module_instantiate(
    v8::Isolate* iso, void* ctxw, void* scope, void* modulew, void* tcw,
    int64_t resolve_id, void** modules, int64_t module_count, int32_t* out) {
  ClearErr();
  CtxWrap* context = AsCtx(ctxw);
  ModuleWrap* root = AsModule(modulew);
  if (context == nullptr || root == nullptr || out == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, context->iso) ||
      !OwnedBy(iso, root->iso) || root->ctx != context ||
      (modules == nullptr && module_count > 0)) {
    SetErr("invalid argument"); return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = context->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (tcw != nullptr) {
      TcWrap* tw = AsTc(tcw);
      if (tw == nullptr) { SetErr("invalid trycatch handle"); return kErrMagic; }
      if (!OwnedBy(iso, tw->iso)) return kErrBadArg;
      tc = TcOf(tw);
    }
    if (tc == nullptr) tc = fallback.Init(iso);
    std::vector<ModuleWrap*> known;
    known.reserve(static_cast<size_t>(module_count) + 1);
    for (int64_t n = 0; n < module_count; ++n) {
      ModuleWrap* candidate = AsModule(modules[n]);
      if (candidate != nullptr && candidate->iso == iso) known.push_back(candidate);
    }
    if (std::find(known.begin(), known.end(), root) == known.end()) known.push_back(root);
    int64_t old_id = tls_module_resolve_id;
    v8::Isolate* old_iso = tls_module_resolve_iso;
    auto* old_modules = tls_module_resolve_modules;
    tls_module_resolve_id = resolve_id;
    tls_module_resolve_iso = iso;
    tls_module_resolve_modules = &known;
    v8::Maybe<bool> result = root->module->Get(iso)->InstantiateModule(ctx, Gov8ResolveModule);
    tls_module_resolve_id = old_id;
    tls_module_resolve_iso = old_iso;
    tls_module_resolve_modules = old_modules;
    if (result.IsNothing()) { SetErr("module instantiate failed"); return kErrException; }
    *out = result.FromJust() ? 1 : 0;
    return kOk;
  } catch (...) {
    tls_module_resolve_id = 0;
    tls_module_resolve_iso = nullptr;
    tls_module_resolve_modules = nullptr;
    SetErr("C++ exception in module_instantiate");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_module_evaluate(
    v8::Isolate* iso, void* ctxw, void* scope, void* modulew, void* tcw,
    void** out_value) {
  ClearErr();
  CtxWrap* context = AsCtx(ctxw);
  ModuleWrap* module = AsModule(modulew);
  if (context == nullptr || module == nullptr || out_value == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, context->iso) ||
      !OwnedBy(iso, module->iso) || module->ctx != context) {
    SetErr("invalid argument"); return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = context->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (tcw != nullptr) {
      TcWrap* tw = AsTc(tcw);
      if (tw == nullptr) { SetErr("invalid trycatch handle"); return kErrMagic; }
      if (!OwnedBy(iso, tw->iso)) return kErrBadArg;
      tc = TcOf(tw);
    }
    if (tc == nullptr) tc = fallback.Init(iso);
    v8::Local<v8::Value> value;
    if (!module->module->Get(iso)->Evaluate(ctx).ToLocal(&value)) {
      SetErr("module evaluate failed"); return kErrException;
    }
    *out_value = ToWire(value);
    return kOk;
  } catch (...) { SetErr("C++ exception in module_evaluate"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_module_namespace(
    v8::Isolate* iso, void* scope, void* modulew, void** out_value) {
  ClearErr();
  ModuleWrap* module = AsModule(modulew);
  if (module == nullptr || out_value == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, module->iso)) { SetErr("invalid argument"); return kErrBadArg; }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = module->ctx->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    *out_value = ToWire(module->module->Get(iso)->GetModuleNamespace());
    return kOk;
  } catch (...) { SetErr("C++ exception in module_namespace"); return kErrCpp; }
}

__declspec(dllexport) int64_t gov8_module_exception(
    v8::Isolate* iso, void* scope, void* modulew, void** out_value) {
  ClearErr();
  ModuleWrap* module = AsModule(modulew);
  if (module == nullptr || out_value == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, module->iso)) { SetErr("invalid argument"); return kErrBadArg; }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = module->ctx->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    *out_value = ToWire(module->module->Get(iso)->GetException());
    return kOk;
  } catch (...) { SetErr("C++ exception in module_exception"); return kErrCpp; }
}

}  // extern "C"
