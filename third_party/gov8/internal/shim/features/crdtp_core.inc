// Isolate-free CRDTP conversion, parsed-message, response, and Serializable
// bindings for rusty_v8 152.2.0. This include deliberately excludes the
// FrontendChannel/UberDispatcher/DomainDispatcher callback layer.

#include <cstdint>
#include <cstring>
#include <memory>
#include <new>
#include <string>
#include <vector>

namespace v8_crdtp {
class Dispatchable;
class Serializable;
}  // namespace v8_crdtp

struct DispatchResponseWrapper;

extern "C" {
void crdtp__Serializable__DELETE(v8_crdtp::Serializable*);
void crdtp__Serializable__AppendSerialized(
    const v8_crdtp::Serializable*, std::vector<uint8_t>*);
v8_crdtp::Dispatchable* crdtp__Dispatchable__new(
    const uint8_t*, size_t, const uint8_t*, size_t, void*);
void crdtp__Dispatchable__DELETE(v8_crdtp::Dispatchable*);
bool crdtp__Dispatchable__ok(const v8_crdtp::Dispatchable*);
int32_t crdtp__Dispatchable__callId(const v8_crdtp::Dispatchable*);
bool crdtp__Dispatchable__hasCallId(const v8_crdtp::Dispatchable*);
size_t crdtp__Dispatchable__methodLen(const v8_crdtp::Dispatchable*);
void crdtp__Dispatchable__methodCopy(const v8_crdtp::Dispatchable*, uint8_t*);
size_t crdtp__Dispatchable__sessionIdLen(const v8_crdtp::Dispatchable*);
void crdtp__Dispatchable__sessionIdCopy(const v8_crdtp::Dispatchable*,
                                        uint8_t*);
size_t crdtp__Dispatchable__paramsLen(const v8_crdtp::Dispatchable*);
void crdtp__Dispatchable__paramsCopy(const v8_crdtp::Dispatchable*, uint8_t*);
size_t crdtp__Dispatchable__associatedDataLen(
    const v8_crdtp::Dispatchable*);
void crdtp__Dispatchable__associatedDataCopy(
    const v8_crdtp::Dispatchable*, uint8_t*);

DispatchResponseWrapper* crdtp__DispatchResponse__Success();
DispatchResponseWrapper* crdtp__DispatchResponse__FallThrough();
DispatchResponseWrapper* crdtp__DispatchResponse__ParseError(const char*,
                                                              size_t);
DispatchResponseWrapper* crdtp__DispatchResponse__InvalidRequest(const char*,
                                                                 size_t);
DispatchResponseWrapper* crdtp__DispatchResponse__MethodNotFound(const char*,
                                                                 size_t);
DispatchResponseWrapper* crdtp__DispatchResponse__InvalidParams(const char*,
                                                                size_t);
DispatchResponseWrapper* crdtp__DispatchResponse__ServerError(const char*,
                                                              size_t);
void crdtp__DispatchResponse__DELETE(DispatchResponseWrapper*);
bool crdtp__DispatchResponse__isSuccess(const DispatchResponseWrapper*);
bool crdtp__DispatchResponse__isError(const DispatchResponseWrapper*);
bool crdtp__DispatchResponse__isFallThrough(const DispatchResponseWrapper*);
int crdtp__DispatchResponse__code(const DispatchResponseWrapper*);
size_t crdtp__DispatchResponse__messageLen(const DispatchResponseWrapper*);
void crdtp__DispatchResponse__messageCopy(const DispatchResponseWrapper*,
                                          char*);

std::vector<uint8_t>* crdtp__vec_u8__new();
void crdtp__vec_u8__DELETE(std::vector<uint8_t>*);
size_t crdtp__vec_u8__size(const std::vector<uint8_t>*);
void crdtp__vec_u8__copy(const std::vector<uint8_t>*, uint8_t*);
bool crdtp__json__ConvertJSONToCBOR(const uint8_t*, size_t,
                                    std::vector<uint8_t>*);
bool crdtp__json__ConvertCBORToJSON(const uint8_t*, size_t,
                                    std::vector<uint8_t>*);

v8_crdtp::Serializable* crdtp__CreateErrorResponse(
    int, DispatchResponseWrapper*);
v8_crdtp::Serializable* crdtp__CreateResponse(int,
                                              v8_crdtp::Serializable*);
v8_crdtp::Serializable* crdtp__CreateNotification(
    const char*, v8_crdtp::Serializable*);
v8_crdtp::Serializable* crdtp__CreateErrorNotification(
    DispatchResponseWrapper*);
}

