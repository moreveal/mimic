// wasm_core.inc -- synchronous WebAssembly module and memory bindings.

#include <cstddef>
#include <cstdint>
#include <cstring>

namespace gov8_wasm_core_impl {

extern "C" {
const v8::WasmModuleObject* v8__WasmModuleObject__Compile(
    v8::Isolate*, uint8_t*, size_t);
const v8::WasmModuleObject* v8__WasmModuleObject__FromCompiledModule(
    v8::Isolate*, const v8::CompiledWasmModule*);
v8::CompiledWasmModule* v8__WasmModuleObject__GetCompiledModule(
    const v8::WasmModuleObject*);
const uint8_t* v8__CompiledWasmModule__GetWireBytesRef(
    v8::CompiledWasmModule*, size_t*);
const char* v8__CompiledWasmModule__SourceUrl(v8::CompiledWasmModule*, size_t*);
void v8__CompiledWasmModule__DELETE(v8::CompiledWasmModule*);
const v8::ArrayBuffer* v8__WasmMemoryObject__Buffer(
    const v8::WasmMemoryObject*);
bool v8__Value__IsWasmMemoryObject(const v8::Value*);
bool v8__Value__IsWasmModuleObject(const v8::Value*);
}

}  // namespace gov8_wasm_core_impl

