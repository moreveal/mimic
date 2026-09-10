// ===========================================================================
// fast_api_residual.inc -- native-only callbacks and observations for the
// residual Fast API conformance fixture.
//
// These entry points deliberately never dispatch a fast callback into Go.
// V8 calls the process-lifetime native functions below with its exact machine
// ABI; Go receives only addresses, counters and copied scalar observations.
// ===========================================================================

#include <atomic>
#include <cstddef>
#include <cstdint>
#include <limits>

namespace gov8_fast_api_residual_impl {

std::atomic<size_t> g_options_fast{0};
std::atomic<size_t> g_string_fast{0};
std::atomic<size_t> g_flag_fast{0};
std::atomic<size_t> g_shared_fast{0};
std::atomic<uint32_t> g_options_observations{0};
// Callback-options data is immutable for a FunctionTemplate. Remember the
// most recently observed V8 data object so steady-state benchmark calls do
// only the same counter increment and value computation as the Rust oracle.
// A different object (or a moved object after GC) is observed again, and the
// reset export clears the cache before each independent conformance case.
std::atomic<const void*> g_last_observed_options_data{nullptr};
uintptr_t g_external_sentinel = 0x51a7;

enum OptionObservation : uint32_t {
  kDataExternal = 1u << 0,
  kExternalPointerMatches = 1u << 1,
  kIsolateMatches = 1u << 2,
  kCurrentContextPresent = 1u << 3,
  kDataUndefined = 1u << 4,
};

void ObserveOptions(const v8::FastApiCallbackOptions& options) {
  uint32_t observations = 0;
  // gov8's slow-callback trampoline stores its numeric registry channel in
  // FunctionTemplate::Data and retains the public FunctionOptions.Data in the
  // channel's Global. Resolve that native-only indirection here; no borrowed
  // Local or Go pointer leaves this callback.
  v8::Local<v8::Value> data = options.data;
  if (options.isolate != nullptr) {
    if (GoCallbackContext* context =
            CallbackContextFromData(options.isolate, options.data)) {
      data = context->user_data.Get(options.isolate);
      if (data.IsEmpty()) data = v8::Undefined(options.isolate);
    }
  }
  if (!data.IsEmpty() && data->IsExternal()) {
    observations |= kDataExternal;
    if (data.As<v8::External>()->Value(
            v8::kExternalPointerTypeTagDefault) == &g_external_sentinel) {
      observations |= kExternalPointerMatches;
    }
  }
  if (!data.IsEmpty() && data->IsUndefined()) {
    observations |= kDataUndefined;
  }
  if (options.isolate != nullptr) {
    // rusty_v8's immutable and mutable accessors return this same pointer.
    observations |= kIsolateMatches;
    if (!options.isolate->GetCurrentContext().IsEmpty()) {
      observations |= kCurrentContextPresent;
    }
  }
  g_options_observations.fetch_or(observations, std::memory_order_seq_cst);
}

void ObserveOptionsOncePerData(
    const v8::FastApiCallbackOptions& options) {
  const v8::Local<v8::Value> data = options.data;
  const void* identity =
      data.IsEmpty() ? nullptr : static_cast<const void*>(data.operator->());
  const void* previous =
      g_last_observed_options_data.load(std::memory_order_relaxed);
  if (identity == nullptr || identity != previous) {
    ObserveOptions(options);
    g_last_observed_options_data.store(identity, std::memory_order_relaxed);
  }
}

uint32_t FastOptionsPointer(v8::Local<v8::Object>, uint32_t value,
                            v8::FastApiCallbackOptions* options) {
  g_options_fast.fetch_add(1, std::memory_order_seq_cst);
  if (options != nullptr) ObserveOptionsOncePerData(*options);
  return value + 1;
}

// Benchmark counterpart to rust-oracle's fast_increment. The callback-options
// parameter is part of the exercised Fast API ABI, but the timed Rust target
// deliberately does not inspect it. Keep that workload separate from
// FastOptionsPointer, whose observations are required by conformance tests.
uint32_t FastOptionsBenchmark(v8::Local<v8::Object>, uint32_t value,
                              v8::FastApiCallbackOptions*) {
  g_options_fast.fetch_add(1, std::memory_order_seq_cst);
  return value + 1;
}

uint32_t FastOptionsReference(v8::Local<v8::Object>, uint32_t value,
                              const v8::FastApiCallbackOptions& options) {
  g_options_fast.fetch_add(1, std::memory_order_seq_cst);
  ObserveOptions(options);
  return value + 1;
}

size_t OneByteLength(const v8::FastOneByteString& value) {
  return value.data == nullptr ? 0 : value.length;
}

uint32_t FastOneByte(v8::Local<v8::Object>,
                     const v8::FastOneByteString* input) {
  g_string_fast.fetch_add(1, std::memory_order_seq_cst);
  if (input == nullptr || input->data == nullptr) return 0;
  uint32_t sum = 0;
  for (uint32_t index = 0; index < input->length; ++index) {
    sum += (index + 1) * static_cast<uint8_t>(input->data[index]);
  }
  return sum;
}

int32_t FastInt32(v8::Local<v8::Object>, int32_t value) {
  g_flag_fast.fetch_add(1, std::memory_order_seq_cst);
  return value;
}

int32_t FastRestricted(v8::Local<v8::Object>, double value) {
  g_flag_fast.fetch_add(1, std::memory_order_seq_cst);
  return static_cast<int32_t>(value * 100.0);
}

uint32_t FastAllowShared(v8::Local<v8::Object>,
                         v8::Local<v8::Value> value) {
  g_shared_fast.fetch_add(1, std::memory_order_seq_cst);
  if (value->IsArrayBuffer()) return 1;
  if (value->IsSharedArrayBuffer()) return 2;
  return 3;
}

}  // namespace gov8_fast_api_residual_impl

