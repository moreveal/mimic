// ===========================================================================
// Core-advanced feature slice: escapable scopes, the SharedIsolate/Locker
// thread model, context enter/security-token/embedder-data/slots, raw and
// host isolate slots, script origins, the ScriptCompiler surface (unbound
// eager compile, CompileFunction, code caches with Go-side prevalidation),
// UnboundScript rebinding, Message/StackTrace/StackFrame, exception stack
// capture, interrupts, GC notifications, and heap statistics.
//
// All engine dispatch goes through the pinned artifact's own flat bindings
// (src/binding.cc of the pinned crate, compiled into the static library) or
// through vendored-header calls that link against symbols the artifact
// exports the same way the core shim already does (Script::Compile,
// Message position getters, CachedData::CompatibilityCheck).
//
// Exports (all invoked from Go through the dynamic proc lookup):
//   - EscapableHandleScope enter/escape/exit. The pinned crate's
//     "escape() called twice" guard is enforced on the GO side (the C++
//     Escape overwrites the slot silently in release builds); a second
//     escape never reaches the engine.
//   - Shared-isolate locking: conversion exit, v8::Locker construct/enter,
//     exit/destruct, the Unlocker window (Locker::unlock), Locker::IsLocked
//     mirroring, and dispose of a converted isolate (Dispose WITHOUT the
//     creation Exit, which the conversion already performed).
//   - Context enter/exit (v8__Context__Enter/Exit), current and
//     entered-or-microtask context reads with v8__Context__EQ identity,
//     security tokens (get/set/use-default) with v8__Value__SameValue,
//     embedder data values and aligned pointers. Embedder slot indices are
//     OFFSET BY 2 exactly like the pinned crate (its two internal slots:
//     annex + ANNEX_SLOT), and Go validates the index and the
//     value-vs-pointer kind of each slot before the engine is touched
//     (mixing them corrupts slot contents; upstream CHECKs or worse).
//   - Raw isolate data slots (GetNumberOfDataSlots/GetData/SetData) with a
//     Go-side bound check: upstream SetData/GetData with an out-of-range
//     slot is an unchecked OOB write/read in release V8.
//   - ScriptOrigin compile (including the is_module flag, which the pinned
//     build ApiCheck-fatals on for classic compiles -- characterized in a
//     dedicated subprocess by the Go negative tests, never in-process),
//     ScriptCompiler CompileUnboundScript/Compile/CompileFunction,
//     CachedDataVersionTag, UnboundScript bind/id/CreateCodeCache, and the
//     ConsumeCodeCache compile. Consumer cache bytes are prevalidated with
//     CachedData::CompatibilityCheck (graceful header sanity check) BEFORE
//     the deserializer is entered: a cache failing that check is rejected
//     with a Go error instead of the upstream deserializer fatal.
//   - Message detail getters (text, line, source line, resource name,
//     start/end positions and columns, error level, opaque/shared flags,
//     stack trace), StackTrace::CurrentStackTrace / frame accessors with a
//     Go-side index bound check (upstream GetFrame OOB is unchecked),
//     Exception::CaptureStackTrace / GetStackTrace, and
//     SetCaptureStackTraceForUncaughtExceptions.
//   - Interrupts (v8__Isolate__RequestInterrupt through a Go integer
//     registry, no Go pointers cross the boundary) and GC
//     prologue/epilogue callbacks with a GCType filter, plus heap
//     statistics (v8::HeapStatistics filled field-by-field, no layout
//     assumptions), AdjustAmountOfExternalAllocatedMemory and
//     NumberOfHeapSpaces.
//
// Wire validity: local-handle wires are object words valid in the CALLER's
// open HandleScope (the Go-owned scope, validated via ScopeIs for every
// call that creates or reads engine locals). Persistent wrappers carry a
// magic word so stale handles fail with kErrMagic.
// ===========================================================================

#include <cstring>
#include <vector>

namespace gov8_ca_impl {

constexpr uint64_t kEscMagic = 0x674f563845534350ULL;    // "gOV8ESCP"
constexpr uint64_t kLockMagic = 0x674f56384c4b4352ULL;   // "gOV8LKCR"
constexpr uint64_t kUnboundMagic = 0x674f5638554e4253ULL;  // "gOV8UNBS"

// The pinned crate reserves the first two engine embedder-data slots for
// its internal annex (ANNEX_SLOT + one more; Context::INTERNAL_SLOT_COUNT
// == 2) and offsets every user slot by that amount. The Go slice mirrors
// the offset so embedder-data observability matches the oracle exactly
// (including which raw slots hold what).
constexpr int kCtxInternalSlotCount = 2;

// The pinned crate's Isolate::INTERNAL_DATA_SLOT_COUNT: one engine isolate
// data slot is reserved for its annex; user slots are offset by this count
// and the reported slot count reduced by it.
constexpr int kIsolateInternalDataSlots = 1;

// --- artifact bindings used by this slice (compiled into the static lib) ---

extern "C" {
// Locker / Unlocker (src/binding.cc; static_asserts pin the sizes).
void v8__Locker__CONSTRUCT(void* buf, v8::Isolate* isolate);
void v8__Locker__DESTRUCT(v8::Locker* self);
bool v8__Locker__IsLocked(v8::Isolate* isolate);
void v8__Unlocker__CONSTRUCT(void* buf, v8::Isolate* isolate);
void v8__Unlocker__DESTRUCT(v8::Unlocker* self);
v8::Isolate* v8__Isolate__TryGetCurrent();

// Context surface.
void v8__Context__Enter(const v8::Context& self);
void v8__Context__Exit(const v8::Context& self);
bool v8__Context__EQ(const v8::Context& self, const v8::Context& other);
const v8::Context* v8__Isolate__GetCurrentContext(v8::Isolate* isolate);
const v8::Context* v8__Isolate__GetEnteredOrMicrotaskContext(
    v8::Isolate* isolate);
// Security-token bindings are NOT declared here: the templates slice owns
// that surface and calls the C++ methods directly.
uint32_t v8__Context__GetNumberOfEmbedderDataFields(const v8::Context& self);
const v8::Value* v8__Context__GetEmbedderData(const v8::Context& self,
                                              int index);
void v8__Context__SetEmbedderData(v8::Context& self, int index,
                                  const v8::Value* value);
void* v8__Context__GetAlignedPointerFromEmbedderData(const v8::Context& self,
                                                     int index);
void v8__Context__SetAlignedPointerInEmbedderData(v8::Context& self, int index,
                                                  void* value);

// Values.
bool v8__Value__SameValue(const v8::Value& self, const v8::Value& that);

// Isolate slots and heap controls.
uint32_t v8__Isolate__GetNumberOfDataSlots(v8::Isolate* isolate);
void* v8__Isolate__GetData(v8::Isolate* isolate, uint32_t slot);
void v8__Isolate__SetData(v8::Isolate* isolate, uint32_t slot, void* data);
void v8__Isolate__GetHeapStatistics(v8::Isolate* isolate,
                                    v8::HeapStatistics* s);
int64_t v8__Isolate__AdjustAmountOfExternalAllocatedMemory(
    v8::Isolate* isolate, int64_t change_in_bytes);
uint32_t v8__Isolate__NumberOfHeapSpaces(v8::Isolate* isolate);
void v8__Isolate__RequestInterrupt(v8::Isolate* isolate,
                                   v8::InterruptCallback callback, void* data);
void v8__Isolate__AddGCPrologueCallback(
    v8::Isolate* isolate, v8::Isolate::GCCallbackWithData callback, void* data,
    v8::GCType gc_type_filter);
void v8__Isolate__RemoveGCPrologueCallback(
    v8::Isolate* isolate, v8::Isolate::GCCallbackWithData callback, void* data);
void v8__Isolate__AddGCEpilogueCallback(
    v8::Isolate* isolate, v8::Isolate::GCCallbackWithData callback, void* data,
    v8::GCType gc_type_filter);
void v8__Isolate__RemoveGCEpilogueCallback(
    v8::Isolate* isolate, v8::Isolate::GCCallbackWithData callback, void* data);
void v8__Isolate__SetCaptureStackTraceForUncaughtExceptions(
    v8::Isolate* isolate, bool capture, int frame_limit);

// Script origin / compiler / unbound scripts.
const v8::Script* v8__Script__Compile(const v8::Context& context,
                                      const v8::String& source,
                                      const v8::ScriptOrigin& origin);
const v8::UnboundScript* v8__ScriptCompiler__CompileUnboundScript(
    v8::Isolate* isolate, v8::ScriptCompiler::Source* source,
    v8::ScriptCompiler::CompileOptions options,
    v8::ScriptCompiler::NoCacheReason no_cache_reason);
const v8::Script* v8__ScriptCompiler__Compile(
    const v8::Context* context, v8::ScriptCompiler::Source* source,
    v8::ScriptCompiler::CompileOptions options,
    v8::ScriptCompiler::NoCacheReason no_cache_reason);
const v8::Function* v8__ScriptCompiler__CompileFunction(
    const v8::Context* context, v8::ScriptCompiler::Source* source,
    size_t arguments_count, const v8::String** arguments,
    size_t context_extensions_count, const v8::Object** context_extensions,
    v8::ScriptCompiler::CompileOptions options,
    v8::ScriptCompiler::NoCacheReason no_cache_reason);
uint32_t v8__ScriptCompiler__CachedDataVersionTag();
v8::ScriptCompiler::CachedData* v8__ScriptCompiler__CachedData__NEW(
    const uint8_t* data, int length);
void v8__ScriptCompiler__CachedData__DELETE(
    v8::ScriptCompiler::CachedData* self);
const v8::ScriptCompiler::CachedData*
v8__ScriptCompiler__Source__GetCachedData(
    const v8::ScriptCompiler::Source* source);
void v8__ScriptCompiler__Source__CONSTRUCT(
    void* buf, const v8::String& source_string, const v8::ScriptOrigin* origin,
    v8::ScriptCompiler::CachedData* cached_data);
void v8__ScriptCompiler__Source__DESTRUCT(void* self);
const v8::Script* v8__UnboundScript__BindToCurrentContext(
    const v8::UnboundScript& unbound_script);
int v8__UnboundScript__ScriptId(const v8::UnboundScript& unbound_script);
v8::ScriptCompiler::CachedData* v8__UnboundScript__CreateCodeCache(
    const v8::UnboundScript& unbound_script);
const v8::Value* v8__Function__Call(const v8::Function& self,
                                    const v8::Context& context,
                                    const v8::Value& recv, int argc,
                                    const v8::Value* const argv[]);

// Exceptions / stack traces.
const v8::StackTrace* v8__Exception__GetStackTrace(
    const v8::Value& exception);
int32_t v8__Exception__CaptureStackTrace(const v8::Context& context,
                                         const v8::Object& object);
const v8::StackTrace* v8__StackTrace__CurrentStackTrace(v8::Isolate* isolate,
                                                        int frame_limit);
const v8::String* v8__StackTrace__CurrentScriptNameOrSourceURL(
    v8::Isolate* isolate);
int v8__StackTrace__GetFrameCount(const v8::StackTrace& self);
const v8::StackFrame* v8__StackTrace__GetFrame(const v8::StackTrace& self,
                                               v8::Isolate* isolate,
                                               uint32_t index);
const v8::String* v8__StackFrame__GetFunctionName(const v8::StackFrame& self);
const v8::String* v8__StackFrame__GetScriptName(const v8::StackFrame& self);
int v8__StackFrame__GetLineNumber(const v8::StackFrame& self);
int v8__StackFrame__GetColumn(const v8::StackFrame& self);
int v8__StackFrame__GetScriptId(const v8::StackFrame& self);
bool v8__StackFrame__IsEval(const v8::StackFrame& self);
bool v8__StackFrame__IsConstructor(const v8::StackFrame& self);
bool v8__StackFrame__IsWasm(const v8::StackFrame& self);
bool v8__StackFrame__IsUserJavaScript(const v8::StackFrame& self);

// Messages.
const v8::StackTrace* v8__Message__GetStackTrace(const v8::Message& self);
int v8__Message__GetLineNumber(const v8::Message& self,
                               const v8::Context& context);
const v8::String* v8__Message__GetSourceLine(const v8::Message& self,
                                             const v8::Context& context);
const v8::Value* v8__Message__GetScriptResourceName(const v8::Message& self);
int v8__Message__ErrorLevel(const v8::Message& self);
bool v8__Message__IsOpaque(const v8::Message& self);
bool v8__Message__IsSharedCrossOrigin(const v8::Message& self);
}

// CachedData::CompatibilityCheck runs the graceful header sanity check
// (magic / version / source / flags / payload length) without entering the
// code-cache deserializer: the safe prevalidation step for consumer cache
// bytes. Out-of-line in v8/src/api/api.cc of the pinned build, linked like
// the other vendored-header calls in this shim.
using SanityCheckResultType = int;

// --- wrapper structs --------------------------------------------------------

struct EscWrap {
  uint64_t magic;
  v8::Isolate* iso;
  // v8::EscapableHandleScope derives from HandleScope and inherits the
  // deleted class operator new; placement-constructed into this storage.
  alignas(v8::EscapableHandleScope)
      unsigned char esc_storage[sizeof(v8::EscapableHandleScope)];

