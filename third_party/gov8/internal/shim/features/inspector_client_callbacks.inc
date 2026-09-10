// Optional V8InspectorClient callback capabilities.

extern "C" {
v8_inspector::StringBuffer* v8_inspector__StringBuffer__create(
    v8_inspector::StringView);
}

namespace gov8_inspector_client_callbacks_impl {
using GoDispatch = uintptr_t (*)(uint64_t, int32_t, uintptr_t, uintptr_t,
                                 uintptr_t, uintptr_t, uintptr_t, uintptr_t,
                                 uintptr_t, uintptr_t, uintptr_t, uintptr_t,
                                 uintptr_t);
GoDispatch go_dispatch = nullptr;
}

extern "C" {
using namespace gov8_inspector_client_callbacks_impl;

uintptr_t gov8_icc_dispatch_client(uint64_t id, int32_t kind, uintptr_t a0,
                                   uintptr_t a1, uintptr_t a2, uintptr_t a3,
                                   uintptr_t a4, uintptr_t a5, uintptr_t a6,
                                   uintptr_t a7, uintptr_t a8, uintptr_t a9,
                                   uintptr_t a10) {
  if (!id || !gov8_inspector_client_callbacks_impl::go_dispatch) return 0;
  return gov8_inspector_client_callbacks_impl::go_dispatch(
      id, kind, a0, a1, a2, a3, a4, a5, a6, a7, a8, a9, a10);
}

__declspec(dllexport) int64_t gov8_icc_set_dispatch(uintptr_t dispatch) {
  ClearErr();
  if (!dispatch) return kErrBadArg;
  gov8_inspector_client_callbacks_impl::go_dispatch =
      reinterpret_cast<gov8_inspector_client_callbacks_impl::GoDispatch>(dispatch);
  return kOk;
}

__declspec(dllexport) int64_t gov8_icc_string_buffer_create(
    int32_t is8, const void* data, size_t length, void** out) {
  ClearErr();
  if (out) *out = nullptr;
  if (!out || (is8 != 0 && is8 != 1) || (!data && length)) return kErrBadArg;
  try {
    *out = v8_inspector__StringBuffer__create(
        gov8_inspector_impl::View(is8, data, length));
    return *out ? kOk : kErrGeneric;
  } catch (...) {
    SetErr("C++ exception creating Inspector StringBuffer");
    return kErrCpp;
  }
}
}
