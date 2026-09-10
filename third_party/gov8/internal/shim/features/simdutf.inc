// simdutf.inc -- state-free wrappers for rusty_v8's pinned simdutf bindings.

#include <cstddef>
#include <cstdint>

namespace gov8_simdutf_impl {

struct SimdUtfFfiResult {
  int error;
  size_t count;
};

extern "C" {
bool simdutf__validate_utf8(const char*, size_t);
SimdUtfFfiResult simdutf__validate_utf8_with_errors(const char*, size_t);
bool simdutf__validate_ascii(const char*, size_t);
SimdUtfFfiResult simdutf__validate_ascii_with_errors(const char*, size_t);
bool simdutf__validate_utf16le(const char16_t*, size_t);
SimdUtfFfiResult simdutf__validate_utf16le_with_errors(const char16_t*, size_t);
bool simdutf__validate_utf16be(const char16_t*, size_t);
SimdUtfFfiResult simdutf__validate_utf16be_with_errors(const char16_t*, size_t);
bool simdutf__validate_utf32(const char32_t*, size_t);
SimdUtfFfiResult simdutf__validate_utf32_with_errors(const char32_t*, size_t);

size_t simdutf__convert_utf8_to_utf16le(const char*, size_t, char16_t*);
SimdUtfFfiResult simdutf__convert_utf8_to_utf16le_with_errors(const char*, size_t, char16_t*);
size_t simdutf__convert_valid_utf8_to_utf16le(const char*, size_t, char16_t*);
size_t simdutf__convert_utf16le_to_utf8(const char16_t*, size_t, char*);
SimdUtfFfiResult simdutf__convert_utf16le_to_utf8_with_errors(const char16_t*, size_t, char*);
size_t simdutf__convert_valid_utf16le_to_utf8(const char16_t*, size_t, char*);
size_t simdutf__convert_utf8_to_utf16be(const char*, size_t, char16_t*);
size_t simdutf__convert_utf16be_to_utf8(const char16_t*, size_t, char*);
size_t simdutf__convert_utf8_to_latin1(const char*, size_t, char*);
SimdUtfFfiResult simdutf__convert_utf8_to_latin1_with_errors(const char*, size_t, char*);
size_t simdutf__convert_valid_utf8_to_latin1(const char*, size_t, char*);
size_t simdutf__convert_latin1_to_utf8(const char*, size_t, char*);
size_t simdutf__convert_latin1_to_utf16le(const char*, size_t, char16_t*);
size_t simdutf__convert_utf16le_to_latin1(const char16_t*, size_t, char*);
size_t simdutf__convert_utf8_to_utf32(const char*, size_t, char32_t*);
size_t simdutf__convert_utf32_to_utf8(const char32_t*, size_t, char*);

size_t simdutf__utf8_length_from_utf16le(const char16_t*, size_t);
size_t simdutf__utf8_length_from_utf16be(const char16_t*, size_t);
size_t simdutf__utf16_length_from_utf8(const char*, size_t);
size_t simdutf__utf8_length_from_latin1(const char*, size_t);
size_t simdutf__latin1_length_from_utf8(const char*, size_t);
size_t simdutf__utf32_length_from_utf8(const char*, size_t);
size_t simdutf__utf8_length_from_utf32(const char32_t*, size_t);
size_t simdutf__utf16_length_from_utf32(const char32_t*, size_t);
size_t simdutf__utf32_length_from_utf16le(const char16_t*, size_t);
size_t simdutf__count_utf8(const char*, size_t);
size_t simdutf__count_utf16le(const char16_t*, size_t);
size_t simdutf__count_utf16be(const char16_t*, size_t);
int simdutf__detect_encodings(const char*, size_t);

size_t simdutf__maximal_binary_length_from_base64(const char*, size_t);
SimdUtfFfiResult simdutf__base64_to_binary(const char*, size_t, char*, uint64_t, uint64_t);
size_t simdutf__base64_length_from_binary(size_t, uint64_t);
size_t simdutf__binary_to_base64(const char*, size_t, char*, uint64_t);
}

}  // namespace gov8_simdutf_impl

