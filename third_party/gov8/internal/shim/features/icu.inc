// icu.inc -- process-global ICU bindings exposed by rusty_v8 152.2.0.

#include <cstddef>
#include <cstdint>
#include <cstring>
#include <memory>
#include <mutex>
#include <new>
#include <vector>

namespace gov8_icu_impl {

extern "C" {
size_t icu_get_default_locale(char* output, size_t output_len);
void icu_set_default_locale(const char* locale);
size_t icu_get_default_time_zone(char* output, size_t output_len);
bool icu_set_default_time_zone(const char* time_zone_id);
void udata_setCommonData_78(const uint8_t* data, int32_t* error_code);
}

struct AlignedDataDelete {
  void operator()(uint8_t* data) const noexcept {
    ::operator delete[](data, std::align_val_t(16));
  }
};

using AlignedData = std::unique_ptr<uint8_t[], AlignedDataDelete>;

// ICU defaults and its common-data package list are process-global. Serialize
// all accesses made through gov8, including reads, so a concurrent setter
// cannot race a getter inside ICU. This cannot synchronize foreign ICU users.
std::mutex g_icu_mutex;

// ICU retains successful common-data pointers for the remainder of the
// process. The vector owns their 16-aligned native copies until DLL unload.
std::vector<AlignedData> g_common_data;

}  // namespace gov8_icu_impl

extern "C" {

using namespace gov8_icu_impl;

__declspec(dllexport) int64_t gov8_icu_set_common_data_78(
    const uint8_t* data, size_t length, int32_t* out_error_code,
    int32_t* out_pointer_mod_16) {
  ClearErr();
  if ((data == nullptr && length != 0) || out_error_code == nullptr ||
      out_pointer_mod_16 == nullptr) {
    SetErr("invalid ICU common-data argument");
    return kErrBadArg;
  }
  try {
    // udata_setCommonData has no length parameter and inspects a fixed header.
    // Pad short malformed inputs so ICU can reject them without reading beyond
    // the allocation. Successful packages continue to retain the exact bytes.
    const size_t allocation_length = length < 64 ? 64 : length;
    AlignedData copy(static_cast<uint8_t*>(::operator new[](
        allocation_length, std::align_val_t(16), std::nothrow)));
    if (!copy) {
      SetErr("ICU common-data allocation failed");
      return kErrNoMemory;
    }
    std::memset(copy.get(), 0, allocation_length);
    if (length != 0) std::memcpy(copy.get(), data, length);

    *out_pointer_mod_16 =
        static_cast<int32_t>(reinterpret_cast<uintptr_t>(copy.get()) % 16);

    std::lock_guard<std::mutex> lock(g_icu_mutex);

    // Publish ownership before ICU sees the pointer. Vector growth can throw,
    // but after udata_setCommonData succeeds there must be no allocating step
    // which could free memory ICU has retained.
    g_common_data.emplace_back(std::move(copy));
    int32_t error_code = 0;
    udata_setCommonData_78(g_common_data.back().get(), &error_code);
    *out_error_code = error_code;
    if (error_code != 0) g_common_data.pop_back();
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_icu_get_default(int32_t kind, char* output,
                                                   int64_t capacity) {
  ClearErr();
  if ((kind != 0 && kind != 1) || output == nullptr || capacity < 0) {
    SetErr("invalid ICU default getter argument");
    return kErrBadArg;
  }
  try {
    char buffer[1024];
    size_t length = 0;
    {
      std::lock_guard<std::mutex> lock(g_icu_mutex);
      length = kind == 0 ? icu_get_default_locale(buffer, sizeof(buffer))
                         : icu_get_default_time_zone(buffer, sizeof(buffer));
    }
    if (length > sizeof(buffer)) {
      SetErr("ICU returned an oversized string");
      return kErrCpp;
    }
    return CopyOut(buffer, length, output, capacity);
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_icu_set_default_locale(
    const char* locale) {
  ClearErr();
  if (locale == nullptr) {
    SetErr("null ICU locale");
    return kErrBadArg;
  }
  try {
    std::lock_guard<std::mutex> lock(g_icu_mutex);
    icu_set_default_locale(locale);
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_icu_set_default_time_zone(
    const char* time_zone_id, int32_t* out_accepted) {
  ClearErr();
  if (time_zone_id == nullptr || out_accepted == nullptr) {
    SetErr("invalid ICU time-zone argument");
    return kErrBadArg;
  }
  try {
    std::lock_guard<std::mutex> lock(g_icu_mutex);
    *out_accepted = icu_set_default_time_zone(time_zone_id) ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception");
    return kErrCpp;
  }
}

}  // extern "C"
