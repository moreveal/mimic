// wasm_streaming.inc -- WebAssembly streaming and asynchronous compilation.

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <memory>
#include <mutex>
#include <string>
#include <unordered_map>

namespace gov8_wasm_streaming_impl {

struct WasmStreamingSharedPtr {
  void* words[2];
};
static_assert(sizeof(WasmStreamingSharedPtr) == 2 * sizeof(void*),
              "WasmStreamingSharedPtr ABI");

struct ConstMemorySpan {
  const uint8_t* data;
  size_t size;
};
static_assert(sizeof(ConstMemorySpan) == 2 * sizeof(void*),
              "ConstMemorySpan ABI");

struct StreamWrap {
  WasmStreamingSharedPtr stream;
};

struct StreamingFrame {
  uint64_t id;
  v8::Isolate* isolate;
  void* context_wire;
  void* scope_wire;
  void* source_wire;
  StreamWrap* stream;
};

struct ResolutionFrame {
  uint64_t id;
  v8::Isolate* isolate;
  void* context_wire;
  void* scope_wire;
  void* module_wire;
  void* error_wire;
};

using GoStreamingDispatcher = uintptr_t (*)(StreamingFrame*);
using GoCachingDispatcher = uintptr_t (*)(uint64_t, void*);
using GoResolutionDispatcher = uintptr_t (*)(ResolutionFrame*);
using GoDropDispatcher = uintptr_t (*)(uint64_t, uintptr_t);
using GoSerializationDispatcher = uintptr_t (*)(uint64_t,
                                                  v8::CompiledWasmModule*);

GoStreamingDispatcher streaming_dispatcher = nullptr;
GoCachingDispatcher caching_dispatcher = nullptr;
GoResolutionDispatcher resolution_dispatcher = nullptr;
GoDropDispatcher drop_dispatcher = nullptr;
GoSerializationDispatcher serialization_dispatcher = nullptr;

std::mutex stream_bindings_mutex;
std::unordered_map<v8::Isolate*, uint64_t> stream_bindings;
thread_local uint64_t active_caching_id = 0;

struct ResolutionData {
  uint64_t id;
  v8::Isolate* isolate;
  std::mutex transfer_mutex;
  bool caller_owns = true;
  bool native_dropped = false;
};

struct SerializationData {
  uint64_t id;
  std::mutex transfer_mutex;
  bool caller_owns = true;
  bool native_dropped = false;
};

template <class T>
void CompleteCallbackTransfer(std::unique_ptr<T>& owner) {
  std::lock_guard<std::mutex> lock(owner->transfer_mutex);
  owner->caller_owns = false;
  if (!owner->native_dropped) owner.release();
}

template <class T>
bool MarkNativeCallbackDropped(T* data) {
  std::lock_guard<std::mutex> lock(data->transfer_mutex);
  if (data->caller_owns) {
    data->native_dropped = true;
    return false;
  }
  return true;
}

extern "C" {
bool v8__Platform__PumpMessageLoop(v8::Platform*, v8::Isolate*, bool);
void v8__Isolate__SetWasmStreamingCallback(
    v8::Isolate*, v8::WasmStreamingCallback);
void v8__WasmStreaming__Unpack(v8::Isolate*, const v8::Value*,
                               WasmStreamingSharedPtr*);
void v8__WasmStreaming__shared_ptr_DESTRUCT(WasmStreamingSharedPtr*);
void v8__WasmStreaming__SetHasCompiledModuleBytes(WasmStreamingSharedPtr*);
void v8__WasmStreaming__OnBytesReceived(WasmStreamingSharedPtr*,
                                        const uint8_t*, size_t);
void v8__WasmStreaming__Finish(
    WasmStreamingSharedPtr*,
    void (*)(v8::WasmStreaming::ModuleCachingInterface&));
void v8__WasmStreaming__Abort(WasmStreamingSharedPtr*, const v8::Value*);
void v8__WasmStreaming__SetUrl(WasmStreamingSharedPtr*, const char*, size_t);
ConstMemorySpan v8__ModuleCachingInterface__GetWireBytes(
    const v8::WasmStreaming::ModuleCachingInterface*);
bool v8__ModuleCachingInterface__SetCachedCompiledModuleBytes(
    v8::WasmStreaming::ModuleCachingInterface*, ConstMemorySpan);

v8::WasmModuleCompilation* v8__WasmModuleCompilation__NEW();
void v8__WasmModuleCompilation__DELETE(v8::WasmModuleCompilation*);
void v8__WasmModuleCompilation__OnBytesReceived(v8::WasmModuleCompilation*,
                                                const uint8_t*, size_t);
void v8__WasmModuleCompilation__Finish(
    v8::WasmModuleCompilation*, v8::Isolate*,
    void (*)(v8::WasmStreaming::ModuleCachingInterface&),
    void (*)(void*, const v8::WasmModuleObject*, const v8::Value*), void*,
    void (*)(void*));
void v8__WasmModuleCompilation__Abort(v8::WasmModuleCompilation*);
void v8__WasmModuleCompilation__SetHasCompiledModuleBytes(
    v8::WasmModuleCompilation*);
void v8__WasmModuleCompilation__SetMoreFunctionsCanBeSerializedCallback(
    v8::WasmModuleCompilation*,
    void (*)(void*, v8::CompiledWasmModule*), void*, void (*)(void*));
void v8__WasmModuleCompilation__SetUrl(v8::WasmModuleCompilation*,
                                       const char*, size_t);
}

void StreamingTrampoline(
    const v8::FunctionCallbackInfo<v8::Value>& info) {
  v8::Isolate* isolate = info.GetIsolate();
  uint64_t id = 0;
  {
    std::lock_guard<std::mutex> lock(stream_bindings_mutex);
    auto it = stream_bindings.find(isolate);
    if (it != stream_bindings.end()) id = it->second;
  }
  if (id == 0 || streaming_dispatcher == nullptr) {
    std::abort();
  }
  auto* stream = new StreamWrap{};
  v8__WasmStreaming__Unpack(
      isolate, static_cast<const v8::Value*>(ToWire(info.Data())),
      &stream->stream);
  GoScope callback_scope{};
  callback_scope.magic = kScopeMagic;
  callback_scope.iso = isolate;
  GlobalPlacementNew<v8::HandleScope>(callback_scope.hs_storage, isolate);
  v8::Local<v8::Context> context = isolate->GetCurrentContext();
  StreamingFrame frame{id,
                       isolate,
                       context.IsEmpty() ? nullptr : ToWire(context),
                       &callback_scope,
                       ToWire(info[0]),
                       stream};
  const uintptr_t failed = streaming_dispatcher(&frame);
  callback_scope.hs()->~HandleScope();
  callback_scope.magic = 0;
  if (failed != 0) {
    v8__WasmStreaming__shared_ptr_DESTRUCT(&stream->stream);
    delete stream;
    std::abort();
  }
}

void CachingTrampoline(v8::WasmStreaming::ModuleCachingInterface& interface) {
  if (active_caching_id == 0 || caching_dispatcher == nullptr ||
      caching_dispatcher(active_caching_id, &interface) != 0) {
    std::abort();
  }
}

void ResolutionTrampoline(void* opaque, const v8::WasmModuleObject* module,
                          const v8::Value* error) {
  auto* data = static_cast<ResolutionData*>(opaque);
  if (data == nullptr || resolution_dispatcher == nullptr) std::abort();
  GoScope callback_scope{};
  callback_scope.magic = kScopeMagic;
  callback_scope.iso = data->isolate;
  GlobalPlacementNew<v8::HandleScope>(callback_scope.hs_storage,
                                      data->isolate);
  v8::Local<v8::Context> context = data->isolate->GetCurrentContext();
  ResolutionFrame frame{data->id,
                        data->isolate,
                        context.IsEmpty() ? nullptr : ToWire(context),
                        &callback_scope,
                        const_cast<v8::WasmModuleObject*>(module),
                        const_cast<v8::Value*>(error)};
  const uintptr_t failed = resolution_dispatcher(&frame);
  callback_scope.hs()->~HandleScope();
  callback_scope.magic = 0;
  if (failed != 0) std::abort();
}

void DropResolutionData(void* opaque) {
  auto* data = static_cast<ResolutionData*>(opaque);
  if (data == nullptr) return;
  if (drop_dispatcher != nullptr) drop_dispatcher(data->id, 0);
  if (MarkNativeCallbackDropped(data)) delete data;
}

void SerializationTrampoline(void* opaque,
                             v8::CompiledWasmModule* compiled) {
  auto* data = static_cast<SerializationData*>(opaque);
  if (data == nullptr || serialization_dispatcher == nullptr ||
      serialization_dispatcher(data->id, compiled) != 0) {
    std::abort();
  }
}

void DropSerializationData(void* opaque) {
  auto* data = static_cast<SerializationData*>(opaque);
  if (data == nullptr) return;
  if (drop_dispatcher != nullptr) drop_dispatcher(data->id, 1);
  if (MarkNativeCallbackDropped(data)) delete data;
}

template <class F>
int64_t WithStream(StreamWrap* stream, const char* operation, F&& fn) {
  ClearErr();
  if (stream == nullptr) {
    SetErr("invalid wasm stream");
    return kErrBadArg;
  }
  try {
    fn(stream);
    return kOk;
  } catch (...) {
    SetErr(operation);
    return kErrCpp;
  }
}

void DestroyStream(StreamWrap* stream) {
  if (stream == nullptr) return;
  v8__WasmStreaming__shared_ptr_DESTRUCT(&stream->stream);
  delete stream;
}

void DestroyCompilation(v8::WasmModuleCompilation* compilation) {
  if (compilation != nullptr) {
    v8__WasmModuleCompilation__DELETE(compilation);
  }
}

}  // namespace gov8_wasm_streaming_impl

