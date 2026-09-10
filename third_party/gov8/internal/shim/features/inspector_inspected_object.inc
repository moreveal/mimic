// Inspector Inspectable/addInspectedObject for pinned V8 15.2. Include after
// inspector_transport.inc (prefer after inspector_object_wrapping.inc). The
// session owns transferred InspectableHost instances; each native destructor
// synchronously tells Go to remove its registry entry exactly once.

#include "v8-inspector.h"
#include <cstdint>
#include <cstdlib>
#include <memory>

namespace gov8_inspector_inspected_object_impl {

struct InspectableFrame {
  uint64_t id;
  v8::Isolate* isolate;
  void* scope_wire;
  void* context_wire;
  void* result_wire;
};

static_assert(sizeof(InspectableFrame) == 40,
              "Go/native Inspector Inspectable frame layout drift");

using GoInspectableGet = uintptr_t (*)(InspectableFrame*);
using GoInspectableDrop = uintptr_t (*)(uint64_t);

GoInspectableGet g_get_dispatch = nullptr;
GoInspectableDrop g_drop_dispatch = nullptr;

class InspectableHost final
    : public v8_inspector::V8InspectorSession::Inspectable {
 public:
  explicit InspectableHost(uint64_t id) : id_(id) {}

  ~InspectableHost() override {
    if (g_drop_dispatch == nullptr || g_drop_dispatch(id_) != 0) std::abort();
  }

  v8::Local<v8::Value> get(v8::Local<v8::Context> context) override {
    v8::Isolate* isolate = v8::Isolate::GetCurrent();
    if (isolate == nullptr || context.IsEmpty() || g_get_dispatch == nullptr) {
      std::abort();
    }
    // This must be the only HandleScope opened around the Go getter. The
    // returned callback local is allocated directly in this escapable scope.
    v8::EscapableHandleScope handle_scope(isolate);
    v8::Context::Scope context_scope(context);
    GoScope borrowed_scope{};
    borrowed_scope.magic = kScopeMagic;
    borrowed_scope.iso = isolate;
    InspectableFrame frame{id_, isolate, &borrowed_scope, ToWire(context),
                           nullptr};
    if (g_get_dispatch(&frame) != 0 || frame.result_wire == nullptr) {
      borrowed_scope.magic = 0;
      std::abort();
    }
    const v8::Local<v8::Value> result =
        FromWire<v8::Value>(frame.result_wire);
    borrowed_scope.magic = 0;
    return handle_scope.Escape(result);
  }

 private:
  uint64_t id_;
};

}  // namespace gov8_inspector_inspected_object_impl

extern "C" {

__declspec(dllexport) int64_t gov8_iio_set_dispatchers(void* get_dispatch,
                                                       void* drop_dispatch) {
  using namespace gov8_inspector_inspected_object_impl;
  ClearErr();
  if (get_dispatch == nullptr || drop_dispatch == nullptr) return kErrBadArg;
  g_get_dispatch = reinterpret_cast<GoInspectableGet>(get_dispatch);
  g_drop_dispatch = reinterpret_cast<GoInspectableDrop>(drop_dispatch);
  return kOk;
}

__declspec(dllexport) int64_t gov8_iio_new(uint64_t id, void** out) {
  using namespace gov8_inspector_inspected_object_impl;
  ClearErr();
  if (out != nullptr) *out = nullptr;
  if (id == 0 || out == nullptr || g_get_dispatch == nullptr ||
      g_drop_dispatch == nullptr) {
    return kErrBadArg;
  }
  try {
    auto value = std::make_unique<InspectableHost>(id);
    *out = value.release();
    return kOk;
  } catch (const std::bad_alloc&) {
    SetErr("out of memory creating Inspector Inspectable");
    return kErrNoMemory;
  } catch (...) {
    SetErr("C++ exception creating Inspector Inspectable");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_iio_delete(void* raw) {
  using namespace gov8_inspector_inspected_object_impl;
  ClearErr();
  if (raw == nullptr) return kErrBadArg;
  try {
    delete static_cast<InspectableHost*>(raw);
    return kOk;
  } catch (...) {
    // InspectableHost's destructor dispatch is fail-fast and cannot throw.
    SetErr("C++ exception deleting Inspector Inspectable");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_iio_add(void* session_raw, void* raw,
                                           int32_t* out_consumed) {
  using namespace gov8_inspector_impl;
  using namespace gov8_inspector_inspected_object_impl;
  ClearErr();
  if (out_consumed != nullptr) *out_consumed = 0;
  auto* session = AsSession(session_raw);
  if (session == nullptr || raw == nullptr || out_consumed == nullptr) {
    return kErrBadArg;
  }
  try {
    std::unique_ptr<v8_inspector::V8InspectorSession::Inspectable> owned(
        static_cast<InspectableHost*>(raw));
    *out_consumed = 1;
    session->value->addInspectedObject(std::move(owned));
    return kOk;
  } catch (const std::bad_alloc&) {
    SetErr("out of memory adding Inspector Inspectable");
    return kErrNoMemory;
  } catch (...) {
    SetErr("C++ exception adding Inspector Inspectable");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_iio_context_matches(
    v8::Isolate* isolate, void* context_wire, void* context_raw,
    int32_t* out) {
  ClearErr();
  if (out != nullptr) *out = 0;
  auto* context = AsCtx(context_raw);
  if (isolate == nullptr || context_wire == nullptr || context == nullptr ||
      context->iso != isolate || out == nullptr) {
    return kErrBadArg;
  }
  try {
    const v8::Local<v8::Context> callback_context =
        FromWire<v8::Context>(context_wire);
    *out = context->ctx->Get(isolate) == callback_context ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception matching Inspector callback context");
    return kErrCpp;
  }
}

}  // extern "C"
