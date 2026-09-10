// cppgc object wrapping for pinned rusty_v8 152.2.0.
// Allocation, metadata construction, and Object::Wrap occur in one native
// call. No raw or temporarily unrooted cppgc pointer is returned to Go.

#include <atomic>
#include <cppgc/member.h>
#include <new>
#include <string>

class RustObj : public v8::Object::Wrappable {
 public:
  ~RustObj();
  void Trace(cppgc::Visitor* visitor) const;
  const char* GetHumanReadableName() const final;
};

extern "C" {
v8::CppHeap* v8__Isolate__GetCppHeap(v8::Isolate* isolate);
RustObj* cppgc__make_garbage_collectable(v8::CppHeap* heap, size_t size,
                                         size_t alignment);
void v8__Object__Wrap(v8::Isolate* isolate, const v8::Object& wrapper,
                      RustObj* value, v8::CppHeapPointerTag tag);
RustObj* v8__Object__Unwrap(v8::Isolate* isolate, const v8::Object& wrapper,
                            v8::CppHeapPointerTag tag);
void cppgc__Visitor__Trace__TracedReference(
    cppgc::Visitor* visitor, v8::TracedReference<v8::Data>* reference);
void cppgc__Visitor__Trace__Member(cppgc::Visitor* visitor,
                                   cppgc::Member<RustObj>* member);
void cppgc__Visitor__Trace__WeakMember(cppgc::Visitor* visitor,
                                       cppgc::WeakMember<RustObj>* member);
const v8::Data* v8__TracedReference__Get(
    v8::TracedReference<v8::Data>* self, v8::Isolate* isolate);
bool v8__Object__IsApiWrapper(const v8::Object& self);
}

namespace gov8_cppgc_impl {

namespace generic {
void Dispatch(uint64_t id, uint32_t kind, int32_t value);
}

constexpr uint64_t kPayloadMagic = 0x674f563843504743ULL;  // "gOV8CPGC"
constexpr uint16_t kMaxTag = 0x7ffe;
using GoDispatcher = uintptr_t (*)(uint64_t registry_id, uint32_t kind);
constexpr uint32_t kTrace = 1;
constexpr uint32_t kDestroy = 2;

struct Payload {
  uint64_t magic;
  uint64_t registry_id;
  int32_t object_id;
  uint16_t tag;
  uint8_t observe_trace;
  uint8_t reserved;
  uint64_t generic_registry_id;
  int32_t cell_value;
  uint32_t requested_size;
  uint32_t requested_alignment;
  const int32_t* cell_address;
  std::string custom_name;
  cppgc::Member<RustObj> strong;
  cppgc::WeakMember<RustObj> weak;
  v8::TracedReference<v8::Data> traced;
  void* generic_graph;

  Payload(uint64_t registry, int32_t id, uint16_t pointer_tag,
          bool should_observe_trace)
      : magic(kPayloadMagic),
        registry_id(registry),
        object_id(id),
        tag(pointer_tag),
        observe_trace(should_observe_trace ? 1 : 0),
        reserved(0),
        generic_registry_id(0),
        cell_value(0),
        requested_size(0),
        requested_alignment(1),
        cell_address(&cell_value),
        custom_name(),
        strong(nullptr),
        weak(nullptr),
        traced(),
        generic_graph(nullptr) {}
};

namespace graph {
void Trace(const Payload* payload, cppgc::Visitor* visitor);
const char* Name(const Payload* payload);
void Drop(Payload* payload);
}  // namespace graph

constexpr size_t AlignUp(size_t value, size_t alignment) {
  return (value + alignment - 1) & ~(alignment - 1);
}
constexpr size_t kPayloadOffset = AlignUp(sizeof(RustObj), alignof(Payload));
constexpr size_t kAdditionalBytes =
    kPayloadOffset + sizeof(Payload) - sizeof(RustObj);
constexpr size_t kAllocationAlignment =
    alignof(RustObj) > alignof(Payload) ? alignof(RustObj) : alignof(Payload);
static_assert(kAllocationAlignment <= 16,
              "cppgc supports at most 16-byte object alignment");

Payload* PayloadOf(RustObj* object) {
  return reinterpret_cast<Payload*>(reinterpret_cast<unsigned char*>(object) +
                                    kPayloadOffset);
}
const Payload* PayloadOf(const RustObj* object) {
  return PayloadOf(const_cast<RustObj*>(object));
}

std::atomic<GoDispatcher> dispatcher{nullptr};

[[noreturn]] void Abort() { std::abort(); }

void Dispatch(uint64_t id, uint32_t kind) {
  GoDispatcher callback = dispatcher.load(std::memory_order_acquire);
  if (callback == nullptr) Abort();
  callback(id, kind);
}

}  // namespace gov8_cppgc_impl

