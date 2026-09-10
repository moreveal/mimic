// ===========================================================================
// Residual persistent handles: Eternal and TracedReference.
//
// The pinned rusty_v8 artifact exposes both types as eight-byte opaque values.
// Keep their storage opaque and dispatch exclusively through the artifact's
// flat bindings: the vendored C++ headers are used only for ordinary locals.
// Wrapper-side isolate identity makes cross-isolate Get deterministic instead
// of reproducing the oracle's unsafe corruption/access-violation boundary.
// ===========================================================================

namespace gov8_handles_residual_impl {

constexpr uint64_t kEternalMagic = 0x674f563845544552ULL;  // "gOV8ETER"
constexpr uint64_t kTracedMagic = 0x674f563854524143ULL;   // "gOV8TRAC"
constexpr size_t kOpaqueHandleSize = 8;

struct EternalWrap {
  uint64_t magic;
  // Bound by the first non-empty Set and deliberately retained after Clear,
  // so reuse and Get cannot silently cross isolates.
  v8::Isolate* iso;
  alignas(void*) unsigned char storage[kOpaqueHandleSize];
};

struct TracedWrap {
  uint64_t magic;
  // Bound by the first non-empty Reset and retained after Reset(None).
  v8::Isolate* iso;
  alignas(void*) unsigned char storage[kOpaqueHandleSize];
};

EternalWrap* AsEternal(void* p) {
  if (p == nullptr) return nullptr;
  EternalWrap* w = static_cast<EternalWrap*>(p);
  return w->magic == kEternalMagic ? w : nullptr;
}

TracedWrap* AsTraced(void* p) {
  if (p == nullptr) return nullptr;
  TracedWrap* w = static_cast<TracedWrap*>(p);
  return w->magic == kTracedMagic ? w : nullptr;
}

}  // namespace gov8_handles_residual_impl

extern "C" {
void v8__TracedReference__CONSTRUCT(void* self);
void v8__TracedReference__DESTRUCT(void* self);
void v8__TracedReference__Reset(void* self, v8::Isolate* isolate,
                                const v8::Data* other);
const v8::Data* v8__TracedReference__Get(
    v8::TracedReference<v8::Data>* self, v8::Isolate* isolate);
void v8__Eternal__CONSTRUCT(void* self);
void v8__Eternal__DESTRUCT(void* self);
void v8__Eternal__Clear(void* self);
bool v8__Eternal__IsEmpty(void* self);
void v8__Eternal__Set(void* self, v8::Isolate* isolate,
                      const v8::Data* value);
const v8::Data* v8__Eternal__Get(void* self, v8::Isolate* isolate);
}

