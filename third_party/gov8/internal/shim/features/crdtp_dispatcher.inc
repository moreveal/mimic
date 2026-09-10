// CRDTP callback and dispatcher bridge for pinned rusty_v8 152.2.0. Include
// after crdtp_core.inc so the owned core wrappers are available. Native
// callback payloads contain registry IDs only; Go pointers never cross here.

#include <cstdint>
#include <cstddef>
#include <cstdlib>
#include <cstring>
#include <memory>
#include <mutex>
#include <new>
#include <unordered_map>
#include <vector>

struct crdtp__FrontendChannel__BASE;
struct crdtp__DomainDispatcher__BASE;
struct UberDispatcher;

extern "C" {
void crdtp__FrontendChannel__BASE__CONSTRUCT(void*);
UberDispatcher* crdtp__UberDispatcher__new(crdtp__FrontendChannel__BASE*);
void crdtp__UberDispatcher__DELETE(UberDispatcher*);
crdtp__FrontendChannel__BASE* crdtp__UberDispatcher__channel(UberDispatcher*);
void crdtp__UberDispatcher__Dispatch(UberDispatcher*,
                                     v8_crdtp::Dispatchable*);
crdtp__DomainDispatcher__BASE* crdtp__DomainDispatcher__new(
    crdtp__FrontendChannel__BASE*, void*);
void crdtp__DomainDispatcher__sendResponse(
    crdtp__DomainDispatcher__BASE*, int, DispatchResponseWrapper*,
    v8_crdtp::Serializable*);
void crdtp__UberDispatcher__WireBackend(
    UberDispatcher*, const uint8_t*, size_t,
    crdtp__DomainDispatcher__BASE*);
}

namespace gov8_crdtp_dispatcher_impl {

constexpr uint64_t kChannelMagic = 0x674f56384348414eULL;
constexpr uint64_t kUberMagic = 0x674f563855424552ULL;

using GoDomainDispatch = uintptr_t (*)(uint64_t, const uint8_t*, size_t,
                                       const v8_crdtp::Dispatchable*, uintptr_t,
                                       uintptr_t);
using GoDrop = void (*)(uint64_t);
using GoFallthrough = void (*)(uint64_t, int32_t, const uint8_t*, size_t,
                               const uint8_t*, size_t, const uint8_t*,
                               size_t);
using GoChannel = void (*)(uint64_t, int32_t, int32_t,
                           v8_crdtp::Serializable*);
using GoChannelV2 = void (*)(uint64_t, int32_t, int32_t,
                             v8_crdtp::Serializable*, const uint8_t*, size_t);

GoDomainDispatch g_domain_dispatch = nullptr;
GoDrop g_domain_drop = nullptr;
GoFallthrough g_fallthrough = nullptr;
GoDrop g_fallthrough_drop = nullptr;
GoChannel g_channel = nullptr;
GoChannelV2 g_channel_v2 = nullptr;
GoDrop g_channel_drop = nullptr;

// The flat FrontendChannel base is exactly one vtable pointer in the pinned
// binding. Its placement-constructed storage must remain at offset zero so a
// callback can recover the owning wrapper without another native registry.
struct ChannelWrap {
  alignas(void*) unsigned char raw[sizeof(void*)]{};
  uint64_t magic = kChannelMagic;
  uint64_t id = 0;
};
static_assert(offsetof(ChannelWrap, raw) == 0);

struct UberWrap {
  uint64_t magic = kUberMagic;
  UberDispatcher* value = nullptr;
  ChannelWrap* channel = nullptr;
  std::vector<std::unique_ptr<std::vector<uint8_t>>> domain_names;