// The pinned artifact invokes these from cppgc marking and sweeping. They
// replace the link-only RustObj stubs now that RustObj is reachable from Go.
extern "C" void rusty_v8_RustObj_trace(const RustObj* object,
                                         cppgc::Visitor* visitor) {
  using namespace gov8_cppgc_impl;
  if (object == nullptr || visitor == nullptr) Abort();
  const Payload* payload = PayloadOf(object);
  if (payload->magic != kPayloadMagic) Abort();
  if (payload->observe_trace != 0) {
    Dispatch(payload->registry_id, kTrace);
  }
  cppgc__Visitor__Trace__Member(
      visitor, const_cast<cppgc::Member<RustObj>*>(&payload->strong));
  cppgc__Visitor__Trace__WeakMember(
      visitor, const_cast<cppgc::WeakMember<RustObj>*>(&payload->weak));
  cppgc__Visitor__Trace__TracedReference(
      visitor, const_cast<v8::TracedReference<v8::Data>*>(&payload->traced));
  if (payload->generic_graph != nullptr) {
    graph::Trace(payload, visitor);
  }
}

extern "C" const char* rusty_v8_RustObj_get_name(const RustObj* object) {
  using namespace gov8_cppgc_impl;
  if (object == nullptr) Abort();
  const Payload* payload = PayloadOf(object);
  if (payload->magic != kPayloadMagic) Abort();
  if (payload->generic_graph != nullptr) {
    return graph::Name(payload);
  }
  if (payload->generic_registry_id != 0) {
    generic::Dispatch(payload->generic_registry_id, 2, 0);
    return payload->custom_name.c_str();
  }
  return "gov8.CppGCObject";
}

extern "C" void rusty_v8_RustObj_drop(RustObj* object) {
  using namespace gov8_cppgc_impl;
  if (object == nullptr) Abort();
  Payload* payload = PayloadOf(object);
  if (payload->magic != kPayloadMagic) Abort();
  const uint64_t registry_id = payload->registry_id;
  const uint64_t generic_registry_id = payload->generic_registry_id;
  const int32_t cell_value = payload->cell_value;
  if (payload->generic_graph != nullptr) {
    graph::Drop(payload);
  }
  payload->magic = 0;
  payload->traced.Reset();
  payload->~Payload();
  if (generic_registry_id != 0) {
    generic::Dispatch(generic_registry_id, 1, cell_value);
    generic::Dispatch(generic_registry_id, 3, 0);
  }
  Dispatch(registry_id, kDestroy);
}

