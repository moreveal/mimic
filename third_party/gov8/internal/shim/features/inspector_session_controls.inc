// Inspector session controls layered on inspector_transport.inc. This feature
// uses rusty_v8's exported C bridge so it shares the exact pinned StringView
// and session ABI rather than reconstructing C++ ownership.

extern "C" {
bool v8_inspector__V8InspectorSession__canDispatchMethod(
    v8_inspector::StringView);
void v8_inspector__V8InspectorSession__releaseObjectGroup(
    v8_inspector::V8InspectorSession*, v8_inspector::StringView);
void v8_inspector__V8InspectorSession__schedulePauseOnNextStatement(
    v8_inspector::V8InspectorSession*, v8_inspector::StringView,
    v8_inspector::StringView);
void v8_inspector__V8InspectorSession__cancelPauseOnNextStatement(
    v8_inspector::V8InspectorSession*);
}

namespace gov8_inspector_session_controls_impl {
using GoClientDispatch = uintptr_t (*)(uint64_t, int32_t, int32_t);
GoClientDispatch go_client_dispatch = nullptr;

bool ValidView(int32_t is8, const void* data, size_t length) {
  return (is8 == 0 || is8 == 1) && (data != nullptr || length == 0);
}
}  // namespace gov8_inspector_session_controls_impl

extern "C" {
using namespace gov8_inspector_impl;
using namespace gov8_inspector_session_controls_impl;

int64_t gov8_isc_dispatch_client(uint64_t id, int32_t kind,
                                 int32_t context_group_id) {
  if (!id || !go_client_dispatch) return 1;
  return static_cast<int64_t>(go_client_dispatch(id, kind, context_group_id));
}

__declspec(dllexport) int64_t gov8_isc_set_client_dispatch(uintptr_t dispatch) {
  ClearErr();
  if (!dispatch) {
    SetErr("null Inspector client dispatcher");
    return kErrBadArg;
  }
  go_client_dispatch = reinterpret_cast<GoClientDispatch>(dispatch);
  return kOk;
}

__declspec(dllexport) int64_t gov8_isc_bind_client(void* inspector_raw,
                                                   uint64_t id) {
  ClearErr();
  auto* inspector = AsInspector(inspector_raw);
  if (!inspector || !inspector->client || !id || inspector->client->id) {
    SetErr("invalid Inspector client binding");
    return kErrBadArg;
  }
  inspector->client->id = id;
  return kOk;
}

__declspec(dllexport) int64_t gov8_isc_can_dispatch_method(
    int32_t is8, const void* data, size_t length, int32_t* out) {
  ClearErr();
  if (out) *out = 0;
  if (!out || !ValidView(is8, data, length)) {
    SetErr("invalid Inspector method view");
    return kErrBadArg;
  }
  try {
    *out = v8_inspector__V8InspectorSession__canDispatchMethod(
               View(is8, data, length))
               ? 1
               : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception checking Inspector method dispatch");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_isc_release_object_group(
    void* session_raw, int32_t is8, const void* data, size_t length) {
  ClearErr();
  auto* session = AsSession(session_raw);
  if (!session || !ValidView(is8, data, length)) {
    SetErr("invalid Inspector object group");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope enter(session->owner->iso);
    v8_inspector__V8InspectorSession__releaseObjectGroup(
        session->value, View(is8, data, length));
    return kOk;
  } catch (...) {
    SetErr("C++ exception releasing Inspector object group");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_isc_schedule_pause(
    void* session_raw, int32_t reason_is8, const void* reason,
    size_t reason_length, int32_t detail_is8, const void* detail,
    size_t detail_length) {
  ClearErr();
  auto* session = AsSession(session_raw);
  if (!session || !ValidView(reason_is8, reason, reason_length) ||
      !ValidView(detail_is8, detail, detail_length)) {
    SetErr("invalid Inspector scheduled-pause input");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope enter(session->owner->iso);
    v8_inspector__V8InspectorSession__schedulePauseOnNextStatement(
        session->value, View(reason_is8, reason, reason_length),
        View(detail_is8, detail, detail_length));
    return kOk;
  } catch (...) {
    SetErr("C++ exception scheduling Inspector pause");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_isc_cancel_pause(void* session_raw) {
  ClearErr();
  auto* session = AsSession(session_raw);
  if (!session) {
    SetErr("invalid Inspector session");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope enter(session->owner->iso);
    v8_inspector__V8InspectorSession__cancelPauseOnNextStatement(
        session->value);
    return kOk;
  } catch (...) {
    SetErr("C++ exception cancelling Inspector pause");
    return kErrCpp;
  }
}
}  // extern "C"
