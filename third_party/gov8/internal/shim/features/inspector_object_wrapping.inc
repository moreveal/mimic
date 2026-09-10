// Inspector Runtime.RemoteObject wrapping for pinned V8 15.2. Include after
// inspector_transport.inc: this feature reuses SessionWrap/AsSession,
// InspectorWrap, View, AsCtx, ScopeIs, FromWire, and ToWire. RemoteWrap owns
// protocol data only and intentionally has no V8/isolate lifetime dependency.

#include "v8-inspector.h"
#include <cstdint>
#include <cstring>
#include <memory>
#include <new>
#include <vector>

extern "C" {
v8_inspector::protocol::Runtime::API::RemoteObject*
v8_inspector__V8InspectorSession__wrapObject(
    v8_inspector::V8InspectorSession*, const v8::Context*, const v8::Value*,
    v8_inspector::StringView, bool);
bool v8_inspector__V8InspectorSession__unwrapObject(
    v8_inspector::V8InspectorSession*, v8_inspector::StringBuffer**,
    v8_inspector::StringView, const v8::Value**, const v8::Context**,
    v8_inspector::StringBuffer**);
void v8_inspector__RemoteObject__DELETE(
    v8_inspector::protocol::Runtime::API::RemoteObject*);
std::vector<uint8_t>* v8_inspector__RemoteObject__toBytes(
    const v8_inspector::protocol::Runtime::API::RemoteObject*);
void crdtp__vec_u8__DELETE(std::vector<uint8_t>*);
bool crdtp__json__ConvertCBORToJSON(const uint8_t*, size_t,
                                    std::vector<uint8_t>*);
}

namespace gov8_inspector_object_wrapping_impl {

constexpr uint64_t kRemoteMagic = 0x674f563852454d4fULL;
constexpr uint64_t kStringMagic = 0x674f5638494f5753ULL;

using RawRemoteObject =
    v8_inspector::protocol::Runtime::API::RemoteObject;
using RemoteOwner =
    std::unique_ptr<RawRemoteObject,
                    decltype(&v8_inspector__RemoteObject__DELETE)>;
using StringOwner =
    std::unique_ptr<v8_inspector::StringBuffer,
                    decltype(&v8_inspector__StringBuffer__DELETE)>;

struct RemoteWrap {
  uint64_t magic = kRemoteMagic;
  RawRemoteObject* value = nullptr;
};

struct StringWrap {
  uint64_t magic = kStringMagic;
  v8_inspector::StringBuffer* value = nullptr;
};

RemoteWrap* AsRemote(void* raw) {
  auto* value = static_cast<RemoteWrap*>(raw);
  return value != nullptr && value->magic == kRemoteMagic ? value : nullptr;
}

StringWrap* AsString(void* raw) {
  auto* value = static_cast<StringWrap*>(raw);
  return value != nullptr && value->magic == kStringMagic ? value : nullptr;
}

template <class T>
v8::Local<T> ObjectPointerToLocal(const T* const& pointer) {
  static_assert(sizeof(v8::Local<T>) == sizeof(pointer),
                "Local/object pointer ABI mismatch");
  // rusty_v8's pinned flat bridge represents Local<T> as its object pointer.
  // Immediately materializing Local::New copies that handle into the active
  // caller-owned GoScope before this ABI call returns.
  return *reinterpret_cast<const v8::Local<T>*>(&pointer);
}

StringWrap* OwnString(StringOwner value) {
  if (!value) return nullptr;
  auto result = std::make_unique<StringWrap>();
  result->value = value.release();
  return result.release();
}

}  // namespace gov8_inspector_object_wrapping_impl

