// Traced cppgc Member/WeakMember edges embedded in gov8's managed payload.
// The ABI accepts only rooted owner handles and copies pointee metadata back;
// raw garbage-collected pointers never cross into Go.

extern "C" {
void cppgc__Member__Assign(cppgc::Member<RustObj>* member, RustObj* other);
RustObj* cppgc__Member__Get(cppgc::Member<RustObj>* member);
void cppgc__WeakMember__Assign(cppgc::WeakMember<RustObj>* member,
                               RustObj* other);
RustObj* cppgc__WeakMember__Get(cppgc::WeakMember<RustObj>* member);
}

namespace gov8_cppgc_member_impl {

using gov8_cppgc_impl::Payload;
using gov8_cppgc_impl::PayloadOf;
using gov8_cppgc_impl::kMaxTag;
using gov8_cppgc_impl::kPayloadMagic;
using gov8_cppgc_persistent_impl::Checked;
using gov8_cppgc_persistent_impl::Get;
using gov8_cppgc_persistent_impl::PersistentWrap;
using gov8_cppgc_persistent_impl::Unwrap;

PersistentWrap* StrongOwner(void* wire, v8::Isolate* isolate) {
  PersistentWrap* owner = Checked(wire, isolate);
  if (owner == nullptr || owner->weak) return nullptr;
  return owner;
}

bool Valid(RustObj* object) {
  if (object == nullptr) return false;
  const Payload* payload = PayloadOf(object);
  return payload->magic == kPayloadMagic && payload->registry_id != 0 &&
         payload->tag <= kMaxTag;
}

RustObj* Edge(Payload* payload, bool weak) {
  return weak ? cppgc__WeakMember__Get(&payload->weak)
              : cppgc__Member__Get(&payload->strong);
}

void Assign(Payload* payload, bool weak, RustObj* child) {
  if (weak) {
    cppgc__WeakMember__Assign(&payload->weak, child);
  } else {
    cppgc__Member__Assign(&payload->strong, child);
  }
}

}  // namespace gov8_cppgc_member_impl

extern "C" {

__declspec(dllexport) int64_t gov8_cppgc_member_set(
    void* owner_wire, v8::Isolate* isolate, void* scope,
    void* child_wrapper_wire, uint32_t child_tag, uint64_t child_registry_id,
    int32_t weak) {
  using namespace gov8_cppgc_member_impl;
  ClearErr();
  PersistentWrap* owner = StrongOwner(owner_wire, isolate);
  if (owner == nullptr || !ScopeIs(isolate, scope) ||
      child_wrapper_wire == nullptr || child_registry_id == 0 ||
      child_tag > kMaxTag || (weak != 0 && weak != 1)) {
    SetErr("invalid cppgc member set argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    RustObj* owner_object = Get(owner);
    RustObj* child = Unwrap(isolate, child_wrapper_wire, child_tag,
                            child_registry_id);
    if (!Valid(owner_object) || !Valid(child)) {
      SetErr("cppgc member owner or child is not live");
      return kErrState;
    }
    Assign(PayloadOf(owner_object), weak != 0, child);
    return kOk;
  } catch (...) {
    SetErr("C++ exception assigning cppgc member");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_member_clear(
    void* owner_wire, v8::Isolate* isolate, int32_t weak) {
  using namespace gov8_cppgc_member_impl;
  ClearErr();
  PersistentWrap* owner = StrongOwner(owner_wire, isolate);
  if (owner == nullptr || (weak != 0 && weak != 1)) {
    SetErr("invalid cppgc member clear argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    RustObj* owner_object = Get(owner);
    if (!Valid(owner_object)) {
      SetErr("cppgc member owner is not live");
      return kErrState;
    }
    Assign(PayloadOf(owner_object), weak != 0, nullptr);
    return kOk;
  } catch (...) {
    SetErr("C++ exception clearing cppgc member");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_member_get(
    void* owner_wire, v8::Isolate* isolate, int32_t weak,
    uint64_t* registry_id, int32_t* object_id, uint16_t* tag,
    int32_t* present) {
  using namespace gov8_cppgc_member_impl;
  ClearErr();
  if (registry_id != nullptr) *registry_id = 0;
  if (object_id != nullptr) *object_id = 0;
  if (tag != nullptr) *tag = 0;
  if (present != nullptr) *present = 0;
  PersistentWrap* owner = StrongOwner(owner_wire, isolate);
  if (owner == nullptr || (weak != 0 && weak != 1) ||
      registry_id == nullptr || object_id == nullptr || tag == nullptr ||
      present == nullptr) {
    SetErr("invalid cppgc member get argument");
    return kErrBadArg;
  }
  try {
    RustObj* owner_object = Get(owner);
    if (!Valid(owner_object)) {
      SetErr("cppgc member owner is not live");
      return kErrState;
    }
    RustObj* child = Edge(PayloadOf(owner_object), weak != 0);
    if (child == nullptr) return kOk;
    if (!Valid(child)) {
      SetErr("cppgc member target is not live");
      return kErrState;
    }
    const Payload* child_payload = PayloadOf(child);
    *registry_id = child_payload->registry_id;
    *object_id = child_payload->object_id;
    *tag = child_payload->tag;
    *present = 1;
    return kOk;
  } catch (...) {
    SetErr("C++ exception reading cppgc member");
    return kErrCpp;
  }
}

}  // extern "C"
