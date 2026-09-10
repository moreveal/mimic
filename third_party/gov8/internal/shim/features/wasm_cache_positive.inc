// wasm_cache_positive.inc -- serialized CompiledWasmModule cache production.
//
// rusty_v8 152.2.0 does not expose CompiledWasmModule::Serialize even though
// the pinned V8 public header and library do. This narrow bridge is the Go
// producer counterpart needed to use ModuleCachingInterface safely.

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <v8-wasm.h>

extern "C" {

__declspec(dllexport) int64_t gov8_wcp_compiled_serialize(
    v8::CompiledWasmModule* compiled, uint8_t* output, size_t capacity,
    size_t* out_length) {
  ClearErr();
  if (compiled == nullptr || out_length == nullptr ||
      (output == nullptr && capacity != 0)) {
    SetErr("invalid compiled wasm serialization argument");
    return kErrBadArg;
  }
  try {
    v8::OwnedBuffer serialized = compiled->Serialize();
    *out_length = serialized.size;
    if (serialized.buffer == nullptr || serialized.size == 0) {
      SetErr("compiled wasm module is not serializable");
      return kErrState;
    }
    if (output == nullptr) return kOk;
    if (capacity < serialized.size) {
      SetErr("compiled wasm serialization destination too small");
      return kErrNoMemory;
    }
    std::memcpy(output, serialized.buffer.get(), serialized.size);
    return kOk;
  } catch (...) {
    SetErr("C++ exception serializing compiled wasm module");
    return kErrCpp;
  }
}

}  // extern "C"
