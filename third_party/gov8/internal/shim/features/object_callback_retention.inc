// Object callback-retention and generic Template::Set(Data) residuals for
// pinned rusty_v8 152.2.0. This feature is included after
// templates_callbacks.inc and object_ops.inc and reuses their callback
// trampolines, host contexts, receiver checks, and Maybe<bool> helpers.

#include <cstdint>

extern "C" {

__declspec(dllexport) int64_t gov8_ocr_object_set_accessor_configuration(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    void* host_ctx, int32_t with_setter, int32_t attr, int32_t* ok) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  GoCallbackContext* cbctx = static_cast<GoCallbackContext*>(host_ctx);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      cbctx == nullptr || !OwnedBy(iso, w->iso) || obj == nullptr ||
      key == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    if (!gov8_oo_impl::IsReceiverWire(obj)) {
      SetErr("receiver is not an object");
      return kErrBadArg;
    }
    v8::Local<v8::Value> key_value = FromWire<v8::Value>(key);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(key_value))) {
      SetErr("key is not a Name (string or symbol)");
      return kErrBadArg;
    }
    void* dispatch_data = HostContextExternalWire(iso, cbctx);
    if (dispatch_data == nullptr) {
      return kErrNoMemory;
    }
    v8::Local<v8::Context> context = w->ctx->Get(iso);
    v8::Context::Scope context_scope(context);
    FallbackTryCatch fallback;
    (void)fallback.Init(iso);
    return gov8_rv_impl::MaybeBoolStatus(
        v8__Object__SetAccessor(
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
            gov8_snap_impl::DerefLocal(context),
            gov8_snap_impl::DerefLocal(key_value.As<v8::Name>()),
            HostAccessorGetTrampoline,
            with_setter != 0 ? HostAccessorSetTrampoline : nullptr,
            *FromWire<v8::Value>(dispatch_data),
            static_cast<v8::PropertyAttribute>(attr)),
        ok);
  } GOV8_RV_END_CATCH("ocr_object_set_accessor_configuration")
}

__declspec(dllexport) int64_t gov8_ocr_object_set_lazy_data_property(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    void* host_ctx, int32_t attr, int32_t getter_side_effect,
    int32_t setter_side_effect, int32_t* ok) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  GoCallbackContext* cbctx = static_cast<GoCallbackContext*>(host_ctx);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      cbctx == nullptr || !OwnedBy(iso, w->iso) || obj == nullptr ||
      key == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    if (!gov8_oo_impl::IsReceiverWire(obj)) {
      SetErr("receiver is not an object");
      return kErrBadArg;
    }
    v8::Local<v8::Value> key_value = FromWire<v8::Value>(key);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(key_value))) {
      SetErr("key is not a Name (string or symbol)");
      return kErrBadArg;
    }
    void* dispatch_data = HostContextExternalWire(iso, cbctx);
    if (dispatch_data == nullptr) {
      return kErrNoMemory;
    }
    v8::Local<v8::Context> context = w->ctx->Get(iso);
    v8::Context::Scope context_scope(context);
    FallbackTryCatch fallback;
    (void)fallback.Init(iso);
    return gov8_rv_impl::MaybeBoolStatus(
        v8__Object__SetLazyDataProperty(
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
            gov8_snap_impl::DerefLocal(context),
            gov8_snap_impl::DerefLocal(key_value.As<v8::Name>()),
            HostAccessorGetTrampoline, *FromWire<v8::Value>(dispatch_data),
            static_cast<v8::PropertyAttribute>(attr),
            static_cast<v8::SideEffectType>(getter_side_effect),
            static_cast<v8::SideEffectType>(setter_side_effect)),
        ok);
  } GOV8_RV_END_CATCH("ocr_object_set_lazy_data_property")
}

// Direct-status variant used by the Go hot path. Keeping the Maybe<bool>
// result in native storage avoids passing a movable Go stack pointer across
// the DLL call while preserving the existing negative shim statuses.
__declspec(dllexport) int64_t gov8_ocr_object_set_lazy_data_property_direct(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    void* host_ctx, int32_t attr, int32_t getter_side_effect,
    int32_t setter_side_effect) {
  int32_t ok = 0;
  int64_t status = gov8_ocr_object_set_lazy_data_property(
      iso, ctxw, scope, obj, key, host_ctx, attr, getter_side_effect,
      setter_side_effect, &ok);
  if (status < 0) return status;
  return ok == 1 ? 1 : 0;
}

__declspec(dllexport) int64_t gov8_ocr_template_set_data_with_attr(
    v8::Isolate* iso, void* scope, void* template_wire,
    int32_t template_kind, void* key_wire, void* data_wire, int32_t attr) {
  ClearErr();
  if (!ScopeIs(iso, scope) || template_wire == nullptr ||
      key_wire == nullptr || data_wire == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Data> template_data = FromWire<v8::Data>(template_wire);
    const bool valid_template =
        template_kind == 0
            ? v8__Data__IsObjectTemplate(
                  gov8_snap_impl::DerefLocal(template_data))
            : template_kind == 1 &&
                  v8__Data__IsFunctionTemplate(
                      gov8_snap_impl::DerefLocal(template_data));
    if (!valid_template) {
      SetErr("template handle has the wrong type");
      return kErrBadArg;
    }
    v8::Local<v8::Data> key_data = FromWire<v8::Data>(key_wire);
    if (!v8__Data__IsName(gov8_snap_impl::DerefLocal(key_data))) {
      SetErr("key is not a Name (string or symbol)");
      return kErrBadArg;
    }
    v8::Local<v8::Data> data = FromWire<v8::Data>(data_wire);
    const bool safe_data =
        v8__Data__IsFunctionTemplate(gov8_snap_impl::DerefLocal(data)) ||
        v8__Data__IsObjectTemplate(gov8_snap_impl::DerefLocal(data)) ||
        (data->IsValue() && data.As<v8::Value>()->IsPrimitive());
    if (!safe_data) {
      SetErr("template data must be primitive or a Function/ObjectTemplate");
      return kErrBadArg;
    }
    FromWire<v8::Template>(template_wire)
        ->Set(FromWire<v8::Name>(key_wire), data,
              static_cast<v8::PropertyAttribute>(attr));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in ocr_template_set_data_with_attr");
    return kErrCpp;
  }
}

}  // extern "C"
