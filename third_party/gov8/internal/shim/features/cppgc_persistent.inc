// Native-owned cppgc Persistent/WeakPersistent handles for the fixed gov8
// managed payload. Raw cppgc object pointers never cross the Go ABI.

struct Gov8CppGCPersistentInner;
struct Gov8CppGCWeakPersistentInner;

extern "C" {
Gov8CppGCPersistentInner* cppgc__Persistent__CONSTRUCT(RustObj* object);
void cppgc__Persistent__DESTRUCT(Gov8CppGCPersistentInner* self);
void cppgc__Persistent__Assign(Gov8CppGCPersistentInner* self,
                               RustObj* object);
RustObj* cppgc__Persistent__Get(Gov8CppGCPersistentInner* self);
Gov8CppGCWeakPersistentInner* cppgc__WeakPersistent__CONSTRUCT(
    RustObj* object);
void cppgc__WeakPersistent__DESTRUCT(Gov8CppGCWeakPersistentInner* self);
void cppgc__WeakPersistent__Assign(Gov8CppGCWeakPersistentInner* self,
                                   RustObj* object);
RustObj* cppgc__WeakPersistent__Get(Gov8CppGCWeakPersistentInner* self);
}

namespace gov8_cppgc_persistent_impl {

constexpr uint64_t kPersistentMagic = 0x674f563843505250ULL;  // "gOV8CPRP"

struct PersistentWrap {
  uint64_t magic;
  v8::Isolate* isolate;
  bool weak;
  void* root;

  PersistentWrap(v8::Isolate* owner, bool is_weak, void* native_root)
      : magic(kPersistentMagic),
        isolate(owner),
        weak(is_weak),
        root(native_root) {}
};

PersistentWrap* Checked(void* wire, v8::Isolate* isolate) {
  auto* handle = static_cast<PersistentWrap*>(wire);
  if (handle == nullptr || handle->magic != kPersistentMagic ||
      handle->isolate != isolate || handle->root == nullptr) {
    return nullptr;
  }
  return handle;
}

RustObj* Get(PersistentWrap* handle) {
  if (handle->weak) {
    return cppgc__WeakPersistent__Get(
        static_cast<Gov8CppGCWeakPersistentInner*>(handle->root));
  }
  return cppgc__Persistent__Get(
      static_cast<Gov8CppGCPersistentInner*>(handle->root));
}

void Assign(PersistentWrap* handle, RustObj* object) {
  if (handle->weak) {
    cppgc__WeakPersistent__Assign(
        static_cast<Gov8CppGCWeakPersistentInner*>(handle->root), object);
  } else {
    cppgc__Persistent__Assign(
        static_cast<Gov8CppGCPersistentInner*>(handle->root), object);
  }
}

void Destroy(PersistentWrap* handle) {
  if (handle->weak) {
    cppgc__WeakPersistent__DESTRUCT(
        static_cast<Gov8CppGCWeakPersistentInner*>(handle->root));
  } else {
    cppgc__Persistent__DESTRUCT(
        static_cast<Gov8CppGCPersistentInner*>(handle->root));
  }
  handle->root = nullptr;
  handle->magic = 0;
  delete handle;
}

RustObj* Unwrap(v8::Isolate* isolate, void* wrapper_wire, uint32_t tag,
                uint64_t registry_id) {
  using namespace gov8_cppgc_impl;
  if (wrapper_wire == nullptr || tag > kMaxTag || registry_id == 0) {
    return nullptr;
  }
  v8::Local<v8::Value> value = FromWire<v8::Value>(wrapper_wire);
  if (!value->IsObject()) return nullptr;
  RustObj* object = v8__Object__Unwrap(
      isolate, gov8_snap_impl::DerefLocal(value.As<v8::Object>()),
      static_cast<v8::CppHeapPointerTag>(tag));
  if (object == nullptr) return nullptr;
  Payload* payload = PayloadOf(object);
  if (payload->magic != kPayloadMagic || payload->tag != tag ||
      payload->registry_id != registry_id) {
    return nullptr;
  }
  return object;
}

}  // namespace gov8_cppgc_persistent_impl

