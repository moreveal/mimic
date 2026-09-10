// Typed-Go-state and indexed-edge cppgc graph substrate. Native storage keeps
// only numeric Go registry IDs, native Member/WeakMember slots, a copied name,
// and a TracedReference. No Go pointer is retained by V8 or C++.

#include <atomic>
#include <memory>
#include <string>
#include <vector>

namespace gov8_cppgc_impl::graph {

using GoDispatcher = uintptr_t (*)(uint64_t graph_id, uint32_t kind,
                                   uint64_t state_id);
constexpr uint32_t kStateDrop = 1;
constexpr uint32_t kName = 2;
constexpr uint32_t kDestroy = 3;
constexpr uint32_t kTrace = 4;

std::atomic<GoDispatcher> dispatcher{nullptr};

void Dispatch(uint64_t graph_id, uint32_t kind, uint64_t state_id) {
  GoDispatcher callback = dispatcher.load(std::memory_order_acquire);
  if (callback == nullptr || graph_id == 0) gov8_cppgc_impl::Abort();
  callback(graph_id, kind, state_id);
}

bool Install(void* raw) {
  if (raw == nullptr) return false;
  GoDispatcher supplied = reinterpret_cast<GoDispatcher>(raw);
  GoDispatcher current = dispatcher.load(std::memory_order_acquire);
  if (current != nullptr && current != supplied) return false;
  dispatcher.store(supplied, std::memory_order_release);
  return true;
}

struct GraphStorage {
  uint64_t graph_id;
  uint64_t state_id;
  std::string name;
  std::vector<cppgc::Member<RustObj>> strong;
  std::vector<cppgc::WeakMember<RustObj>> weak;
  v8::TracedReference<v8::Data> traced;

  GraphStorage(uint64_t graph, uint64_t state, std::string copied_name,
               size_t strong_count, size_t weak_count)
      : graph_id(graph),
        state_id(state),
        name(std::move(copied_name)),
        strong(strong_count),
        weak(weak_count),
        traced() {}
};

GraphStorage* Storage(gov8_cppgc_impl::Payload* payload) {
  return payload == nullptr
             ? nullptr
             : static_cast<GraphStorage*>(payload->generic_graph);
}

const GraphStorage* Storage(const gov8_cppgc_impl::Payload* payload) {
  return Storage(const_cast<gov8_cppgc_impl::Payload*>(payload));
}

gov8_cppgc_persistent_impl::PersistentWrap* StrongRoot(
    void* wire, v8::Isolate* isolate) {
  auto* root = gov8_cppgc_persistent_impl::Checked(wire, isolate);
  return root != nullptr && !root->weak ? root : nullptr;
}

gov8_cppgc_impl::Payload* GraphPayload(void* wire, v8::Isolate* isolate) {
  auto* root = StrongRoot(wire, isolate);
  if (root == nullptr) return nullptr;
  RustObj* object = gov8_cppgc_persistent_impl::Get(root);
  if (!gov8_cppgc_member_impl::Valid(object)) return nullptr;
  auto* payload = gov8_cppgc_impl::PayloadOf(object);
  return Storage(payload) == nullptr ? nullptr : payload;
}

void Trace(const gov8_cppgc_impl::Payload* payload,
           cppgc::Visitor* visitor) {
  const GraphStorage* storage = Storage(payload);
  if (storage == nullptr || visitor == nullptr) gov8_cppgc_impl::Abort();
  Dispatch(storage->graph_id, kTrace, storage->state_id);
  for (const auto& member : storage->strong) {
    cppgc__Visitor__Trace__Member(
        visitor, const_cast<cppgc::Member<RustObj>*>(&member));
  }
  for (const auto& member : storage->weak) {
    cppgc__Visitor__Trace__WeakMember(
        visitor, const_cast<cppgc::WeakMember<RustObj>*>(&member));
  }
  cppgc__Visitor__Trace__TracedReference(
      visitor,
      const_cast<v8::TracedReference<v8::Data>*>(&storage->traced));
}

const char* Name(const gov8_cppgc_impl::Payload* payload) {
  const GraphStorage* storage = Storage(payload);
  if (storage == nullptr) gov8_cppgc_impl::Abort();
  Dispatch(storage->graph_id, kName, storage->state_id);
  return storage->name.c_str();
}

void Drop(gov8_cppgc_impl::Payload* payload) {
  GraphStorage* storage = Storage(payload);
  if (storage == nullptr) gov8_cppgc_impl::Abort();
  payload->generic_graph = nullptr;
  const uint64_t graph_id = storage->graph_id;
  const uint64_t state_id = storage->state_id;
  storage->traced.Reset();
  delete storage;
  Dispatch(graph_id, kStateDrop, state_id);
  Dispatch(graph_id, kDestroy, 0);
}

}  // namespace gov8_cppgc_impl::graph