extern "C" {

__declspec(dllexport) int64_t gov8_iow_wrap_object(
    void* session_raw, v8::Isolate* isolate, void* scope,
    void* context_raw, void* value_wire, int32_t group_is8,
    const void* group_data, size_t group_len, int32_t generate_preview,
    void** out_remote) {
  using namespace gov8_inspector_impl;
  using namespace gov8_inspector_object_wrapping_impl;
  ClearErr();
  if (out_remote != nullptr) *out_remote = nullptr;
  auto* session = AsSession(session_raw);
  auto* context = AsCtx(context_raw);
  if (session == nullptr || isolate == nullptr || out_remote == nullptr ||
      !ScopeIs(isolate, scope) || session->owner->iso != isolate ||
      context == nullptr || context->iso != isolate || value_wire == nullptr ||
      (group_is8 != 0 && group_is8 != 1) ||
      (group_data == nullptr && group_len != 0) ||
      (generate_preview != 0 && generate_preview != 1)) {
    SetErr("invalid Inspector WrapObject argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    // Do not open a temporary HandleScope. Inspector-created locals must be
    // allocated in the caller's already-active GoScope.
    const v8::Local<v8::Context> local_context = context->ctx->Get(isolate);
    const v8::Local<v8::Value> local_value = FromWire<v8::Value>(value_wire);
    RemoteOwner raw(v8_inspector__V8InspectorSession__wrapObject(
                        session->value, local_context.operator->(),
                        local_value.operator->(),
                        View(group_is8, group_data, group_len),
                        generate_preview != 0),
                    v8_inspector__RemoteObject__DELETE);
    if (!raw) return kOk;
    auto wrapped = std::make_unique<RemoteWrap>();
    wrapped->value = raw.release();
    *out_remote = wrapped.release();
    return kOk;
  } catch (const std::bad_alloc&) {
    SetErr("out of memory wrapping Inspector object");
    return kErrNoMemory;
  } catch (...) {
    SetErr("C++ exception wrapping Inspector object");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_iow_unwrap_object(
    void* session_raw, v8::Isolate* isolate, void* scope,
    int32_t id_is8, const void* id_data, size_t id_len,
    void* const* context_candidates, size_t context_candidate_count,
    void** out_value, size_t* out_context_index, void** out_group,
    void** out_error) {
  using namespace gov8_inspector_impl;
  using namespace gov8_inspector_object_wrapping_impl;
  ClearErr();
  if (out_value != nullptr) *out_value = nullptr;
  if (out_context_index != nullptr) *out_context_index = SIZE_MAX;
  if (out_group != nullptr) *out_group = nullptr;
  if (out_error != nullptr) *out_error = nullptr;
  auto* session = AsSession(session_raw);
  if (session == nullptr || isolate == nullptr || !ScopeIs(isolate, scope) ||
      session->owner->iso != isolate ||
      (id_is8 != 0 && id_is8 != 1) ||
      (id_data == nullptr && id_len != 0) ||
      (context_candidates == nullptr && context_candidate_count != 0) ||
      out_value == nullptr || out_context_index == nullptr ||
      out_group == nullptr || out_error == nullptr) {
    SetErr("invalid Inspector UnwrapObject argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    // Keep the caller's GoScope as the active HandleScope. The flat binding
    // returns object pointers; copy them into fresh caller-scope Local slots.
    v8_inspector::StringBuffer* error = nullptr;
    const v8::Value* raw_value = nullptr;
    const v8::Context* raw_context = nullptr;
    v8_inspector::StringBuffer* group = nullptr;
    const bool success = v8_inspector__V8InspectorSession__unwrapObject(
        session->value, &error, View(id_is8, id_data, id_len), &raw_value,
        &raw_context, &group);
    StringOwner error_owner(error, v8_inspector__StringBuffer__DELETE);
    StringOwner group_owner(group, v8_inspector__StringBuffer__DELETE);
    if (!success) {
      if (error == nullptr) {
        SetErr("Inspector UnwrapObject failed without an error buffer");
        return kErrState;
      }
      *out_error = OwnString(std::move(error_owner));
      return kOk;
    }
    if (error != nullptr || raw_value == nullptr || raw_context == nullptr ||
        group == nullptr) {
      SetErr("Inspector UnwrapObject returned an incomplete result");
      return kErrState;
    }

    const v8::Local<v8::Value> local_value = v8::Local<v8::Value>::New(
        isolate, ObjectPointerToLocal(raw_value));
    const v8::Local<v8::Context> local_context = v8::Local<v8::Context>::New(
        isolate, ObjectPointerToLocal(raw_context));
    size_t match = SIZE_MAX;
    for (size_t index = 0; index < context_candidate_count; ++index) {
      auto* candidate = AsCtx(context_candidates[index]);
      if (candidate == nullptr || candidate->iso != isolate) {
        SetErr("invalid registered Inspector context candidate");
        return kErrBadArg;
      }
      if (candidate->ctx->Get(isolate) == local_context) {
        match = index;
        break;
      }
    }
    if (match == SIZE_MAX) {
      SetErr("Inspector UnwrapObject returned an unregistered context");
      return kErrState;
    }

    *out_value = ToWire(local_value);
    *out_context_index = match;
    *out_group = OwnString(std::move(group_owner));
    return kOk;
  } catch (const std::bad_alloc&) {
    SetErr("out of memory unwrapping Inspector object");
    return kErrNoMemory;
  } catch (...) {
    SetErr("C++ exception unwrapping Inspector object");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_iow_string_info(
    void* string_raw, int32_t* out_is8, size_t* out_len) {
  using namespace gov8_inspector_object_wrapping_impl;
  ClearErr();
  if (out_is8 != nullptr) *out_is8 = 0;
  if (out_len != nullptr) *out_len = 0;
  auto* string = AsString(string_raw);
  if (string == nullptr || out_is8 == nullptr || out_len == nullptr ||
      string->value == nullptr) {
    SetErr("invalid Inspector string buffer");
    return kErrBadArg;
  }
  const auto view = string->value->string();
  *out_is8 = view.is8Bit() ? 1 : 0;
  *out_len = view.length();
  return kOk;
}

__declspec(dllexport) int64_t gov8_iow_string_copy(
    void* string_raw, void* out_data, size_t capacity) {
  using namespace gov8_inspector_object_wrapping_impl;
  ClearErr();
  auto* string = AsString(string_raw);
  if (string == nullptr || string->value == nullptr) {
    SetErr("invalid Inspector string buffer");
    return kErrBadArg;
  }
  const auto view = string->value->string();
  const size_t bytes = view.length() * (view.is8Bit() ? 1 : 2);
  if (capacity < bytes || (out_data == nullptr && bytes != 0)) {
    SetErr("Inspector string destination is too small");
    return kErrNoMemory;
  }
  if (bytes != 0) {
    std::memcpy(out_data,
                view.is8Bit() ? static_cast<const void*>(view.characters8())
                              : static_cast<const void*>(view.characters16()),
                bytes);
  }
  return kOk;
}

__declspec(dllexport) int64_t gov8_iow_string_delete(void* string_raw) {
  using namespace gov8_inspector_object_wrapping_impl;
  ClearErr();
  auto* string = AsString(string_raw);
  if (string == nullptr) {
    SetErr("invalid Inspector string buffer");
    return kErrMagic;
  }
  if (string->value != nullptr) {
    v8_inspector__StringBuffer__DELETE(string->value);
  }
  string->magic = 0;
  string->value = nullptr;
  delete string;
  return kOk;
}

__declspec(dllexport) int64_t gov8_iow_remote_to_bytes(
    void* remote_raw, void* out_data, size_t capacity, size_t* out_len) {
  using namespace gov8_inspector_object_wrapping_impl;
  ClearErr();
  if (out_len != nullptr) *out_len = 0;
  auto* remote = AsRemote(remote_raw);
  if (remote == nullptr || remote->value == nullptr || out_len == nullptr) {
    SetErr("invalid Inspector remote object");
    return kErrBadArg;
  }
  try {
    std::unique_ptr<std::vector<uint8_t>,
                    decltype(&crdtp__vec_u8__DELETE)>
        bytes(v8_inspector__RemoteObject__toBytes(remote->value),
              crdtp__vec_u8__DELETE);
    if (!bytes) {
      SetErr("Inspector remote serialization returned null");
      return kErrState;
    }
    *out_len = bytes->size();
    if (capacity < bytes->size() ||
        (out_data == nullptr && !bytes->empty())) {
      return kErrNoMemory;
    }
    if (!bytes->empty()) std::memcpy(out_data, bytes->data(), bytes->size());
    return kOk;
  } catch (const std::bad_alloc&) {
    SetErr("out of memory serializing Inspector remote object");
    return kErrNoMemory;
  } catch (...) {
    SetErr("C++ exception serializing Inspector remote object");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_iow_remote_delete(void* remote_raw) {
  using namespace gov8_inspector_object_wrapping_impl;
  ClearErr();
  auto* remote = AsRemote(remote_raw);
  if (remote == nullptr) {
    SetErr("invalid Inspector remote object");
    return kErrMagic;
  }
  v8_inspector__RemoteObject__DELETE(remote->value);
  remote->magic = 0;
  remote->value = nullptr;
  delete remote;
  return kOk;
}

// Conformance-only bridge mirroring rusty_v8::crdtp::cbor_to_json.
__declspec(dllexport) int64_t gov8_iow_test_cbor_to_json(
    const uint8_t* cbor, size_t cbor_len, void* out_data, size_t capacity,
    size_t* out_len) {
  ClearErr();
  if (out_len != nullptr) *out_len = 0;
  if ((cbor == nullptr && cbor_len != 0) || out_len == nullptr) return kErrBadArg;
  try {
    std::vector<uint8_t> json;
    if (!crdtp__json__ConvertCBORToJSON(cbor, cbor_len, &json)) {
      SetErr("invalid CRDTP CBOR");
      return kErrBadArg;
    }
    *out_len = json.size();
    if (capacity < json.size() || (out_data == nullptr && !json.empty())) {
      return kErrNoMemory;
    }
    if (!json.empty()) std::memcpy(out_data, json.data(), json.size());
    return kOk;
  } catch (const std::bad_alloc&) {
    SetErr("out of memory converting CRDTP CBOR");
    return kErrNoMemory;
  } catch (...) {
    SetErr("C++ exception converting CRDTP CBOR");
    return kErrCpp;
  }
}

}  // extern "C"
