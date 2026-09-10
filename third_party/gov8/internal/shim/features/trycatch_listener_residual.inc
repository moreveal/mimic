// Residual TryCatch unwind, Message identity, and uncaught compile support.
// Include after core_advanced.inc and exceptions_advanced.inc.

#include <climits>

extern "C" {

// Calls ReThrow and destroys the innermost TryCatch before returning to Go.
// This makes propagation to the next outer handler immediate and prevents
// host code from continuing with a TryCatch whose exception is pending.
__declspec(dllexport) int64_t gov8_tlr_tc_rethrow_and_close(
    v8::Isolate* iso, void* tcw, void* scope, void** out) {
  ClearErr();
  if (out) *out = nullptr;
  TcWrap* w = AsTc(tcw);
  if (!iso || !w || !out || !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid rethrow argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope enter(iso);
    *out = ToWire(w->tc()->ReThrow());
    w->tc()->~TryCatch();
    w->magic = 0;
    delete w;
    return kOk;
  } catch (...) {
    SetErr("C++ exception rethrowing TryCatch");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_tlr_message_same(
    v8::Isolate* iso, void* scope, void* left, void* right) {
  ClearErr();
  if (!iso || !ScopeIs(iso, scope) || !left || !right) {
    SetErr("invalid message identity argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope enter(iso);
    return FromWire<v8::Message>(left) == FromWire<v8::Message>(right) ? 1 : 0;
  } catch (...) {
    SetErr("C++ exception comparing messages");
    return kErrCpp;
  }
}

// Compiles without installing the shim's fallback TryCatch so syntax errors
// reach isolate message listeners exactly like rusty_v8 Script::compile.
__declspec(dllexport) int64_t gov8_tlr_compile_uncaught_origin(
    v8::Isolate* iso, void* ctxw, void* scope, const char* src,
    int64_t src_len, int32_t has_origin, const char* name, int64_t name_len,
    int64_t line_offset, int64_t column_offset, int64_t script_id,
    const char* smap, int64_t smap_len, int64_t flags, void** out_script) {
  ClearErr();
  if (out_script) *out_script = nullptr;
  CtxWrap* w = AsCtx(ctxw);
  if (!iso || !w || !out_script || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso) || (has_origin != 0 && has_origin != 1) ||
      src_len < 0 || src_len > INT_MAX || name_len < 0 ||
      name_len > INT_MAX || smap_len < 0 || smap_len > INT_MAX ||
      (!src && src_len) || (!name && name_len) || (!smap && smap_len)) {
    SetErr("invalid uncaught compile argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope enter(iso);
    auto ctx = w->ctx->Get(iso);
    v8::Context::Scope context_scope(ctx);
    v8::Local<v8::String> source;
    if (!v8::String::NewFromUtf8(iso, src, v8::NewStringType::kNormal,
                                 static_cast<int>(src_len)).ToLocal(&source)) {
      SetErr("String::NewFromUtf8 failed");
      return kErrGeneric;
    }
    v8::MaybeLocal<v8::Script> maybe;
    if (has_origin) {
      v8::Local<v8::String> name_s;
      v8::Local<v8::String> smap_s;
      if (!CaOriginStrings(iso, name, name_len, smap, smap_len, &name_s,
                           &smap_s)) return kErrBadArg;
      auto origin = CaMakeOrigin(name_s, line_offset, column_offset,
                                 script_id, smap_s, flags);
      maybe = v8::Script::Compile(ctx, source, &origin);
    } else {
      maybe = v8::Script::Compile(ctx, source);
    }
    v8::Local<v8::Script> script;
    if (!maybe.ToLocal(&script)) {
      SetErr("compile failed");
      return kErrException;
    }
    auto* sw = new ScriptWrap{};
    sw->magic = kScriptMagic;
    sw->iso = iso;
    sw->script = new v8::Global<v8::Script>(iso, script);
    *out_script = sw;
    return kOk;
  } catch (...) {
    SetErr("C++ exception compiling without TryCatch");
    return kErrCpp;
  }
}

}  // extern "C"