  v8::EscapableHandleScope* esc() {
    return reinterpret_cast<v8::EscapableHandleScope*>(esc_storage);
  }
};

// One allocation backing either a live Locker or a live unlock window
// (never both at once; the Go state machine enforces that).
struct LockerWrap {
  uint64_t magic;
  v8::Isolate* iso;
  alignas(16) unsigned char locker_storage[sizeof(v8::Locker)];
  alignas(16) unsigned char unlocker_storage[sizeof(v8::Unlocker)];
};

struct UnboundWrap {
  uint64_t magic;
  v8::Isolate* iso;
  v8::Global<v8::UnboundScript>* unbound;
};

EscWrap* AsEsc(void* p) {
  if (p == nullptr) return nullptr;
  EscWrap* w = static_cast<EscWrap*>(p);
  return w->magic == kEscMagic ? w : nullptr;
}

LockerWrap* AsLocker(void* p) {
  if (p == nullptr) return nullptr;
  LockerWrap* w = static_cast<LockerWrap*>(p);
  return w->magic == kLockMagic ? w : nullptr;
}

UnboundWrap* AsUnbound(void* p) {
  if (p == nullptr) return nullptr;
  UnboundWrap* w = static_cast<UnboundWrap*>(p);
  return w->magic == kUnboundMagic ? w : nullptr;
}

// Resolves the optional TryCatch exactly like the core script exports:
// the caller's TryCatch (validated) receives thrown exceptions; with no
// TryCatch supplied a fallback swallows the exception so the isolate is
// never left with a poisoned pending exception.
inline bool CaResolveTc(v8::Isolate* iso, void* tcw, FallbackTryCatch* fallback,
                        v8::TryCatch** out_tc) {
  if (tcw == nullptr) {
    *out_tc = fallback->Init(iso);
    return true;
  }
  TcWrap* tw = AsTc(tcw);
  if (tw == nullptr) {
    SetErr("invalid trycatch handle");
    return false;
  }
  if (!OwnedBy(iso, tw->iso)) {
    return false;
  }
  *out_tc = TcOf(tw);
  return true;
}

// Origin flag bits shared by the compile exports (mirrors the Go Origin
// struct's bool fields packed by the wrapper).
constexpr int64_t kOriginFlagOpaque = 1;
constexpr int64_t kOriginFlagSharedCrossOrigin = 2;
constexpr int64_t kOriginFlagWasm = 4;
constexpr int64_t kOriginFlagModule = 8;

// Builds a v8::ScriptOrigin on the caller's C++ stack from the wire
// parameters. Strings may be absent (len <= 0 / null pointer): the source
// map URL is then omitted, mirroring the crate's None.
v8::ScriptOrigin CaMakeOrigin(v8::Local<v8::Value> name, int64_t line_offset,
                              int64_t column_offset, int64_t script_id,
                              v8::Local<v8::String> source_map,
                              int64_t flags) {
  v8::Local<v8::Value> smap_value = source_map;
  return v8::ScriptOrigin(
      name, static_cast<int>(line_offset), static_cast<int>(column_offset),
      (flags & kOriginFlagSharedCrossOrigin) != 0,
      static_cast<int>(script_id), smap_value,
      (flags & kOriginFlagOpaque) != 0, (flags & kOriginFlagWasm) != 0,
      (flags & kOriginFlagModule) != 0, v8::Local<v8::Data>());
}

// Creates the string locals the origin needs inside the CURRENT handle
// scope (the caller's Go scope or a shim-local one; the locals are
// consumed by the compile call in the same scope).
bool CaOriginStrings(v8::Isolate* iso, const char* name, int64_t name_len,
                     const char* smap, int64_t smap_len,
                     v8::Local<v8::String>* out_name,
                     v8::Local<v8::String>* out_smap) {
  if (name == nullptr || name_len <= 0) {
    SetErr("origin resource name required");
    return false;
  }
  if (!v8::String::NewFromUtf8(iso, name, v8::NewStringType::kNormal,
                               static_cast<int>(name_len))
           .ToLocal(out_name)) {
    SetErr("origin resource name allocation failed");
    return false;
  }
  if (smap != nullptr && smap_len > 0) {
    if (!v8::String::NewFromUtf8(iso, smap, v8::NewStringType::kNormal,
                                 static_cast<int>(smap_len))
             .ToLocal(out_smap)) {
      SetErr("origin source map url allocation failed");
      return false;
    }
  }
  return true;
}

// Resolves either the existing UTF-8 resource-name form or an arbitrary
// caller-owned Value wire. A raw wire takes precedence and is never coerced;
// the Go side has already validated its scope, isolate, and thread affinity.
bool CaOriginValues(v8::Isolate* iso, void* resource_value,
                    const char* name, int64_t name_len, const char* smap,
                    int64_t smap_len, bool required,
                    v8::Local<v8::Value>* out_name,
                    v8::Local<v8::String>* out_smap, bool* out_present) {
  if (out_name == nullptr || out_smap == nullptr || out_present == nullptr ||
      (name == nullptr && name_len > 0) ||
      (smap == nullptr && smap_len > 0)) {
    SetErr("invalid origin argument");
    return false;
  }
  *out_present = false;
  if (resource_value != nullptr) {
    *out_name = FromWire<v8::Value>(resource_value);
    *out_present = true;
  } else if (name != nullptr && name_len > 0) {
    v8::Local<v8::String> name_s;
    if (!v8::String::NewFromUtf8(iso, name, v8::NewStringType::kNormal,
                                 static_cast<int>(name_len))
             .ToLocal(&name_s)) {
      SetErr("origin resource name allocation failed");
      return false;
    }
    *out_name = name_s;
    *out_present = true;
  } else if (required) {
    SetErr("origin resource name required");
    return false;
  }
  if (*out_present && smap != nullptr && smap_len > 0) {
    if (!v8::String::NewFromUtf8(iso, smap, v8::NewStringType::kNormal,
                                 static_cast<int>(smap_len))
             .ToLocal(out_smap)) {
      SetErr("origin source map url allocation failed");
      return false;
    }
  }
  return true;
}

// --- Go trampolines for interrupts and GC callbacks --------------------------
//
// Same INTEGER-registry pattern as the weak finalizers: the engine only
// ever sees an integer id, and the single Go dispatch entry is installed
// once per process.

using CaInterruptEntry = void (*)(uintptr_t isolate, uintptr_t data);
CaInterruptEntry g_ca_interrupt_entry = nullptr;

void gov8_ca_interrupt_trampoline(v8::Isolate* isolate, void* data) {
  CaInterruptEntry entry = g_ca_interrupt_entry;
  if (entry != nullptr) {
    entry(reinterpret_cast<uintptr_t>(isolate),
          reinterpret_cast<uintptr_t>(data));
  }
}

using CaGcEntry = void (*)(uintptr_t isolate, int32_t gc_type, int32_t flags,
                           uintptr_t data);
CaGcEntry g_ca_gc_entry = nullptr;

void gov8_ca_gc_trampoline(v8::Isolate* isolate, v8::GCType gc_type,
                           v8::GCCallbackFlags flags, void* data) {
  CaGcEntry entry = g_ca_gc_entry;
  if (entry != nullptr) {
    entry(reinterpret_cast<uintptr_t>(isolate),
          static_cast<int32_t>(gc_type), static_cast<int32_t>(flags),
          reinterpret_cast<uintptr_t>(data));
  }
}

// The kEmbedderDataArrayMaxLength guard the engine applies on grow paths,
// mirrored Go-side as a conservative slot bound (prevents the upstream
// "Index too large" ApiCheck fatal from ever firing through this API).
constexpr int64_t kMaxEmbedderSlot = 4096;

// Shared tail of the compiler entry points: builds the Source on the C++
// stack (origin optional, cached data optional), calls `compile_fn`, and
// destroys the Source before returning. `origin_present`/`cached_len`
// select the Source flavor exactly like the pinned Source constructors.
template <class Out, class CompileFn>
// compile_fn receives the entered context Local plus the Source storage.
int64_t CaCompilerCall(v8::Isolate* iso, void* ctxw, void* scope, void* tcw,
                       const char* src, int64_t src_len, const char* name,
                       int64_t name_len, int64_t line_offset,
                       int64_t column_offset, int64_t script_id,
                       const char* smap, int64_t smap_len, int64_t flags,
                       int64_t options, const char* cache, int64_t cache_len,
                       CompileFn compile_fn, Out* out, int32_t* out_rejected) {
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if ((src == nullptr && src_len > 0) || (cache == nullptr && cache_len > 0)) {
    SetErr("null source buffer");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!CaResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::String> source;
    if (!v8::String::NewFromUtf8(iso, src, v8::NewStringType::kNormal,
                                 static_cast<int>(src_len))
             .ToLocal(&source)) {
      SetErr("String::NewFromUtf8 failed");
      return kErrGeneric;
    }
    v8::Local<v8::String> name_s;
    v8::Local<v8::String> smap_s;
    const bool has_origin = CaOriginStrings(iso, name, name_len, smap, smap_len,
                                            &name_s, &smap_s);
    if (!has_origin && !(name == nullptr && name_len <= 0)) {
      // Origin requested but its strings failed to allocate.
      return kErrBadArg;
    }
    v8::ScriptOrigin origin =
        has_origin ? CaMakeOrigin(name_s, line_offset, column_offset,
                                  script_id, smap_s, flags)
                   : v8::ScriptOrigin(v8::Local<v8::Value>());
    v8::ScriptCompiler::CachedData* cached = nullptr;
    if (cache != nullptr && cache_len > 0) {
      // BufferNotOwned: the Source's destructor deletes the CachedData
      // OBJECT only, never the caller's bytes.
      cached = v8__ScriptCompiler__CachedData__NEW(
          reinterpret_cast<const uint8_t*>(cache),
          static_cast<int>(cache_len));
      if (cached == nullptr) {
        SetErr("CachedData allocation failed");
        return kErrGeneric;
      }
    }
    alignas(v8::ScriptCompiler::Source)
        unsigned char src_storage[sizeof(v8::ScriptCompiler::Source)];
    v8__ScriptCompiler__Source__CONSTRUCT(
        src_storage, gov8_snap_impl::DerefLocal(source),
        has_origin ? &origin : nullptr, cached);
    const v8::ScriptCompiler::CompileOptions opt =
        options == 2 ? v8::ScriptCompiler::kEagerCompile
        : options == 1
            ? v8::ScriptCompiler::kConsumeCodeCache
            : v8::ScriptCompiler::kNoCompileOptions;
    const v8::ScriptCompiler::NoCacheReason reason =
        v8::ScriptCompiler::kNoCacheNoReason;
    bool ok = compile_fn(ctx, src_storage, opt, reason, out);
    if (out_rejected != nullptr) {
      const v8::ScriptCompiler::CachedData* cd =
          v8__ScriptCompiler__Source__GetCachedData(
              reinterpret_cast<const v8::ScriptCompiler::Source*>(src_storage));
      *out_rejected = (cd == nullptr) ? 1 : (cd->rejected ? 1 : 0);
    }
    v8__ScriptCompiler__Source__DESTRUCT(src_storage);
    if (!ok) {
      SetErr("compile failed");
      return kErrException;
    }
    return kOk;
  } catch (...) {
    SetErr("C++ exception in script compiler");
    return kErrCpp;
  }
}

}  // namespace gov8_ca_impl