  ~UberWrap() {
    if (value != nullptr) {
      crdtp__UberDispatcher__DELETE(value);
    }
  }
};

ChannelWrap* AsChannel(void* raw) {
  auto* value = static_cast<ChannelWrap*>(raw);
  return value != nullptr && value->magic == kChannelMagic ? value : nullptr;
}

ChannelWrap* ChannelFromBase(crdtp__FrontendChannel__BASE* raw) {
  return AsChannel(static_cast<void*>(raw));
}

UberWrap* AsUber(void* raw) {
  auto* value = static_cast<UberWrap*>(raw);
  return value != nullptr && value->magic == kUberMagic ? value : nullptr;
}

std::mutex g_domains_mu;
std::unordered_map<uint64_t, crdtp__DomainDispatcher__BASE*> g_domains;

crdtp__DomainDispatcher__BASE* FindDomain(uint64_t id) {
  std::lock_guard<std::mutex> lock(g_domains_mu);
  auto found = g_domains.find(id);
  return found == g_domains.end() ? nullptr : found->second;
}

void DeliverChannelMessage(ChannelWrap* channel, int32_t kind, int32_t call_id,
                           v8_crdtp::Serializable* message) {
  auto callback = g_channel;
  auto callback_v2 = g_channel_v2;
  if (channel == nullptr || (callback == nullptr && callback_v2 == nullptr) ||
      message == nullptr) {
    std::abort();
  }
  auto* owned = gov8_crdtp_core_impl::WrapSerializable(message);
  if (owned == nullptr) {
    std::abort();
  }
  auto* owned_raw = reinterpret_cast<v8_crdtp::Serializable*>(owned);
  if (callback_v2 != nullptr) {
    const uint8_t* serialized = nullptr;
    size_t serialized_len = 0;
    if (gov8_crdtp_core_impl::SerializableView(
            owned, &serialized, &serialized_len) != kOk) {
      serialized = nullptr;
      serialized_len = 0;
    }
    callback_v2(channel->id, kind, call_id, owned_raw, serialized,
                serialized_len);
    return;
  }
  callback(channel->id, kind, call_id, owned_raw);
}

int64_t SetCallbacks(uintptr_t domain_dispatch, uintptr_t domain_drop,
                     uintptr_t fallthrough, uintptr_t fallthrough_drop,
                     uintptr_t channel, uintptr_t channel_drop,
                     bool channel_v2) {
  if (domain_dispatch == 0 || domain_drop == 0 || fallthrough == 0 ||
      fallthrough_drop == 0 || channel == 0 || channel_drop == 0) {
    return kErrBadArg;
  }
  g_domain_dispatch = reinterpret_cast<GoDomainDispatch>(domain_dispatch);
  g_domain_drop = reinterpret_cast<GoDrop>(domain_drop);
  g_fallthrough = reinterpret_cast<GoFallthrough>(fallthrough);
  g_fallthrough_drop = reinterpret_cast<GoDrop>(fallthrough_drop);
  g_channel = channel_v2 ? nullptr : reinterpret_cast<GoChannel>(channel);
  g_channel_v2 = channel_v2 ? reinterpret_cast<GoChannelV2>(channel) : nullptr;
  g_channel_drop = reinterpret_cast<GoDrop>(channel_drop);
  return kOk;
}

}  // namespace gov8_crdtp_dispatcher_impl