extern "C" {

using namespace gov8_wasm_core_impl;

__declspec(dllexport) int64_t gov8_wasm_value_predicate(
    v8::Isolate* iso, void* scope, void* value_wire, int32_t kind,
    int32_t* out) {
  ClearErr();
  if (!ScopeIs(iso, scope) || value_wire == nullptr || out == nullptr ||
      kind < 0 || kind > 1) {
    SetErr("invalid wasm predicate argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    *out = (kind == 0 ? v8__Value__IsWasmModuleObject(
                            static_cast<const v8::Value*>(value_wire))
                      : v8__Value__IsWasmMemoryObject(
                            static_cast<const v8::Value*>(value_wire)))
               ? 1
               : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception checking wasm value");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wasm_module_compile(
    v8::Isolate* iso, void* ctxw, void* scope, void* tcw,
    const uint8_t* bytes, size_t length, void** out_module) {
  ClearErr();
  CtxWrap* context = AsCtx(ctxw);
  if (context == nullptr || !OwnedBy(iso, context->iso) ||
      !ScopeIs(iso, scope) || (bytes == nullptr && length != 0) ||
      out_module == nullptr) {
    SetErr("invalid wasm compile argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::Context> ctx = context->ctx->Get(iso);
    v8::Context::Scope context_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* try_catch = nullptr;
    if (tcw != nullptr) {
      TcWrap* wrap = AsTc(tcw);
      if (wrap == nullptr || !OwnedBy(iso, wrap->iso)) {
        SetErr("invalid trycatch handle");
        return kErrBadArg;
      }
      try_catch = TcOf(wrap);
    }
    if (try_catch == nullptr) try_catch = fallback.Init(iso);
    uint8_t empty = 0;
    uint8_t* data = const_cast<uint8_t*>(bytes == nullptr ? &empty : bytes);
    const v8::WasmModuleObject* result =
        v8__WasmModuleObject__Compile(iso, data, length);
    if (result == nullptr) {
      SetErr("wasm module compile failed");
      return kErrException;
    }
    *out_module = const_cast<v8::WasmModuleObject*>(result);
    return kOk;
  } catch (...) {
    SetErr("C++ exception compiling wasm module");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wasm_module_from_compiled(
    v8::Isolate* iso, void* ctxw, void* scope,
    v8::CompiledWasmModule* compiled, void** out_module) {
  ClearErr();
  CtxWrap* context = AsCtx(ctxw);
  if (context == nullptr || !OwnedBy(iso, context->iso) ||
      !ScopeIs(iso, scope) || compiled == nullptr || out_module == nullptr) {
    SetErr("invalid compiled wasm module argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    // The C bridge takes only isolate + compiled, but V8 consults the
    // isolate's current context internally while recreating the module.
    // Entering the caller's validated context is therefore required.
    v8::Local<v8::Context> ctx = context->ctx->Get(iso);
    v8::Context::Scope context_scope(ctx);
    const v8::WasmModuleObject* result =
        v8__WasmModuleObject__FromCompiledModule(iso, compiled);
    if (result == nullptr) {
      SetErr("wasm module recreation failed");
      return kErrGeneric;
    }
    *out_module = const_cast<v8::WasmModuleObject*>(result);
    return kOk;
  } catch (...) {
    SetErr("C++ exception recreating wasm module");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wasm_module_get_compiled(
    v8::Isolate* iso, void* scope, void* module_wire,
    v8::CompiledWasmModule** out_compiled) {
  ClearErr();
  if (!ScopeIs(iso, scope) || module_wire == nullptr ||
      out_compiled == nullptr) {
    SetErr("invalid wasm module argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    *out_compiled = v8__WasmModuleObject__GetCompiledModule(
        static_cast<const v8::WasmModuleObject*>(module_wire));
    if (*out_compiled == nullptr) {
      SetErr("GetCompiledModule returned null");
      return kErrGeneric;
    }
    return kOk;
  } catch (...) {
    SetErr("C++ exception getting compiled wasm module");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wasm_compiled_bytes(
    v8::CompiledWasmModule* compiled, int32_t source_url, uint8_t* output,
    size_t capacity, size_t* out_length) {
  ClearErr();
  if (compiled == nullptr || out_length == nullptr ||
      (output == nullptr && capacity != 0)) {
    SetErr("invalid compiled wasm module argument");
    return kErrBadArg;
  }
  try {
    size_t length = 0;
    const uint8_t* data = nullptr;
    if (source_url != 0) {
      data = reinterpret_cast<const uint8_t*>(
          v8__CompiledWasmModule__SourceUrl(compiled, &length));
    } else {
      data = v8__CompiledWasmModule__GetWireBytesRef(compiled, &length);
    }
    *out_length = length;
    if (data == nullptr && length != 0) {
      SetErr("compiled wasm returned null data");
      return kErrGeneric;
    }
    if (output == nullptr) return kOk;
    if (capacity < length) {
      SetErr("compiled wasm destination too small");
      return kErrBadArg;
    }
    if (length != 0) std::memcpy(output, data, length);
    return kOk;
  } catch (...) {
    SetErr("C++ exception reading compiled wasm module");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wasm_compiled_dispose(
    v8::CompiledWasmModule* compiled) {
  ClearErr();
  if (compiled == nullptr) {
    SetErr("invalid compiled wasm module");
    return kErrBadArg;
  }
  try {
    v8__CompiledWasmModule__DELETE(compiled);
    return kOk;
  } catch (...) {
    SetErr("C++ exception disposing compiled wasm module");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_wasm_memory_buffer(
    v8::Isolate* iso, void* ctxw, void* scope, void* memory_wire,
    void** out_buffer) {
  ClearErr();
  CtxWrap* context = AsCtx(ctxw);
  if (context == nullptr || !OwnedBy(iso, context->iso) ||
      !ScopeIs(iso, scope) || memory_wire == nullptr || out_buffer == nullptr) {
    SetErr("invalid wasm memory argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope isolate_scope(iso);
    v8::Local<v8::Context> ctx = context->ctx->Get(iso);
    v8::Context::Scope context_scope(ctx);
    const v8::ArrayBuffer* buffer =
        v8__WasmMemoryObject__Buffer(
            static_cast<const v8::WasmMemoryObject*>(memory_wire));
    if (buffer == nullptr) {
      SetErr("wasm memory buffer returned null");
      return kErrGeneric;
    }
    *out_buffer = const_cast<v8::ArrayBuffer*>(buffer);
    return kOk;
  } catch (...) {
    SetErr("C++ exception getting wasm memory buffer");
    return kErrCpp;
  }
}

}  // extern "C"