extern "C" {

using namespace gov8_ca_impl;

// --- escapable handle scopes -------------------------------------------------

__declspec(dllexport) void* gov8_ca_esc_scope_enter(v8::Isolate* iso) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    EscWrap* w = new EscWrap{};
    w->magic = kEscMagic;
    w->iso = iso;
    GlobalPlacementNew<v8::EscapableHandleScope>(w->esc_storage, iso);
    return w;
  } catch (...) {
    SetErr("C++ exception in esc_scope_enter");
    return nullptr;
  }
}

// Second escapes are rejected on the Go side before reaching the engine
// (the C++ release-mode Escape silently overwrites the escape slot, which
// would corrupt the first escaped value -- the pinned crate's Option guard
// is reproduced in Go and surfaced as that exact error).
__declspec(dllexport) int64_t gov8_ca_esc_scope_escape(v8::Isolate* iso,
                                                       void* scope,
                                                       void* escw, void* v,
                                                       void** out) {
  ClearErr();
  EscWrap* w = AsEsc(escw);
  if (w == nullptr || out == nullptr || v == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    *out = ToWire(w->esc()->Escape(FromWire<v8::Value>(v)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in esc_scope_escape");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_esc_scope_exit(void* escw) {
  ClearErr();
  EscWrap* w = AsEsc(escw);
  if (w == nullptr) {
    SetErr("invalid escapable scope handle");
    return kErrMagic;
  }
  try {
    w->esc()->~EscapableHandleScope();
    delete w;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in esc_scope_exit");
    w->esc()->~EscapableHandleScope();
    delete w;
    return kErrCpp;
  }
}

// --- shared isolates / Locker -------------------------------------------------

// The pinned crate's conversion guard: Isolate::GetCurrent must be this
// isolate (the creation Enter makes it so on the owning thread; any other
// isolate entered on top, or a foreign thread, fails it).
__declspec(dllexport) v8::Isolate* gov8_ca_try_current(void) {
  return v8__Isolate__TryGetCurrent();
}

__declspec(dllexport) int64_t gov8_ca_is_locked(v8::Isolate* iso) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return kErrBadArg;
  }
  return v8__Locker__IsLocked(iso) ? 1 : 0;
}

// Conversion success path: exit the creation Enter exactly like the pinned
// OwnedIsolate::try_into_shared (the isolate is re-entered per Lock and
// exited on the final dispose).
__declspec(dllexport) int64_t gov8_ca_shared_convert(v8::Isolate* iso) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return kErrBadArg;
  }
  try {
    iso->Exit();
    return kOk;
  } catch (...) {
    SetErr("C++ exception in shared_convert");
    return kErrCpp;
  }
}

// Lock: construct the engine Locker (blocking while another thread holds
// it), then enter the isolate on the locking thread -- the exact pair the
// pinned SharedIsolate::lock performs. Returns the wrap the Go Locker
// holds until unlock.
__declspec(dllexport) void* gov8_ca_shared_lock(v8::Isolate* iso) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return nullptr;
  }
  LockerWrap* w = new LockerWrap{};
  w->magic = kLockMagic;
  w->iso = iso;
  try {
    v8__Locker__CONSTRUCT(w->locker_storage, iso);
    iso->Enter();
    return w;
  } catch (...) {
    SetErr("C++ exception in shared_lock");
    delete w;
    return nullptr;
  }
}

// Unlock: exit the isolate on the locking thread, then destroy the Locker
// (releasing the engine lock) -- Locker::drop in the pinned crate.
__declspec(dllexport) int64_t gov8_ca_shared_unlock(v8::Isolate* iso,
                                                    void* lockerw) {
  ClearErr();
  LockerWrap* w = AsLocker(lockerw);
  if (w == nullptr || iso == nullptr) {
    SetErr("invalid locker handle");
    return kErrMagic;
  }
  try {
    iso->Exit();
    v8__Locker__DESTRUCT(reinterpret_cast<v8::Locker*>(w->locker_storage));
    w->magic = 0;
    delete w;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in shared_unlock");
    w->magic = 0;
    delete w;
    return kErrCpp;
  }
}

// Unlock window begin: exit the isolate (V8 requires this before an
// Unlocker is constructed), then construct the Unlocker, releasing the
// engine lock so another thread can lock -- Locker::unlock's first half.
__declspec(dllexport) void* gov8_ca_shared_unlock_window_begin(
    v8::Isolate* iso) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return nullptr;
  }
  LockerWrap* w = new LockerWrap{};
  w->magic = kLockMagic;
  w->iso = iso;
  try {
    iso->Exit();
    v8__Unlocker__CONSTRUCT(w->unlocker_storage, iso);
    return w;
  } catch (...) {
    SetErr("C++ exception in unlock_window_begin");
    delete w;
    return nullptr;
  }
}

// Unlock window end: destroy the Unlocker (blocking until the engine lock
// is reacquirable) and re-enter the isolate -- Locker::unlock's second
// half, including the pinned crate's RelockGuard behavior on the relock
// path.
__declspec(dllexport) int64_t gov8_ca_shared_unlock_window_end(void* wraw) {
  ClearErr();
  LockerWrap* w = AsLocker(wraw);
  if (w == nullptr) {
    SetErr("invalid unlock window handle");
    return kErrMagic;
  }
  try {
    v8__Unlocker__DESTRUCT(reinterpret_cast<v8::Unlocker*>(w->unlocker_storage));
    w->iso->Enter();
    w->magic = 0;
    delete w;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in unlock_window_end");
    w->magic = 0;
    delete w;
    return kErrCpp;
  }
}

