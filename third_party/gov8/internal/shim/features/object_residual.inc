// Residual non-callback Object APIs for rusty_v8 152.2.0.
// Included after runtime_values.inc and object_ops.inc; it reuses their
// context/scope validation, wire conversion, receiver guard, and status helpers.

extern "C" {
const v8::Object* v8__Object__New__with_prototype_and_properties(
    v8::Isolate* isolate, const v8::Value& prototype_or_null,
    const v8::Name* const names[], const v8::Value* const values[],
    size_t length);
const v8::Array* v8__Object__GetOwnPropertyNames(
    const v8::Object* self, const v8::Context* context,
    v8::PropertyFilter property_filter, v8::KeyConversionMode key_conversion);
const v8::Array* v8__Object__PreviewEntries(const v8::Object& self,
                                            bool* is_key_value);
bool v8__Object__IsApiWrapper(const v8::Object& self);
}

extern "C" {

__declspec(dllexport) int64_t
gov8_or_object_new_with_prototype_and_properties(
    v8::Isolate* iso, void* ctxw, void* scope, void* prototype,
    void** names, void** values, size_t length, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || prototype == nullptr || names == nullptr ||
      values == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  for (size_t index = 0; index < length; ++index) {
    if (names[index] == nullptr || values[index] == nullptr) {
      SetErr("null property wire");
      return kErrBadArg;
    }
    v8::Local<v8::Value> name = FromWire<v8::Value>(names[index]);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(name))) {
      SetErr("property name is not a Name");
      return kErrBadArg;
    }
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Object* object =
        v8__Object__New__with_prototype_and_properties(
            iso, gov8_snap_impl::DerefLocal(FromWire<v8::Value>(prototype)),
            reinterpret_cast<const v8::Name* const*>(names),
            reinterpret_cast<const v8::Value* const*>(values), length);
    return gov8_rv_impl::MaybeLocalStatus(object, out);
  } GOV8_RV_END_CATCH("or_object_new_with_prototype_and_properties")
}

__declspec(dllexport) int64_t gov8_or_object_get_own_property_names(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj,
    int32_t property_filter, int32_t key_conversion, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || obj == nullptr || out == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso) || property_filter < 0 ||
      property_filter > 31 || key_conversion < 0 || key_conversion > 2) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    GOV8_OO_REQUIRE_RECEIVER(obj)
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Array* names = v8__Object__GetOwnPropertyNames(
        static_cast<const v8::Object*>(obj),
        *reinterpret_cast<const v8::Context* const*>(&ctx),
        static_cast<v8::PropertyFilter>(property_filter),
        static_cast<v8::KeyConversionMode>(key_conversion));
    return gov8_rv_impl::MaybeLocalStatus(names, out);
  } GOV8_RV_END_CATCH("or_object_get_own_property_names")
}

__declspec(dllexport) int64_t gov8_or_object_preview_entries(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void** out,
    int32_t* is_key_value) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (iso == nullptr || w == nullptr || obj == nullptr || out == nullptr ||
      is_key_value == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    GOV8_OO_REQUIRE_RECEIVER(obj)
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    bool key_value = false;
    const v8::Array* entries = v8__Object__PreviewEntries(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)), &key_value);
    *is_key_value = key_value ? 1 : 0;
    *out = const_cast<void*>(static_cast<const void*>(entries));
    return kOk;
  } GOV8_RV_END_CATCH("or_object_preview_entries")
}

__declspec(dllexport) int64_t gov8_or_object_is_api_wrapper(
    v8::Isolate* iso, void* obj) {
  ClearErr();
  if (iso == nullptr || obj == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    GOV8_OO_REQUIRE_RECEIVER(obj)
    v8::Isolate::Scope iso_scope(iso);
    return v8__Object__IsApiWrapper(
               gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)))
               ? 1
               : 0;
  } GOV8_RV_END_CATCH("or_object_is_api_wrapper")
}

}  // extern "C"