extern "C" {

using namespace gov8_simdutf_impl;

// Fixed-purpose entry points keep the hottest operations to one compact ABI
// call. The generic exports below remain the complete public substrate.
__declspec(dllexport) int64_t gov8_simdutf_validate_utf8_fast(
    const char* input, size_t length) {
  if (input == nullptr && length != 0) {
    SetErr("invalid UTF-8 validation argument");
    return kErrBadArg;
  }
  const char empty = 0;
  if (input == nullptr) input = &empty;
  return simdutf__validate_utf8(input, length) ? 1 : 0;
}

__declspec(dllexport) int64_t gov8_simdutf_utf8_to_utf16le_fast(
    const char* input, size_t length, char16_t* output,
    size_t output_length) {
  if ((input == nullptr && length != 0) ||
      (output == nullptr && output_length != 0) || output_length < length) {
    SetErr("invalid UTF-8 to UTF-16LE conversion argument");
    return kErrBadArg;
  }
  const char empty_input = 0;
  char16_t empty_output = 0;
  if (input == nullptr) input = &empty_input;
  if (output == nullptr) output = &empty_output;
  return static_cast<int64_t>(
      simdutf__convert_utf8_to_utf16le(input, length, output));
}

__declspec(dllexport) int64_t gov8_simdutf_utf16le_to_utf8_fast(
    const char16_t* input, size_t length, char* output,
    size_t output_length) {
  if ((input == nullptr && length != 0) ||
      (output == nullptr && output_length != 0) ||
      length > SIZE_MAX / 3 || output_length < length * 3) {
    SetErr("invalid UTF-16LE to UTF-8 conversion argument");
    return kErrBadArg;
  }
  const char16_t empty_input = 0;
  char empty_output = 0;
  if (input == nullptr) input = &empty_input;
  if (output == nullptr) output = &empty_output;
  return static_cast<int64_t>(
      simdutf__convert_utf16le_to_utf8(input, length, output));
}

__declspec(dllexport) int64_t gov8_simdutf_base64_decode_fast(
    const char* input, size_t length, char* output, size_t output_length,
    uint64_t options, uint64_t last_chunk, int32_t* out_error) {
  if ((input == nullptr && length != 0) ||
      (output == nullptr && output_length != 0) || out_error == nullptr ||
      options > 3 || last_chunk > 3) {
    SetErr("invalid simdutf base64 decode argument");
    return kErrBadArg;
  }
  const char empty_input = 0;
  char empty_output = 0;
  if (input == nullptr) input = &empty_input;
  if (output == nullptr) output = &empty_output;
  // This is simdutf's scalar maximal_binary_length_from_base64 formula.
  // Keeping the safe capacity check here avoids a second runtime-selected
  // simdutf dispatch before the actual decode.
  size_t padding = 0;
  if (length > 0 && input[length - 1] == '=') {
    padding++;
    if (length > 1 && input[length - 2] == '=') padding++;
  }
  const size_t actual_length = length - padding;
  const size_t remainder = actual_length % 4;
  const size_t required = actual_length / 4 * 3 +
      (remainder <= 1 ? 0 : remainder - 1);
  if (output_length < required) {
    SetErr("simdutf destination too small");
    return kErrBadArg;
  }
  const auto result = simdutf__base64_to_binary(
      input, length, output, options, last_chunk);
  *out_error = result.error;
  return static_cast<int64_t>(result.count);
}

__declspec(dllexport) int64_t gov8_simdutf_base64_encode_fast(
    const char* input, size_t length, char* output, size_t output_length,
    uint64_t options) {
  if ((input == nullptr && length != 0) ||
      (output == nullptr && output_length != 0) || options > 3) {
    SetErr("invalid simdutf base64 encode argument");
    return kErrBadArg;
  }
  const char empty_input = 0;
  char empty_output = 0;
  if (input == nullptr) input = &empty_input;
  if (output == nullptr) output = &empty_output;
  // Values 0 and 3 use padding; 1 and 2 omit it. This is the exact simdutf
  // base64_length_from_binary formula, kept local to avoid an extra call.
  const bool use_padding = options == 0 || options == 3;
  const size_t required = use_padding
      ? (length + 2) / 3 * 4
      : length / 3 * 4 + (length % 3 == 0 ? 0 : length % 3 + 1);
  if (output_length < required) {
    SetErr("simdutf destination too small");
    return kErrBadArg;
  }
  return static_cast<int64_t>(
      simdutf__binary_to_base64(input, length, output, options));
}

__declspec(dllexport) int64_t gov8_simdutf_validate(
    int32_t kind, const void* input, size_t length, int32_t with_errors,
    int32_t* out_valid, int32_t* out_error, size_t* out_count) {
  ClearErr();
  if ((input == nullptr && length != 0) || out_valid == nullptr ||
      out_error == nullptr || out_count == nullptr || kind < 0 || kind > 4) {
    SetErr("invalid simdutf validation argument");
    return kErrBadArg;
  }
  const char empty8 = 0;
  const char16_t empty16 = 0;
  const char32_t empty32 = 0;
  const void* source = input;
  if (source == nullptr) {
    source = kind < 2 ? static_cast<const void*>(&empty8)
                      : kind < 4 ? static_cast<const void*>(&empty16)
                                 : static_cast<const void*>(&empty32);
  }
  if (with_errors != 0) {
    SimdUtfFfiResult result{};
    switch (kind) {
      case 0: result = simdutf__validate_utf8_with_errors(static_cast<const char*>(source), length); break;
      case 1: result = simdutf__validate_ascii_with_errors(static_cast<const char*>(source), length); break;
      case 2: result = simdutf__validate_utf16le_with_errors(static_cast<const char16_t*>(source), length); break;
      case 3: result = simdutf__validate_utf16be_with_errors(static_cast<const char16_t*>(source), length); break;
      default: result = simdutf__validate_utf32_with_errors(static_cast<const char32_t*>(source), length); break;
    }
    *out_error = result.error;
    *out_count = result.count;
    *out_valid = result.error == 0 ? 1 : 0;
    return kOk;
  }
  bool valid = false;
  switch (kind) {
    case 0: valid = simdutf__validate_utf8(static_cast<const char*>(source), length); break;
    case 1: valid = simdutf__validate_ascii(static_cast<const char*>(source), length); break;
    case 2: valid = simdutf__validate_utf16le(static_cast<const char16_t*>(source), length); break;
    case 3: valid = simdutf__validate_utf16be(static_cast<const char16_t*>(source), length); break;
    default: valid = simdutf__validate_utf32(static_cast<const char32_t*>(source), length); break;
  }
  *out_valid = valid ? 1 : 0;
  *out_error = valid ? 0 : 11;
  *out_count = valid ? length : 0;
  return kOk;
}

__declspec(dllexport) int64_t gov8_simdutf_convert(
    int32_t kind, const void* input, size_t length, void* output,
    size_t output_length, int32_t* out_error, size_t* out_count) {
  ClearErr();
  if ((input == nullptr && length != 0) || (output == nullptr && output_length != 0) ||
      out_error == nullptr || out_count == nullptr || kind < 0 || kind > 15) {
    SetErr("invalid simdutf conversion argument");
    return kErrBadArg;
  }
  size_t multiplier = 1;
  if (kind == 3 || kind == 4 || kind == 5 || kind == 7) multiplier = 3;
  if (kind == 11) multiplier = 2;
  if (kind == 15) multiplier = 4;
  if (length > SIZE_MAX / multiplier || output_length < length * multiplier) {
    SetErr("simdutf destination too small");
    return kErrBadArg;
  }
  char empty8 = 0;
  char16_t empty16 = 0;
  char32_t empty32 = 0;
  const void* source = input;
  void* destination = output;
  if (source == nullptr) {
    source = (kind == 3 || kind == 4 || kind == 5 || kind == 7 || kind == 13)
                 ? static_cast<const void*>(&empty16)
             : kind == 15 ? static_cast<const void*>(&empty32)
                          : static_cast<const void*>(&empty8);
  }
  if (destination == nullptr) {
    destination = (kind <= 2 || kind == 6 || kind == 12)
                      ? static_cast<void*>(&empty16)
                  : kind == 14 ? static_cast<void*>(&empty32)
                               : static_cast<void*>(&empty8);
  }
  *out_error = 0;
  switch (kind) {
    case 0: *out_count = simdutf__convert_utf8_to_utf16le(static_cast<const char*>(source), length, static_cast<char16_t*>(destination)); break;
    case 1: { auto r = simdutf__convert_utf8_to_utf16le_with_errors(static_cast<const char*>(source), length, static_cast<char16_t*>(destination)); *out_error = r.error; *out_count = r.count; break; }
    case 2: *out_count = simdutf__convert_valid_utf8_to_utf16le(static_cast<const char*>(source), length, static_cast<char16_t*>(destination)); break;
    case 3: *out_count = simdutf__convert_utf16le_to_utf8(static_cast<const char16_t*>(source), length, static_cast<char*>(destination)); break;
    case 4: { auto r = simdutf__convert_utf16le_to_utf8_with_errors(static_cast<const char16_t*>(source), length, static_cast<char*>(destination)); *out_error = r.error; *out_count = r.count; break; }
    case 5: *out_count = simdutf__convert_valid_utf16le_to_utf8(static_cast<const char16_t*>(source), length, static_cast<char*>(destination)); break;
    case 6: *out_count = simdutf__convert_utf8_to_utf16be(static_cast<const char*>(source), length, static_cast<char16_t*>(destination)); break;
    case 7: *out_count = simdutf__convert_utf16be_to_utf8(static_cast<const char16_t*>(source), length, static_cast<char*>(destination)); break;
    case 8: *out_count = simdutf__convert_utf8_to_latin1(static_cast<const char*>(source), length, static_cast<char*>(destination)); break;
    case 9: { auto r = simdutf__convert_utf8_to_latin1_with_errors(static_cast<const char*>(source), length, static_cast<char*>(destination)); *out_error = r.error; *out_count = r.count; break; }
    case 10: *out_count = simdutf__convert_valid_utf8_to_latin1(static_cast<const char*>(source), length, static_cast<char*>(destination)); break;
    case 11: *out_count = simdutf__convert_latin1_to_utf8(static_cast<const char*>(source), length, static_cast<char*>(destination)); break;
    case 12: *out_count = simdutf__convert_latin1_to_utf16le(static_cast<const char*>(source), length, static_cast<char16_t*>(destination)); break;
    case 13: *out_count = simdutf__convert_utf16le_to_latin1(static_cast<const char16_t*>(source), length, static_cast<char*>(destination)); break;
    case 14: *out_count = simdutf__convert_utf8_to_utf32(static_cast<const char*>(source), length, static_cast<char32_t*>(destination)); break;
    default: *out_count = simdutf__convert_utf32_to_utf8(static_cast<const char32_t*>(source), length, static_cast<char*>(destination)); break;
  }
  return kOk;
}

__declspec(dllexport) int64_t gov8_simdutf_measure(
    int32_t kind, const void* input, size_t length, size_t* out) {
  ClearErr();
  if ((input == nullptr && length != 0) || out == nullptr || kind < 0 || kind > 12) {
    SetErr("invalid simdutf measurement argument");
    return kErrBadArg;
  }
  const char empty8 = 0;
  const char16_t empty16 = 0;
  const char32_t empty32 = 0;
  const void* source = input;
  if (source == nullptr) {
    source = (kind == 0 || kind == 1 || kind == 8 || kind == 10 || kind == 11)
                 ? static_cast<const void*>(&empty16)
             : (kind == 6 || kind == 7) ? static_cast<const void*>(&empty32)
                                        : static_cast<const void*>(&empty8);
  }
  switch (kind) {
    case 0: *out = simdutf__utf8_length_from_utf16le(static_cast<const char16_t*>(source), length); break;
    case 1: *out = simdutf__utf8_length_from_utf16be(static_cast<const char16_t*>(source), length); break;
    case 2: *out = simdutf__utf16_length_from_utf8(static_cast<const char*>(source), length); break;
    case 3: *out = simdutf__utf8_length_from_latin1(static_cast<const char*>(source), length); break;
    case 4: *out = simdutf__latin1_length_from_utf8(static_cast<const char*>(source), length); break;
    case 5: *out = simdutf__utf32_length_from_utf8(static_cast<const char*>(source), length); break;
    case 6: *out = simdutf__utf8_length_from_utf32(static_cast<const char32_t*>(source), length); break;
    case 7: *out = simdutf__utf16_length_from_utf32(static_cast<const char32_t*>(source), length); break;
    case 8: *out = simdutf__utf32_length_from_utf16le(static_cast<const char16_t*>(source), length); break;
    case 9: *out = simdutf__count_utf8(static_cast<const char*>(source), length); break;
    case 10: *out = simdutf__count_utf16le(static_cast<const char16_t*>(source), length); break;
    case 11: *out = simdutf__count_utf16be(static_cast<const char16_t*>(source), length); break;
    default: *out = static_cast<size_t>(simdutf__detect_encodings(static_cast<const char*>(source), length)); break;
  }
  return kOk;
}

__declspec(dllexport) int64_t gov8_simdutf_base64(
    int32_t kind, const char* input, size_t length, char* output,
    size_t output_length, uint64_t options, uint64_t last_chunk,
    int32_t* out_error, size_t* out_count) {
  ClearErr();
  if ((kind != 2 && input == nullptr && length != 0) || (output == nullptr && output_length != 0) ||
      out_error == nullptr || out_count == nullptr || kind < 0 || kind > 3 ||
      options > 3 || last_chunk > 3) {
    SetErr("invalid simdutf base64 argument");
    return kErrBadArg;
  }
  char empty_input = 0;
  char empty_output = 0;
  if (input == nullptr) input = &empty_input;
  if (output == nullptr) output = &empty_output;
  *out_error = 0;
  if (kind == 0) {
    *out_count = simdutf__maximal_binary_length_from_base64(input, length);
  } else if (kind == 1) {
    const size_t required = simdutf__maximal_binary_length_from_base64(input, length);
    if (output_length < required) { SetErr("simdutf destination too small"); return kErrBadArg; }
    auto result = simdutf__base64_to_binary(input, length, output, options, last_chunk);
    *out_error = result.error;
    *out_count = result.count;
  } else if (kind == 2) {
    *out_count = simdutf__base64_length_from_binary(length, options);
  } else {
    const size_t required = simdutf__base64_length_from_binary(length, options);
    if (output_length < required) { SetErr("simdutf destination too small"); return kErrBadArg; }
    *out_count = simdutf__binary_to_base64(input, length, output, options);
  }
  return kOk;
}

}  // extern "C"