// Dispose of a converted isolate: the conversion already exited the
// creation Enter, and every lock cycle exits after itself, so Dispose
// runs WITHOUT another Exit (an Exit here would pop a foreign isolate's
// entry level or CHECK-fail on an empty stack).
__declspec(dllexport) int64_t gov8_ca_shared_isolate_dispose(
    v8::Isolate* iso) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return kErrBadArg;
  }
  try {
    iso->Dispose();
    return kOk;
  } catch (...) {
    SetErr("C++ exception in shared_isolate_dispose");
    return kErrCpp;
  }
}

// --- context enter / current / identity ---------------------------------------

__declspec(dllexport) int64_t gov8_ca_context_enter(v8::Isolate* iso,
                                                    void* ctxw) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8__Context__Enter(gov8_snap_impl::DerefLocal(w->ctx->Get(iso)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_enter");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_context_exit(v8::Isolate* iso,
                                                   void* ctxw) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8__Context__Exit(gov8_snap_impl::DerefLocal(w->ctx->Get(iso)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_exit");
    return kErrCpp;
  }
}

// Enter/exit a borrowed scope-local Context wire. The owning Go Scope is
// validated on every transition and kept open by the Go ContextScope guard.
__declspec(dllexport) int64_t gov8_ca_context_enter_local(
    v8::Isolate* iso, void* scope, void* context_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope) || context_wire == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8__Context__Enter(
        gov8_snap_impl::DerefLocal(FromWire<v8::Context>(context_wire)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_enter_local");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_context_exit_local(
    v8::Isolate* iso, void* scope, void* context_wire) {
  ClearErr();
  if (!ScopeIs(iso, scope) || context_wire == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8__Context__Exit(
        gov8_snap_impl::DerefLocal(FromWire<v8::Context>(context_wire)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_exit_local");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_context_global_local(
    v8::Isolate* iso, void* scope, void* context_wire, void** out) {
  ClearErr();
  if (!ScopeIs(iso, scope) || context_wire == nullptr || out == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  *out = nullptr;
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> context = FromWire<v8::Context>(context_wire);
    *out = ToWire(context->Global());
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_global_local");
    return kErrCpp;
  }
}

// Current / entered-or-microtask context as a wire in the CALLER's open
// HandleScope (0 when the engine reports none).
__declspec(dllexport) int64_t gov8_ca_context_current(v8::Isolate* iso,
                                                      void* scope,
                                                      void** out) {
  ClearErr();
  if (out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    const v8::Context* p = v8__Isolate__GetCurrentContext(iso);
    *out = const_cast<void*>(static_cast<const void*>(p));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_current");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_context_entered_or_microtask(
    v8::Isolate* iso, void* scope, void** out) {
  ClearErr();
  if (out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    const v8::Context* p = v8__Isolate__GetEnteredOrMicrotaskContext(iso);
    *out = const_cast<void*>(static_cast<const void*>(p));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_entered_or_microtask");
    return kErrCpp;
  }
}

// The context's own wire in the CALLER's open HandleScope (identity
// comparisons against CurrentContext/EnteredOrMicrotaskContext).
__declspec(dllexport) int64_t gov8_ca_context_local(v8::Isolate* iso,
                                                    void* ctxw, void* scope,
                                                    void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    *out = ToWire(w->ctx->Get(iso));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_local");
    return kErrCpp;
  }
}

// Identity comparison of two context wires (v8__Context__EQ compares the
// local handles' pointees). Pure pointer reads.
__declspec(dllexport) int64_t gov8_ca_context_eq(void* a, void* b) {
  ClearErr();
  if (a == nullptr || b == nullptr) {
    SetErr("null context wire");
    return kErrBadArg;
  }
  try {
    v8::Local<v8::Context> ca = FromWire<v8::Context>(a);
    v8::Local<v8::Context> cb = FromWire<v8::Context>(b);
    return v8__Context__EQ(gov8_snap_impl::DerefLocal(ca),
                           gov8_snap_impl::DerefLocal(cb))
               ? 1
               : 0;
  } catch (...) {
    SetErr("C++ exception in context_eq");
    return kErrCpp;
  }
}

// SameValue of two value wires (security-token comparison and object
// identity checks). Read-only.
__declspec(dllexport) int64_t gov8_ca_value_same_value(v8::Isolate* iso,
                                                       void* a, void* b) {
  ClearErr();
  if (a == nullptr || b == nullptr) {
    SetErr("null value wire");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> va = FromWire<v8::Value>(a);
    v8::Local<v8::Value> vb = FromWire<v8::Value>(b);
    return v8__Value__SameValue(gov8_snap_impl::DerefLocal(va),
                                gov8_snap_impl::DerefLocal(vb))
               ? 1
               : 0;
  } catch (...) {
    SetErr("C++ exception in value_same_value");
    return kErrCpp;
  }
}

// --- security tokens -------------------------------------------------------------
//
// Security-token get/set/use-default live in the templates slice
// (gov8_context_get_security_token and friends); this slice reuses that Go
// surface and adds only the SameValue comparison (gov8_ca_value_same_value
// above) the token checks need.

// --- embedder data ------------------------------------------------------------
//
// Slot indices arrive PRE-OFFSET from Go (Go adds kCtxInternalSlotCount)
// and are validated here against the array bounds the engine would
// ApiCheck on. The Go wrapper additionally rejects negative slots and
// value/pointer kind mixing; the shim re-checks the bound so even a
// bypassing caller cannot reach the ApiCheck fatal.

__declspec(dllexport) int64_t gov8_ca_context_embedder_field_count(
    v8::Isolate* iso, void* ctxw, int64_t* out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    *out = static_cast<int64_t>(v8__Context__GetNumberOfEmbedderDataFields(
        gov8_snap_impl::DerefLocal(w->ctx->Get(iso))));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_embedder_field_count");
    return kErrCpp;
  }
}

// ok receives 0 when the engine returned an empty local (unset array);
// a present value stores the wire. Note the engine's fresh-slot default
// is an internal oddball, not a JS value: only predicates are meaningful
// on it (the pinned oracle records exactly those).
__declspec(dllexport) int64_t gov8_ca_context_get_embedder_data(
    v8::Isolate* iso, void* ctxw, void* scope, int64_t slot, void** out,
    int32_t* ok) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || ok == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (slot < 0 || slot >= kMaxEmbedderSlot) {
    SetErr("embedder data slot out of range");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    const v8::Value* p = v8__Context__GetEmbedderData(
        gov8_snap_impl::DerefLocal(w->ctx->Get(iso)), static_cast<int>(slot));
    if (p == nullptr) {
      *ok = 0;
      return kOk;
    }
    *out = const_cast<void*>(static_cast<const void*>(p));
    *ok = 1;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_get_embedder_data");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_context_set_embedder_data(
    v8::Isolate* iso, void* ctxw, void* scope, int64_t slot, void* v) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || v == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (slot < 0 || slot >= kMaxEmbedderSlot) {
    SetErr("embedder data slot out of range");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8__Context__SetEmbedderData(gov8_snap_impl::DerefLocal(w->ctx->Get(iso)),
                                 static_cast<int>(slot),
                                 &gov8_snap_impl::DerefLocal(
                                     FromWire<v8::Value>(v)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_set_embedder_data");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_context_get_aligned_pointer(
    v8::Isolate* iso, void* ctxw, int64_t slot, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (slot < 0 || slot >= kMaxEmbedderSlot) {
    SetErr("embedder data slot out of range");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    *out = v8__Context__GetAlignedPointerFromEmbedderData(
        gov8_snap_impl::DerefLocal(w->ctx->Get(iso)), static_cast<int>(slot));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_get_aligned_pointer");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_context_set_aligned_pointer(
    v8::Isolate* iso, void* ctxw, int64_t slot, void* p) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (slot < 0 || slot >= kMaxEmbedderSlot) {
    SetErr("embedder data slot out of range");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8__Context__SetAlignedPointerInEmbedderData(
        gov8_snap_impl::DerefLocal(w->ctx->Get(iso)), static_cast<int>(slot),
        p);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in context_set_aligned_pointer");
    return kErrCpp;
  }
}

// --- isolate raw data slots -------------------------------------------------------
//
// The pinned crate reserves the FIRST engine data slot for its internal
// annex (Isolate::INTERNAL_DATA_SLOT_COUNT == 1) and offsets every user
// slot by that amount; GetNumberOfDataSlots is reported minus the same
// count. The Go wrapper validates the slot against the reported count
// (upstream Set/GetData with an out-of-range slot is an unchecked OOB in
// release V8); this file re-checks so even a bypassing caller cannot reach
// it.

__declspec(dllexport) int64_t gov8_ca_isolate_data_slot_count(
    v8::Isolate* iso, int64_t* out) {
  ClearErr();
  if (iso == nullptr || out == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  // Mirror the pinned crate: report the engine count minus its one
  // reserved internal slot.
  *out = static_cast<int64_t>(v8__Isolate__GetNumberOfDataSlots(iso)) -
         kIsolateInternalDataSlots;
  return kOk;
}

__declspec(dllexport) int64_t gov8_ca_isolate_get_data(v8::Isolate* iso,
                                                       int64_t slot,
                                                       void** out) {
  ClearErr();
  if (iso == nullptr || out == nullptr || slot < 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  const int64_t n = static_cast<int64_t>(v8__Isolate__GetNumberOfDataSlots(iso)) -
                    kIsolateInternalDataSlots;
  if (slot >= n) {
    SetErr("isolate data slot out of range");
    return kErrBadArg;
  }
  *out = v8__Isolate__GetData(iso,
                              static_cast<uint32_t>(slot + kIsolateInternalDataSlots));
  return kOk;
}

__declspec(dllexport) int64_t gov8_ca_isolate_set_data(v8::Isolate* iso,
                                                       int64_t slot,
                                                       void* data) {
  ClearErr();
  if (iso == nullptr || slot < 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  const int64_t n = static_cast<int64_t>(v8__Isolate__GetNumberOfDataSlots(iso)) -
                    kIsolateInternalDataSlots;
  if (slot >= n) {
    SetErr("isolate data slot out of range");
    return kErrBadArg;
  }
  v8__Isolate__SetData(iso,
                       static_cast<uint32_t>(slot + kIsolateInternalDataSlots),
                       data);
  return kOk;
}

// --- script origins -----------------------------------------------------------------

// Compile with a full ScriptOrigin. flags packs is_opaque (1),
// is_shared_cross_origin (2), is_wasm (4), is_module (8). A module-flagged
// origin handed to this classic compile ApiCheck-fatals inside the pinned
// engine; the Go negative tests characterize that boundary in a dedicated
// subprocess and the Go wrapper keeps is_module reachable for exactly that
// characterization.
__declspec(dllexport) int64_t gov8_ca_script_compile_origin(
    v8::Isolate* iso, void* ctxw, void* scope, void* tcw, const char* src,
    int64_t src_len, const char* name, int64_t name_len, int64_t line_offset,
    int64_t column_offset, int64_t script_id, const char* smap,
    int64_t smap_len, int64_t flags, void** out_script) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out_script == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if ((src == nullptr && src_len > 0) || (name == nullptr && name_len > 0)) {
    SetErr("null source buffer");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!CaResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::String> source;
    if (!v8::String::NewFromUtf8(iso, src, v8::NewStringType::kNormal,
                                 static_cast<int>(src_len))
             .ToLocal(&source)) {
      SetErr("String::NewFromUtf8 failed");
      return kErrGeneric;
    }
    // Origin strings live in the caller's open HandleScope; both the
    // origin object and the compile call run in this same scope.
    v8::Local<v8::String> name_s;
    v8::Local<v8::String> smap_s;
    if (!CaOriginStrings(iso, name, name_len, smap, smap_len, &name_s,
                         &smap_s)) {
      return kErrBadArg;
    }
    v8::ScriptOrigin origin = CaMakeOrigin(name_s, line_offset, column_offset,
                                           script_id, smap_s, flags);
    const v8::Script* compiled = v8__Script__Compile(
        gov8_snap_impl::DerefLocal(ctx), gov8_snap_impl::DerefLocal(source),
        origin);
    if (compiled == nullptr) {
      SetErr("compile failed");
      return kErrException;
    }
    v8::Local<v8::Script> script;
    std::memcpy(&script, &compiled, sizeof(void*));
    ScriptWrap* sw = new ScriptWrap{};
    sw->magic = kScriptMagic;
    sw->iso = iso;
    sw->script = new v8::Global<v8::Script>(iso, script);
    *out_script = sw;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in script_compile_origin");
    return kErrCpp;
  }
}

// Arbitrary-Value resource-name counterpart. The wire is used verbatim so
// Message::GetScriptResourceName preserves its type and identity.
__declspec(dllexport) int64_t gov8_ca_script_compile_origin_value(
    v8::Isolate* iso, void* ctxw, void* scope, void* tcw, const char* src,
    int64_t src_len, const char* name, int64_t name_len, void* resource_value,
    int64_t line_offset, int64_t column_offset, int64_t script_id,
    const char* smap, int64_t smap_len, int64_t flags, void** out_script) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || resource_value == nullptr || out_script == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (src == nullptr && src_len > 0) {
    SetErr("null source buffer");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!CaResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::String> source;
    if (!v8::String::NewFromUtf8(iso, src, v8::NewStringType::kNormal,
                                 static_cast<int>(src_len))
             .ToLocal(&source)) {
      SetErr("String::NewFromUtf8 failed");
      return kErrGeneric;
    }
    v8::Local<v8::Value> name_v;
    v8::Local<v8::String> smap_s;
    bool has_origin = false;
    if (!CaOriginValues(iso, resource_value, name, name_len, smap, smap_len,
                        true, &name_v, &smap_s, &has_origin)) {
      return kErrBadArg;
    }
    v8::ScriptOrigin origin = CaMakeOrigin(
        name_v, line_offset, column_offset, script_id, smap_s, flags);
    const v8::Script* compiled = v8__Script__Compile(
        gov8_snap_impl::DerefLocal(ctx), gov8_snap_impl::DerefLocal(source),
        origin);
    if (compiled == nullptr) {
      SetErr("compile failed");
      return kErrException;
    }
    v8::Local<v8::Script> script;
    std::memcpy(&script, &compiled, sizeof(void*));
    ScriptWrap* sw = new ScriptWrap{};
    sw->magic = kScriptMagic;
    sw->iso = iso;
    sw->script = new v8::Global<v8::Script>(iso, script);
    *out_script = sw;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in script_compile_origin_value");
    return kErrCpp;
  }
}

// --- unbound scripts ------------------------------------------------------------------

// Roots the compiled script's UnboundScript in a persistent wrapper.
__declspec(dllexport) void* gov8_ca_unbound_from_script(void* scriptw) {
  ClearErr();
  ScriptWrap* s = AsScript(scriptw);
  if (s == nullptr) {
    SetErr("invalid script handle");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(s->iso);
    v8::HandleScope hs(s->iso);
    v8::Local<v8::Script> script = s->script->Get(s->iso);
    v8::Local<v8::UnboundScript> unbound = script->GetUnboundScript();
    if (unbound.IsEmpty()) {
      SetErr("GetUnboundScript failed");
      return nullptr;
    }
    UnboundWrap* w = new UnboundWrap{};
    w->magic = kUnboundMagic;
    w->iso = s->iso;
    w->unbound = new v8::Global<v8::UnboundScript>(s->iso, unbound);
    return w;
  } catch (...) {
    SetErr("C++ exception in unbound_from_script");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_ca_unbound_id(v8::Isolate* iso,
                                                 void* unboundw, int32_t* out) {
  ClearErr();
  UnboundWrap* w = AsUnbound(unboundw);
  if (w == nullptr || out == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    *out = v8__UnboundScript__ScriptId(
        gov8_snap_impl::DerefLocal(w->unbound->Get(iso)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in unbound_id");
    return kErrCpp;
  }
}

// Binds into the CURRENTLY ENTERED context; the wire lives in the CALLER's
// open HandleScope. No nested HandleScope may be opened here: the bound
// script's local belongs to the caller's scope and would dangle with a
// shim-local one. The Go wrapper enforces the entered-context
// precondition at runtime (upstream it is enforced by the type system).
__declspec(dllexport) int64_t gov8_ca_unbound_bind(v8::Isolate* iso,
                                                   void* scope, void* unboundw,
                                                   void** out_wire) {
  ClearErr();
  UnboundWrap* w = AsUnbound(unboundw);
  if (w == nullptr || out_wire == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    const v8::Script* p = v8__UnboundScript__BindToCurrentContext(
        gov8_snap_impl::DerefLocal(w->unbound->Get(iso)));
    if (p == nullptr) {
      SetErr("BindToCurrentContext failed");
      return kErrGeneric;
    }
    *out_wire = const_cast<void*>(static_cast<const void*>(p));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in unbound_bind");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_unbound_dispose(void* unboundw) {
  ClearErr();
  UnboundWrap* w = AsUnbound(unboundw);
  if (w == nullptr) {
    SetErr("invalid unbound handle");
    return kErrMagic;
  }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    w->unbound->Reset();
    delete w->unbound;
    delete w;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in unbound_dispose");
    return kErrCpp;
  }
}

// Runs a LOCAL Script wire (the result of unbound_bind) in the context.
// TryCatch routing matches gov8_script_run.
__declspec(dllexport) int64_t gov8_ca_local_script_run(v8::Isolate* iso,
                                                       void* ctxw,
                                                       void* scope,
                                                       void* script_wire,
                                                       void* tcw,
                                                       void** out_result) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || script_wire == nullptr || AsScope(scope) == nullptr ||
      out_result == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!CaResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Script> script = FromWire<v8::Script>(script_wire);
    v8::Local<v8::Value> result;
    if (!script->Run(ctx).ToLocal(&result)) {
      SetErr("run failed");
      return kErrException;
    }
    *out_result = ToWire(result);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in local_script_run");
    return kErrCpp;
  }
}

// --- ScriptCompiler ------------------------------------------------------------------

// CompileUnboundScript with options (0 = NoCompileOptions, 2 = EagerCompile).
// out_wrap receives an UnboundWrap (persistent Global<UnboundScript>).
__declspec(dllexport) int64_t gov8_ca_compile_unbound(
    v8::Isolate* iso, void* ctxw, void* scope, void* tcw, const char* src,
    int64_t src_len, const char* name, int64_t name_len, int64_t line_offset,
    int64_t column_offset, int64_t script_id, const char* smap,
    int64_t smap_len, int64_t flags, int64_t options, void** out_wrap) {
  return CaCompilerCall(
      iso, ctxw, scope, tcw, src, src_len, name, name_len, line_offset,
      column_offset, script_id, smap, smap_len, flags, options, nullptr, 0,
      [iso](v8::Local<v8::Context> ctx, void* source,
            v8::ScriptCompiler::CompileOptions opt,
            v8::ScriptCompiler::NoCacheReason reason, void** out) {
        const v8::UnboundScript* p = v8__ScriptCompiler__CompileUnboundScript(
            iso, static_cast<v8::ScriptCompiler::Source*>(source), opt,
            reason);
        if (p == nullptr) {
          return false;
        }
        v8::Local<v8::UnboundScript> unbound;
        std::memcpy(&unbound, &p, sizeof(void*));
        UnboundWrap* w = new UnboundWrap{};
        w->magic = kUnboundMagic;
        w->iso = iso;
        w->unbound = new v8::Global<v8::UnboundScript>(iso, unbound);
        *out = w;
        return true;
      },
      out_wrap, nullptr);
}

// Compile (classic) with options; used for the ConsumeCodeCache path.
// out_script receives a ScriptWrap; out_rejected reports the cache's
// rejected flag after the compile.
__declspec(dllexport) int64_t gov8_ca_script_compile_cached(
    v8::Isolate* iso, void* ctxw, void* scope, void* tcw, const char* src,
    int64_t src_len, const char* name, int64_t name_len, int64_t line_offset,
    int64_t column_offset, int64_t script_id, const char* smap,
    int64_t smap_len, int64_t flags, int64_t options, const char* cache,
    int64_t cache_len, void** out_script, int32_t* out_rejected) {
  return CaCompilerCall(
      iso, ctxw, scope, tcw, src, src_len, name, name_len, line_offset,
      column_offset, script_id, smap, smap_len, flags, options, cache,
      cache_len,
      [iso](v8::Local<v8::Context> ctx, void* source,
            v8::ScriptCompiler::CompileOptions opt,
            v8::ScriptCompiler::NoCacheReason reason, void** out) {
        const v8::Script* p = v8__ScriptCompiler__Compile(
            &gov8_snap_impl::DerefLocal(ctx),
            static_cast<v8::ScriptCompiler::Source*>(source), opt, reason);
        if (p == nullptr) {
          return false;
        }
        v8::Local<v8::Script> script;
        std::memcpy(&script, &p, sizeof(void*));
        ScriptWrap* sw = new ScriptWrap{};
        sw->magic = kScriptMagic;
        sw->iso = iso;
        sw->script = new v8::Global<v8::Script>(iso, script);
        *out = sw;
        return true;
      },
      out_script, out_rejected);
}

// CompileFunction with declared parameter names. out_wire receives the
// function (a scope-local wire).
__declspec(dllexport) int64_t gov8_ca_compile_function(
    v8::Isolate* iso, void* ctxw, void* scope, void* tcw, const char* src,
    int64_t src_len, const char* const* params, int64_t nparams,
    void** out_wire) {
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out_wire == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (src == nullptr && src_len > 0) {
    SetErr("null source buffer");
    return kErrBadArg;
  }
  if (params == nullptr && nparams > 0) {
    SetErr("null params buffer");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!CaResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::String> source;
    if (!v8::String::NewFromUtf8(iso, src, v8::NewStringType::kNormal,
                                 static_cast<int>(src_len))
             .ToLocal(&source)) {
      SetErr("String::NewFromUtf8 failed");
      return kErrGeneric;
    }
    // Parameter strings are created in the CALLER's open HandleScope (no
    // nested scope here: the returned function wire must belong to the
    // caller's scope).
    std::vector<v8::Local<v8::String>> args;
    args.reserve(static_cast<size_t>(nparams));
    for (int64_t i = 0; i < nparams; i++) {
      const char* p = params[i];
      if (p == nullptr) {
        SetErr("null parameter name");
        return kErrBadArg;
      }
      v8::Local<v8::String> s;
      if (!v8::String::NewFromUtf8(iso, p, v8::NewStringType::kNormal, -1)
               .ToLocal(&s)) {
        SetErr("parameter name allocation failed");
        return kErrGeneric;
      }
      args.push_back(s);
    }
    alignas(v8::ScriptCompiler::Source)
        unsigned char src_storage[sizeof(v8::ScriptCompiler::Source)];
    v8__ScriptCompiler__Source__CONSTRUCT(
        src_storage, gov8_snap_impl::DerefLocal(source), nullptr, nullptr);
    const v8::Function* p = v8__ScriptCompiler__CompileFunction(
        &gov8_snap_impl::DerefLocal(ctx),
        reinterpret_cast<v8::ScriptCompiler::Source*>(src_storage),
        args.size(),
        reinterpret_cast<const v8::String**>(args.data()), 0, nullptr,
        v8::ScriptCompiler::kNoCompileOptions,
        v8::ScriptCompiler::kNoCacheNoReason);
    v8__ScriptCompiler__Source__DESTRUCT(src_storage);
    if (p == nullptr) {
      SetErr("compile function failed");
      return kErrException;
    }
    *out_wire = const_cast<void*>(static_cast<const void*>(p));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in compile_function");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_cached_data_version_tag(int64_t* out) {
  ClearErr();
  if (out == nullptr) {
    SetErr("null out param");
    return kErrBadArg;
  }
  *out = static_cast<int64_t>(v8__ScriptCompiler__CachedDataVersionTag());
  return kOk;
}

// Function::Call with a persistent context. TryCatch routing matches
// script_run; a missed maybe yields kErrException.
__declspec(dllexport) int64_t gov8_ca_function_call(v8::Isolate* iso,
                                                    void* ctxw, void* scope,
                                                    void* fn, void* recv,
                                                    void* argv, int64_t argc,
                                                    void* tcw, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || fn == nullptr || recv == nullptr ||
      AsScope(scope) == nullptr || out == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (argv == nullptr && argc > 0) {
    SetErr("null argv");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!CaResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    // No nested HandleScope: the returned function-call result wire must
    // belong to the CALLER's open scope.
    v8::Local<v8::Value> fnv = FromWire<v8::Value>(fn);
    if (!fnv->IsFunction()) {
      SetErr("target is not a function");
      return kErrBadArg;
    }
    v8::Local<v8::Value> recvv = FromWire<v8::Value>(recv);
    v8::Local<v8::Value>* args =
        reinterpret_cast<v8::Local<v8::Value>*>(argv);
    const v8::Value* p = v8__Function__Call(
        gov8_snap_impl::DerefLocal(fnv.As<v8::Function>()),
        gov8_snap_impl::DerefLocal(ctx), gov8_snap_impl::DerefLocal(recvv),
        static_cast<int>(argc),
        reinterpret_cast<const v8::Value* const*>(args));
    if (p == nullptr) {
      SetErr("function call failed");
      return kErrException;
    }
    *out = const_cast<void*>(static_cast<const void*>(p));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in function_call");
    return kErrCpp;
  }
}

// --- code caches ------------------------------------------------------------------------

// Produces the code cache of an unbound script. Returns the engine's
// CachedData object (BufferOwned); consume the bytes with
// gov8_ca_code_cache_read_delete, which frees it.
__declspec(dllexport) void* gov8_ca_code_cache_new(void* unboundw) {
  ClearErr();
  UnboundWrap* w = AsUnbound(unboundw);
  if (w == nullptr) {
    SetErr("invalid unbound handle");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(w->iso);
    v8::HandleScope hs(w->iso);
    return v8__UnboundScript__CreateCodeCache(
        gov8_snap_impl::DerefLocal(w->unbound->Get(w->iso)));
  } catch (...) {
    SetErr("C++ exception in code_cache_new");
    return nullptr;
  }
}

// Copies the cache bytes into the Go buffer and deletes the engine
// CachedData (its BufferOwned payload with it).
__declspec(dllexport) int64_t gov8_ca_code_cache_read_delete(void* cd,
                                                             char* buf,
                                                             int64_t cap,
                                                             int64_t* out_len) {
  ClearErr();
  if (cd == nullptr || buf == nullptr || out_len == nullptr || cap < 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  v8::ScriptCompiler::CachedData* data =
      static_cast<v8::ScriptCompiler::CachedData*>(cd);
  *out_len = static_cast<int64_t>(data->length);
  if (static_cast<int64_t>(data->length) > cap) {
    v8__ScriptCompiler__CachedData__DELETE(data);
    SetErr("output buffer too small");
    return kErrNoMemory;
  }
  if (data->length > 0 && data->data != nullptr) {
    std::memcpy(buf, data->data, static_cast<size_t>(data->length));
  }
  v8__ScriptCompiler__CachedData__DELETE(data);
  return kOk;
}

// Prevalidates consumer cache bytes with CachedData::CompatibilityCheck
// (the graceful header sanity check) BEFORE the ConsumeCodeCache
// deserializer is entered. Returns the raw sanity-check result (0 =
// compatible) or a negative shim status. Bytes shorter than the minimum
// serialized-code header are rejected without touching the engine. This is
// the corruption guard: mid-payload corruption that passes the header
// checks is NOT detectable without running the deserializer (which in this
// pinned build fatal-aborts instead of rejecting) -- that residual boundary
// is characterized in the Go subprocess tests rather than prevented.
__declspec(dllexport) int64_t gov8_ca_code_cache_precheck(v8::Isolate* iso,
                                                          const char* p,
                                                          int64_t len) {
  ClearErr();
  if (iso == nullptr || (p == nullptr && len > 0)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  // SerializedCodeData headers are well below this; anything shorter is
  // garbage by construction.
  if (len < 32) {
    SetErr("code cache too short");
    return kErrBadArg;
  }
  try {
    v8::ScriptCompiler::CachedData* cd = v8__ScriptCompiler__CachedData__NEW(
        reinterpret_cast<const uint8_t*>(p), static_cast<int>(len));
    if (cd == nullptr) {
      SetErr("CachedData allocation failed");
      return kErrGeneric;
    }
    const int result = static_cast<SanityCheckResultType>(
        cd->CompatibilityCheck(iso));
    v8__ScriptCompiler__CachedData__DELETE(cd);
    return result;
  } catch (...) {
    SetErr("C++ exception in code_cache_precheck");
    return kErrCpp;
  }
}

// --- messages -----------------------------------------------------------------------------

// The TryCatch's Message as a wire in the CALLER's open HandleScope
// (0 when nothing was caught).
__declspec(dllexport) int64_t gov8_ca_tc_message(v8::Isolate* iso, void* tcw,
                                                 void* scope, void** out) {
  ClearErr();
  TcWrap* t = AsTc(tcw);
  if (t == nullptr || !ScopeIs(iso, scope) || out == nullptr ||
      !OwnedBy(iso, t->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Message> msg = t->tc()->Message();
    *out = ToWire(msg);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in tc_message");
    return kErrCpp;
  }
}

// Message text (Message::Get -> ToString, lossy UTF-8).
__declspec(dllexport) int64_t gov8_ca_message_text(v8::Isolate* iso,
                                                   void* ctxw, void* scope,
                                                   void* msg, char* buf,
                                                   int64_t cap,
                                                   int64_t* out_len) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || msg == nullptr || out_len == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    std::string text;
    if (!ValueText(iso, ctx, FromWire<v8::Message>(msg)->Get(), &text)) {
      return kErrException;
    }
    return BufferUtf8(text, buf, cap, out_len);
  } catch (...) {
    SetErr("C++ exception in message_text");
    return kErrCpp;
  }
}

// Message::Get as a String wire in the CALLER's open HandleScope. Unlike
// gov8_ca_message_text this preserves the engine value without conversion.
__declspec(dllexport) int64_t gov8_ca_message_get_value(v8::Isolate* iso,
                                                        void* scope,
                                                        void* msg,
                                                        void** out) {
  ClearErr();
  if (msg == nullptr || out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    *out = ToWire(FromWire<v8::Message>(msg)->Get());
    return kOk;
  } catch (...) {
    SetErr("C++ exception in message_get_value");
    return kErrCpp;
  }
}

#define GOV8_CA_MSG_SIMPLE(name, expr)                                         \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* scope, void* msg) { \
    ClearErr();                                                                \
    if (msg == nullptr || !ScopeIs(iso, scope)) {                              \
      SetErr("invalid argument");                                              \
      return kErrBadArg;                                                       \
    }                                                                          \
    try {                                                                      \
      Gov8IsolateScope iso_scope(iso);                                         \
      v8::HandleScope hs(iso);                                                 \
      v8::Local<v8::Message> m = FromWire<v8::Message>(msg);                   \
      return static_cast<int64_t>(expr);                                       \
    } catch (...) {                                                            \
      SetErr("C++ exception in message getter");                               \
      return kErrCpp;                                                          \
    }                                                                          \
  }

GOV8_CA_MSG_SIMPLE(gov8_ca_message_start_position, m->GetStartPosition())
GOV8_CA_MSG_SIMPLE(gov8_ca_message_end_position, m->GetEndPosition())
GOV8_CA_MSG_SIMPLE(gov8_ca_message_start_column, m->GetStartColumn())
GOV8_CA_MSG_SIMPLE(gov8_ca_message_end_column, m->GetEndColumn())
GOV8_CA_MSG_SIMPLE(gov8_ca_message_error_level,
                   v8__Message__ErrorLevel(gov8_snap_impl::DerefLocal(m)))
GOV8_CA_MSG_SIMPLE(gov8_ca_message_is_opaque,
                   v8__Message__IsOpaque(gov8_snap_impl::DerefLocal(m)) ? 1 : 0)
GOV8_CA_MSG_SIMPLE(
    gov8_ca_message_is_shared_cross_origin,
    v8__Message__IsSharedCrossOrigin(gov8_snap_impl::DerefLocal(m)) ? 1 : 0)

#undef GOV8_CA_MSG_SIMPLE

// 1-based line; ok=0 when the Maybe is empty (v8__Message__GetLineNumber
// maps Nothing to -1, so -1 means absent).
__declspec(dllexport) int64_t gov8_ca_message_line_number(v8::Isolate* iso,
                                                          void* ctxw,
                                                          void* scope,
                                                          void* msg,
                                                          int32_t* out,
                                                          int32_t* ok) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || msg == nullptr || out == nullptr || ok == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    const int line = v8__Message__GetLineNumber(
        gov8_snap_impl::DerefLocal(FromWire<v8::Message>(msg)),
        gov8_snap_impl::DerefLocal(w->ctx->Get(iso)));
    *out = static_cast<int32_t>(line);
    *ok = line >= 0 ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in message_line_number");
    return kErrCpp;
  }
}

// Source line (lossy UTF-8); ok=0 when absent.
__declspec(dllexport) int64_t gov8_ca_message_source_line(v8::Isolate* iso,
                                                          void* ctxw,
                                                          void* scope,
                                                          void* msg, char* buf,
                                                          int64_t cap,
                                                          int64_t* out_len,
                                                          int32_t* ok) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || msg == nullptr || out_len == nullptr || ok == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::String* p = v8__Message__GetSourceLine(
        gov8_snap_impl::DerefLocal(FromWire<v8::Message>(msg)),
        gov8_snap_impl::DerefLocal(ctx));
    if (p == nullptr) {
      *ok = 0;
      return BufferUtf8(std::string(), buf, cap, out_len);
    }
    v8::Local<v8::String> s;
    std::memcpy(&s, &p, sizeof(void*));
    const size_t n = s->Utf8Length(iso);
    std::string text;
    text.resize(n);
    if (n > 0) {
      const size_t written = s->WriteUtf8(
          iso, text.data(), n, v8::String::WriteFlags::kReplaceInvalidUtf8);
      text.resize(written);
    }
    *ok = 1;
    return BufferUtf8(text, buf, cap, out_len);
  } catch (...) {
    SetErr("C++ exception in message_source_line");
    return kErrCpp;
  }
}

// Source line as a String wire in the CALLER's open HandleScope. A null wire
// is the binding's Option::None; an empty String remains a non-null wire.
__declspec(dllexport) int64_t gov8_ca_message_source_line_value(
    v8::Isolate* iso, void* ctxw, void* scope, void* msg, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || msg == nullptr || out == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::String* p = v8__Message__GetSourceLine(
        gov8_snap_impl::DerefLocal(FromWire<v8::Message>(msg)),
        gov8_snap_impl::DerefLocal(ctx));
    *out = const_cast<void*>(static_cast<const void*>(p));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in message_source_line_value");
    return kErrCpp;
  }
}

// Script resource name (lossy UTF-8 of the value; "" when absent).
__declspec(dllexport) int64_t gov8_ca_message_resource_name(v8::Isolate* iso,
                                                            void* ctxw,
                                                            void* scope,
                                                            void* msg,
                                                            char* buf,
                                                            int64_t cap,
                                                            int64_t* out_len) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || msg == nullptr || out_len == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Value* p = v8__Message__GetScriptResourceName(
        gov8_snap_impl::DerefLocal(FromWire<v8::Message>(msg)));
    if (p == nullptr) {
      return BufferUtf8(std::string(), buf, cap, out_len);
    }
    v8::Local<v8::Value> v;
    std::memcpy(&v, &p, sizeof(void*));
    std::string text;
    if (!ValueText(iso, ctx, v, &text)) {
      return kErrException;
    }
    return BufferUtf8(text, buf, cap, out_len);
  } catch (...) {
    SetErr("C++ exception in message_resource_name");
    return kErrCpp;
  }
}

// Script resource name as its original Value wire. No ToString conversion is
// performed, so non-string resources and undefined remain distinguishable.
__declspec(dllexport) int64_t gov8_ca_message_resource_name_value(
    v8::Isolate* iso, void* scope, void* msg, void** out) {
  ClearErr();
  if (msg == nullptr || out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    const v8::Value* p = v8__Message__GetScriptResourceName(
        gov8_snap_impl::DerefLocal(FromWire<v8::Message>(msg)));
    *out = const_cast<void*>(static_cast<const void*>(p));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in message_resource_name_value");
    return kErrCpp;
  }
}

// The message's stack trace as a wire (0 when none). No nested
// HandleScope: the returned wire lives in the CALLER's open scope.
__declspec(dllexport) int64_t gov8_ca_message_get_stack_trace(v8::Isolate* iso,
                                                              void* scope,
                                                              void* msg,
                                                              void** out) {
  ClearErr();
  if (msg == nullptr || out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    const v8::StackTrace* p = v8__Message__GetStackTrace(
        gov8_snap_impl::DerefLocal(FromWire<v8::Message>(msg)));
    *out = const_cast<void*>(static_cast<const void*>(p));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in message_get_stack_trace");
    return kErrCpp;
  }
}

// --- exception stack capture ---------------------------------------------------------------

// MaybeBool of Exception::CaptureStackTrace: 1 = Some(true), 0 = other,
// kErrException for Nothing. The object wire is kind-checked first (the
// typed binding would deref a mis-typed wire as Object).
__declspec(dllexport) int64_t gov8_ca_exception_capture_stack_trace(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, int32_t* ok) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || obj == nullptr || ok == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> val = FromWire<v8::Value>(obj);
    if (!val->IsObject()) {
      SetErr("capture target is not an Object");
      return kErrBadArg;
    }
    const int32_t mb = v8__Exception__CaptureStackTrace(
        gov8_snap_impl::DerefLocal(w->ctx->Get(iso)),
        gov8_snap_impl::DerefLocal(val.As<v8::Object>()));
    if (mb == gov8_rv_impl::kMaybeNothing) {
      return kErrException;
    }
    *ok = mb;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in exception_capture_stack_trace");
    return kErrCpp;
  }
}

// Exception::GetStackTrace (0 when the exception carries none). No nested
// HandleScope: the returned wire lives in the CALLER's open scope.
__declspec(dllexport) int64_t gov8_ca_exception_get_stack_trace(
    v8::Isolate* iso, void* scope, void* exc, void** out) {
  ClearErr();
  if (exc == nullptr || out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    const v8::StackTrace* p = v8__Exception__GetStackTrace(
        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(exc)));
    *out = const_cast<void*>(static_cast<const void*>(p));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in exception_get_stack_trace");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_set_capture_stack_trace_uncaught(
    v8::Isolate* iso, int32_t capture, int64_t frame_limit) {
  ClearErr();
  if (iso == nullptr || frame_limit < 0 || frame_limit > 2147483647LL) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  v8__Isolate__SetCaptureStackTraceForUncaughtExceptions(
      iso, capture != 0, static_cast<int>(frame_limit));
  return kOk;
}

// --- stack traces ---------------------------------------------------------------------------

// StackTrace::CurrentStackTrace; 0 when the engine produced none.
__declspec(dllexport) int64_t gov8_ca_stack_trace_current(v8::Isolate* iso,
                                                          void* scope,
                                                          int64_t frame_limit,
                                                          void** out) {
  ClearErr();
  if (out == nullptr || !ScopeIs(iso, scope) || frame_limit < 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    const v8::StackTrace* p =
        v8__StackTrace__CurrentStackTrace(iso, static_cast<int>(frame_limit));
    *out = const_cast<void*>(static_cast<const void*>(p));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in stack_trace_current");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_stack_trace_current_script_name(
    v8::Isolate* iso, void* scope, void** out) {
  ClearErr();
  if (out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    const v8::String* p = v8__StackTrace__CurrentScriptNameOrSourceURL(iso);
    *out = const_cast<void*>(static_cast<const void*>(p));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in stack_trace_current_script_name");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_stack_trace_frame_count(
    v8::Isolate* iso, void* scope, void* trace) {
  ClearErr();
  if (trace == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    return static_cast<int64_t>(v8__StackTrace__GetFrameCount(
        gov8_snap_impl::DerefLocal(FromWire<v8::StackTrace>(trace))));
  } catch (...) {
    SetErr("C++ exception in stack_trace_frame_count");
    return kErrCpp;
  }
}

// Frame wire for the index; the GO wrapper bounds the index (upstream
// GetFrame with an out-of-range index is unchecked). An engine-side empty
// result yields ok=0. No nested HandleScope: the returned wire lives in
// the CALLER's open scope.
__declspec(dllexport) int64_t gov8_ca_stack_trace_get_frame(v8::Isolate* iso,
                                                            void* scope,
                                                            void* trace,
                                                            int64_t index,
                                                            void** out,
                                                            int32_t* ok) {
  ClearErr();
  if (trace == nullptr || out == nullptr || ok == nullptr ||
      !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (index < 0) {
    SetErr("frame index out of range");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    const v8::StackFrame* p = v8__StackTrace__GetFrame(
        gov8_snap_impl::DerefLocal(FromWire<v8::StackTrace>(trace)), iso,
        static_cast<uint32_t>(index));
    if (p == nullptr) {
      *ok = 0;
      return kOk;
    }
    *out = const_cast<void*>(static_cast<const void*>(p));
    *ok = 1;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in stack_trace_get_frame");
    return kErrCpp;
  }
}

// --- stack frames ------------------------------------------------------------------------------

#define GOV8_CA_FRAME_TEXT(name, artifact)                                     \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* scope, void* frame, \
                                     void** out) {                              \
    ClearErr();                                                                \
    if (frame == nullptr || out == nullptr || !ScopeIs(iso, scope)) {          \
      SetErr("invalid argument");                                              \
      return kErrBadArg;                                                       \
    }                                                                          \
    try {                                                                      \
      /* No nested HandleScope: the returned wire lives in the */              \
      /* CALLER's open scope.        */                                        \
      Gov8IsolateScope iso_scope(iso);                                         \
      *out = const_cast<void*>(static_cast<const void*>(artifact(              \
          gov8_snap_impl::DerefLocal(FromWire<v8::StackFrame>(frame)))));      \
      return kOk;                                                              \
    } catch (...) {                                                            \
      SetErr("C++ exception in frame getter");                                 \
      return kErrCpp;                                                          \
    }                                                                          \
  }

GOV8_CA_FRAME_TEXT(gov8_ca_frame_function_name, v8__StackFrame__GetFunctionName)
GOV8_CA_FRAME_TEXT(gov8_ca_frame_script_name, v8__StackFrame__GetScriptName)

#undef GOV8_CA_FRAME_TEXT

#define GOV8_CA_FRAME_NUM(name, expr)                                          \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* scope, void* frame) { \
    ClearErr();                                                                \
    if (frame == nullptr || !ScopeIs(iso, scope)) {                            \
      SetErr("invalid argument");                                              \
      return kErrBadArg;                                                       \
    }                                                                          \
    try {                                                                      \
      Gov8IsolateScope iso_scope(iso);                                         \
      v8::HandleScope hs(iso);                                                 \
      const v8::StackFrame& m = gov8_snap_impl::DerefLocal(                    \
          FromWire<v8::StackFrame>(frame));                                    \
      return static_cast<int64_t>(expr);                                       \
    } catch (...) {                                                            \
      SetErr("C++ exception in frame getter");                                 \
      return kErrCpp;                                                          \
    }                                                                          \
  }

GOV8_CA_FRAME_NUM(gov8_ca_frame_line_number, v8__StackFrame__GetLineNumber(m))
GOV8_CA_FRAME_NUM(gov8_ca_frame_column, v8__StackFrame__GetColumn(m))
GOV8_CA_FRAME_NUM(gov8_ca_frame_script_id, v8__StackFrame__GetScriptId(m))
GOV8_CA_FRAME_NUM(gov8_ca_frame_is_eval, v8__StackFrame__IsEval(m) ? 1 : 0)
GOV8_CA_FRAME_NUM(gov8_ca_frame_is_constructor,
                  v8__StackFrame__IsConstructor(m) ? 1 : 0)
GOV8_CA_FRAME_NUM(gov8_ca_frame_is_wasm, v8__StackFrame__IsWasm(m) ? 1 : 0)
GOV8_CA_FRAME_NUM(gov8_ca_frame_is_user_javascript,
                  v8__StackFrame__IsUserJavaScript(m) ? 1 : 0)

#undef GOV8_CA_FRAME_NUM

// --- interrupts and GC callbacks ---------------------------------------------------------------

__declspec(dllexport) int64_t gov8_ca_interrupt_set_entry(uintptr_t entry) {
  ClearErr();
  gov8_ca_impl::g_ca_interrupt_entry =
      reinterpret_cast<CaInterruptEntry>(entry);
  return kOk;
}

// Thread-safe like v8__Isolate__RequestInterrupt: the engine posts the
// interrupt and the callback fires on the isolate thread during its next
// JS execution.
__declspec(dllexport) int64_t gov8_ca_isolate_request_interrupt(
    v8::Isolate* iso, int64_t id) {
  ClearErr();
  if (iso == nullptr || id <= 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8__Isolate__RequestInterrupt(iso, gov8_ca_interrupt_trampoline,
                                  reinterpret_cast<void*>(
                                      static_cast<intptr_t>(id)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in isolate_request_interrupt");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_gc_set_entry(uintptr_t entry) {
  ClearErr();
  gov8_ca_impl::g_ca_gc_entry = reinterpret_cast<CaGcEntry>(entry);
  return kOk;
}

__declspec(dllexport) int64_t gov8_ca_add_gc_prologue_callback(
    v8::Isolate* iso, int64_t id, int64_t gc_type_filter) {
  ClearErr();
  if (iso == nullptr || id <= 0 || gc_type_filter < 0 ||
      gc_type_filter > 0x1F) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8__Isolate__AddGCPrologueCallback(
        iso, gov8_ca_gc_trampoline,
        reinterpret_cast<void*>(static_cast<intptr_t>(id)),
        static_cast<v8::GCType>(gc_type_filter));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in add_gc_prologue_callback");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_remove_gc_prologue_callback(
    v8::Isolate* iso, int64_t id) {
  ClearErr();
  if (iso == nullptr || id <= 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8__Isolate__RemoveGCPrologueCallback(
        iso, gov8_ca_gc_trampoline,
        reinterpret_cast<void*>(static_cast<intptr_t>(id)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in remove_gc_prologue_callback");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_add_gc_epilogue_callback(
    v8::Isolate* iso, int64_t id, int64_t gc_type_filter) {
  ClearErr();
  if (iso == nullptr || id <= 0 || gc_type_filter < 0 ||
      gc_type_filter > 0x1F) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8__Isolate__AddGCEpilogueCallback(
        iso, gov8_ca_gc_trampoline,
        reinterpret_cast<void*>(static_cast<intptr_t>(id)),
        static_cast<v8::GCType>(gc_type_filter));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in add_gc_epilogue_callback");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_remove_gc_epilogue_callback(
    v8::Isolate* iso, int64_t id) {
  ClearErr();
  if (iso == nullptr || id <= 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8__Isolate__RemoveGCEpilogueCallback(
        iso, gov8_ca_gc_trampoline,
        reinterpret_cast<void*>(static_cast<intptr_t>(id)));
    return kOk;
  } catch (...) {
    SetErr("C++ exception in remove_gc_epilogue_callback");
    return kErrCpp;
  }
}

// --- heap statistics -----------------------------------------------------------------------------

// Fills 15 int64 slots field-by-field (no struct-layout assumption):
//   [0] total_heap_size            [1] total_heap_size_executable
//   [2] total_physical_size        [3] total_available_size
//   [4] used_heap_size             [5] heap_size_limit
//   [6] malloced_memory            [7] external_memory
//   [8] peak_malloced_memory       [9] does_zap_garbage (0/1)
//   [10] number_of_native_contexts [11] number_of_detached_contexts
//   [12] total_global_handles_size [13] used_global_handles_size
//   [14] total_allocated_bytes
__declspec(dllexport) int64_t gov8_ca_heap_statistics(v8::Isolate* iso,
                                                      int64_t* out) {
  ClearErr();
  if (iso == nullptr || out == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    // Plain data struct with no invariant-keeping constructor: GetHeapStatistics
    // assigns every field, so a zeroed object is fully defined afterwards.
    v8::HeapStatistics s;
    std::memset(&s, 0, sizeof(s));
    v8__Isolate__GetHeapStatistics(iso, &s);
    out[0] = static_cast<int64_t>(s.total_heap_size());
    out[1] = static_cast<int64_t>(s.total_heap_size_executable());
    out[2] = static_cast<int64_t>(s.total_physical_size());
    out[3] = static_cast<int64_t>(s.total_available_size());
    out[4] = static_cast<int64_t>(s.used_heap_size());
    out[5] = static_cast<int64_t>(s.heap_size_limit());
    out[6] = static_cast<int64_t>(s.malloced_memory());
    out[7] = static_cast<int64_t>(s.external_memory());
    out[8] = static_cast<int64_t>(s.peak_malloced_memory());
    out[9] = s.does_zap_garbage() != 0 ? 1 : 0;
    out[10] = static_cast<int64_t>(s.number_of_native_contexts());
    out[11] = static_cast<int64_t>(s.number_of_detached_contexts());
    out[12] = static_cast<int64_t>(s.total_global_handles_size());
    out[13] = static_cast<int64_t>(s.used_global_handles_size());
    out[14] = static_cast<int64_t>(s.total_allocated_bytes());
    return kOk;
  } catch (...) {
    SetErr("C++ exception in heap_statistics");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_adjust_external_memory(
    v8::Isolate* iso, int64_t change_in_bytes, int64_t* out) {
  ClearErr();
  if (iso == nullptr || out == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    *out = v8__Isolate__AdjustAmountOfExternalAllocatedMemory(iso,
                                                              change_in_bytes);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in adjust_external_memory");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_ca_number_of_heap_spaces(v8::Isolate* iso,
                                                            int64_t* out) {
  ClearErr();
  if (iso == nullptr || out == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  *out = static_cast<int64_t>(v8__Isolate__NumberOfHeapSpaces(iso));
  return kOk;
}

}  // extern "C"
