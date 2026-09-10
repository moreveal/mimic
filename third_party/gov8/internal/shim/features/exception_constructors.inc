// ===========================================================================
// Native exception constructors and Exception::CreateMessage/GetStackTrace.
// Every export requires both a live Context wrapper and caller HandleScope,
// then enters that context before invoking the pinned artifact binding. This
// is safety-critical: the context-free constructor path access-violates when
// no V8 context is current.
// ===========================================================================

extern "C" {
const v8::Value* v8__Exception__Error(const v8::String& message);
const v8::Value* v8__Exception__RangeError(const v8::String& message);
const v8::Value* v8__Exception__ReferenceError(const v8::String& message);
const v8::Value* v8__Exception__SyntaxError(const v8::String& message);
const v8::Value* v8__Exception__TypeError(const v8::String& message);
const v8::Message* v8__Exception__CreateMessage(v8::Isolate* isolate,
                                                const v8::Value& exception);
const v8::StackTrace* v8__Exception__GetStackTrace(
    const v8::Value& exception);
}

extern "C" {

__declspec(dllexport) int64_t gov8_ec_exception_new(
    v8::Isolate* iso, void* ctxw, void* scope, int32_t kind,
    void* message_wire, void** out) {
  ClearErr();
  CtxWrap* context_wrap = AsCtx(ctxw);
  if (context_wrap == nullptr || out == nullptr || message_wire == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, context_wrap->iso) || kind < 0 ||
      kind > 4) {
    SetErr("invalid exception constructor argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::Value> message = FromWire<v8::Value>(message_wire);
    if (!v8__Value__IsString(gov8_snap_impl::DerefLocal(message))) {
      SetErr("exception message is not a string");
      return kErrBadArg;
    }
    v8::Local<v8::Context> context = context_wrap->ctx->Get(iso);
    v8::Context::Scope context_scope(context);
    const v8::String& string =
        gov8_snap_impl::DerefLocal(message.As<v8::String>());
    const v8::Value* result = nullptr;
    switch (kind) {
      case 0:
        result = v8__Exception__Error(string);
        break;
      case 1:
        result = v8__Exception__RangeError(string);
        break;
      case 2:
        result = v8__Exception__ReferenceError(string);
        break;
      case 3:
        result = v8__Exception__SyntaxError(string);
        break;
      case 4:
        result = v8__Exception__TypeError(string);
        break;
    }
    if (result == nullptr) {
      SetErr("exception constructor returned empty");
      return kErrException;
    }
    *out = const_cast<v8::Value*>(result);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in exception constructor");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ec_exception_create_message(
    v8::Isolate* iso, void* ctxw, void* scope, void* exception_wire,
    void** out) {
  ClearErr();
  CtxWrap* context_wrap = AsCtx(ctxw);
  if (context_wrap == nullptr || out == nullptr || exception_wire == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, context_wrap->iso)) {
    SetErr("invalid CreateMessage argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::Context> context = context_wrap->ctx->Get(iso);
    v8::Context::Scope context_scope(context);
    const v8::Message* message = v8__Exception__CreateMessage(
        iso, gov8_snap_impl::DerefLocal(
                 FromWire<v8::Value>(exception_wire)));
    if (message == nullptr) {
      SetErr("Exception::CreateMessage returned empty");
      return kErrException;
    }
    *out = const_cast<v8::Message*>(message);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in Exception::CreateMessage");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ec_exception_get_stack_trace(
    v8::Isolate* iso, void* ctxw, void* scope, void* exception_wire,
    void** out) {
  ClearErr();
  CtxWrap* context_wrap = AsCtx(ctxw);
  if (context_wrap == nullptr || out == nullptr || exception_wire == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, context_wrap->iso)) {
    SetErr("invalid GetStackTrace argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::Context> context = context_wrap->ctx->Get(iso);
    v8::Context::Scope context_scope(context);
    const v8::StackTrace* trace = v8__Exception__GetStackTrace(
        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(exception_wire)));
    *out = const_cast<v8::StackTrace*>(trace);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in Exception::GetStackTrace");
    return kErrCpp;
  }
}

// The older core ABI returned signed message locations in the status word,
// making the valid native-message sentinel -1 indistinguishable from an
// error. This output-parameter form preserves all signed observations.
__declspec(dllexport) int64_t gov8_ec_message_location(
    v8::Isolate* iso, void* scope, void* message_wire, int32_t which,
    int32_t* out) {
  ClearErr();
  if (message_wire == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      which < 0 || which > 3) {
    SetErr("invalid message location argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::Message> message =
        FromWire<v8::Message>(message_wire);
    switch (which) {
      case 0:
        *out = message->GetStartPosition();
        break;
      case 1:
        *out = message->GetEndPosition();
        break;
      case 2:
        *out = message->GetStartColumn();
        break;
      case 3:
        *out = message->GetEndColumn();
        break;
    }
    return kOk;
  } catch (...) {
    SetErr("C++ exception in message location");
    return kErrCpp;
  }
}

}  // extern "C"
