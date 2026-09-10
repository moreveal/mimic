// module_cache.inc -- UnboundModuleScript and SourceTextModule code cache.
// Included after modules.inc so ModuleWrap/AsModule are available.

namespace gov8_module_cache_impl {

extern "C" {
const v8::UnboundModuleScript* v8__Module__GetUnboundModuleScript(
    const v8::Module& self);
v8::ScriptCompiler::CachedData*
v8__UnboundModuleScript__CreateCodeCache(
    const v8::UnboundModuleScript& unbound_module_script);
v8::Value* v8__UnboundModuleScript__GetSourceMappingURL(
    const v8::UnboundModuleScript& unbound_module_script);
v8::Value* v8__UnboundModuleScript__GetSourceURL(
    const v8::UnboundModuleScript& unbound_module_script);
void v8__ScriptCompiler__CachedData__DELETE(
    v8::ScriptCompiler::CachedData* self);
}

constexpr uint64_t kUnboundModuleMagic =
    0x674f5638554d4f44ULL;  // "gOV8UMOD"

struct UnboundModuleWrap {
  uint64_t magic;
  v8::Isolate* iso;
  v8::Global<v8::UnboundModuleScript>* script;
};

UnboundModuleWrap* AsUnboundModule(void* value) {
  if (value == nullptr) return nullptr;
  auto* wrap = static_cast<UnboundModuleWrap*>(value);
  return wrap->magic == kUnboundModuleMagic ? wrap : nullptr;
}

template <class T>
T& ModuleCacheDeref(const v8::Local<T>& local) {
  return *local.operator->();
}

}  // namespace gov8_module_cache_impl

