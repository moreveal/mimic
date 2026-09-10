// Minimal Inspector transport through rusty_v8's pinned C bridge. RawChannel
// and RawV8InspectorClient are one-vtable-word bases; ids stay adjacent and no
// Go pointer crosses the ABI.
#include "v8-inspector.h"
#include <memory>

extern "C" {
void v8_inspector__V8Inspector__Channel__BASE__CONSTRUCT(void*);
void v8_inspector__V8InspectorClient__BASE__CONSTRUCT(void*);
v8_inspector::V8Inspector* v8_inspector__V8Inspector__create(
    v8::Isolate*, v8_inspector::V8InspectorClient*);
void v8_inspector__V8Inspector__DELETE(v8_inspector::V8Inspector*);
v8_inspector::V8InspectorSession* v8_inspector__V8Inspector__connect(
    v8_inspector::V8Inspector*, int, v8_inspector::V8Inspector::Channel*,
    v8_inspector::StringView, v8_inspector::V8Inspector::ClientTrustLevel);
void v8_inspector__V8Inspector__contextCreated(
    v8_inspector::V8Inspector*, const v8::Context*, int,
    v8_inspector::StringView, v8_inspector::StringView);
void v8_inspector__V8Inspector__contextDestroyed(
    v8_inspector::V8Inspector*, const v8::Context*);
void v8_inspector__V8InspectorSession__DELETE(
    v8_inspector::V8InspectorSession*);
void v8_inspector__V8InspectorSession__dispatchProtocolMessage(
    v8_inspector::V8InspectorSession*, v8_inspector::StringView);
void v8_inspector__StringBuffer__DELETE(v8_inspector::StringBuffer*);
// Implemented by inspector_session_controls.inc, which is included after this
// transport feature. Zero is success; non-zero is unrecoverable host misuse.
int64_t gov8_isc_dispatch_client(uint64_t id, int32_t kind,
                                 int32_t context_group_id);
uintptr_t gov8_icc_dispatch_client(uint64_t id, int32_t kind, uintptr_t a0,
                                   uintptr_t a1, uintptr_t a2, uintptr_t a3,
                                   uintptr_t a4, uintptr_t a5, uintptr_t a6,
                                   uintptr_t a7, uintptr_t a8, uintptr_t a9,
                                   uintptr_t a10);
v8_inspector::StringBuffer* gov8_icv_value_callback(
    v8_inspector::V8InspectorClient*, int32_t, v8::Context*, v8::Value*);
v8::Context* gov8_icv_default_context(v8_inspector::V8InspectorClient*,
                                      int32_t);
}

namespace gov8_inspector_impl {
constexpr uint64_t kInspectorMagic = 0x674f5638494e5350ULL;
constexpr uint64_t kSessionMagic = 0x674f5638494e5353ULL;
using GoDispatch = void (*)(uint64_t, int32_t, int32_t, int32_t, const void*,
                            size_t);
GoDispatch go_dispatch = nullptr;

v8_inspector::StringView View(int32_t is8, const void* data, size_t len) {
  return is8 ? v8_inspector::StringView(static_cast<const uint8_t*>(data), len)
             : v8_inspector::StringView(static_cast<const uint16_t*>(data),
                                        len);
}
struct ChannelRaw { void* vtable = nullptr; uint64_t id = 0; };
struct ClientRaw { void* vtable = nullptr; uint64_t id = 0; };
struct InspectorWrap {
  uint64_t magic = kInspectorMagic;
  v8::Isolate* iso = nullptr;
  std::unique_ptr<ClientRaw> client;
  v8_inspector::V8Inspector* value = nullptr;
  int sessions = 0;
};
struct SessionWrap {
  uint64_t magic = kSessionMagic;
  InspectorWrap* owner = nullptr;
  std::unique_ptr<ChannelRaw> channel;
  v8_inspector::V8InspectorSession* value = nullptr;
};
InspectorWrap* AsInspector(void* p) {
  auto* v = static_cast<InspectorWrap*>(p);
  return v && v->magic == kInspectorMagic ? v : nullptr;
}
SessionWrap* AsSession(void* p) {
  auto* v = static_cast<SessionWrap*>(p);
  return v && v->magic == kSessionMagic ? v : nullptr;
}
void Dispatch(ChannelRaw* channel, int32_t kind, int32_t call_id,
              v8_inspector::StringBuffer* message) {
  if (!channel || !go_dispatch) {
    if (message) v8_inspector__StringBuffer__DELETE(message);
    return;
  }
  if (!message) {
    go_dispatch(channel->id, kind, call_id, 1, nullptr, 0);
    return;
  }
  auto view = message->string();
  go_dispatch(channel->id, kind, call_id, view.is8Bit() ? 1 : 0,
              view.is8Bit() ? static_cast<const void*>(view.characters8())
                            : static_cast<const void*>(view.characters16()),
              view.length());
  v8_inspector__StringBuffer__DELETE(message);
}
}  // namespace gov8_inspector_impl