namespace gov8_crdtp_core_impl {

constexpr uint64_t kBytesMagic = 0x674f563843425954ULL;
constexpr uint64_t kDispatchableMagic = 0x674f563843445350ULL;
constexpr uint64_t kResponseMagic = 0x674f563843525350ULL;
constexpr uint64_t kSerializableMagic = 0x674f563843534552ULL;
const uint8_t kEmptyByte = 0;

struct BytesWrap {
  uint64_t magic = kBytesMagic;
  std::vector<uint8_t>* value = nullptr;

  ~BytesWrap() {
    if (value != nullptr) {
      crdtp__vec_u8__DELETE(value);
    }
  }
};

struct DispatchableWrap {
  uint64_t magic = kDispatchableMagic;
  std::vector<uint8_t> cbor;
  std::vector<uint8_t> associated;
  v8_crdtp::Dispatchable* value = nullptr;

  ~DispatchableWrap() {
    if (value != nullptr) {
      crdtp__Dispatchable__DELETE(value);
    }
  }
};

struct ResponseWrap {
  uint64_t magic = kResponseMagic;
  DispatchResponseWrapper* value = nullptr;

  ~ResponseWrap() {
    if (value != nullptr) {
      crdtp__DispatchResponse__DELETE(value);
    }
  }
};

struct SerializableWrap {
  uint64_t magic = kSerializableMagic;
  v8_crdtp::Serializable* value = nullptr;
  std::vector<uint8_t>* serialized = nullptr;

