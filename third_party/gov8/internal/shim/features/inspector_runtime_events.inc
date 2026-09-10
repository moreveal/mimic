// Inspector runtime-event forwarding through rusty_v8's pinned C bridge.
// Async task identities are opaque integer words reinterpreted as pointers;
// V8 compares these values but never dereferences them.

#include <cstdint>

extern "C" {
void v8_inspector__V8Inspector__idleStarted(v8_inspector::V8Inspector*);
void v8_inspector__V8Inspector__idleFinished(v8_inspector::V8Inspector*);
void v8_inspector__V8Inspector__asyncTaskScheduled(
    v8_inspector::V8Inspector*, v8_inspector::StringView, void*, bool);
void v8_inspector__V8Inspector__asyncTaskCanceled(v8_inspector::V8Inspector*,
                                                  void*);
void v8_inspector__V8Inspector__asyncTaskStarted(v8_inspector::V8Inspector*,
                                                 void*);
void v8_inspector__V8Inspector__asyncTaskFinished(v8_inspector::V8Inspector*,
                                                  void*);
void v8_inspector__V8Inspector__allAsyncTasksCanceled(
    v8_inspector::V8Inspector*);
unsigned v8_inspector__V8Inspector__exceptionThrown(
    v8_inspector::V8Inspector*, const v8::Context*,
    v8_inspector::StringView, const v8::Value*, v8_inspector::StringView,
    v8_inspector::StringView, unsigned, unsigned,
    v8_inspector::V8StackTrace*, int);
v8_inspector::V8StackTrace* v8_inspector__V8Inspector__createStackTrace(
    v8_inspector::V8Inspector*, const v8::StackTrace*);
void v8_inspector__V8StackTrace__DELETE(v8_inspector::V8StackTrace*);
}

namespace gov8_inspector_runtime_events_impl {

void* TaskIdentity(uintptr_t task) {
  return reinterpret_cast<void*>(task);
}

}  // namespace gov8_inspector_runtime_events_impl