extern "C" {
using namespace gov8_inspector_impl;
__declspec(dllexport) void
v8_inspector__V8Inspector__Channel__BASE__sendResponse(
    v8_inspector::V8Inspector::Channel* self, int id,
    v8_inspector::StringBuffer* message) {
  Dispatch(reinterpret_cast<ChannelRaw*>(self), 0, id, message);
}
__declspec(dllexport) void
v8_inspector__V8Inspector__Channel__BASE__sendNotification(
    v8_inspector::V8Inspector::Channel* self,
    v8_inspector::StringBuffer* message) {
  Dispatch(reinterpret_cast<ChannelRaw*>(self), 1, 0, message);
}
__declspec(dllexport) void
v8_inspector__V8Inspector__Channel__BASE__flushProtocolNotifications(
    v8_inspector::V8Inspector::Channel* self) {
  auto* c = reinterpret_cast<ChannelRaw*>(self);
  if (c && go_dispatch) go_dispatch(c->id, 2, 0, 1, nullptr, 0);
}
__declspec(dllexport) int64_t
v8_inspector__V8InspectorClient__BASE__generateUniqueId(
    v8_inspector::V8InspectorClient* self) {
  auto* client = reinterpret_cast<ClientRaw*>(self);
  return client && client->id
             ? static_cast<int64_t>(gov8_icc_dispatch_client(
                   client->id, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0))
             : 0;
}
__declspec(dllexport) void
v8_inspector__V8InspectorClient__BASE__runMessageLoopOnPause(
    v8_inspector::V8InspectorClient* self, int context_group_id) {
  auto* client = reinterpret_cast<ClientRaw*>(self);
  if (client && client->id &&
      gov8_isc_dispatch_client(client->id, 0, context_group_id) != 0) {
    std::abort();
  }
}
__declspec(dllexport) void
v8_inspector__V8InspectorClient__BASE__quitMessageLoopOnPause(
    v8_inspector::V8InspectorClient* self) {
  auto* client = reinterpret_cast<ClientRaw*>(self);
  if (client && client->id &&
      gov8_isc_dispatch_client(client->id, 1, 0) != 0) {
    std::abort();
  }
}
__declspec(dllexport) void
v8_inspector__V8InspectorClient__BASE__runIfWaitingForDebugger(
    v8_inspector::V8InspectorClient* self, int group) {
  auto* client = reinterpret_cast<ClientRaw*>(self);
  if (client && client->id &&
      gov8_icc_dispatch_client(client->id, 1,
                               static_cast<uintptr_t>(group), 0, 0, 0, 0, 0,
                               0, 0, 0, 0, 0) != 0) std::abort();
}
__declspec(dllexport) void
v8_inspector__V8InspectorClient__BASE__consoleAPIMessage(
    v8_inspector::V8InspectorClient* self, int group,
    v8::Isolate::MessageErrorLevel level,
    const v8_inspector::StringView& message,
    const v8_inspector::StringView& url, unsigned line, unsigned column,
    v8_inspector::V8StackTrace* stack) {
  auto* client = reinterpret_cast<ClientRaw*>(self);
  if (client && client->id &&
      gov8_icc_dispatch_client(
          client->id, 3, static_cast<uintptr_t>(group),
          static_cast<uintptr_t>(level), message.is8Bit() ? 1 : 0,
          reinterpret_cast<uintptr_t>(message.is8Bit()
                                          ? static_cast<const void*>(message.characters8())
                                          : static_cast<const void*>(message.characters16())),
          message.length(), url.is8Bit() ? 1 : 0,
          reinterpret_cast<uintptr_t>(url.is8Bit()
                                          ? static_cast<const void*>(url.characters8())
                                          : static_cast<const void*>(url.characters16())),
          url.length(), line, column, reinterpret_cast<uintptr_t>(stack)) != 0)
    std::abort();
}
__declspec(dllexport) v8_inspector::StringBuffer*
v8_inspector__V8InspectorClient__BASE__valueSubtype(
    v8_inspector::V8InspectorClient* self, v8::Context* context,
    v8::Value* value) {
  return gov8_icv_value_callback(self, 4, context, value);
}
__declspec(dllexport) v8_inspector::StringBuffer*
v8_inspector__V8InspectorClient__BASE__descriptionForValueSubtype(
    v8_inspector::V8InspectorClient* self, v8::Context* context,
    v8::Value* value) {
  return gov8_icv_value_callback(self, 5, context, value);
}
__declspec(dllexport) v8::Context*
v8_inspector__V8InspectorClient__BASE__ensureDefaultContextInGroup(
    v8_inspector::V8InspectorClient* self, int group) {
  return gov8_icv_default_context(self, group);
}
__declspec(dllexport) v8_inspector::StringBuffer*
v8_inspector__V8InspectorClient__BASE__resourceNameToUrl(
    v8_inspector::V8InspectorClient* self,
    const v8_inspector::StringView& resource) {
  auto* client = reinterpret_cast<ClientRaw*>(self);
  if (!client || !client->id) return nullptr;
  return reinterpret_cast<v8_inspector::StringBuffer*>(
      gov8_icc_dispatch_client(
          client->id, 2, resource.is8Bit() ? 1 : 0,
          reinterpret_cast<uintptr_t>(
              resource.is8Bit()
                  ? static_cast<const void*>(resource.characters8())
                  : static_cast<const void*>(resource.characters16())),
          resource.length(), 0, 0, 0, 0, 0, 0, 0, 0));
}

__declspec(dllexport) int64_t gov8_inspector_set_channel_dispatch(void* p) {
  ClearErr();
  if (!p) return kErrBadArg;
  go_dispatch = reinterpret_cast<GoDispatch>(p);
  return kOk;
}
__declspec(dllexport) int64_t gov8_inspector_create(v8::Isolate* iso,
                                                    uint64_t client_id,
                                                    void** out) {
  ClearErr();
  if (out) *out = nullptr;
  if (!iso || !out) return kErrBadArg;
  try {
    Gov8IsolateScope enter(iso);
    auto w = std::make_unique<InspectorWrap>();
    w->iso = iso;
    w->client = std::make_unique<ClientRaw>();
    v8_inspector__V8InspectorClient__BASE__CONSTRUCT(w->client.get());
    w->client->id = client_id;
    w->value = v8_inspector__V8Inspector__create(
        iso, reinterpret_cast<v8_inspector::V8InspectorClient*>(w->client.get()));
    if (!w->value) return kErrGeneric;
    *out = w.release();
    return kOk;
  } catch (...) { SetErr("C++ exception creating inspector"); return kErrCpp; }
}
__declspec(dllexport) int64_t gov8_inspector_dispose(void* raw) {
  ClearErr();
  auto* w = AsInspector(raw);
  if (!w) return kErrMagic;
  if (w->sessions) {
    SetErr("inspector has live sessions");
    return kErrState;
  }
  try {
    Gov8IsolateScope enter(w->iso);
    v8_inspector__V8Inspector__DELETE(w->value);
    w->client->id = 0;
    w->magic = 0;
    delete w;
    return kOk;
  } catch (...) { SetErr("C++ exception disposing inspector"); return kErrCpp; }
}
__declspec(dllexport) int64_t gov8_inspector_context_created(
    void* raw, void* context_raw, int32_t group, int32_t name_is8,
    const void* name, size_t name_len, int32_t aux_is8, const void* aux,
    size_t aux_len) {
  ClearErr();
  auto* w = AsInspector(raw); auto* c = AsCtx(context_raw);
  if (!w || !c || c->iso != w->iso || group <= 0 ||
      (name_is8 != 0 && name_is8 != 1) ||
      (aux_is8 != 0 && aux_is8 != 1) || (!name && name_len) ||
      (!aux && aux_len)) return kErrBadArg;
  try {
    Gov8IsolateScope enter(w->iso); v8::HandleScope hs(w->iso);
    auto local = c->ctx->Get(w->iso);
    v8_inspector__V8Inspector__contextCreated(
        w->value, &gov8_snap_impl::DerefLocal(local), group,
        View(name_is8, name, name_len), View(aux_is8, aux, aux_len));
    return kOk;
  } catch (...) { SetErr("C++ exception registering inspector context"); return kErrCpp; }
}
__declspec(dllexport) int64_t gov8_inspector_context_destroyed(
    void* raw, void* context_raw) {
  ClearErr(); auto* w = AsInspector(raw); auto* c = AsCtx(context_raw);
  if (!w || !c || c->iso != w->iso) return kErrBadArg;
  try {
    Gov8IsolateScope enter(w->iso); v8::HandleScope hs(w->iso);
    auto local = c->ctx->Get(w->iso);
    v8_inspector__V8Inspector__contextDestroyed(
        w->value, &gov8_snap_impl::DerefLocal(local));
    return kOk;
  } catch (...) { SetErr("C++ exception destroying inspector context"); return kErrCpp; }
}
__declspec(dllexport) int64_t gov8_inspector_connect(
    void* raw, int32_t group, uint64_t id, int32_t state_is8,
    const void* state, size_t state_len, int32_t trust, void** out) {
  ClearErr(); auto* owner = AsInspector(raw);
  if (out) *out = nullptr;
  if (!owner || !out || group <= 0 || !id ||
      (state_is8 != 0 && state_is8 != 1) || (!state && state_len) ||
      trust < 0 || trust > 1) return kErrBadArg;
  try {
    Gov8IsolateScope enter(owner->iso);
    auto w = std::make_unique<SessionWrap>(); w->owner = owner;
    w->channel = std::make_unique<ChannelRaw>();
    v8_inspector__V8Inspector__Channel__BASE__CONSTRUCT(w->channel.get());
    w->channel->id = id;
    w->value = v8_inspector__V8Inspector__connect(
        owner->value, group,
        reinterpret_cast<v8_inspector::V8Inspector::Channel*>(w->channel.get()),
        View(state_is8, state, state_len),
        static_cast<v8_inspector::V8Inspector::ClientTrustLevel>(trust));
    if (!w->value) return kErrGeneric;
    owner->sessions++; *out = w.release(); return kOk;
  } catch (...) { SetErr("C++ exception connecting inspector session"); return kErrCpp; }
}
__declspec(dllexport) int64_t gov8_inspector_session_dispatch(
    void* raw, int32_t is8, const void* data, size_t len) {
  ClearErr(); auto* w = AsSession(raw);
  if (!w || (is8 != 0 && is8 != 1) || (!data && len)) return kErrBadArg;
  try {
    Gov8IsolateScope enter(w->owner->iso);
    v8_inspector__V8InspectorSession__dispatchProtocolMessage(
        w->value, View(is8, data, len));
    return kOk;
  } catch (...) { SetErr("C++ exception dispatching inspector message"); return kErrCpp; }
}
__declspec(dllexport) int64_t gov8_inspector_session_dispose(void* raw) {
  ClearErr(); auto* w = AsSession(raw); if (!w) return kErrMagic;
  try {
    Gov8IsolateScope enter(w->owner->iso);
    v8_inspector__V8InspectorSession__DELETE(w->value);
    w->owner->sessions--; w->magic = 0; delete w; return kOk;
  } catch (...) { SetErr("C++ exception disposing inspector session"); return kErrCpp; }
}
}  // extern "C"