extern "C" {

// These seven symbols replace the link-only fail-fast definitions used by the
// Inspector object-wrapping slice and are the exact callbacks referenced by
// rusty_v8's crdtp_binding.obj.
void crdtp__FallthroughCallback__Run(
    void* callback, int call_id, const uint8_t* method_data,
    size_t method_len, const uint8_t* message_data, size_t message_len,
    const uint8_t* associated_data, size_t associated_data_len) {
  auto dispatch = gov8_crdtp_dispatcher_impl::g_fallthrough;
  if (dispatch == nullptr || callback == nullptr) {
    std::abort();
  }
  dispatch(static_cast<uint64_t>(reinterpret_cast<uintptr_t>(callback)),
           call_id, method_data, method_len, message_data, message_len,
           associated_data, associated_data_len);
}

void crdtp__FallthroughCallback__Drop(void* callback) {
  auto dispatch = gov8_crdtp_dispatcher_impl::g_fallthrough_drop;
  if (dispatch == nullptr || callback == nullptr) {
    std::abort();
  }
  dispatch(static_cast<uint64_t>(reinterpret_cast<uintptr_t>(callback)));
}

bool crdtp__DomainDispatcher__BASE__Dispatch(
    void* dispatcher, const uint8_t* command_data, size_t command_len,
    const v8_crdtp::Dispatchable& dispatchable) {
  auto callback = gov8_crdtp_dispatcher_impl::g_domain_dispatch;
  if (callback == nullptr || dispatcher == nullptr) {
    std::abort();
  }
  const bool has_call_id = crdtp__Dispatchable__hasCallId(&dispatchable);
  return callback(static_cast<uint64_t>(reinterpret_cast<uintptr_t>(dispatcher)),
                  command_data, command_len, &dispatchable,
                  static_cast<uintptr_t>(has_call_id ? 1 : 0),
                  static_cast<uintptr_t>(static_cast<uint32_t>(
                      has_call_id ? crdtp__Dispatchable__callId(&dispatchable)
                                  : 0))) !=
         0;
}

void crdtp__DomainDispatcher__BASE__Drop(void* dispatcher) {
  if (dispatcher == nullptr) {
    std::abort();
  }
  const uint64_t id =
      static_cast<uint64_t>(reinterpret_cast<uintptr_t>(dispatcher));
  {
    std::lock_guard<std::mutex> lock(
        gov8_crdtp_dispatcher_impl::g_domains_mu);
    gov8_crdtp_dispatcher_impl::g_domains.erase(id);
  }
  auto callback = gov8_crdtp_dispatcher_impl::g_domain_drop;
  if (callback == nullptr) {
    std::abort();
  }
  callback(id);
}

void crdtp__FrontendChannel__BASE__sendProtocolResponse(
    crdtp__FrontendChannel__BASE* self, int call_id,
    v8_crdtp::Serializable* message) {
  auto* channel = gov8_crdtp_dispatcher_impl::ChannelFromBase(self);
  gov8_crdtp_dispatcher_impl::DeliverChannelMessage(channel, 0, call_id,
                                                     message);
}

void crdtp__FrontendChannel__BASE__sendProtocolNotification(
    crdtp__FrontendChannel__BASE* self, v8_crdtp::Serializable* message) {
  auto* channel = gov8_crdtp_dispatcher_impl::ChannelFromBase(self);
  gov8_crdtp_dispatcher_impl::DeliverChannelMessage(channel, 1, 0, message);
}

void crdtp__FrontendChannel__BASE__flushProtocolNotifications(
    crdtp__FrontendChannel__BASE* self) {
  auto* channel = gov8_crdtp_dispatcher_impl::ChannelFromBase(self);
  auto callback = gov8_crdtp_dispatcher_impl::g_channel;
  auto callback_v2 = gov8_crdtp_dispatcher_impl::g_channel_v2;
  if (channel == nullptr || (callback == nullptr && callback_v2 == nullptr)) {
    std::abort();
  }
  if (callback_v2 != nullptr) {
    callback_v2(channel->id, 2, 0, nullptr, nullptr, 0);
  } else {
    callback(channel->id, 2, 0, nullptr);
  }
}

__declspec(dllexport) int64_t gov8_crdtp_dispatcher_set_callbacks(
    uintptr_t domain_dispatch, uintptr_t domain_drop, uintptr_t fallthrough,
    uintptr_t fallthrough_drop, uintptr_t channel, uintptr_t channel_drop) {
  ClearErr();
  return gov8_crdtp_dispatcher_impl::SetCallbacks(
      domain_dispatch, domain_drop, fallthrough, fallthrough_drop, channel,
      channel_drop, false);
}

__declspec(dllexport) int64_t gov8_crdtp_dispatcher_set_callbacks_v2(
    uintptr_t domain_dispatch, uintptr_t domain_drop, uintptr_t fallthrough,
    uintptr_t fallthrough_drop, uintptr_t channel, uintptr_t channel_drop) {
  ClearErr();
  return gov8_crdtp_dispatcher_impl::SetCallbacks(
      domain_dispatch, domain_drop, fallthrough, fallthrough_drop, channel,
      channel_drop, true);
}

__declspec(dllexport) int64_t gov8_crdtp_channel_new(uint64_t id,
                                                     void** out) {
  ClearErr();
  if (id == 0 || out == nullptr) {
    return kErrBadArg;
  }
  *out = nullptr;
  try {
    auto channel = std::make_unique<gov8_crdtp_dispatcher_impl::ChannelWrap>();
    channel->id = id;
    crdtp__FrontendChannel__BASE__CONSTRUCT(channel->raw);
    *out = channel.release();
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_channel_delete(void* raw) {
  ClearErr();
  auto* channel = gov8_crdtp_dispatcher_impl::AsChannel(raw);
  if (channel == nullptr) {
    return kErrMagic;
  }
  const uint64_t id = channel->id;
  channel->magic = 0;
  delete channel;
  auto callback = gov8_crdtp_dispatcher_impl::g_channel_drop;
  if (callback == nullptr) {
    std::abort();
  }
  callback(id);
  return kOk;
}

__declspec(dllexport) int64_t gov8_crdtp_uber_new(void* channel_raw,
                                                  void** out) {
  ClearErr();
  auto* channel = gov8_crdtp_dispatcher_impl::AsChannel(channel_raw);
  if (channel == nullptr || out == nullptr) {
    return kErrBadArg;
  }
  *out = nullptr;
  try {
    auto uber = std::make_unique<gov8_crdtp_dispatcher_impl::UberWrap>();
    uber->channel = channel;
    uber->value = crdtp__UberDispatcher__new(
        reinterpret_cast<crdtp__FrontendChannel__BASE*>(channel->raw));
    if (uber->value == nullptr) {
      return kErrCpp;
    }
    *out = uber.release();
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_uber_wire(
    void* uber_raw, const uint8_t* domain, size_t domain_len, uint64_t id,
    int32_t* consumed) {
  ClearErr();
  auto* uber = gov8_crdtp_dispatcher_impl::AsUber(uber_raw);
  if (uber == nullptr || uber->value == nullptr || id == 0 ||
      consumed == nullptr || (domain == nullptr && domain_len != 0)) {
    return kErrBadArg;
  }
  *consumed = 0;
  try {
    auto name = std::make_unique<std::vector<uint8_t>>();
    if (domain_len != 0) {
      name->assign(domain, domain + domain_len);
    }
    uber->domain_names.push_back(std::move(name));
    const auto& retained_name = *uber->domain_names.back();
    {
      std::lock_guard<std::mutex> lock(
          gov8_crdtp_dispatcher_impl::g_domains_mu);
      auto inserted = gov8_crdtp_dispatcher_impl::g_domains.emplace(id, nullptr);
      if (!inserted.second) {
        SetErr("duplicate CRDTP domain callback ID");
        return kErrState;
      }
    }
    auto* backend = crdtp__DomainDispatcher__new(
        crdtp__UberDispatcher__channel(uber->value),
        reinterpret_cast<void*>(static_cast<uintptr_t>(id)));
    if (backend == nullptr) {
      std::lock_guard<std::mutex> lock(
          gov8_crdtp_dispatcher_impl::g_domains_mu);
      gov8_crdtp_dispatcher_impl::g_domains.erase(id);
      return kErrCpp;
    }
    {
      std::lock_guard<std::mutex> lock(
          gov8_crdtp_dispatcher_impl::g_domains_mu);
      gov8_crdtp_dispatcher_impl::g_domains.find(id)->second = backend;
    }
    *consumed = 1;
    crdtp__UberDispatcher__WireBackend(
        uber->value, retained_name.data(), retained_name.size(), backend);
    return kOk;
  } catch (...) {
    if (*consumed == 0) {
      std::lock_guard<std::mutex> lock(
          gov8_crdtp_dispatcher_impl::g_domains_mu);
      gov8_crdtp_dispatcher_impl::g_domains.erase(id);
    }
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_uber_dispatch(
    void* uber_raw, void* dispatchable_raw) {
  ClearErr();
  auto* uber = gov8_crdtp_dispatcher_impl::AsUber(uber_raw);
  auto* dispatchable = gov8_crdtp_core_impl::AsDispatchable(dispatchable_raw);
  if (uber == nullptr || uber->value == nullptr || dispatchable == nullptr ||
      dispatchable->value == nullptr) {
    return kErrBadArg;
  }
  if (!crdtp__Dispatchable__ok(dispatchable->value)) {
    SetErr("CRDTP Dispatchable is invalid");
    return kErrBadArg;
  }
  try {
    crdtp__UberDispatcher__Dispatch(uber->value, dispatchable->value);
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_uber_delete(void* raw) {
  ClearErr();
  auto* uber = gov8_crdtp_dispatcher_impl::AsUber(raw);
  if (uber == nullptr) {
    return kErrMagic;
  }
  uber->magic = 0;
  delete uber;
  return kOk;
}

__declspec(dllexport) int64_t gov8_crdtp_dispatchable_fallthrough_new(
    const uint8_t* data, size_t len, const uint8_t* associated,
    size_t associated_len, uint64_t id, void** out) {
  ClearErr();
  if (id == 0 || out == nullptr || (data == nullptr && len != 0) ||
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
        wrap->associated.size(),
        reinterpret_cast<void*>(static_cast<uintptr_t>(id)));
    if (wrap->value == nullptr) {
      return kErrCpp;
    }
    *out = wrap.release();
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_request_call_id(
    const v8_crdtp::Dispatchable* dispatchable, int32_t* has_call_id,
    int32_t* call_id) {
  ClearErr();
  if (dispatchable == nullptr || has_call_id == nullptr || call_id == nullptr ||
      !crdtp__Dispatchable__ok(dispatchable)) {
    return kErrBadArg;
  }
  *has_call_id = crdtp__Dispatchable__hasCallId(dispatchable) ? 1 : 0;
  *call_id = crdtp__Dispatchable__callId(dispatchable);
  return kOk;
}

__declspec(dllexport) int64_t gov8_crdtp_request_bytes(
    const v8_crdtp::Dispatchable* dispatchable, int32_t kind, void** out) {
  ClearErr();
  if (dispatchable == nullptr || out == nullptr ||
      !crdtp__Dispatchable__ok(dispatchable)) {
    return kErrBadArg;
  }
  *out = nullptr;
  try {
    size_t len = 0;
    switch (kind) {
      case 0:
        len = crdtp__Dispatchable__methodLen(dispatchable);
        break;
      case 1:
        len = crdtp__Dispatchable__sessionIdLen(dispatchable);
        break;
      case 2:
        len = crdtp__Dispatchable__paramsLen(dispatchable);
        break;
      case 3:
        len = crdtp__Dispatchable__associatedDataLen(dispatchable);
        break;
      default:
        return kErrBadArg;
    }
    auto bytes = std::make_unique<std::vector<uint8_t>>(len);
    if (len != 0) {
      switch (kind) {
        case 0:
          crdtp__Dispatchable__methodCopy(dispatchable, bytes->data());
          break;
        case 1:
          crdtp__Dispatchable__sessionIdCopy(dispatchable, bytes->data());
          break;
        case 2:
          crdtp__Dispatchable__paramsCopy(dispatchable, bytes->data());
          break;
        case 3:
          crdtp__Dispatchable__associatedDataCopy(dispatchable, bytes->data());
          break;
      }
    }
    *out = gov8_crdtp_core_impl::WrapBytes(bytes.release());
    return *out == nullptr ? kErrCpp : kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_crdtp_domain_send_response(
    uint64_t domain_id, int32_t call_id, void* response_raw, void* result_raw,
    int32_t* response_consumed, int32_t* result_consumed) {
  ClearErr();
  auto* backend = gov8_crdtp_dispatcher_impl::FindDomain(domain_id);
  auto* response = gov8_crdtp_core_impl::AsResponse(response_raw);
  auto* result = result_raw == nullptr
                     ? nullptr
                     : gov8_crdtp_core_impl::AsSerializable(result_raw);
  if (backend == nullptr || response == nullptr || response->value == nullptr ||
      (result_raw != nullptr && (result == nullptr || result->value == nullptr)) ||
      response_consumed == nullptr || result_consumed == nullptr) {
    return kErrBadArg;
  }
  *response_consumed = 0;
  *result_consumed = 0;
  try {
    crdtp__DomainDispatcher__sendResponse(
        backend, call_id, response->value,
        result == nullptr ? nullptr : result->value);
    response->value = nullptr;
    response->magic = 0;
    delete response;
    *response_consumed = 1;
    if (result != nullptr) {
      result->value = nullptr;
      result->magic = 0;
      delete result;
      *result_consumed = 1;
    }
    return kOk;
  } catch (...) {
    crdtp__DispatchResponse__DELETE(response->value);
    response->value = nullptr;
    response->magic = 0;
    delete response;
    *response_consumed = 1;
    if (result != nullptr) {
      result->value = nullptr;
      result->magic = 0;
      delete result;
      *result_consumed = 1;
    }
    SetErr("C++ exception");
    return kErrCpp;
  }
}

}  // extern "C"