  ~SerializableWrap() {
    if (serialized != nullptr) {
      crdtp__vec_u8__DELETE(serialized);
    }
    if (value != nullptr) {
      crdtp__Serializable__DELETE(value);
    }
  }
};

BytesWrap* AsBytes(void* raw) {
  auto* value = static_cast<BytesWrap*>(raw);
  return value != nullptr && value->magic == kBytesMagic ? value : nullptr;
}

DispatchableWrap* AsDispatchable(void* raw) {
  auto* value = static_cast<DispatchableWrap*>(raw);
  return value != nullptr && value->magic == kDispatchableMagic ? value
                                                                : nullptr;
}

ResponseWrap* AsResponse(void* raw) {
  auto* value = static_cast<ResponseWrap*>(raw);
  return value != nullptr && value->magic == kResponseMagic ? value : nullptr;
}

SerializableWrap* AsSerializable(void* raw) {
  auto* value = static_cast<SerializableWrap*>(raw);
  return value != nullptr && value->magic == kSerializableMagic ? value
                                                                : nullptr;
}

const uint8_t* StableData(const std::vector<uint8_t>& value) {
  return value.empty() ? &kEmptyByte : value.data();
}

std::vector<uint8_t> CopyInput(const uint8_t* data, size_t len) {
  if (len == 0) {
    return {};
  }
  return std::vector<uint8_t>(data, data + len);
}

BytesWrap* WrapBytes(std::vector<uint8_t>* value) {
  if (value == nullptr) {
    return nullptr;
  }
  std::unique_ptr<std::vector<uint8_t>,
                  decltype(&crdtp__vec_u8__DELETE)>
      owner(value, crdtp__vec_u8__DELETE);
  auto wrap = std::make_unique<BytesWrap>();
  wrap->value = owner.release();
  return wrap.release();
}

SerializableWrap* WrapSerializable(v8_crdtp::Serializable* value) {
  if (value == nullptr) {
    return nullptr;
  }
  std::unique_ptr<v8_crdtp::Serializable,
                  decltype(&crdtp__Serializable__DELETE)>
      owner(value, crdtp__Serializable__DELETE);
  auto wrap = std::make_unique<SerializableWrap>();
  wrap->value = owner.release();
  return wrap.release();
}

int64_t SerializableView(SerializableWrap* wrap, const uint8_t** out,
                         size_t* out_len) {
  if (wrap == nullptr || wrap->value == nullptr || out == nullptr ||
      out_len == nullptr) {
    return kErrBadArg;
  }
  *out = nullptr;
  *out_len = 0;
  try {
    if (wrap->serialized == nullptr) {
      auto bytes = std::unique_ptr<std::vector<uint8_t>,
                                   decltype(&crdtp__vec_u8__DELETE)>(
          crdtp__vec_u8__new(), crdtp__vec_u8__DELETE);
      if (!bytes) {
        SetErr("CRDTP Serializable byte allocation failed");
        return kErrCpp;
      }
      crdtp__Serializable__AppendSerialized(wrap->value, bytes.get());
      wrap->serialized = bytes.release();
    }
    *out = StableData(*wrap->serialized);
    *out_len = wrap->serialized->size();
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

int64_t ConvertSized(bool json_to_cbor, const uint8_t* data, size_t len,
                     void** out, int64_t* out_len) {
  if (out == nullptr || out_len == nullptr ||
      (data == nullptr && len != 0)) {
    return kErrBadArg;
  }
  *out = nullptr;
  *out_len = 0;
  try {
    auto bytes = std::unique_ptr<std::vector<uint8_t>,
                                 decltype(&crdtp__vec_u8__DELETE)>(
        crdtp__vec_u8__new(), crdtp__vec_u8__DELETE);
    if (!bytes) {
      SetErr("CRDTP byte allocation failed");
      return kErrCpp;
    }
    const uint8_t* input =
        len == 0 ? &gov8_crdtp_core_impl::kEmptyByte : data;
    const bool ok = json_to_cbor
                        ? crdtp__json__ConvertJSONToCBOR(input, len,
                                                        bytes.get())
                        : crdtp__json__ConvertCBORToJSON(input, len,
                                                        bytes.get());
    if (!ok) {
      return 0;
    }
    if (bytes->size() > static_cast<size_t>(INT64_MAX)) {
      SetErr("CRDTP converted bytes exceed int64");
      return kErrBadArg;
    }
    *out_len = static_cast<int64_t>(bytes->size());
    *out = gov8_crdtp_core_impl::WrapBytes(bytes.release());
    return *out != nullptr ? 1 : kErrCpp;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

}  // namespace gov8_crdtp_core_impl

extern "C" {

__declspec(dllexport) int64_t gov8_crdtp_bytes_copy(void* raw, uint8_t* out,
                                                    int64_t cap) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsBytes(raw);
  if (wrap == nullptr || wrap->value == nullptr) {
    SetErr("invalid CRDTP byte buffer");
    return kErrMagic;
  }
  if (cap < 0) {
    return kErrBadArg;
  }
  const size_t len = wrap->value->size();
  if (len > static_cast<size_t>(INT64_MAX)) {
    SetErr("CRDTP byte buffer exceeds int64");
    return kErrBadArg;
  }
  if (out == nullptr) {
    return static_cast<int64_t>(len);
  }
  if (static_cast<uint64_t>(cap) < len) {
    SetErr("CRDTP destination is too small");
    return kErrNoMemory;
  }
  if (len != 0) {
    std::memcpy(out, wrap->value->data(), len);
  }
  return static_cast<int64_t>(len);
}

// Copies and consumes a BytesWrap in one boundary crossing. On failure the
// wrapper remains owned by the caller so its ordinary delete path is safe.
__declspec(dllexport) int64_t gov8_crdtp_bytes_copy_delete(void* raw,
                                                           uint8_t* out,
                                                           int64_t cap) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsBytes(raw);
  if (wrap == nullptr || wrap->value == nullptr) {
    SetErr("invalid CRDTP byte buffer");
    return kErrMagic;
  }
  if (cap < 0) {
    return kErrBadArg;
  }
  const size_t len = wrap->value->size();
  if (len > static_cast<size_t>(INT64_MAX)) {
    SetErr("CRDTP byte buffer exceeds int64");
    return kErrBadArg;
  }
  if (static_cast<uint64_t>(cap) < len || (out == nullptr && len != 0)) {
    SetErr("CRDTP destination is too small");
    return kErrNoMemory;
  }
  if (len != 0) {
    std::memcpy(out, wrap->value->data(), len);
  }
  wrap->magic = 0;
  delete wrap;
  return static_cast<int64_t>(len);
}

__declspec(dllexport) int64_t gov8_crdtp_bytes_delete(void* raw) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsBytes(raw);
  if (wrap == nullptr) {
    SetErr("invalid CRDTP byte buffer");
    return kErrMagic;
  }
  wrap->magic = 0;
  delete wrap;
  return kOk;
}

__declspec(dllexport) int64_t gov8_crdtp_json_to_cbor(
    const uint8_t* data, size_t len, void** out) {
  ClearErr();
  int64_t ignored = 0;
  return gov8_crdtp_core_impl::ConvertSized(true, data, len, out, &ignored);
}

__declspec(dllexport) int64_t gov8_crdtp_cbor_to_json(
    const uint8_t* data, size_t len, void** out) {
  ClearErr();
  int64_t ignored = 0;
  return gov8_crdtp_core_impl::ConvertSized(false, data, len, out, &ignored);
}

__declspec(dllexport) int64_t gov8_crdtp_json_to_cbor_sized(
    const uint8_t* data, size_t len, void** out, int64_t* out_len) {
  ClearErr();
  return gov8_crdtp_core_impl::ConvertSized(true, data, len, out, out_len);
}

__declspec(dllexport) int64_t gov8_crdtp_cbor_to_json_sized(
    const uint8_t* data, size_t len, void** out, int64_t* out_len) {
  ClearErr();
  return gov8_crdtp_core_impl::ConvertSized(false, data, len, out, out_len);
}

__declspec(dllexport) int64_t gov8_crdtp_dispatchable_new(
    const uint8_t* data, size_t len, const uint8_t* associated,
    size_t associated_len, void** out) {
  ClearErr();
  if (out == nullptr || (data == nullptr && len != 0) ||
      (associated == nullptr && associated_len != 0)) {
    return kErrBadArg;
  }
  *out = nullptr;
  try {
    auto wrap = std::make_unique<gov8_crdtp_core_impl::DispatchableWrap>();
    wrap->cbor = gov8_crdtp_core_impl::CopyInput(data, len);
    wrap->associated =
        gov8_crdtp_core_impl::CopyInput(associated, associated_len);
    wrap->value = crdtp__Dispatchable__new(
        gov8_crdtp_core_impl::StableData(wrap->cbor), wrap->cbor.size(),
        gov8_crdtp_core_impl::StableData(wrap->associated),
        wrap->associated.size(), nullptr);
    if (wrap->value == nullptr) {
      SetErr("creating CRDTP Dispatchable failed");
      return kErrCpp;
    }
    *out = wrap.release();
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_dispatchable_ok(void* raw,
                                                         int32_t* out) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsDispatchable(raw);
  if (wrap == nullptr || wrap->value == nullptr || out == nullptr) {
    return kErrBadArg;
  }
  *out = crdtp__Dispatchable__ok(wrap->value) ? 1 : 0;
  return kOk;
}

__declspec(dllexport) int64_t gov8_crdtp_dispatchable_call_id(
    void* raw, int32_t* has_call_id, int32_t* call_id) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsDispatchable(raw);
  if (wrap == nullptr || wrap->value == nullptr || has_call_id == nullptr ||
      call_id == nullptr) {
    return kErrBadArg;
  }
  *has_call_id = 0;
  *call_id = 0;
  if (!crdtp__Dispatchable__ok(wrap->value)) {
    SetErr("CRDTP Dispatchable is invalid");
    return kErrBadArg;
  }
  *has_call_id = crdtp__Dispatchable__hasCallId(wrap->value) ? 1 : 0;
  *call_id = crdtp__Dispatchable__callId(wrap->value);
  return kOk;
}

__declspec(dllexport) int64_t gov8_crdtp_dispatchable_bytes(
    void* raw, int32_t kind, void** out) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsDispatchable(raw);
  if (wrap == nullptr || wrap->value == nullptr || out == nullptr) {
    return kErrBadArg;
  }
  *out = nullptr;
  if (!crdtp__Dispatchable__ok(wrap->value)) {
    SetErr("CRDTP Dispatchable is invalid");
    return kErrBadArg;
  }
  try {
    size_t len = 0;
    switch (kind) {
      case 0:
        len = crdtp__Dispatchable__methodLen(wrap->value);
        break;
      case 1:
        len = crdtp__Dispatchable__sessionIdLen(wrap->value);
        break;
      case 2:
        len = crdtp__Dispatchable__paramsLen(wrap->value);
        break;
      case 3:
        len = crdtp__Dispatchable__associatedDataLen(wrap->value);
        break;
      default:
        return kErrBadArg;
    }
    auto bytes = std::unique_ptr<std::vector<uint8_t>>(
        new std::vector<uint8_t>(len));
    if (len != 0) {
      switch (kind) {
        case 0:
          crdtp__Dispatchable__methodCopy(wrap->value, bytes->data());
          break;
        case 1:
          crdtp__Dispatchable__sessionIdCopy(wrap->value, bytes->data());
          break;
        case 2:
          crdtp__Dispatchable__paramsCopy(wrap->value, bytes->data());
          break;
        case 3:
          crdtp__Dispatchable__associatedDataCopy(wrap->value,
                                                  bytes->data());
          break;
      }
    }
    *out = gov8_crdtp_core_impl::WrapBytes(bytes.release());
    return *out != nullptr ? kOk : kErrCpp;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_dispatchable_delete(void* raw) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsDispatchable(raw);
  if (wrap == nullptr) {
    SetErr("invalid CRDTP Dispatchable");
    return kErrMagic;
  }
  wrap->magic = 0;
  delete wrap;
  return kOk;
}

__declspec(dllexport) int64_t gov8_crdtp_response_new(
    int32_t kind, const uint8_t* message, size_t message_len, void** out) {
  ClearErr();
  if (out == nullptr || (message == nullptr && message_len != 0)) {
    return kErrBadArg;
  }
  *out = nullptr;
  try {
    const char* text = reinterpret_cast<const char*>(
        message_len == 0 ? &gov8_crdtp_core_impl::kEmptyByte : message);
    DispatchResponseWrapper* value = nullptr;
    switch (kind) {
      case 0:
        value = crdtp__DispatchResponse__Success();
        break;
      case 1:
        value = crdtp__DispatchResponse__FallThrough();
        break;
      case 2:
        value = crdtp__DispatchResponse__ParseError(text, message_len);
        break;
      case 3:
        value = crdtp__DispatchResponse__InvalidRequest(text, message_len);
        break;
      case 4:
        value = crdtp__DispatchResponse__MethodNotFound(text, message_len);
        break;
      case 5:
        value = crdtp__DispatchResponse__InvalidParams(text, message_len);
        break;
      case 6:
        value = crdtp__DispatchResponse__ServerError(text, message_len);
        break;
      default:
        return kErrBadArg;
    }
    std::unique_ptr<DispatchResponseWrapper,
                    decltype(&crdtp__DispatchResponse__DELETE)>
        owner(value, crdtp__DispatchResponse__DELETE);
    if (!owner) {
      SetErr("creating CRDTP DispatchResponse failed");
      return kErrCpp;
    }
    auto wrap = std::make_unique<gov8_crdtp_core_impl::ResponseWrap>();
    wrap->value = owner.release();
    *out = wrap.release();
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_response_query(
    void* raw, int32_t kind, int32_t* out) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsResponse(raw);
  if (wrap == nullptr || wrap->value == nullptr || out == nullptr) {
    return kErrBadArg;
  }
  switch (kind) {
    case 0:
      *out = crdtp__DispatchResponse__isSuccess(wrap->value) ? 1 : 0;
      break;
    case 1:
      *out = crdtp__DispatchResponse__isError(wrap->value) ? 1 : 0;
      break;
    case 2:
      *out = crdtp__DispatchResponse__isFallThrough(wrap->value) ? 1 : 0;
      break;
    case 3:
      *out = crdtp__DispatchResponse__code(wrap->value);
      break;
    default:
      return kErrBadArg;
  }
  return kOk;
}

__declspec(dllexport) int64_t gov8_crdtp_response_message(void* raw,
                                                          void** out) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsResponse(raw);
  if (wrap == nullptr || wrap->value == nullptr || out == nullptr) {
    return kErrBadArg;
  }
  *out = nullptr;
  try {
    const size_t len = crdtp__DispatchResponse__messageLen(wrap->value);
    auto bytes = std::make_unique<std::vector<uint8_t>>(len);
    if (len != 0) {
      crdtp__DispatchResponse__messageCopy(
          wrap->value, reinterpret_cast<char*>(bytes->data()));
    }
    *out = gov8_crdtp_core_impl::WrapBytes(bytes.release());
    return *out != nullptr ? kOk : kErrCpp;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_response_delete(void* raw) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsResponse(raw);
  if (wrap == nullptr) {
    SetErr("invalid CRDTP DispatchResponse");
    return kErrMagic;
  }
  wrap->magic = 0;
  delete wrap;
  return kOk;
}

__declspec(dllexport) int64_t gov8_crdtp_serializable_bytes(void* raw,
                                                            void** out) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsSerializable(raw);
  if (wrap == nullptr || wrap->value == nullptr || out == nullptr) {
    return kErrBadArg;
  }
  *out = nullptr;
  try {
    auto bytes = std::unique_ptr<std::vector<uint8_t>,
                                 decltype(&crdtp__vec_u8__DELETE)>(
        crdtp__vec_u8__new(), crdtp__vec_u8__DELETE);
    if (!bytes) {
      return kErrCpp;
    }
    crdtp__Serializable__AppendSerialized(wrap->value, bytes.get());
    *out = gov8_crdtp_core_impl::WrapBytes(bytes.release());
    return *out != nullptr ? kOk : kErrCpp;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

// Materializes a Serializable into storage owned by its wrapper. The returned
// view stays valid until the wrapper is deleted; Go copies it synchronously
// while holding the Serializable's mutex and never retains the native pointer.
__declspec(dllexport) int64_t gov8_crdtp_serializable_view(
    void* raw, const uint8_t** out, size_t* out_len) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsSerializable(raw);
  return gov8_crdtp_core_impl::SerializableView(wrap, out, out_len);
}

__declspec(dllexport) int64_t gov8_crdtp_serializable_delete(void* raw) {
  ClearErr();
  auto* wrap = gov8_crdtp_core_impl::AsSerializable(raw);
  if (wrap == nullptr) {
    SetErr("invalid CRDTP Serializable");
    return kErrMagic;
  }
  wrap->magic = 0;
  delete wrap;
  return kOk;
}

__declspec(dllexport) int64_t gov8_crdtp_create_error_response(
    int32_t call_id, void* response_raw, void** out, int32_t* consumed) {
  ClearErr();
  auto* response = gov8_crdtp_core_impl::AsResponse(response_raw);
  if (response == nullptr || response->value == nullptr || out == nullptr ||
      consumed == nullptr) {
    return kErrBadArg;
  }
  *out = nullptr;
  *consumed = 0;
  bool invoked = false;
  try {
    auto result = std::make_unique<gov8_crdtp_core_impl::SerializableWrap>();
    invoked = true;
    v8_crdtp::Serializable* value =
        crdtp__CreateErrorResponse(call_id, response->value);
    crdtp__DispatchResponse__DELETE(response->value);
    response->value = nullptr;
    response->magic = 0;
    delete response;
    *consumed = 1;
    result->value = value;
    *out = result.release();
    return value != nullptr ? kOk : kErrCpp;
  } catch (...) {
    if (invoked) {
      crdtp__DispatchResponse__DELETE(response->value);
      response->value = nullptr;
      response->magic = 0;
      delete response;
      *consumed = 1;
    }
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_create_error_notification(
    void* response_raw, void** out, int32_t* consumed) {
  ClearErr();
  auto* response = gov8_crdtp_core_impl::AsResponse(response_raw);
  if (response == nullptr || response->value == nullptr || out == nullptr ||
      consumed == nullptr) {
    return kErrBadArg;
  }
  *out = nullptr;
  *consumed = 0;
  bool invoked = false;
  try {
    auto result = std::make_unique<gov8_crdtp_core_impl::SerializableWrap>();
    invoked = true;
    v8_crdtp::Serializable* value =
        crdtp__CreateErrorNotification(response->value);
    crdtp__DispatchResponse__DELETE(response->value);
    response->value = nullptr;
    response->magic = 0;
    delete response;
    *consumed = 1;
    result->value = value;
    *out = result.release();
    return value != nullptr ? kOk : kErrCpp;
  } catch (...) {
    if (invoked) {
      crdtp__DispatchResponse__DELETE(response->value);
      response->value = nullptr;
      response->magic = 0;
      delete response;
      *consumed = 1;
    }
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_create_response(
    int32_t call_id, void* params_raw, void** out, int32_t* consumed) {
  ClearErr();
  auto* params = params_raw == nullptr
                     ? nullptr
                     : gov8_crdtp_core_impl::AsSerializable(params_raw);
  if ((params_raw != nullptr &&
       (params == nullptr || params->value == nullptr)) ||
      out == nullptr || consumed == nullptr) {
    return kErrBadArg;
  }
  *out = nullptr;
  *consumed = 0;
  bool invoked = false;
  try {
    auto result = std::make_unique<gov8_crdtp_core_impl::SerializableWrap>();
    invoked = true;
    v8_crdtp::Serializable* value =
        crdtp__CreateResponse(call_id, params == nullptr ? nullptr
                                                        : params->value);
    if (params != nullptr) {
      params->value = nullptr;
      params->magic = 0;
      delete params;
      *consumed = 1;
    }
    result->value = value;
    *out = result.release();
    return value != nullptr ? kOk : kErrCpp;
  } catch (...) {
    if (invoked && params != nullptr) {
      params->value = nullptr;
      params->magic = 0;
      delete params;
      *consumed = 1;
    }
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_create_notification(
    const char* method, size_t method_len, void* params_raw, void** out,
    int32_t* consumed) {
  ClearErr();
  auto* params = params_raw == nullptr
                     ? nullptr
                     : gov8_crdtp_core_impl::AsSerializable(params_raw);
  if ((method == nullptr && method_len != 0) ||
      (params_raw != nullptr &&
       (params == nullptr || params->value == nullptr)) ||
      out == nullptr || consumed == nullptr) {
    return kErrBadArg;
  }
  *out = nullptr;
  *consumed = 0;
  if (method_len != 0 && std::memchr(method, '\0', method_len) != nullptr) {
    SetErr("CRDTP notification method contains NUL");
    return kErrBadArg;
  }
  bool invoked = false;
  try {
    std::string method_copy(method_len == 0 ? "" : method, method_len);
    auto result = std::make_unique<gov8_crdtp_core_impl::SerializableWrap>();
    invoked = true;
    v8_crdtp::Serializable* value = crdtp__CreateNotification(
        method_copy.c_str(), params == nullptr ? nullptr : params->value);
    if (params != nullptr) {
      params->value = nullptr;
      params->magic = 0;
      delete params;
      *consumed = 1;
    }
    result->value = value;
    *out = result.release();
    return value != nullptr ? kOk : kErrCpp;
  } catch (...) {
    if (invoked && params != nullptr) {
      params->value = nullptr;
      params->magic = 0;
      delete params;
      *consumed = 1;
    }
    SetErr("C++ exception");
    return kErrCpp;
  }
}

}  // extern "C"