extern "C" {

__declspec(dllexport) int64_t gov8_cppgc_allocate_and_wrap(
    v8::Isolate* isolate, void* scope, void* wrapper_wire,
    void* target_wire, uint64_t registry_id, int32_t object_id,
    uint32_t tag, int32_t observe_trace, void* dispatch, int32_t* consumed) {
  using namespace gov8_cppgc_impl;
  ClearErr();
  if (consumed != nullptr) *consumed = 0;
  if (isolate == nullptr || !ScopeIs(isolate, scope) ||
      wrapper_wire == nullptr || target_wire == nullptr || registry_id == 0 ||
      dispatch == nullptr || consumed == nullptr || tag > kMaxTag ||
      (observe_trace != 0 && observe_trace != 1)) {
    SetErr("invalid cppgc wrap argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    v8::Local<v8::Value> wrapper_value = FromWire<v8::Value>(wrapper_wire);
    if (!wrapper_value->IsObject()) {
      SetErr("cppgc wrapper is not an object");
      return kErrBadArg;
    }
    v8::Local<v8::Object> wrapper = wrapper_value.As<v8::Object>();
    if (!v8__Object__IsApiWrapper(gov8_snap_impl::DerefLocal(wrapper))) {
      SetErr("cppgc wrapper is not an API wrapper");
      return kErrBadArg;
    }
    v8::CppHeap* heap = v8__Isolate__GetCppHeap(isolate);
    if (heap == nullptr) {
      SetErr("isolate has no cppgc heap");
      return kErrState;
    }
    GoDispatcher supplied = reinterpret_cast<GoDispatcher>(dispatch);
    GoDispatcher installed = dispatcher.load(std::memory_order_acquire);
    if (installed != nullptr && installed != supplied) {
      SetErr("cppgc dispatcher changed");
      return kErrState;
    }
    dispatcher.store(supplied, std::memory_order_release);
    RustObj* object = cppgc__make_garbage_collectable(
        heap, kAdditionalBytes, kAllocationAlignment);
    if (object == nullptr) {
      SetErr("cppgc allocation failed");
      return kErrNoMemory;
    }
    *consumed = 1;
    v8::Local<v8::Value> target = FromWire<v8::Value>(target_wire);
    Payload* payload = new (PayloadOf(object))
        Payload(registry_id, object_id, static_cast<uint16_t>(tag),
                observe_trace != 0);
    payload->traced.Reset(isolate, target);
    v8__Object__Wrap(isolate, gov8_snap_impl::DerefLocal(wrapper), object,
                     static_cast<v8::CppHeapPointerTag>(tag));
    return kOk;
  } catch (...) {
    SetErr("C++ exception allocating cppgc object");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_unwrap(
    v8::Isolate* isolate, void* scope, void* wrapper_wire, uint32_t tag,
    uint64_t* registry_id, int32_t* object_id, void** target_out) {
  using namespace gov8_cppgc_impl;
  ClearErr();
  if (registry_id != nullptr) *registry_id = 0;
  if (object_id != nullptr) *object_id = 0;
  if (target_out != nullptr) *target_out = nullptr;
  if (isolate == nullptr || !ScopeIs(isolate, scope) ||
      wrapper_wire == nullptr || tag > kMaxTag || registry_id == nullptr ||
      object_id == nullptr || target_out == nullptr) {
    SetErr("invalid cppgc unwrap argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    v8::Local<v8::Value> wrapper_value = FromWire<v8::Value>(wrapper_wire);
    if (!wrapper_value->IsObject()) return kOk;
    v8::Local<v8::Object> wrapper = wrapper_value.As<v8::Object>();
    RustObj* object = v8__Object__Unwrap(
        isolate, gov8_snap_impl::DerefLocal(wrapper),
        static_cast<v8::CppHeapPointerTag>(tag));
    if (object == nullptr) return kOk;
    Payload* payload = PayloadOf(object);
    // The pinned raw API can return a pointer for a mismatched tag. Go checks
    // the exact recorded tag before exposing the managed payload.
    if (payload->magic != kPayloadMagic || payload->tag != tag) return kOk;
    const v8::Data* target = v8__TracedReference__Get(&payload->traced, isolate);
    if (target == nullptr) {
      SetErr("cppgc traced target is empty");
      return kErrState;
    }
    *registry_id = payload->registry_id;
    *object_id = payload->object_id;
    *target_out = const_cast<v8::Data*>(target);
    return kOk;
  } catch (...) {
    SetErr("C++ exception unwrapping cppgc object");
    return kErrCpp;
  }
}

}  // extern "C"
