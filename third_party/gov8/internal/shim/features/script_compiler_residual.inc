// Executable residual ScriptCompiler surface for rusty_v8 152.2.0.

extern "C" {
const v8::Data* v8__Isolate__GetCurrentHostDefinedOptions(
    v8::Isolate* isolate);
}

namespace gov8_scr_impl {

struct SourceGuard {
  alignas(v8::ScriptCompiler::Source)
      unsigned char storage[sizeof(v8::ScriptCompiler::Source)];
  bool constructed = false;

  v8::ScriptCompiler::Source* get() {
    return reinterpret_cast<v8::ScriptCompiler::Source*>(storage);
  }
  ~SourceGuard() {
    if (constructed) {
      gov8_ca_impl::v8__ScriptCompiler__Source__DESTRUCT(storage);
    }
  }
};

struct CachedDataDeleter {
  void operator()(v8::ScriptCompiler::CachedData* data) const {
    if (data != nullptr) {
      gov8_ca_impl::v8__ScriptCompiler__CachedData__DELETE(data);
    }
  }
};

}  // namespace gov8_scr_impl

extern "C" {

using namespace gov8_ca_impl;

__declspec(dllexport) int64_t gov8_scr_compile(
    v8::Isolate* isolate, void* context_wrap, void* scope, void* try_catch,
    const char* source_bytes, int64_t source_len, int32_t origin_present,
    void* resource_name, int32_t line_offset, int32_t column_offset,
    int32_t script_id, void* source_map_url, void* host_defined_options,
    int32_t origin_flags, uint32_t compile_options, int32_t no_cache_reason,
    int32_t cache_present, const uint8_t* cache_bytes, int64_t cache_len,
    void** out_script, int32_t* out_rejected) {
  ClearErr();
  CtxWrap* context = AsCtx(context_wrap);
  if (isolate == nullptr || context == nullptr || !OwnedBy(isolate, context->iso) ||
      !ScopeIs(isolate, scope) || out_script == nullptr ||
      out_rejected == nullptr || source_len < 0 || cache_len < 0 ||
      source_len > INT32_MAX || cache_len > INT32_MAX ||
      (source_bytes == nullptr && source_len != 0) ||
      (cache_bytes == nullptr && cache_len != 0) ||
      (origin_present != 0 && origin_present != 1) ||
      (cache_present != 0 && cache_present != 1) ||
      (origin_present != 0 && resource_name == nullptr) ||
      no_cache_reason < 0 || no_cache_reason > 14) {
    SetErr("invalid script compiler argument");
    return kErrBadArg;
  }
  if ((compile_options & 1u) != 0 && cache_present == 0) {
    SetErr("ConsumeCodeCache requires cached data");
    return kErrBadArg;
  }
  *out_script = nullptr;
  *out_rejected = 0;
  try {
    Gov8IsolateScope isolate_scope(isolate);
    v8::Local<v8::Context> context_local = context->ctx->Get(isolate);
    v8::Context::Scope context_scope(context_local);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!gov8_ca_impl::CaResolveTc(isolate, try_catch, &fallback, &tc)) {
      SetErr("invalid trycatch");
      return kErrBadArg;
    }
    v8::Local<v8::String> source;
    if (!v8::String::NewFromUtf8(isolate, source_bytes,
                                 v8::NewStringType::kNormal,
                                 static_cast<int>(source_len)).ToLocal(&source)) {
      SetErr("source string allocation failed");
      return kErrGeneric;
    }

    std::optional<v8::ScriptOrigin> origin;
    if (origin_present != 0) {
      v8::Local<v8::Value> resource = FromWire<v8::Value>(resource_name);
      v8::Local<v8::Value> source_map;
      if (source_map_url != nullptr) {
        source_map = FromWire<v8::Value>(source_map_url);
      }
      v8::Local<v8::Data> host_options;
      if (host_defined_options != nullptr) {
        host_options = FromWire<v8::Data>(host_defined_options);
      }
      origin.emplace(resource, line_offset, column_offset,
                     (origin_flags & 1) != 0, script_id, source_map,
                     (origin_flags & 2) != 0, (origin_flags & 4) != 0,
                     false, host_options);
    }

    std::unique_ptr<v8::ScriptCompiler::CachedData,
                    gov8_scr_impl::CachedDataDeleter>
        cached_owner;
    if (cache_present != 0) {
      const uint8_t empty_cache_sentinel = 0;
      const uint8_t* cache_input =
          cache_bytes != nullptr ? cache_bytes : &empty_cache_sentinel;
      cached_owner.reset(v8__ScriptCompiler__CachedData__NEW(
          cache_input, static_cast<int>(cache_len)));
      if (cached_owner == nullptr) {
        SetErr("CachedData allocation failed");
        return kErrGeneric;
      }
    }
    gov8_scr_impl::SourceGuard source_guard;
    v8__ScriptCompiler__Source__CONSTRUCT(
        source_guard.storage, gov8_snap_impl::DerefLocal(source),
        origin.has_value() ? &*origin : nullptr, cached_owner.get());
    source_guard.constructed = true;
    cached_owner.release();  // Source owns the CachedData object now.

    const v8::Script* raw = v8__ScriptCompiler__Compile(
        &gov8_snap_impl::DerefLocal(context_local), source_guard.get(),
        static_cast<v8::ScriptCompiler::CompileOptions>(compile_options),
        static_cast<v8::ScriptCompiler::NoCacheReason>(no_cache_reason));
    if (cache_present != 0) {
      const v8::ScriptCompiler::CachedData* observed =
          v8__ScriptCompiler__Source__GetCachedData(source_guard.get());
      if (observed == nullptr) {
        SetErr("cached data disappeared from source");
        return kErrState;
      }
      *out_rejected = observed->rejected ? 1 : 0;
    }
    if (raw == nullptr) {
      SetErr("script compilation failed");
      return kErrException;
    }
    v8::Local<v8::Script> script;
    std::memcpy(&script, &raw, sizeof(void*));
    auto wrap = std::make_unique<ScriptWrap>();
    auto global = std::make_unique<v8::Global<v8::Script>>(isolate, script);
    wrap->magic = kScriptMagic;
    wrap->iso = isolate;
    wrap->script = global.release();
    *out_script = wrap.release();
    return kOk;
  } catch (...) {
    SetErr("C++ exception in residual script compiler");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_scr_current_host_options(
    v8::Isolate* isolate, void* scope, void** out) {
  ClearErr();
  if (isolate == nullptr || !ScopeIs(isolate, scope) || out == nullptr) {
    SetErr("invalid current host options argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    const v8::Data* raw = v8__Isolate__GetCurrentHostDefinedOptions(isolate);
    *out = const_cast<void*>(static_cast<const void*>(raw));
    return kOk;
  } catch (...) {
    SetErr("C++ exception getting current host options");
    return kErrCpp;
  }
}

}  // extern "C"
