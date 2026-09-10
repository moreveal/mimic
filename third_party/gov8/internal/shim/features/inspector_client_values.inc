// Inspector value/default-context callbacks. V8 supplies an active HandleScope;
// the stack GoScope below is validation metadata only and opens no nested scope.

namespace gov8_inspector_client_values_impl {

v8_inspector::StringBuffer* ValueCallback(
    v8_inspector::V8InspectorClient* self, int32_t kind,
    v8::Context* context, v8::Value* value) {
  auto* client = reinterpret_cast<gov8_inspector_impl::ClientRaw*>(self);
  v8::Isolate* iso = v8::Isolate::TryGetCurrent();
  if (!client || !client->id || !iso || !context || !value) return nullptr;
  GoScope borrowed{};
  borrowed.magic = kScopeMagic;
  borrowed.iso = iso;
  return reinterpret_cast<v8_inspector::StringBuffer*>(
      gov8_icc_dispatch_client(
          client->id, kind, reinterpret_cast<uintptr_t>(context),
          reinterpret_cast<uintptr_t>(value),
          reinterpret_cast<uintptr_t>(&borrowed), 0, 0, 0, 0, 0, 0, 0, 0));
}

v8::Context* DefaultContext(v8_inspector::V8InspectorClient* self,
                            int32_t group) {
  auto* client = reinterpret_cast<gov8_inspector_impl::ClientRaw*>(self);
  v8::Isolate* iso = v8::Isolate::TryGetCurrent();
  if (!client || !client->id || !iso) return nullptr;
  void* context_raw = reinterpret_cast<void*>(gov8_icc_dispatch_client(
      client->id, 6, static_cast<uintptr_t>(group), 0, 0, 0, 0, 0, 0, 0, 0,
      0, 0));
  auto* context = AsCtx(context_raw);
  if (!context || context->iso != iso) return nullptr;
  auto local = context->ctx->Get(iso);
  return local.IsEmpty() ? nullptr : &gov8_snap_impl::DerefLocal(local);
}

}  // namespace gov8_inspector_client_values_impl

extern "C" {

v8_inspector::StringBuffer* gov8_icv_value_callback(
    v8_inspector::V8InspectorClient* self, int32_t kind,
    v8::Context* context, v8::Value* value) {
  return gov8_inspector_client_values_impl::ValueCallback(
      self, kind, context, value);
}

v8::Context* gov8_icv_default_context(
    v8_inspector::V8InspectorClient* self, int32_t group) {
  return gov8_inspector_client_values_impl::DefaultContext(self, group);
}

__declspec(dllexport) int64_t gov8_icv_context_matches(
    void* context_raw, void* context_wire) {
  ClearErr();
  auto* context = AsCtx(context_raw);
  v8::Isolate* iso = v8::Isolate::TryGetCurrent();
  if (!context || !iso || context->iso != iso || !context_wire) return kErrBadArg;
  auto expected = context->ctx->Get(iso);
  auto supplied = FromWire<v8::Context>(context_wire);
  return !expected.IsEmpty() && !supplied.IsEmpty() && expected == supplied ? 1 : 0;
}

}  // extern "C"
