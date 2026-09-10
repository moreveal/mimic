// v8-gn.h — embedder configuration header for the pinned V8 build.
//
// v8config.h requires this header (via -DV8_GN_HEADER) when the public V8
// headers are compiled outside the GN build. The defines below must replicate
// EXACTLY the external defines V8's GN build would generate for the pinned
// prebuilt static library (rusty_v8_release_x86_64-pc-windows-msvc.lib,
// v8 crate 152.2.0), because inline helper code in the public headers (e.g.
// v8-internal.h Internals constants) is compiled into every embedder TU and
// must agree with the engine binary.
//
// Derivation (see rust-oracle/README.md "Build configuration" and the crate's
// build.rs): the pinned artifact is built on Windows x64 with
//
//   is_debug=false  use_custom_libcxx=true
//   v8_enable_sandbox=false
//   v8_enable_pointer_compression=false
//   v8_enable_v8_checks=false
//
// and every other GN argument at its upstream default. The enabled/disabled
// sets below mirror the `enabled_external_defines` / `disabled_external_defines`
// lists computed by v8/BUILD.gn for that configuration, rendered in the same
// format the gen-v8-gn.py tool emits:
//
//   enabled  => #ifndef X / #define X <value> / #else #error ... / #endif
//   disabled => #ifdef X / #error "X is disabled by V8's GN build arguments"
//
// In particular note that cppgc (the C++ garbage collector for embedder
// heaps) keeps its own pointer compression enabled on x64 even though V8's
// main pointer compression is disabled — that asymmetry is upstream default
// behavior, faithfully reproduced here.
//
// AUTOMATICALLY DERIVED. DO NOT EDIT without re-deriving from v8/BUILD.gn +
// tools/gen-v8-gn.py for the pinned build configuration.

#ifndef V8_ARRAY_BUFFER_INTERNAL_FIELD_COUNT
#define V8_ARRAY_BUFFER_INTERNAL_FIELD_COUNT 0
#else
#if V8_ARRAY_BUFFER_INTERNAL_FIELD_COUNT != 0
#error "V8_ARRAY_BUFFER_INTERNAL_FIELD_COUNT defined but not set to 0"
#endif
#endif  // V8_ARRAY_BUFFER_INTERNAL_FIELD_COUNT

#ifndef V8_ARRAY_BUFFER_VIEW_INTERNAL_FIELD_COUNT
#define V8_ARRAY_BUFFER_VIEW_INTERNAL_FIELD_COUNT 0
#else
#if V8_ARRAY_BUFFER_VIEW_INTERNAL_FIELD_COUNT != 0
#error "V8_ARRAY_BUFFER_VIEW_INTERNAL_FIELD_COUNT defined but not set to 0"
#endif
#endif  // V8_ARRAY_BUFFER_VIEW_INTERNAL_FIELD_COUNT

#ifndef V8_PROMISE_INTERNAL_FIELD_COUNT
#define V8_PROMISE_INTERNAL_FIELD_COUNT 0
#else
#if V8_PROMISE_INTERNAL_FIELD_COUNT != 0
#error "V8_PROMISE_INTERNAL_FIELD_COUNT defined but not set to 0"
#endif
#endif  // V8_PROMISE_INTERNAL_FIELD_COUNT

#ifndef V8_USE_DEFAULT_HASHER_SECRET
#define V8_USE_DEFAULT_HASHER_SECRET true
#else
#if V8_USE_DEFAULT_HASHER_SECRET != true
#error "V8_USE_DEFAULT_HASHER_SECRET defined but not set to true"
#endif
#endif  // V8_USE_DEFAULT_HASHER_SECRET

#ifndef V8_HAVE_TARGET_OS
#define V8_HAVE_TARGET_OS 1
#else
#if V8_HAVE_TARGET_OS != 1
#error "V8_HAVE_TARGET_OS defined but not set to 1"
#endif
#endif  // V8_HAVE_TARGET_OS

#ifndef V8_TARGET_OS_WIN
#define V8_TARGET_OS_WIN 1
#else
#if V8_TARGET_OS_WIN != 1
#error "V8_TARGET_OS_WIN defined but not set to 1"
#endif
#endif  // V8_TARGET_OS_WIN