extern "C" {

using namespace gov8_wasm_streaming_impl;

__declspec(dllexport) int64_t gov8_ws_set_dispatchers(
    void* stream, void* cache, void* resolution, void* drop,
    void* serialization) {
  ClearErr();
  if (stream == nullptr || cache == nullptr || resolution == nullptr ||
      drop == nullptr || serialization == nullptr) {
    SetErr("invalid wasm dispatcher");
    return kErrBadArg;
  }
  streaming_dispatcher = reinterpret_cast<GoStreamingDispatcher>(stream);
  caching_dispatcher = reinterpret_cast<GoCachingDispatcher>(cache);
  resolution_dispatcher = reinterpret_cast<GoResolutionDispatcher>(resolution);
  drop_dispatcher = reinterpret_cast<GoDropDispatcher>(drop);
  serialization_dispatcher =
      reinterpret_cast<GoSerializationDispatcher>(serialization);
  return kOk;
}

__declspec(dllexport) int64_t gov8_ws_set_callback(v8::Isolate* isolate,
                                                   uint64_t id) {
  ClearErr();
  if (isolate == nullptr || id == 0 || streaming_dispatcher == nullptr) {
    SetErr("invalid wasm streaming callback");
    return kErrBadArg;
  }
  try {
    std::lock_guard<std::mutex> lock(stream_bindings_mutex);
    if (stream_bindings.find(isolate) != stream_bindings.end()) {
      SetErr("wasm streaming callback already installed");
      return kErrState;
    }
    stream_bindings[isolate] = id;
    v8__Isolate__SetWasmStreamingCallback(isolate, StreamingTrampoline);
    return kOk;
  } catch (...) {
    {
      std::lock_guard<std::mutex> lock(stream_bindings_mutex);
      stream_bindings.erase(isolate);
    }
    SetErr("C++ exception setting wasm streaming callback");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ws_clear_callback(v8::Isolate* isolate) {
  ClearErr();
  if (isolate == nullptr) {
    SetErr("invalid isolate");
    return kErrBadArg;
  }
  try {
    std::lock_guard<std::mutex> lock(stream_bindings_mutex);
    v8__Isolate__SetWasmStreamingCallback(isolate, nullptr);
    stream_bindings.erase(isolate);
    return kOk;
  } catch (...) {
    SetErr("C++ exception clearing wasm streaming callback");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ws_on_bytes(StreamWrap* stream,
                                               const uint8_t* bytes,
                                               size_t length) {
  if (bytes == nullptr && length != 0) {
    ClearErr();
    SetErr("invalid wasm byte chunk");
    return kErrBadArg;
  }
  uint8_t empty = 0;
  return WithStream(stream, "C++ exception feeding wasm stream",
                    [&](StreamWrap* value) {
                      v8__WasmStreaming__OnBytesReceived(
                          &value->stream, bytes == nullptr ? &empty : bytes,
                          length);
                    });
}

__declspec(dllexport) int64_t gov8_ws_set_url(StreamWrap* stream,
                                              const char* url,
                                              size_t length) {
  if (url == nullptr && length != 0) {
    ClearErr();
    SetErr("invalid wasm URL");
    return kErrBadArg;
  }
  return WithStream(stream, "C++ exception setting wasm stream URL",
                    [&](StreamWrap* value) {
                      std::string terminated(url == nullptr ? "" : url,
                                             length);
                      terminated.push_back('\0');
                      v8__WasmStreaming__SetUrl(&value->stream,
                                                terminated.data(), length);
                    });
}

__declspec(dllexport) int64_t gov8_ws_set_has_cache(StreamWrap* stream) {
  return WithStream(stream, "C++ exception selecting wasm stream cache",
                    [](StreamWrap* value) {
                      v8__WasmStreaming__SetHasCompiledModuleBytes(
                          &value->stream);
                    });
}

__declspec(dllexport) int64_t gov8_ws_finish(StreamWrap* stream,
                                             uint64_t cache_id) {
  ClearErr();
  if (stream == nullptr) {
    SetErr("invalid wasm stream");
    return kErrBadArg;
  }
  std::unique_ptr<StreamWrap, decltype(&DestroyStream)> owner(stream,
                                                              DestroyStream);
  try {
    active_caching_id = cache_id;
    v8__WasmStreaming__Finish(
        &stream->stream, cache_id == 0 ? nullptr : CachingTrampoline);
    active_caching_id = 0;
    return kOk;
  } catch (...) {
    active_caching_id = 0;
    SetErr("C++ exception finishing wasm stream");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ws_abort(StreamWrap* stream,
                                            const v8::Value* exception) {
  ClearErr();
  if (stream == nullptr) {
    SetErr("invalid wasm stream");
    return kErrBadArg;
  }
  std::unique_ptr<StreamWrap, decltype(&DestroyStream)> owner(stream,
                                                              DestroyStream);
  try {
    v8__WasmStreaming__Abort(&stream->stream, exception);
    return kOk;
  } catch (...) {
    SetErr("C++ exception aborting wasm stream");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ws_dispose(StreamWrap* stream) {
  ClearErr();
  if (stream == nullptr) {
    SetErr("invalid wasm stream");
    return kErrBadArg;
  }
  try {
    DestroyStream(stream);
    return kOk;
  } catch (...) {
    SetErr("C++ exception disposing wasm stream");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ws_cache_wire(
    v8::WasmStreaming::ModuleCachingInterface* interface, uint8_t* output,
    size_t capacity, size_t* out_length) {
  ClearErr();
  if (interface == nullptr || out_length == nullptr ||
      (output == nullptr && capacity != 0)) {
    SetErr("invalid module caching interface");
    return kErrBadArg;
  }
  try {
    ConstMemorySpan bytes =
        v8__ModuleCachingInterface__GetWireBytes(interface);
    *out_length = bytes.size;
    if (bytes.data == nullptr && bytes.size != 0) {
      SetErr("module caching interface returned null bytes");
      return kErrGeneric;
    }
    if (output == nullptr) return kOk;
    if (capacity < bytes.size) {
      SetErr("module caching destination too small");
      return kErrBadArg;
    }
    if (bytes.size != 0) std::memcpy(output, bytes.data, bytes.size);
    return kOk;
  } catch (...) {
    SetErr("C++ exception reading wasm cache wire bytes");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ws_cache_set(
    v8::WasmStreaming::ModuleCachingInterface* interface,
    const uint8_t* bytes, size_t length, int32_t* accepted) {
  ClearErr();
  if (interface == nullptr || accepted == nullptr ||
      (bytes == nullptr && length != 0)) {
    SetErr("invalid module caching bytes");
    return kErrBadArg;
  }
  try {
    uint8_t empty = 0;
    *accepted = v8__ModuleCachingInterface__SetCachedCompiledModuleBytes(
                    interface,
                    ConstMemorySpan{bytes == nullptr ? &empty : bytes, length})
                    ? 1
                    : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception setting wasm cache bytes");
    return kErrCpp;
  }
}

__declspec(dllexport) void* gov8_wmc_new() {
  ClearErr();
  try {
    return v8__WasmModuleCompilation__NEW();
  } catch (...) {
    SetErr("C++ exception creating wasm module compilation");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_wmc_on_bytes(
    v8::WasmModuleCompilation* compilation, const uint8_t* bytes,
    size_t length) {
  ClearErr();
  if (compilation == nullptr || (bytes == nullptr && length != 0)) {
    SetErr("invalid wasm module compilation bytes");
    return kErrBadArg;
  }
  try {
    uint8_t empty = 0;
    v8__WasmModuleCompilation__OnBytesReceived(
        compilation, bytes == nullptr ? &empty : bytes, length);
    return kOk;
  } catch (...) {
    SetErr("C++ exception feeding wasm module compilation");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wmc_set_url(
    v8::WasmModuleCompilation* compilation, const char* url,
    size_t length) {
  ClearErr();
  if (compilation == nullptr || (url == nullptr && length != 0)) {
    SetErr("invalid wasm module compilation URL");
    return kErrBadArg;
  }
  try {
    std::string terminated(url == nullptr ? "" : url, length);
    terminated.push_back('\0');
    v8__WasmModuleCompilation__SetUrl(compilation, terminated.data(), length);
    return kOk;
  } catch (...) {
    SetErr("C++ exception setting wasm module compilation URL");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wmc_set_has_cache(
    v8::WasmModuleCompilation* compilation) {
  ClearErr();
  if (compilation == nullptr) {
    SetErr("invalid wasm module compilation");
    return kErrBadArg;
  }
  try {
    v8__WasmModuleCompilation__SetHasCompiledModuleBytes(compilation);
    return kOk;
  } catch (...) {
    SetErr("C++ exception selecting wasm module compilation cache");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wmc_set_serialization_callback(
    v8::WasmModuleCompilation* compilation, uint64_t id) {
  ClearErr();
  if (compilation == nullptr || id == 0 ||
      serialization_dispatcher == nullptr || drop_dispatcher == nullptr) {
    SetErr("invalid wasm serialization callback");
    return kErrBadArg;
  }
  try {
    auto data = std::make_unique<SerializationData>();
    data->id = id;
    v8__WasmModuleCompilation__SetMoreFunctionsCanBeSerializedCallback(
        compilation, SerializationTrampoline, data.get(),
        DropSerializationData);
    CompleteCallbackTransfer(data);
    return kOk;
  } catch (...) {
    SetErr("C++ exception setting wasm serialization callback");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wmc_finish(
    v8::WasmModuleCompilation* compilation, v8::Isolate* isolate,
    void* ctxw, void* scope, uint64_t cache_id, uint64_t resolution_id) {
  ClearErr();
  if (compilation == nullptr) {
    SetErr("invalid wasm module compilation finish argument");
    return kErrBadArg;
  }
  std::unique_ptr<v8::WasmModuleCompilation, decltype(&DestroyCompilation)>
      owner(compilation, DestroyCompilation);
  CtxWrap* context = AsCtx(ctxw);
  if (isolate == nullptr || context == nullptr ||
      !OwnedBy(isolate, context->iso) ||
      !ScopeIs(isolate, scope) || resolution_id == 0 ||
      resolution_dispatcher == nullptr || drop_dispatcher == nullptr) {
    SetErr("invalid wasm module compilation finish argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    v8::Local<v8::Context> ctx = context->ctx->Get(isolate);
    v8::Context::Scope context_scope(ctx);
    auto data = std::make_unique<ResolutionData>();
    data->id = resolution_id;
    data->isolate = isolate;
    active_caching_id = cache_id;
    v8__WasmModuleCompilation__Finish(
        compilation, isolate,
        cache_id == 0 ? nullptr : CachingTrampoline, ResolutionTrampoline,
        data.get(), DropResolutionData);
    active_caching_id = 0;
    CompleteCallbackTransfer(data);
    return kOk;
  } catch (...) {
    active_caching_id = 0;
    SetErr("C++ exception finishing wasm module compilation");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ws_pump_message_loop(
    v8::Isolate* isolate, int32_t wait_for_work) {
  ClearErr();
  if (isolate == nullptr || g_platform == nullptr ||
      (wait_for_work != 0 && wait_for_work != 1)) {
    SetErr("invalid wasm message-loop argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(isolate);
    return v8__Platform__PumpMessageLoop(g_platform, isolate,
                                         wait_for_work != 0)
               ? 1
               : 0;
  } catch (...) {
    SetErr("C++ exception pumping wasm message loop");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wmc_abort(
    v8::WasmModuleCompilation* compilation) {
  ClearErr();
  if (compilation == nullptr) {
    SetErr("invalid wasm module compilation");
    return kErrBadArg;
  }
  std::unique_ptr<v8::WasmModuleCompilation, decltype(&DestroyCompilation)>
      owner(compilation, DestroyCompilation);
  try {
    v8__WasmModuleCompilation__Abort(compilation);
    return kOk;
  } catch (...) {
    SetErr("C++ exception aborting wasm module compilation");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wmc_dispose(
    v8::WasmModuleCompilation* compilation) {
  ClearErr();
  if (compilation == nullptr) {
    SetErr("invalid wasm module compilation");
    return kErrBadArg;
  }
  try {
    DestroyCompilation(compilation);
    return kOk;
  } catch (...) {
    SetErr("C++ exception disposing wasm module compilation");
    return kErrCpp;
  }
}

}  // extern "C"