extern "C" {

using namespace gov8_handles_residual_impl;

__declspec(dllexport) void* gov8_hr_eternal_empty() {
  ClearErr();
  try {
    EternalWrap* w = new EternalWrap{};
    w->magic = kEternalMagic;
    v8__Eternal__CONSTRUCT(w->storage);
    return w;
  } catch (...) {
    SetErr("C++ exception constructing Eternal");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_hr_eternal_is_empty(void* handle) {
  ClearErr();
  EternalWrap* w = AsEternal(handle);
  if (w == nullptr) {
    SetErr("invalid Eternal handle");
    return kErrMagic;
  }
  try {
    return v8__Eternal__IsEmpty(w->storage) ? 1 : 0;
  } catch (...) {
    SetErr("C++ exception checking Eternal");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_hr_eternal_set(v8::Isolate* iso,
                                                   void* scope,
                                                   void* handle,
                                                   void* value_wire) {
  ClearErr();
  EternalWrap* w = AsEternal(handle);
  if (w == nullptr || value_wire == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid Eternal.Set argument");
    return kErrBadArg;
  }
  if (w->iso != nullptr && !OwnedBy(iso, w->iso)) {
    SetErr("Eternal belongs to a different isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::Data> value = FromWire<v8::Data>(value_wire);
    v8__Eternal__Set(w->storage, iso,
                     &gov8_snap_impl::DerefLocal(value));
    if (w->iso == nullptr) w->iso = iso;
    return kOk;
  } catch (...) {
    SetErr("C++ exception setting Eternal");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_hr_eternal_get(v8::Isolate* iso,
                                                   void* scope,
                                                   void* handle,
                                                   void** out) {
  ClearErr();
  EternalWrap* w = AsEternal(handle);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid Eternal.Get argument");
    return kErrBadArg;
  }
  *out = nullptr;
  if (w->iso == nullptr) return kOk;
  if (!OwnedBy(iso, w->iso)) {
    SetErr("Eternal belongs to a different isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    *out = const_cast<v8::Data*>(v8__Eternal__Get(w->storage, iso));
    return kOk;
  } catch (...) {
    SetErr("C++ exception getting Eternal");
    return kErrCpp;
  }
}

// Eternal::Clear only zeros the opaque Eternal slot. The pinned subprocess
// oracle proves this is safe even for a non-empty handle after isolate drop.
__declspec(dllexport) int64_t gov8_hr_eternal_clear(void* handle) {
  ClearErr();
  EternalWrap* w = AsEternal(handle);
  if (w == nullptr) {
    SetErr("invalid Eternal handle");
    return kErrMagic;
  }
  try {
    v8__Eternal__Clear(w->storage);
    return kOk;
  } catch (...) {
    SetErr("C++ exception clearing Eternal");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_hr_eternal_dispose(void* handle) {
  ClearErr();
  EternalWrap* w = AsEternal(handle);
  if (w == nullptr) {
    SetErr("invalid Eternal handle");
    return kErrMagic;
  }
  try {
    v8__Eternal__DESTRUCT(w->storage);
    w->magic = 0;
    delete w;
    return kOk;
  } catch (...) {
    SetErr("C++ exception destroying Eternal");
    return kErrCpp;
  }
}

__declspec(dllexport) void* gov8_hr_traced_empty() {
  ClearErr();
  try {
    TracedWrap* w = new TracedWrap{};
    w->magic = kTracedMagic;
    v8__TracedReference__CONSTRUCT(w->storage);
    return w;
  } catch (...) {
    SetErr("C++ exception constructing TracedReference");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_hr_traced_reset(v8::Isolate* iso,
                                                   void* scope,
                                                   void* handle,
                                                   void* value_wire) {
  ClearErr();
  TracedWrap* w = AsTraced(handle);
  if (w == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid TracedReference.Reset argument");
    return kErrBadArg;
  }
  if (w->iso != nullptr && !OwnedBy(iso, w->iso)) {
    SetErr("TracedReference belongs to a different isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    const v8::Data* value = nullptr;
    if (value_wire != nullptr) {
      value = &gov8_snap_impl::DerefLocal(
          FromWire<v8::Data>(value_wire));
    }
    v8__TracedReference__Reset(w->storage, iso, value);
    if (value != nullptr && w->iso == nullptr) w->iso = iso;
    return kOk;
  } catch (...) {
    SetErr("C++ exception resetting TracedReference");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_hr_traced_get(v8::Isolate* iso,
                                                 void* scope,
                                                 void* handle,
                                                 void** out) {
  ClearErr();
  TracedWrap* w = AsTraced(handle);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid TracedReference.Get argument");
    return kErrBadArg;
  }
  *out = nullptr;
  if (w->iso == nullptr) return kOk;
  if (!OwnedBy(iso, w->iso)) {
    SetErr("TracedReference belongs to a different isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    *out = const_cast<v8::Data*>(v8__TracedReference__Get(
        reinterpret_cast<v8::TracedReference<v8::Data>*>(w->storage), iso));
    return kOk;
  } catch (...) {
    SetErr("C++ exception getting TracedReference");
    return kErrCpp;
  }
}

// TracedReference's destructor intentionally does not Reset: the pinned type
// requires an external traced owner to manage reachability, and its destructor
// is proven safe after isolate disposal even while non-empty.
__declspec(dllexport) int64_t gov8_hr_traced_dispose(void* handle) {
  ClearErr();
  TracedWrap* w = AsTraced(handle);
  if (w == nullptr) {
    SetErr("invalid TracedReference handle");
    return kErrMagic;
  }
  try {
    v8__TracedReference__DESTRUCT(w->storage);
    w->magic = 0;
    delete w;
    return kOk;
  } catch (...) {
    SetErr("C++ exception destroying TracedReference");
    return kErrCpp;
  }
}

}  // extern "C"