extern "C" {

__declspec(dllexport) int64_t gov8_cppgc_persistent_empty(
    v8::Isolate* isolate, int32_t weak, void** out) {
  using namespace gov8_cppgc_persistent_impl;
  ClearErr();
  if (out != nullptr) *out = nullptr;
  if (isolate == nullptr || (weak != 0 && weak != 1) || out == nullptr) {
    SetErr("invalid cppgc persistent empty argument");
    return kErrBadArg;
  }
  try {
    void* root = weak != 0
                     ? static_cast<void*>(cppgc__WeakPersistent__CONSTRUCT(nullptr))
                     : static_cast<void*>(cppgc__Persistent__CONSTRUCT(nullptr));
    if (root == nullptr) {
      SetErr("cppgc persistent allocation failed");
      return kErrNoMemory;
    }
    std::unique_ptr<PersistentWrap> handle(
        new (std::nothrow) PersistentWrap(isolate, weak != 0, root));
    if (!handle) {
      if (weak != 0) {
        cppgc__WeakPersistent__DESTRUCT(
            static_cast<Gov8CppGCWeakPersistentInner*>(root));
      } else {
        cppgc__Persistent__DESTRUCT(
            static_cast<Gov8CppGCPersistentInner*>(root));
      }
      SetErr("cppgc persistent wrapper allocation failed");
      return kErrNoMemory;
    }
    *out = handle.release();
    return kOk;
  } catch (...) {
    SetErr("C++ exception creating empty cppgc persistent");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_persistent_from_wrapper(
    v8::Isolate* isolate, void* scope, void* wrapper_wire, uint32_t tag,
    uint64_t registry_id, int32_t weak, void** out) {
  using namespace gov8_cppgc_persistent_impl;
  ClearErr();
  if (out != nullptr) *out = nullptr;
  if (isolate == nullptr || !ScopeIs(isolate, scope) || wrapper_wire == nullptr ||
      (weak != 0 && weak != 1) || out == nullptr) {
    SetErr("invalid cppgc persistent constructor argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    RustObj* object = Unwrap(isolate, wrapper_wire, tag, registry_id);
    if (object == nullptr) {
      SetErr("cppgc persistent source is not a live gov8 managed object");
      return kErrState;
    }
    void* root = weak != 0
                     ? static_cast<void*>(cppgc__WeakPersistent__CONSTRUCT(object))
                     : static_cast<void*>(cppgc__Persistent__CONSTRUCT(object));
    if (root == nullptr) {
      SetErr("cppgc persistent allocation failed");
      return kErrNoMemory;
    }
    std::unique_ptr<PersistentWrap> handle(
        new (std::nothrow) PersistentWrap(isolate, weak != 0, root));
    if (!handle) {
      if (weak != 0) {
        cppgc__WeakPersistent__DESTRUCT(
            static_cast<Gov8CppGCWeakPersistentInner*>(root));
      } else {
        cppgc__Persistent__DESTRUCT(
            static_cast<Gov8CppGCPersistentInner*>(root));
      }
      SetErr("cppgc persistent wrapper allocation failed");
      return kErrNoMemory;
    }
    *out = handle.release();
    return kOk;
  } catch (...) {
    SetErr("C++ exception creating cppgc persistent");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_persistent_set(
    void* handle_wire, v8::Isolate* isolate, void* scope, void* wrapper_wire,
    uint32_t tag, uint64_t registry_id) {
  using namespace gov8_cppgc_persistent_impl;
  ClearErr();
  PersistentWrap* handle = Checked(handle_wire, isolate);
  if (handle == nullptr || !ScopeIs(isolate, scope) || wrapper_wire == nullptr) {
    SetErr("invalid cppgc persistent set argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    RustObj* object = Unwrap(isolate, wrapper_wire, tag, registry_id);
    if (object == nullptr) {
      SetErr("cppgc persistent source is not a live gov8 managed object");
      return kErrState;
    }
    Assign(handle, object);
    return kOk;
  } catch (...) {
    SetErr("C++ exception assigning cppgc persistent");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_persistent_set_from_persistent(
    void* handle_wire, void* source_wire, v8::Isolate* isolate) {
  using namespace gov8_cppgc_persistent_impl;
  ClearErr();
  PersistentWrap* handle = Checked(handle_wire, isolate);
  PersistentWrap* source = Checked(source_wire, isolate);
  if (handle == nullptr || source == nullptr) {
    SetErr("invalid cppgc persistent source assignment");
    return kErrBadArg;
  }
  try {
    // Assigning nullptr is the pinned BasicPersistent empty-source behavior.
    Assign(handle, Get(source));
    return kOk;
  } catch (...) {
    SetErr("C++ exception assigning from cppgc persistent");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_persistent_get(
    void* handle_wire, v8::Isolate* isolate, uint64_t* registry_id,
    int32_t* object_id, uint16_t* tag, int32_t* present) {
  using namespace gov8_cppgc_persistent_impl;
  using namespace gov8_cppgc_impl;
  ClearErr();
  if (registry_id != nullptr) *registry_id = 0;
  if (object_id != nullptr) *object_id = 0;
  if (tag != nullptr) *tag = 0;
  if (present != nullptr) *present = 0;
  PersistentWrap* handle = Checked(handle_wire, isolate);
  if (handle == nullptr || registry_id == nullptr || object_id == nullptr ||
      tag == nullptr || present == nullptr) {
    SetErr("invalid cppgc persistent get argument");
    return kErrBadArg;
  }
  try {
    RustObj* object = Get(handle);
    if (object == nullptr) return kOk;
    Payload* payload = PayloadOf(object);
    if (payload->magic != kPayloadMagic || payload->registry_id == 0 ||
        payload->tag > kMaxTag) {
      SetErr("cppgc persistent target is not a live gov8 managed object");
      return kErrState;
    }
    *registry_id = payload->registry_id;
    *object_id = payload->object_id;
    *tag = payload->tag;
    *present = 1;
    return kOk;
  } catch (...) {
    SetErr("C++ exception reading cppgc persistent");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_cppgc_persistent_close(
    void* handle_wire, v8::Isolate* isolate) {
  using namespace gov8_cppgc_persistent_impl;
  ClearErr();
  PersistentWrap* handle = Checked(handle_wire, isolate);
  if (handle == nullptr) {
    SetErr("invalid cppgc persistent close argument");
    return kErrBadArg;
  }
  try {
    Destroy(handle);
    return kOk;
  } catch (...) {
    SetErr("C++ exception closing cppgc persistent");
    return kErrCpp;
  }
}

// The isolate pointer is an opaque identity here. V8 has already disposed the
// isolate and its cppgc heap, which cleared the persistent node; deleting the
// wrapper is the pinned same-thread post-isolate Rust Drop behavior.
__declspec(dllexport) int64_t gov8_cppgc_persistent_close_after_isolate_dispose(
    void* handle_wire, void* disposed_isolate) {
  using namespace gov8_cppgc_persistent_impl;
  ClearErr();
  auto* handle = static_cast<PersistentWrap*>(handle_wire);
  if (handle == nullptr || handle->magic != kPersistentMagic ||
      handle->isolate != disposed_isolate || handle->root == nullptr) {
    SetErr("invalid post-isolate cppgc persistent close argument");
    return kErrBadArg;
  }
  try {
    Destroy(handle);
    return kOk;
  } catch (...) {
    SetErr("C++ exception closing post-isolate cppgc persistent");
    return kErrCpp;
  }
}

}  // extern "C"
