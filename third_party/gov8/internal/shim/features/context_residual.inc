// ===========================================================================
// Residual Context snapshot-option feature slice.
//
// Context::FromSnapshot's option path is kept here because the base snapshot
// slice predates global-proxy reuse and explicit microtask queues. The flat
// rusty_v8 binding is declared by snapshots_handles.inc earlier in this TU.
// ===========================================================================

extern "C" {

__declspec(dllexport) int64_t gov8_cr_context_from_snapshot_options(
    v8::Isolate* iso, void* scope, uint64_t index, void* global_wire,
    void* mqw, void** out, int32_t* found) {
  ClearErr();
  MqWrap* mq = mqw == nullptr ? nullptr : AsMq(mqw);
  if (iso == nullptr || out == nullptr || found == nullptr ||
      !ScopeIs(iso, scope) || (mqw != nullptr && mq == nullptr) ||
      (mq != nullptr && !OwnedBy(iso, mq->iso))) {
    SetErr("invalid context snapshot options");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    const v8::Context* context = v8__Context__FromSnapshot(
        iso, static_cast<size_t>(index),
        static_cast<v8::Value*>(global_wire),
        mq == nullptr ? nullptr : mq->queue);
    if (context == nullptr) {
      *out = nullptr;
      *found = 0;
      return kOk;
    }
    v8::Local<v8::Context> local =
        FromWire<v8::Context>(const_cast<v8::Context*>(context));
    auto* wrap = new CtxWrap{};
    wrap->magic = kCtxMagic;
    wrap->iso = iso;
    wrap->ctx = new v8::Global<v8::Context>(iso, local);
    *out = wrap;
    *found = 1;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_from_snapshot_options");
    return kErrCpp;
  }
}

}  // extern "C"