extern "C" {

__declspec(dllexport) int64_t gov8_cppgc_graph_new(
    v8::Isolate* isolate, void* scope, uint64_t object_registry_id,
    uint64_t graph_registry_id, uint64_t state_id, uint32_t strong_count,
    uint32_t weak_count, const char* name, size_t name_length,
    void* traced_value, void* object_dispatch, void* graph_dispatch,
    void** root_out, int32_t* consumed) {
  using namespace gov8_cppgc_impl;
  using namespace gov8_cppgc_impl::graph;
  using namespace gov8_cppgc_persistent_impl;
  ClearErr();
  if (root_out != nullptr) *root_out = nullptr;
  if (consumed != nullptr) *consumed = 0;
  if (isolate == nullptr || !ScopeIs(isolate, scope) ||
      object_registry_id == 0 || graph_registry_id == 0 || state_id == 0 ||
      (name == nullptr && name_length != 0) || object_dispatch == nullptr ||
      !Install(graph_dispatch) || root_out == nullptr || consumed == nullptr) {
    SetErr("invalid generic cppgc graph allocation argument");
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
    std::string copied_name;
    if (name_length != 0) copied_name.assign(name, name_length);
    auto storage = std::make_unique<GraphStorage>(
        graph_registry_id, state_id, std::move(copied_name), strong_count,
        weak_count);
    Gov8IsolateScope isolate_scope(isolate);
    if (traced_value != nullptr) {
      storage->traced.Reset(isolate, FromWire<v8::Value>(traced_value));
    }
    v8::CppHeap* heap = v8__Isolate__GetCppHeap(isolate);
    if (heap == nullptr) {
      SetErr("isolate has no cppgc heap");
      return kErrState;
    }
    RustObj* object = cppgc__make_garbage_collectable(
        heap, kAdditionalBytes, kAllocationAlignment);
    if (object == nullptr) {
      SetErr("generic cppgc graph allocation failed");
      return kErrNoMemory;
    }
    *consumed = 1;
    Payload* payload = new (PayloadOf(object))
        Payload(object_registry_id, 0, 0, false);
    payload->generic_graph = storage.release();
    Gov8CppGCPersistentInner* root = cppgc__Persistent__CONSTRUCT(object);
    if (root == nullptr) {
      SetErr("generic cppgc graph root allocation failed");
      return kErrNoMemory;
    }
    std::unique_ptr<PersistentWrap> wrapper(
        new (std::nothrow) PersistentWrap(isolate, false, root));
    if (!wrapper) {
      cppgc__Persistent__DESTRUCT(root);
      SetErr("generic cppgc graph root wrapper allocation failed");
      return kErrNoMemory;
    }
    *root_out = wrapper.release();
    return kOk;
  } catch (...) {
    SetErr("C++ exception allocating generic cppgc graph");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_graph_state_replace(
    void* root, v8::Isolate* isolate, uint64_t state_id) {
  using namespace gov8_cppgc_impl::graph;
  ClearErr();
  auto* payload = GraphPayload(root, isolate);
  GraphStorage* storage = Storage(payload);
  if (storage == nullptr || state_id == 0) {
    SetErr("invalid generic cppgc graph state replacement");
    return kErrBadArg;
  }
  const uint64_t replaced = storage->state_id;
  storage->state_id = state_id;
  Dispatch(storage->graph_id, kStateDrop, replaced);
  return kOk;
}

__declspec(dllexport) int64_t gov8_cppgc_graph_edge_set(
    void* owner_root, void* child_root, v8::Isolate* isolate, int32_t weak,
    uint32_t index) {
  using namespace gov8_cppgc_impl::graph;
  ClearErr();
  auto* owner_payload = GraphPayload(owner_root, isolate);
  auto* child_payload = GraphPayload(child_root, isolate);
  GraphStorage* owner = Storage(owner_payload);
  GraphStorage* child = Storage(child_payload);
  if (owner == nullptr || child == nullptr || (weak != 0 && weak != 1) ||
      (weak == 0 && index >= owner->strong.size()) ||
      (weak == 1 && index >= owner->weak.size())) {
    SetErr("invalid generic cppgc graph edge assignment");
    return kErrBadArg;
  }
  try {
    RustObj* child_object = gov8_cppgc_persistent_impl::Get(
        StrongRoot(child_root, isolate));
    if (weak == 0) {
      cppgc__Member__Assign(&owner->strong[index], child_object);
    } else {
      cppgc__WeakMember__Assign(&owner->weak[index], child_object);
    }
    return kOk;
  } catch (...) {
    SetErr("C++ exception assigning generic cppgc graph edge");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_graph_edge_clear(
    void* owner_root, v8::Isolate* isolate, int32_t weak, uint32_t index) {
  using namespace gov8_cppgc_impl::graph;
  ClearErr();
  GraphStorage* owner = Storage(GraphPayload(owner_root, isolate));
  if (owner == nullptr || (weak != 0 && weak != 1) ||
      (weak == 0 && index >= owner->strong.size()) ||
      (weak == 1 && index >= owner->weak.size())) {
    SetErr("invalid generic cppgc graph edge clear");
    return kErrBadArg;
  }
  if (weak == 0) {
    cppgc__Member__Assign(&owner->strong[index], nullptr);
  } else {
    cppgc__WeakMember__Assign(&owner->weak[index], nullptr);
  }
  return kOk;
}

__declspec(dllexport) int64_t gov8_cppgc_graph_edge_get(
    void* owner_root, v8::Isolate* isolate, int32_t weak, uint32_t index,
    uint64_t* graph_id, uint64_t* state_id, int32_t* present) {
  using namespace gov8_cppgc_impl::graph;
  ClearErr();
  if (graph_id != nullptr) *graph_id = 0;
  if (state_id != nullptr) *state_id = 0;
  if (present != nullptr) *present = 0;
  GraphStorage* owner = Storage(GraphPayload(owner_root, isolate));
  if (owner == nullptr || graph_id == nullptr || state_id == nullptr ||
      present == nullptr || (weak != 0 && weak != 1) ||
      (weak == 0 && index >= owner->strong.size()) ||
      (weak == 1 && index >= owner->weak.size())) {
    SetErr("invalid generic cppgc graph edge read");
    return kErrBadArg;
  }
  RustObj* target = weak == 0 ? cppgc__Member__Get(&owner->strong[index])
                              : cppgc__WeakMember__Get(&owner->weak[index]);
  if (target == nullptr) return kOk;
  auto* target_payload = gov8_cppgc_impl::PayloadOf(target);
  GraphStorage* target_storage = Storage(target_payload);
  if (target_payload->magic != gov8_cppgc_impl::kPayloadMagic ||
      target_storage == nullptr) {
    SetErr("generic cppgc graph edge target is stale");
    return kErrState;
  }
  *graph_id = target_storage->graph_id;
  *state_id = target_storage->state_id;
  *present = 1;
  return kOk;
}

__declspec(dllexport) int64_t gov8_cppgc_graph_traced_set(
    void* root, v8::Isolate* isolate, void* scope, void* value) {
  using namespace gov8_cppgc_impl::graph;
  ClearErr();
  GraphStorage* storage = Storage(GraphPayload(root, isolate));
  if (storage == nullptr || !ScopeIs(isolate, scope)) {
    SetErr("invalid generic cppgc graph traced value assignment");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    storage->traced.Reset(
        isolate, value == nullptr ? v8::Local<v8::Value>()
                                  : FromWire<v8::Value>(value));
    return kOk;
  } catch (...) {
    SetErr("C++ exception assigning generic cppgc graph traced value");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_graph_traced_get(
    void* root, v8::Isolate* isolate, void* scope, void** value,
    int32_t* present) {
  using namespace gov8_cppgc_impl::graph;
  ClearErr();
  if (value != nullptr) *value = nullptr;
  if (present != nullptr) *present = 0;
  GraphStorage* storage = Storage(GraphPayload(root, isolate));
  if (storage == nullptr || !ScopeIs(isolate, scope) || value == nullptr ||
      present == nullptr) {
    SetErr("invalid generic cppgc graph traced value read");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    const v8::Data* data = v8__TracedReference__Get(&storage->traced, isolate);
    if (data == nullptr) return kOk;
    *value = const_cast<v8::Data*>(data);
    *present = 1;
    return kOk;
  } catch (...) {
    SetErr("C++ exception reading generic cppgc graph traced value");
    return kErrCpp;
  }
}

}  // extern "C"
