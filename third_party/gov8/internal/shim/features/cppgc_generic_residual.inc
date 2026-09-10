// Safe copied-state adaptation of rusty_v8's generic cppgc surface.
// Native managed allocations retain scalar state, native Member edges, and a
// copied name. Raw garbage-collected pointers never cross the Go ABI.

#include <atomic>
#include <limits>
#include <memory>
#include <string>

namespace gov8_cppgc_impl::generic {

using GoDispatcher = uintptr_t (*)(uint64_t id, uint32_t kind, int32_t value);
std::atomic<GoDispatcher> dispatcher{nullptr};

void Dispatch(uint64_t id, uint32_t kind, int32_t value) {
  GoDispatcher callback = dispatcher.load(std::memory_order_acquire);
  if (callback == nullptr || id == 0) gov8_cppgc_impl::Abort();
  callback(id, kind, value);
}

bool Install(void* raw) {
  if (raw == nullptr) return false;
  GoDispatcher supplied = reinterpret_cast<GoDispatcher>(raw);
  GoDispatcher current = dispatcher.load(std::memory_order_acquire);
  if (current != nullptr && current != supplied) return false;
  dispatcher.store(supplied, std::memory_order_release);
  return true;
}

gov8_cppgc_persistent_impl::PersistentWrap* Strong(void* wire,
                                                    v8::Isolate* isolate) {
  auto* root = gov8_cppgc_persistent_impl::Checked(wire, isolate);
  return root != nullptr && !root->weak ? root : nullptr;
}

gov8_cppgc_impl::Payload* GenericPayload(void* wire, v8::Isolate* isolate) {
  auto* root = Strong(wire, isolate);
  if (root == nullptr) return nullptr;
  RustObj* object = gov8_cppgc_persistent_impl::Get(root);
  if (object == nullptr) return nullptr;
  auto* payload = gov8_cppgc_impl::PayloadOf(object);
  if (payload->magic != gov8_cppgc_impl::kPayloadMagic ||
      payload->generic_registry_id == 0) {
    return nullptr;
  }
  return payload;
}

bool ValidAlignment(uint32_t value) {
  return value != 0 && value <= 16 && (value & (value - 1)) == 0;
}

}  // namespace gov8_cppgc_impl::generic