#ifndef CPPGC_ENABLE_LARGER_CAGE
#define CPPGC_ENABLE_LARGER_CAGE 1
#else
#if CPPGC_ENABLE_LARGER_CAGE != 1
#error "CPPGC_ENABLE_LARGER_CAGE defined but not set to 1"
#endif
#endif  // CPPGC_ENABLE_LARGER_CAGE

#ifndef CPPGC_CAGED_HEAP
#define CPPGC_CAGED_HEAP 1
#else
#if CPPGC_CAGED_HEAP != 1
#error "CPPGC_CAGED_HEAP defined but not set to 1"
#endif
#endif  // CPPGC_CAGED_HEAP

#ifndef CPPGC_YOUNG_GENERATION
#define CPPGC_YOUNG_GENERATION 1
#else
#if CPPGC_YOUNG_GENERATION != 1
#error "CPPGC_YOUNG_GENERATION defined but not set to 1"
#endif
#endif  // CPPGC_YOUNG_GENERATION

#ifndef CPPGC_POINTER_COMPRESSION
#define CPPGC_POINTER_COMPRESSION 1
#else
#if CPPGC_POINTER_COMPRESSION != 1
#error "CPPGC_POINTER_COMPRESSION defined but not set to 1"
#endif
#endif  // CPPGC_POINTER_COMPRESSION

#ifndef CPPGC_SLIM_WRITE_BARRIER
#define CPPGC_SLIM_WRITE_BARRIER 1
#else
#if CPPGC_SLIM_WRITE_BARRIER != 1
#error "CPPGC_SLIM_WRITE_BARRIER defined but not set to 1"
#endif
#endif  // CPPGC_SLIM_WRITE_BARRIER

#ifdef V8_ENABLE_CHECKS
#error "V8_ENABLE_CHECKS is defined but is disabled by V8's GN build arguments"
#endif  // V8_ENABLE_CHECKS

#ifdef V8_ENABLE_MEMORY_ACCOUNTING_CHECKS
#error "V8_ENABLE_MEMORY_ACCOUNTING_CHECKS is defined but is disabled by V8's GN build arguments"
#endif  // V8_ENABLE_MEMORY_ACCOUNTING_CHECKS

#ifdef V8_COMPRESS_POINTERS
#error "V8_COMPRESS_POINTERS is defined but is disabled by V8's GN build arguments"
#endif  // V8_COMPRESS_POINTERS

#ifdef V8_COMPRESS_POINTERS_IN_SHARED_CAGE
#error "V8_COMPRESS_POINTERS_IN_SHARED_CAGE is defined but is disabled by V8's GN build arguments"
#endif  // V8_COMPRESS_POINTERS_IN_SHARED_CAGE

#ifdef V8_31BIT_SMIS_ON_64BIT_ARCH
#error "V8_31BIT_SMIS_ON_64BIT_ARCH is defined but is disabled by V8's GN build arguments"
#endif  // V8_31BIT_SMIS_ON_64BIT_ARCH

#ifdef V8_COMPRESS_ZONES
#error "V8_COMPRESS_ZONES is defined but is disabled by V8's GN build arguments"
#endif  // V8_COMPRESS_ZONES

#ifdef V8_ENABLE_SANDBOX
#error "V8_ENABLE_SANDBOX is defined but is disabled by V8's GN build arguments"
#endif  // V8_ENABLE_SANDBOX

#ifdef V8_DEPRECATION_WARNINGS
#error "V8_DEPRECATION_WARNINGS is defined but is disabled by V8's GN build arguments"
#endif  // V8_DEPRECATION_WARNINGS

#ifdef V8_IMMINENT_DEPRECATION_WARNINGS
#error "V8_IMMINENT_DEPRECATION_WARNINGS is defined but is disabled by V8's GN build arguments"
#endif  // V8_IMMINENT_DEPRECATION_WARNINGS

#ifdef V8_USE_PERFETTO
#error "V8_USE_PERFETTO is defined but is disabled by V8's GN build arguments"
#endif  // V8_USE_PERFETTO

#ifdef V8_USE_PERFETTO_JSON_EXPORT
#error "V8_USE_PERFETTO_JSON_EXPORT is defined but is disabled by V8's GN build arguments"
#endif  // V8_USE_PERFETTO_JSON_EXPORT

#ifdef V8_USE_PERFETTO_SDK
#error "V8_USE_PERFETTO_SDK is defined but is disabled by V8's GN build arguments"
#endif  // V8_USE_PERFETTO_SDK