extern "C" {

using namespace gov8_module_cache_impl;

__declspec(dllexport) int64_t gov8_module_cache_get_unbound(
    void* modulew, void** out_unbound) {
  ClearErr();
  ModuleWrap* module = AsModule(modulew);
  if (module == nullptr || out_unbound == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(module->iso);
    v8::HandleScope handle_scope(module->iso);
    v8::Local<v8::Module> local = module->module->Get(module->iso);
    if (!local->IsSourceTextModule()) {
      SetErr("module is not a SourceTextModule");
      return kErrState;
    }
    const v8::UnboundModuleScript* raw =
        v8__Module__GetUnboundModuleScript(ModuleCacheDeref(local));
    if (raw == nullptr) {
      SetErr("GetUnboundModuleScript returned empty");
      return kErrState;
    }
    v8::Local<v8::UnboundModuleScript> unbound =
        FromWire<v8::UnboundModuleScript>(
            const_cast<v8::UnboundModuleScript*>(raw));
    auto* wrap = new UnboundModuleWrap{};
    wrap->magic = kUnboundModuleMagic;
    wrap->iso = module->iso;
    wrap->script = new v8::Global<v8::UnboundModuleScript>(
        module->iso, unbound);
    *out_unbound = wrap;
    return kOk;
  } catch (...) {
    SetErr("C++ exception getting unbound module script");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_module_cache_unbound_dispose(
    void* unboundw) {
  ClearErr();
  UnboundModuleWrap* wrap = AsUnboundModule(unboundw);
  if (wrap == nullptr) {
    SetErr("invalid unbound module handle");
    return kErrMagic;
  }
  try {
    Gov8IsolateScope isolate_scope(wrap->iso);
    wrap->script->Reset();
    delete wrap->script;
    wrap->magic = 0;
    delete wrap;
    return kOk;
  } catch (...) {
    SetErr("C++ exception disposing unbound module script");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_module_cache_unbound_metadata(
    v8::Isolate* iso, void* scope, void* unboundw, int32_t field,
    void** out_value) {
  ClearErr();
  UnboundModuleWrap* wrap = AsUnboundModule(unboundw);
  if (wrap == nullptr || out_value == nullptr || field < 0 || field > 1 ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, wrap->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::UnboundModuleScript> local =
        wrap->script->Get(wrap->iso);
    v8::Value* value = field == 0
                           ? v8__UnboundModuleScript__GetSourceURL(
                                 ModuleCacheDeref(local))
                           : v8__UnboundModuleScript__GetSourceMappingURL(
                                 ModuleCacheDeref(local));
    *out_value = value;
    return kOk;
  } catch (...) {
    SetErr("C++ exception reading unbound module metadata");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_module_cache_unbound_script_id(
    void* unboundw, int32_t* out_id) {
  ClearErr();
  UnboundModuleWrap* wrap = AsUnboundModule(unboundw);
  if (wrap == nullptr || out_id == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(wrap->iso);
    v8::HandleScope handle_scope(wrap->iso);
    *out_id = wrap->script->Get(wrap->iso)->ScriptId();
    return kOk;
  } catch (...) {
    SetErr("C++ exception reading unbound module script id");
    return kErrCpp;
  }
}

__declspec(dllexport) void* gov8_module_cache_create(void* unboundw) {
  ClearErr();
  UnboundModuleWrap* wrap = AsUnboundModule(unboundw);
  if (wrap == nullptr) {
    SetErr("invalid unbound module handle");
    return nullptr;
  }
  try {
    Gov8IsolateScope isolate_scope(wrap->iso);
    v8::HandleScope handle_scope(wrap->iso);
    return v8__UnboundModuleScript__CreateCodeCache(
        ModuleCacheDeref(wrap->script->Get(wrap->iso)));
  } catch (...) {
    SetErr("C++ exception creating module code cache");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_module_cache_read_delete(
    void* cache, uint8_t* output, int64_t capacity, int64_t* out_length) {
  ClearErr();
  auto* data = static_cast<v8::ScriptCompiler::CachedData*>(cache);
  if (data == nullptr || out_length == nullptr || capacity < 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  *out_length = data->length;
  if (data->length < 0 || output == nullptr || capacity < data->length) {
    v8__ScriptCompiler__CachedData__DELETE(data);
    SetErr("output buffer too small");
    return kErrNoMemory;
  }
  if (data->length > 0) {
    std::memcpy(output, data->data, static_cast<size_t>(data->length));
  }
  v8__ScriptCompiler__CachedData__DELETE(data);
  return kOk;
}

__declspec(dllexport) int64_t gov8_module_cache_compile(
    v8::Isolate* iso, void* ctxw, void* scope, void* tcw,
    const char* src, int64_t src_len, const char* resource,
    int64_t resource_len, int32_t line_offset, int32_t column_offset,
    const uint8_t* cache, int64_t cache_len, int32_t consume,
    void** out_module, int32_t* out_rejected) {
  ClearErr();
  CtxWrap* context = AsCtx(ctxw);
  if (context == nullptr || out_module == nullptr || out_rejected == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, context->iso) ||
      (src == nullptr && src_len > 0) || src_len < 0 || src_len > INT_MAX ||
      (resource == nullptr && resource_len > 0) || resource_len < 0 ||
      resource_len > INT_MAX || (consume != 0 && consume != 1) ||
      (consume != 0 && (cache == nullptr || cache_len <= 0)) ||
      cache_len > INT_MAX) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::Context> ctx = context->ctx->Get(iso);
    v8::Context::Scope context_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* try_catch = nullptr;
    if (tcw != nullptr) {
      TcWrap* wrap = AsTc(tcw);
      if (wrap == nullptr || !OwnedBy(iso, wrap->iso)) {
        SetErr("invalid trycatch handle");
        return kErrBadArg;
      }
      try_catch = TcOf(wrap);
    }
    if (try_catch == nullptr) try_catch = fallback.Init(iso);

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
    v8::ScriptCompiler::CachedData* cached = nullptr;
    if (consume != 0) {
      cached = new v8::ScriptCompiler::CachedData(
          cache, static_cast<int>(cache_len),
          v8::ScriptCompiler::CachedData::BufferNotOwned);
      if (cached == nullptr) {
        SetErr("CachedData allocation failed");
        return kErrGeneric;
      }
    }
    v8::ScriptCompiler::Source compiler_source(source, origin, cached);
    v8::Local<v8::Module> module;
    const auto options = consume != 0
                             ? v8::ScriptCompiler::kConsumeCodeCache
                             : v8::ScriptCompiler::kNoCompileOptions;
    const bool compiled = v8::ScriptCompiler::CompileModule(
                              iso, &compiler_source, options,
                              v8::ScriptCompiler::kNoCacheNoReason)
                              .ToLocal(&module);
    const v8::ScriptCompiler::CachedData* consumed =
        compiler_source.GetCachedData();
    *out_rejected = consumed != nullptr && consumed->rejected ? 1 : 0;
    if (!compiled) {
      SetErr("module compile failed");
      return kErrException;
    }
    auto* wrap = new ModuleWrap{};
    wrap->magic = kModuleMagic;
    wrap->iso = iso;
    wrap->ctx = context;
    wrap->module = new v8::Global<v8::Module>(iso, module);
    *out_module = wrap;
    return kOk;
  } catch (...) {
    SetErr("C++ exception compiling cached module");
    return kErrCpp;
  }
}

__declspec(dllexport) void* gov8_module_cache_compile_local(
    v8::Isolate* iso, void* ctxw, void* scope, void* tcw,
    const char* src, int64_t src_len, const char* resource,
    int64_t resource_len, int32_t line_offset, int32_t column_offset,
    const uint8_t* cache, int64_t cache_len, int32_t consume,
    int32_t* out_rejected) {
  ClearErr();
  CtxWrap* context = AsCtx(ctxw);
  if (context == nullptr || out_rejected == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, context->iso) ||
      (src == nullptr && src_len > 0) || src_len < 0 || src_len > INT_MAX ||
      (resource == nullptr && resource_len > 0) || resource_len < 0 ||
      resource_len > INT_MAX || (consume != 0 && consume != 1) ||
      (consume != 0 && (cache == nullptr || cache_len <= 0)) ||
      cache_len > INT_MAX) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::Context> ctx = context->ctx->Get(iso);
    v8::Context::Scope context_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* try_catch = nullptr;
    if (tcw != nullptr) {
      TcWrap* wrap = AsTc(tcw);
      if (wrap == nullptr || !OwnedBy(iso, wrap->iso)) {
        SetErr("invalid trycatch handle");
        return nullptr;
      }
      try_catch = TcOf(wrap);
    }
    if (try_catch == nullptr) try_catch = fallback.Init(iso);

    v8::Local<v8::String> source;
    v8::Local<v8::String> name;
    if (!v8::String::NewFromUtf8(iso, src, v8::NewStringType::kNormal,
                                 static_cast<int>(src_len)).ToLocal(&source) ||
        !v8::String::NewFromUtf8(iso, resource, v8::NewStringType::kNormal,
                                 static_cast<int>(resource_len)).ToLocal(&name)) {
      SetErr("String::NewFromUtf8 failed");
      return nullptr;
    }
    v8::ScriptOrigin origin(name, line_offset, column_offset, false, -1,
                            v8::Local<v8::Value>(), false, false, true);
    v8::ScriptCompiler::CachedData* cached = nullptr;
    if (consume != 0) {
      cached = new v8::ScriptCompiler::CachedData(
          cache, static_cast<int>(cache_len),
          v8::ScriptCompiler::CachedData::BufferNotOwned);
      if (cached == nullptr) {
        SetErr("CachedData allocation failed");
        return nullptr;
      }
    }
    v8::ScriptCompiler::Source compiler_source(source, origin, cached);
    v8::Local<v8::Module> module;
    const auto options = consume != 0
                             ? v8::ScriptCompiler::kConsumeCodeCache
                             : v8::ScriptCompiler::kNoCompileOptions;
    const bool compiled = v8::ScriptCompiler::CompileModule(
                              iso, &compiler_source, options,
                              v8::ScriptCompiler::kNoCacheNoReason)
                              .ToLocal(&module);
    const v8::ScriptCompiler::CachedData* consumed =
        compiler_source.GetCachedData();
    *out_rejected = consumed != nullptr && consumed->rejected ? 1 : 0;
    if (!compiled) {
      SetErr("module compile failed");
      return nullptr;
    }
    return ToWire(module);
  } catch (...) {
    SetErr("C++ exception compiling scope-local cached module");
    return nullptr;
  }
}

}  // extern "C"