extern "C" {

__declspec(dllexport) int64_t gov8_cppgc_generic_collect(
    v8::Isolate* isolate, int32_t stack_state) {
  ClearErr();
  if (isolate == nullptr || stack_state < 0 || stack_state > 1) {
    SetErr("invalid isolate cppgc collection argument");
    return kErrBadArg;
  }
  try {
    v8::CppHeap* heap = v8__Isolate__GetCppHeap(isolate);
    if (heap == nullptr) {
      SetErr("isolate has no cppgc heap");
      return kErrState;
    }
    cppgc__heap__collect_garbage_for_testing(
        heap, static_cast<cppgc::EmbedderStackState>(stack_state));
    return kOk;
  } catch (...) {
    SetErr("C++ exception collecting isolate cppgc heap");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_generic_new(
    v8::Isolate* isolate, uint64_t registry_id, uint64_t generic_registry_id,
    int32_t object_id, int32_t initial_cell, uint32_t requested_size,
    uint32_t requested_alignment, const char* name, size_t name_length,
    void* object_dispatch, void* generic_dispatch, void** root_out,
    int32_t* consumed) {
  using namespace gov8_cppgc_impl;
  using namespace gov8_cppgc_impl::generic;
  using namespace gov8_cppgc_persistent_impl;
  ClearErr();
  if (root_out != nullptr) *root_out = nullptr;
  if (consumed != nullptr) *consumed = 0;
  if (isolate == nullptr || registry_id == 0 || generic_registry_id == 0 ||
      !ValidAlignment(requested_alignment) ||
      (name == nullptr && name_length != 0) || root_out == nullptr ||
      consumed == nullptr || object_dispatch == nullptr ||
      !Install(generic_dispatch)) {
    SetErr("invalid generic cppgc allocation argument");
    return kErrBadArg;
  }
  try {
    gov8_cppgc_impl::GoDispatcher supplied_object_dispatch =
        reinterpret_cast<gov8_cppgc_impl::GoDispatcher>(object_dispatch);
    gov8_cppgc_impl::GoDispatcher installed_object_dispatch =
        gov8_cppgc_impl::dispatcher.load(std::memory_order_acquire);
    if (installed_object_dispatch != nullptr &&
        installed_object_dispatch != supplied_object_dispatch) {
      SetErr("cppgc object dispatcher changed");
      return kErrState;
    }
    gov8_cppgc_impl::dispatcher.store(supplied_object_dispatch,
                                      std::memory_order_release);
    std::string name_copy;
    if (name_length != 0) name_copy.assign(name, name_length);
    v8::CppHeap* heap = v8__Isolate__GetCppHeap(isolate);
    if (heap == nullptr) {
      SetErr("isolate has no cppgc heap");
      return kErrState;
    }
    const size_t allocation_alignment =
        requested_alignment > kAllocationAlignment ? requested_alignment
                                                    : kAllocationAlignment;
    RustObj* object = cppgc__make_garbage_collectable(
        heap, kAdditionalBytes + requested_size, allocation_alignment);
    if (object == nullptr) {
      SetErr("generic cppgc allocation failed");
      return kErrNoMemory;
    }
    *consumed = 1;
    Payload* payload = new (PayloadOf(object))
        Payload(registry_id, object_id, 0, false);
    payload->generic_registry_id = generic_registry_id;
    payload->cell_value = initial_cell;
    payload->cell_address = &payload->cell_value;
    payload->requested_size = requested_size;
    payload->requested_alignment = requested_alignment;
    payload->custom_name = std::move(name_copy);

    Gov8CppGCPersistentInner* root = cppgc__Persistent__CONSTRUCT(object);
    if (root == nullptr) {
      SetErr("generic cppgc root allocation failed");
      return kErrNoMemory;
    }
    std::unique_ptr<PersistentWrap> wrapper(
        new (std::nothrow) PersistentWrap(isolate, false, root));
    if (!wrapper) {
      cppgc__Persistent__DESTRUCT(root);
      SetErr("generic cppgc root wrapper allocation failed");
      return kErrNoMemory;
    }
    *root_out = wrapper.release();
    return kOk;
  } catch (...) {
    SetErr("C++ exception allocating generic cppgc object");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_generic_cell_get(
    void* root, v8::Isolate* isolate, int32_t* value) {
  using namespace gov8_cppgc_impl::generic;
  ClearErr();
  if (value != nullptr) *value = 0;
  auto* payload = GenericPayload(root, isolate);
  if (payload == nullptr || value == nullptr) {
    SetErr("invalid generic cppgc cell get argument");
    return kErrBadArg;
  }
  *value = payload->cell_value;
  return kOk;
}

__declspec(dllexport) int64_t gov8_cppgc_generic_cell_set(
    void* root, v8::Isolate* isolate, int32_t value) {
  using namespace gov8_cppgc_impl::generic;
  ClearErr();
  auto* payload = GenericPayload(root, isolate);
  if (payload == nullptr) {
    SetErr("invalid generic cppgc cell set argument");
    return kErrBadArg;
  }
  const int32_t replaced = payload->cell_value;
  payload->cell_value = value;
  Dispatch(payload->generic_registry_id, 1, replaced);
  return kOk;
}

__declspec(dllexport) int64_t gov8_cppgc_generic_cell_update(
    void* root, v8::Isolate* isolate, int32_t delta, int32_t* value) {
  using namespace gov8_cppgc_impl::generic;
  ClearErr();
  if (value != nullptr) *value = 0;
  auto* payload = GenericPayload(root, isolate);
  if (payload == nullptr || value == nullptr) {
    SetErr("invalid generic cppgc cell update argument");
    return kErrBadArg;
  }
  const int64_t updated = static_cast<int64_t>(payload->cell_value) + delta;
  if (updated < std::numeric_limits<int32_t>::min() ||
      updated > std::numeric_limits<int32_t>::max()) {
    SetErr("generic cppgc cell update overflow");
    return kErrBadArg;
  }
  payload->cell_value = static_cast<int32_t>(updated);
  *value = payload->cell_value;
  return kOk;
}

__declspec(dllexport) int64_t gov8_cppgc_generic_layout(
    void* root, v8::Isolate* isolate, uint32_t* size, uint32_t* alignment,
    int32_t* address_aligned, int32_t* cell_storage_stable) {
  using namespace gov8_cppgc_impl::generic;
  ClearErr();
  if (size != nullptr) *size = 0;
  if (alignment != nullptr) *alignment = 0;
  if (address_aligned != nullptr) *address_aligned = 0;
  if (cell_storage_stable != nullptr) *cell_storage_stable = 0;
  auto* persistent = Strong(root, isolate);
  auto* payload = GenericPayload(root, isolate);
  RustObj* object = persistent == nullptr
                        ? nullptr
                        : gov8_cppgc_persistent_impl::Get(persistent);
  if (payload == nullptr || object == nullptr || size == nullptr ||
      alignment == nullptr || address_aligned == nullptr ||
      cell_storage_stable == nullptr) {
    SetErr("invalid generic cppgc layout argument");
    return kErrBadArg;
  }
  *size = payload->requested_size;
  *alignment = payload->requested_alignment;
  *address_aligned =
      reinterpret_cast<uintptr_t>(object) % payload->requested_alignment == 0;
  *cell_storage_stable = payload->cell_address == &payload->cell_value;
  return kOk;
}

__declspec(dllexport) int64_t gov8_cppgc_generic_member_set(
    void* owner_root, void* child_root, v8::Isolate* isolate) {
  using namespace gov8_cppgc_impl::generic;
  ClearErr();
  auto* owner = GenericPayload(owner_root, isolate);
  auto* child_persistent = Strong(child_root, isolate);
  RustObj* child = child_persistent == nullptr
                       ? nullptr
                       : gov8_cppgc_persistent_impl::Get(child_persistent);
  if (owner == nullptr || child == nullptr ||
      !gov8_cppgc_member_impl::Valid(child)) {
    SetErr("invalid generic cppgc member assignment");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    cppgc__Member__Assign(&owner->strong, child);
    return kOk;
  } catch (...) {
    SetErr("C++ exception assigning generic cppgc member");
    return kErrCpp;
  }
}

}  // extern "C"