#ifdef V8_CPPGC_MICROTASK_QUEUE
#error "V8_CPPGC_MICROTASK_QUEUE is defined but is disabled by V8's GN build arguments"
#endif  // V8_CPPGC_MICROTASK_QUEUE

#ifdef V8_MAP_PACKING
#error "V8_MAP_PACKING is defined but is disabled by V8's GN build arguments"
#endif  // V8_MAP_PACKING

#ifdef V8_IS_TSAN
#error "V8_IS_TSAN is defined but is disabled by V8's GN build arguments"
#endif  // V8_IS_TSAN

#ifdef V8_ENABLE_DIRECT_HANDLE
#error "V8_ENABLE_DIRECT_HANDLE is defined but is disabled by V8's GN build arguments"
#endif  // V8_ENABLE_DIRECT_HANDLE

#ifdef V8_MINORMS_STRING_SHORTCUTTING
#error "V8_MINORMS_STRING_SHORTCUTTING is defined but is disabled by V8's GN build arguments"
#endif  // V8_MINORMS_STRING_SHORTCUTTING

#ifdef V8_TARGET_OS_ANDROID
#error "V8_TARGET_OS_ANDROID is defined but is disabled by V8's GN build arguments"
#endif  // V8_TARGET_OS_ANDROID

#ifdef V8_TARGET_OS_FUCHSIA
#error "V8_TARGET_OS_FUCHSIA is defined but is disabled by V8's GN build arguments"
#endif  // V8_TARGET_OS_FUCHSIA

#ifdef V8_TARGET_OS_IOS
#error "V8_TARGET_OS_IOS is defined but is disabled by V8's GN build arguments"
#endif  // V8_TARGET_OS_IOS

#ifdef V8_TARGET_OS_LINUX
#error "V8_TARGET_OS_LINUX is defined but is disabled by V8's GN build arguments"
#endif  // V8_TARGET_OS_LINUX

#ifdef V8_TARGET_OS_MACOS
#error "V8_TARGET_OS_MACOS is defined but is disabled by V8's GN build arguments"
#endif  // V8_TARGET_OS_MACOS

#ifdef V8_TARGET_OS_CHROMEOS
#error "V8_TARGET_OS_CHROMEOS is defined but is disabled by V8's GN build arguments"
#endif  // V8_TARGET_OS_CHROMEOS

#ifdef V8_TARGET_ARCH_ARM64
#error "V8_TARGET_ARCH_ARM64 is defined but is disabled by V8's GN build arguments"
#endif  // V8_TARGET_ARCH_ARM64

#ifdef V8_TARGET_ARCH_PPC64
#error "V8_TARGET_ARCH_PPC64 is defined but is disabled by V8's GN build arguments"
#endif  // V8_TARGET_ARCH_PPC64

#ifdef V8_TARGET_ARCH_MIPS64
#error "V8_TARGET_ARCH_MIPS64 is defined but is disabled by V8's GN build arguments"
#endif  // V8_TARGET_ARCH_MIPS64

#ifdef V8_TARGET_ARCH_LOONG64
#error "V8_TARGET_ARCH_LOONG64 is defined but is disabled by V8's GN build arguments"
#endif  // V8_TARGET_ARCH_LOONG64

#ifdef CPPGC_ENABLE_API_CHECKS
#error "CPPGC_ENABLE_API_CHECKS is defined but is disabled by V8's GN build arguments"
#endif  // CPPGC_ENABLE_API_CHECKS

#ifdef CPPGC_ENABLE_SLOW_API_CHECKS
#error "CPPGC_ENABLE_SLOW_API_CHECKS is defined but is disabled by V8's GN build arguments"
#endif  // CPPGC_ENABLE_SLOW_API_CHECKS

#ifdef CPPGC_SUPPORTS_OBJECT_NAMES
#error "CPPGC_SUPPORTS_OBJECT_NAMES is defined but is disabled by V8's GN build arguments"
#endif  // CPPGC_SUPPORTS_OBJECT_NAMES

#ifdef CPPGC_ENABLE_OBJECT_SECTION_GCINFO
#error "CPPGC_ENABLE_OBJECT_SECTION_GCINFO is defined but is disabled by V8's GN build arguments"
#endif  // CPPGC_ENABLE_OBJECT_SECTION_GCINFO
