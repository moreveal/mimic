// Advanced exception/message/stack-frame bindings for rusty_v8 152.2.0.
//
// This file is included after core_advanced.inc, so the core wrapper types,
// validation helpers, wire helpers, and error protocol are already visible.
// Scope-local results are deliberately returned without opening an inner
// HandleScope: their slots must remain valid in the caller's Go Scope.

extern "C" {

bool v8__TryCatch__IsVerbose(const v8::TryCatch& self);
void v8__TryCatch__SetVerbose(v8::TryCatch* self, bool value);
void v8__TryCatch__SetCaptureMessage(v8::TryCatch* self, bool value);
const v8::Value* v8__TryCatch__Exception(const v8::TryCatch& self);
const v8::Value* v8__TryCatch__StackTrace(const v8::TryCatch& self,
                                          const v8::Context& context);
const v8::Value* v8__TryCatch__ReThrow(v8::TryCatch* self);

int v8__Message__GetWasmFunctionIndex(const v8::Message& self);

const v8::String* v8__StackFrame__GetScriptNameOrSourceURL(
    const v8::StackFrame& self);
const v8::String* v8__StackFrame__GetScriptSource(
    const v8::StackFrame& self);
const v8::String* v8__StackFrame__GetScriptSourceMappingURL(
    const v8::StackFrame& self);

__declspec(dllexport) int64_t gov8_ea_tc_is_verbose(void* tcw) {
  ClearErr();
  TcWrap* w = AsTc(tcw);
  if (w == nullptr) {
    SetErr("invalid trycatch handle");
    return kErrMagic;
  }
  try {
    return v8__TryCatch__IsVerbose(*w->tc()) ? 1 : 0;
  } catch (...) {
    SetErr("C++ exception in tc_is_verbose");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ea_tc_set_verbose(void* tcw,
                                                     int32_t value) {
  ClearErr();
  TcWrap* w = AsTc(tcw);
  if (w == nullptr) {
    SetErr("invalid trycatch handle");
    return kErrMagic;
  }
  try {
    v8__TryCatch__SetVerbose(w->tc(), value != 0);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in tc_set_verbose");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ea_tc_set_capture_message(void* tcw,
                                                             int32_t value) {
  ClearErr();
  TcWrap* w = AsTc(tcw);
  if (w == nullptr) {
    SetErr("invalid trycatch handle");
    return kErrMagic;
  }
  try {
    v8__TryCatch__SetCaptureMessage(w->tc(), value != 0);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in tc_set_capture_message");
    return kErrCpp;
  }
}

// Returns Exception() in the caller's HandleScope, or a null wire when the
// TryCatch is empty.
__declspec(dllexport) int64_t gov8_ea_tc_exception(v8::Isolate* iso,
                                                   void* tcw, void* scope,
                                                   void** out) {
  ClearErr();
  TcWrap* w = AsTc(tcw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    *out = const_cast<void*>(static_cast<const void*>(
        v8__TryCatch__Exception(*w->tc())));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in tc_exception");
    return kErrCpp;
  }
}

// ReThrow schedules the caught exception for the next outer handler. The
// pinned binding returns V8's ReThrow return value (observably undefined in
// this build), not the originally caught value.
__declspec(dllexport) int64_t gov8_ea_tc_rethrow(v8::Isolate* iso, void* tcw,
                                                 void* scope, void** out) {
  ClearErr();
  TcWrap* w = AsTc(tcw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    *out = const_cast<void*>(static_cast<const void*>(
        v8__TryCatch__ReThrow(w->tc())));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in tc_rethrow");
    return kErrCpp;
  }
}

// Returns the caught exception's .stack value, or a null wire when there is
// no caught exception / stack property.
__declspec(dllexport) int64_t gov8_ea_tc_stack_trace(
    v8::Isolate* iso, void* ctxw, void* tcw, void* scope, void** out) {
  ClearErr();
  CtxWrap* cw = AsCtx(ctxw);
  TcWrap* tw = AsTc(tcw);
  if (cw == nullptr || tw == nullptr || out == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, cw->iso) ||
      !OwnedBy(iso, tw->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = cw->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    *out = const_cast<void*>(static_cast<const void*>(
        v8__TryCatch__StackTrace(*tw->tc(),
                                 gov8_snap_impl::DerefLocal(ctx))));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in tc_stack_trace");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ea_message_wasm_function_index(
    v8::Isolate* iso, void* scope, void* msg, int32_t* out) {
  ClearErr();
  if (msg == nullptr || out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    *out = static_cast<int32_t>(v8__Message__GetWasmFunctionIndex(
        gov8_snap_impl::DerefLocal(FromWire<v8::Message>(msg))));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in message_wasm_function_index");
    return kErrCpp;
  }
}

#define GOV8_EA_FRAME_TEXT(name, binding)                                      \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* scope,            \
                                     void* frame, void** out) {                \
    ClearErr();                                                                \
    if (frame == nullptr || out == nullptr || !ScopeIs(iso, scope)) {          \
      SetErr("invalid argument");                                              \
      return kErrBadArg;                                                       \
    }                                                                          \
    try {                                                                      \
      Gov8IsolateScope iso_scope(iso);                                         \
      *out = const_cast<void*>(static_cast<const void*>(binding(               \
          gov8_snap_impl::DerefLocal(FromWire<v8::StackFrame>(frame)))));      \
      return kOk;                                                              \
    } catch (...) {                                                            \
      SetErr("C++ exception in advanced frame getter");                       \
      return kErrCpp;                                                          \
    }                                                                          \
  }

GOV8_EA_FRAME_TEXT(gov8_ea_frame_script_name_or_source_url,
                   v8__StackFrame__GetScriptNameOrSourceURL)
GOV8_EA_FRAME_TEXT(gov8_ea_frame_script_source,
                   v8__StackFrame__GetScriptSource)
GOV8_EA_FRAME_TEXT(gov8_ea_frame_script_source_mapping_url,
                   v8__StackFrame__GetScriptSourceMappingURL)

#undef GOV8_EA_FRAME_TEXT

}  // extern "C"