extern "C" {

__declspec(dllexport) uintptr_t gov8_fast_api_residual_address(int32_t kind) {
  using namespace gov8_fast_api_residual_impl;
  switch (kind) {
    case 0:
      return reinterpret_cast<uintptr_t>(&FastOptionsPointer);
    case 1:
      return reinterpret_cast<uintptr_t>(&FastOptionsReference);
    case 2:
      return reinterpret_cast<uintptr_t>(&FastOneByte);
    case 3:
      return reinterpret_cast<uintptr_t>(&FastInt32);
    case 4:
      return reinterpret_cast<uintptr_t>(&FastRestricted);
    case 5:
      return reinterpret_cast<uintptr_t>(&FastAllowShared);
    case 6:
      return reinterpret_cast<uintptr_t>(&FastOptionsBenchmark);
    default:
      return 0;
  }
}

__declspec(dllexport) int64_t gov8_fast_api_residual_reset(void) {
  using namespace gov8_fast_api_residual_impl;
  g_options_fast.store(0, std::memory_order_seq_cst);
  g_string_fast.store(0, std::memory_order_seq_cst);
  g_flag_fast.store(0, std::memory_order_seq_cst);
  g_shared_fast.store(0, std::memory_order_seq_cst);
  g_options_observations.store(0, std::memory_order_seq_cst);
  g_last_observed_options_data.store(nullptr, std::memory_order_relaxed);
  return kOk;
}

__declspec(dllexport) uintptr_t gov8_fast_api_residual_counter(int32_t kind) {
  using namespace gov8_fast_api_residual_impl;
  switch (kind) {
    case 0:
      return g_options_fast.load(std::memory_order_seq_cst);
    case 1:
      return g_string_fast.load(std::memory_order_seq_cst);
    case 2:
      return g_flag_fast.load(std::memory_order_seq_cst);
    case 3:
      return g_shared_fast.load(std::memory_order_seq_cst);
    default:
      return 0;
  }
}

__declspec(dllexport) uintptr_t gov8_fast_api_residual_observations(void) {
  using namespace gov8_fast_api_residual_impl;
  return g_options_observations.load(std::memory_order_seq_cst);
}

__declspec(dllexport) uintptr_t gov8_fast_api_residual_external_sentinel(void) {
  using namespace gov8_fast_api_residual_impl;
  return reinterpret_cast<uintptr_t>(&g_external_sentinel);
}

__declspec(dllexport) uintptr_t gov8_fast_api_residual_layout(int32_t kind) {
  switch (kind) {
    case 0:
      return sizeof(v8::FastApiCallbackOptions);
    case 1:
      return alignof(v8::FastApiCallbackOptions);
    case 2:
      return sizeof(v8::FastOneByteString);
    case 3:
      return alignof(v8::FastOneByteString);
    default:
      return 0;
  }
}

__declspec(dllexport) int64_t gov8_fast_api_residual_direct_byte(
    int32_t index) {
  static constexpr char kBytes[] = {'a', '\0', static_cast<char>(0xff), 'z'};
  const v8::FastOneByteString direct{kBytes, 4};
  if (index < 0 || static_cast<uint32_t>(index) >= direct.length) return -1;
  return static_cast<uint8_t>(direct.data[index]);
}

__declspec(dllexport) uintptr_t gov8_fast_api_residual_null_length(
    uint32_t input_length) {
  const v8::FastOneByteString value{nullptr, input_length};
  return gov8_fast_api_residual_impl::OneByteLength(value);
}

}  // extern "C"