extern "C" {

using namespace gov8_inspector_impl;
using namespace gov8_inspector_runtime_events_impl;

__declspec(dllexport) int64_t gov8_ire_idle_started(void* raw) {
  ClearErr();
  auto* inspector = AsInspector(raw);
  if (!inspector) return kErrMagic;
  try {
    Gov8IsolateScope enter(inspector->iso);
    v8_inspector__V8Inspector__idleStarted(inspector->value);
    return kOk;
  } catch (...) {
    SetErr("C++ exception starting Inspector idle period");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ire_idle_finished(void* raw) {
  ClearErr();
  auto* inspector = AsInspector(raw);
  if (!inspector) return kErrMagic;
  try {
    Gov8IsolateScope enter(inspector->iso);
    v8_inspector__V8Inspector__idleFinished(inspector->value);
    return kOk;
  } catch (...) {
    SetErr("C++ exception finishing Inspector idle period");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ire_async_task_scheduled(
    void* raw, int32_t is8, const void* name, size_t name_len,
    uintptr_t task, int32_t recurring) {
  ClearErr();
  auto* inspector = AsInspector(raw);
  if (!inspector || (is8 != 0 && is8 != 1) || (!name && name_len) ||
      (recurring != 0 && recurring != 1)) return kErrBadArg;
  try {
    Gov8IsolateScope enter(inspector->iso);
    v8_inspector__V8Inspector__asyncTaskScheduled(
        inspector->value, View(is8, name, name_len), TaskIdentity(task),
        recurring != 0);
    return kOk;
  } catch (...) {
    SetErr("C++ exception scheduling Inspector async task");
    return kErrCpp;
  }
}

#define GOV8_INSPECTOR_ASYNC_EVENT(export_name, bridge_name, error_text)       \
  __declspec(dllexport) int64_t export_name(void* raw, uintptr_t task) {       \
    ClearErr();                                                                \
    auto* inspector = AsInspector(raw);                                        \
    if (!inspector) return kErrMagic;                                           \
    try {                                                                      \
      Gov8IsolateScope enter(inspector->iso);                                  \
      bridge_name(inspector->value, TaskIdentity(task));                        \
      return kOk;                                                              \
    } catch (...) {                                                            \
      SetErr(error_text);                                                      \
      return kErrCpp;                                                          \
    }                                                                          \
  }

GOV8_INSPECTOR_ASYNC_EVENT(gov8_ire_async_task_canceled,
                           v8_inspector__V8Inspector__asyncTaskCanceled,
                           "C++ exception canceling Inspector async task")
GOV8_INSPECTOR_ASYNC_EVENT(gov8_ire_async_task_started,
                           v8_inspector__V8Inspector__asyncTaskStarted,
                           "C++ exception starting Inspector async task")
GOV8_INSPECTOR_ASYNC_EVENT(gov8_ire_async_task_finished,
                           v8_inspector__V8Inspector__asyncTaskFinished,
                           "C++ exception finishing Inspector async task")

#undef GOV8_INSPECTOR_ASYNC_EVENT

__declspec(dllexport) int64_t gov8_ire_all_async_tasks_canceled(void* raw) {
  ClearErr();
  auto* inspector = AsInspector(raw);
  if (!inspector) return kErrMagic;
  try {
    Gov8IsolateScope enter(inspector->iso);
    v8_inspector__V8Inspector__allAsyncTasksCanceled(inspector->value);
    return kOk;
  } catch (...) {
    SetErr("C++ exception canceling all Inspector async tasks");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ire_create_stack_trace(
    void* raw, void* stack_trace_wire, void** out) {
  ClearErr();
  if (out) *out = nullptr;
  auto* inspector = AsInspector(raw);
  if (!inspector || !out) return kErrBadArg;
  try {
    Gov8IsolateScope enter(inspector->iso);
    const v8::StackTrace* stack_trace = nullptr;
    if (stack_trace_wire) {
      auto local = FromWire<v8::StackTrace>(stack_trace_wire);
      if (local.IsEmpty()) return kErrBadArg;
      stack_trace = &gov8_snap_impl::DerefLocal(local);
    }
    *out = v8_inspector__V8Inspector__createStackTrace(
        inspector->value, stack_trace);
    return kOk;
  } catch (...) {
    SetErr("C++ exception creating Inspector stack trace");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ire_stack_trace_delete(void* raw) {
  ClearErr();
  if (!raw) return kErrBadArg;
  try {
    v8_inspector__V8StackTrace__DELETE(
        static_cast<v8_inspector::V8StackTrace*>(raw));
    return kOk;
  } catch (...) {
    SetErr("C++ exception deleting Inspector stack trace");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ire_exception_thrown(
    void* raw, void* context_raw, void* scope_raw,
    int32_t message_is8, const void* message, size_t message_len,
    void* exception_wire,
    int32_t detailed_is8, const void* detailed, size_t detailed_len,
    int32_t url_is8, const void* url, size_t url_len,
    uint32_t line_number, uint32_t column_number,
    void** stack_trace_io, int32_t script_id, uint32_t* out) {
  ClearErr();
  if (out) *out = 0;
  auto* inspector = AsInspector(raw);
  auto* context = AsCtx(context_raw);
  if (!inspector || !context || context->iso != inspector->iso ||
      !ScopeIs(inspector->iso, scope_raw) || !exception_wire ||
      !stack_trace_io || !out ||
      (message_is8 != 0 && message_is8 != 1) ||
      (detailed_is8 != 0 && detailed_is8 != 1) ||
      (url_is8 != 0 && url_is8 != 1) || (!message && message_len) ||
      (!detailed && detailed_len) || (!url && url_len)) return kErrBadArg;
  try {
    Gov8IsolateScope enter(inspector->iso);
    auto current = inspector->iso->GetCurrentContext();
    auto expected = context->ctx->Get(inspector->iso);
    if (current.IsEmpty() || expected.IsEmpty() || current != expected) {
      SetErr("context is not the current entered context");
      return kErrState;
    }
    auto exception = FromWire<v8::Value>(exception_wire);
    if (exception.IsEmpty()) return kErrBadArg;
    auto* stack_trace = static_cast<v8_inspector::V8StackTrace*>(
        *stack_trace_io);
    // The rusty bridge takes unique ownership even when Inspector has no
    // registered context and returns exception id zero.
    *stack_trace_io = nullptr;
    *out = v8_inspector__V8Inspector__exceptionThrown(
        inspector->value, &gov8_snap_impl::DerefLocal(expected),
        View(message_is8, message, message_len),
        &gov8_snap_impl::DerefLocal(exception),
        View(detailed_is8, detailed, detailed_len),
        View(url_is8, url, url_len), line_number, column_number, stack_trace,
        script_id);
    return kOk;
  } catch (...) {
    SetErr("C++ exception reporting Inspector exception");
    return kErrCpp;
  }
}

}  // extern "C"
