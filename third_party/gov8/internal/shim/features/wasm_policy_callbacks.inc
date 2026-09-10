// Isolate-level WebAssembly policy callbacks for pinned rusty_v8 152.2.0.
// Include after wasm_streaming.inc (for conceptual callback ordering); this
// slice otherwise depends only on shim.cc's core wire/scope helpers.

#include <cstdint>
#include <optional>

namespace gov8_wasm_policy_impl {

struct AllowFrame {
  v8::Isolate* isolate;
  void* context_wire;
  void* scope_wire;
  void* source_wire;
};

struct AsyncFrame {
  v8::Isolate* isolate;
  void* context_wire;
  void* scope_wire;
  void* resolver_wire;
  void* result_wire;
  int32_t success;
  int32_t pad;
};

using GoAllowDispatcher = uintptr_t (*)(AllowFrame*);
using GoAsyncDispatcher = uintptr_t (*)(AsyncFrame*);

GoAllowDispatcher allow_dispatcher = nullptr;
GoAsyncDispatcher async_dispatcher = nullptr;

extern "C" {
void v8__Isolate__SetAllowWasmCodeGenerationCallback(
    v8::Isolate*, v8::AllowWasmCodeGenerationCallback);
void v8__Isolate__SetWasmAsyncResolvePromiseCallback(
    v8::Isolate*, v8::WasmAsyncResolvePromiseCallback);
}

class CallbackFrameScope {
 public:
  CallbackFrameScope(v8::Isolate* isolate, v8::Local<v8::Context> context)
      : scope_{} {
    scope_.magic = kScopeMagic;
    scope_.iso = isolate;
    GlobalPlacementNew<v8::HandleScope>(scope_.hs_storage, isolate);
	context_scope_.emplace(context);
  }
  ~CallbackFrameScope() {
	context_scope_.reset();
    scope_.hs()->~HandleScope();
    scope_.magic = 0;
  }
  GoScope* get() { return &scope_; }

 private:
  GoScope scope_;
	std::optional<v8::Context::Scope> context_scope_;
};

bool AllowTrampoline(v8::Local<v8::Context> context,
                     v8::Local<v8::String> source) {
  if (allow_dispatcher == nullptr || context.IsEmpty()) std::abort();
  v8::Isolate* isolate = v8::Isolate::GetCurrent();
  if (isolate == nullptr) std::abort();
  CallbackFrameScope callback_scope(isolate, context);
  AllowFrame frame{isolate, ToWire(context), callback_scope.get(),
                   ToWire(source)};
  return allow_dispatcher(&frame) != 0;
}

void AsyncTrampoline(v8::Isolate* isolate, v8::Local<v8::Context> context,
                     v8::Local<v8::Promise::Resolver> resolver,
                     v8::Local<v8::Value> result,
                     v8::WasmAsyncSuccess success) {
  if (async_dispatcher == nullptr || isolate == nullptr || context.IsEmpty() ||
      resolver.IsEmpty() || result.IsEmpty()) {
    std::abort();
  }
  CallbackFrameScope callback_scope(isolate, context);
  AsyncFrame frame{isolate,
                   ToWire(context),
                   callback_scope.get(),
                   ToWire(resolver),
                   ToWire(result),
                   success == v8::WasmAsyncSuccess::kSuccess ? 0 : 1,
                   0};
  if (async_dispatcher(&frame) != 0) std::abort();
}

}  // namespace gov8_wasm_policy_impl

extern "C" {

using namespace gov8_wasm_policy_impl;

__declspec(dllexport) int64_t gov8_wpc_set_dispatchers(void* allow,
                                                       void* async) {
  ClearErr();
  if (allow == nullptr || async == nullptr) return kErrBadArg;
  allow_dispatcher = reinterpret_cast<GoAllowDispatcher>(allow);
  async_dispatcher = reinterpret_cast<GoAsyncDispatcher>(async);
  return kOk;
}

__declspec(dllexport) int64_t gov8_wpc_set_allow(v8::Isolate* isolate) {
  ClearErr();
  if (isolate == nullptr || allow_dispatcher == nullptr) return kErrBadArg;
  try {
    v8__Isolate__SetAllowWasmCodeGenerationCallback(isolate, AllowTrampoline);
    return kOk;
  } catch (...) {
    SetErr("C++ exception setting Wasm code-generation callback");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wpc_set_async(v8::Isolate* isolate) {
  ClearErr();
  if (isolate == nullptr || async_dispatcher == nullptr) return kErrBadArg;
  try {
    v8__Isolate__SetWasmAsyncResolvePromiseCallback(isolate,
                                                    AsyncTrampoline);
    return kOk;
  } catch (...) {
    SetErr("C++ exception setting Wasm async-resolve callback");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wpc_context_global(
    v8::Isolate* isolate, void* context_wire, void* scope, void** out) {
  ClearErr();
  if (out != nullptr) *out = nullptr;
  if (isolate == nullptr || context_wire == nullptr || out == nullptr ||
      !ScopeIs(isolate, scope)) {
    return kErrBadArg;
  }
  try {
    v8::Local<v8::Context> context = FromWire<v8::Context>(context_wire);
    *out = ToWire(context->Global());
    return kOk;
  } catch (...) {
    SetErr("C++ exception getting Wasm callback context global");
    return kErrCpp;
  }
}

}  // extern "C"
