// ===========================================================================
// Runtime values feature slice: the JavaScript built-ins reachable through
// native APIs — Date, RegExp, JSON, Array, Map, Set, Proxy, Symbol and
// private symbols, primitive-wrapper predicates, and the property
// attribute / integrity / descriptor surface.
//
// All engine dispatch goes through the pinned artifact's own flat bindings
// (src/binding.cc of the pinned crate, compiled into the static library), so
// no method compiled from the vendored headers is ever entered.
//
// Exports (all invoked from Go through the dynamic proc lookup):
//   - Value predicates (IsDate/IsRegExp/IsMap/IsSet/IsProxy/IsSymbol/IsTrue/
//     IsName/IsBigInt plus the four wrapper-object predicates),
//     StrictEquals and TypeOf.
//   - Date::New / Date::ValueOf (the double time argument sits in positional
//     slot 3 so Go's SyscallN lands the IEEE-754 bits in an XMM register;
//     Go's asmstdcall copies the first four integer args into XMM0-3 for
//     exactly this case).
//   - RegExp::New / Exec / GetSource; RegExp::New reports the
//     invalid-pattern SyntaxError through the caller's TryCatch (or a
//     fallback when none is supplied) and returns kErrException.
//   - JSON::Parse / Stringify with the same TryCatch routing (circular
//     structures and toJSON failures surface as kErrException plus a caught
//     exception, matching the script_compile/script_run shape).
//   - Array::New (negative lengths forwarded verbatim: the engine collapses
//     them to an empty array — the pinned boundary), Array::New_with_elements,
//     Length, and the index property family (Get/Set/Has).
//   - Map / Set native collections: New, insert (Set/Add returns the
//     collection itself), Get, Has, Delete, Size, Clear, AsArray.
//   - Proxy::New / GetTarget / GetHandler / IsRevoked / Revoke.
//   - Symbol::New / For / ForApi, the well-known getters used by this slice
//     (toStringTag, iterator, hasInstance), Description, Private::New /
//     ForApi / Name, and the Object private-key family.
//   - Object::Get / Set with arbitrary key values, CreateDataProperty,
//     DefineOwnProperty, GetPropertyAttributes (Maybe<PropertyAttribute>),
//     SetIntegrityLevel, DefineProperty (PropertyDescriptor),
//     GetOwnPropertyDescriptor, and GetPropertyNames with the four-way
//     filter enumeration.
//   - PropertyDescriptor construction flavors, presence flags, readers and
//     in-place setters, guarded by a magic-word wrapper like the other
//     persistent resources.
//
// Wire validity: local-handle wires are object words valid in the CALLER's
// open HandleScope (validated via ScopeIs for every call that creates or
// reads engine locals). Typed artifact bindings take T& / const T& params;
// every such call dereferences the wire (DerefLocal) only after the Go
// wrapper and this slice validated the value kind with the artifact's own
// Is* bindings, so a mis-typed wire fails with kErrBadArg instead of
// undefined behavior.
//
// Maybe semantics preserved across the ABI:
//   - MaybeLocal<T> results: the wire is written through *out and kOk
//     returned; an empty maybe (a pending exception) yields kErrException.
//   - Maybe<bool> results: *ok receives Just(true)/Just(false) (1/0);
//     Nothing (a pending exception) yields kErrException.
//   - RegExp::Exec, Map::Get, Symbol::Description, and
//     Object::GetOwnPropertyDescriptor return real VALUES for misses (the
//     null / undefined oddballs), never an empty maybe; only a thrown
//     exception empties the maybe. Proxy::GetTarget on a revoked proxy
//     resolves to the null value (V8 clears the revoked proxy's target).
// ===========================================================================

#include <cstring>

namespace gov8_rv_impl {

// Direct-return NumberValue uses a per-thread result slot because every
// uint64 bit pattern is a valid IEEE-754 double, leaving no in-band encoding
// for an empty Maybe or a negative shim status. The export below publishes to
// this slot only after conversion (and any nested JavaScript/Go re-entry) has
// completed.
thread_local double g_value_number_value_result = 0;

// support.h's MaybeBool mirror. The ABI type is a 4-byte enumeration
// (JustFalse = 0, JustTrue = 1, Nothing = 2); declared here because
// support.h is not part of the vendored include set.
constexpr int32_t kMaybeJustFalse = 0;
constexpr int32_t kMaybeJustTrue = 1;
constexpr int32_t kMaybeNothing = 2;

// All RegExp flag bits the pinned engine defines (kGlobal .. kUnicodeSets).
constexpr int32_t kAllRegExpFlags = 0x1FF;

// Local dereference for the artifact's `T&` parameters. Defined by the
// snapshots slice (gov8_snap_impl::DerefLocal) and shared here: v8::Local's
// operator* returns T* (the V8 API keeps the slot word), so an explicit
// deref of the slot value is required.

// Maps an artifact MaybeBool result onto the shim's status protocol:
// Nothing (a pending exception) -> kErrException; otherwise *ok receives
// the Just value and kOk is returned.
inline int64_t MaybeBoolStatus(int32_t mb, int32_t* ok) {
  if (mb == kMaybeNothing) {
    return kErrException;
  }
  *ok = mb == kMaybeJustTrue ? 1 : 0;
  return kOk;
}

// Maps an artifact MaybeLocal result (a wire word) onto the status protocol:
// null (empty maybe, i.e. a pending exception) -> kErrException; otherwise
// the wire is stored and kOk returned.
inline int64_t MaybeLocalStatus(const void* p, void** out) {
  if (p == nullptr) {
    return kErrException;
  }
  *out = const_cast<void*>(p);
  return kOk;
}

// PropertyDescriptor wrapper. sizeof(v8::PropertyDescriptor) is one word in
// the pinned artifact (asserted by its binding), but the storage size is
// taken from the type so a future layout change fails to compile instead of
// corrupting memory.
constexpr uint64_t kPdMagic = 0x674f563850444553ULL;  // "gOV8PDES"

struct PdWrap {
  uint64_t magic;
  v8::Isolate* iso;
  alignas(v8::PropertyDescriptor) unsigned char pd_storage[sizeof(v8::PropertyDescriptor)];

  v8::PropertyDescriptor* pd() {
    return reinterpret_cast<v8::PropertyDescriptor*>(pd_storage);
  }
};

PdWrap* AsPd(void* p) {
  if (p == nullptr) return nullptr;
  PdWrap* w = static_cast<PdWrap*>(p);
  return w->magic == kPdMagic ? w : nullptr;
}

// Resolves the optional TryCatch exactly like script_compile/script_run:
// the caller's TryCatch (validated) receives thrown exceptions; with no
// TryCatch supplied a fallback swallows the exception so the isolate is
// never left with a poisoned pending exception. Static: internal linkage.
inline bool RvResolveTc(v8::Isolate* iso, void* tcw, FallbackTryCatch* fallback,
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

}  // namespace gov8_rv_impl

// --- artifact bindings used by this slice (compiled into the static lib) ----

extern "C" {
bool v8__Value__IsTrue(const v8::Value& self);
bool v8__Value__IsName(const v8::Value& self);
bool v8__Value__IsSymbol(const v8::Value& self);
bool v8__Value__IsBigInt(const v8::Value& self);
bool v8__Value__IsDate(const v8::Value& self);
bool v8__Value__IsRegExp(const v8::Value& self);
bool v8__Value__IsMap(const v8::Value& self);
bool v8__Value__IsSet(const v8::Value& self);
bool v8__Value__IsProxy(const v8::Value& self);
bool v8__Value__IsString(const v8::Value& self);
bool v8__Value__IsObject(const v8::Value& self);
bool v8__Value__IsArray(const v8::Value& self);
bool v8__Value__IsNumberObject(const v8::Value& self);
bool v8__Value__IsBooleanObject(const v8::Value& self);
bool v8__Value__IsStringObject(const v8::Value& self);
bool v8__Value__IsBigIntObject(const v8::Value& self);
bool v8__Value__StrictEquals(const v8::Value& self, const v8::Value& that);
const v8::String* v8__Value__TypeOf(v8::Value& self, v8::Isolate* isolate);
const v8::Object* v8__Object__New(v8::Isolate* isolate);

const v8::Date* v8__Date__New(const v8::Context& context, double time);
double v8__Date__ValueOf(const v8::Date& self);

const v8::RegExp* v8__RegExp__New(const v8::Context& context,
                                  const v8::String& pattern,
                                  v8::RegExp::Flags flags);
const v8::Object* v8__RegExp__Exec(v8::RegExp& self,
                                   const v8::Context& context,
                                   const v8::String& subject);
const v8::String* v8__RegExp__GetSource(const v8::RegExp& self);

const v8::Value* v8__JSON__Parse(const v8::Context& context,
                                 const v8::String& json_string);
const v8::String* v8__JSON__Stringify(const v8::Context& context,
                                      const v8::Value& json_object);

const v8::Array* v8__Array__New(v8::Isolate* isolate, int length);
const v8::Array* v8__Array__New_with_elements(
    v8::Isolate* isolate, const v8::Value* const elements[], size_t length);
uint32_t v8__Array__Length(const v8::Array& self);

const v8::Value* v8__Object__GetIndex(const v8::Object& self,
                                      const v8::Context& context,
                                      uint32_t index);
int32_t v8__Object__SetIndex(const v8::Object& self, const v8::Context& context,
                             uint32_t index, const v8::Value& value);
int32_t v8__Object__HasIndex(const v8::Object& self, const v8::Context& context,
                             uint32_t index);

const v8::Map* v8__Map__New(v8::Isolate* isolate);
v8::Map* v8__Map__Set(const v8::Map& self, const v8::Context& context,
                      const v8::Value& key, const v8::Value& value);
const v8::Value* v8__Map__Get(const v8::Map& self, const v8::Context& context,
                              const v8::Value& key);
int32_t v8__Map__Has(const v8::Map& self, const v8::Context& context,
                     const v8::Value& key);
int32_t v8__Map__Delete(const v8::Map& self, const v8::Context& context,
                        const v8::Value& key);
size_t v8__Map__Size(const v8::Map& self);
void v8__Map__Clear(const v8::Map& self);
const v8::Array* v8__Map__As__Array(const v8::Map& self);

const v8::Set* v8__Set__New(v8::Isolate* isolate);
v8::Set* v8__Set__Add(const v8::Set& self, const v8::Context& context,
                      const v8::Value& key);
int32_t v8__Set__Has(const v8::Set& self, const v8::Context& context,
                     const v8::Value& key);
int32_t v8__Set__Delete(const v8::Set& self, const v8::Context& context,
                        const v8::Value& key);
size_t v8__Set__Size(const v8::Set& self);
void v8__Set__Clear(const v8::Set& self);
const v8::Array* v8__Set__As__Array(const v8::Set& self);

const v8::Proxy* v8__Proxy__New(const v8::Context& context,
                                const v8::Object& target,
                                const v8::Object& handler);
const v8::Value* v8__Proxy__GetTarget(const v8::Proxy& self);
const v8::Value* v8__Proxy__GetHandler(const v8::Proxy& self);
bool v8__Proxy__IsRevoked(const v8::Proxy& self);
void v8__Proxy__Revoke(const v8::Proxy& self);

const v8::Symbol* v8__Symbol__New(v8::Isolate* isolate,
                                  const v8::String* description);
const v8::Symbol* v8__Symbol__For(v8::Isolate* isolate,
                                  const v8::String& description);
const v8::Symbol* v8__Symbol__ForApi(v8::Isolate* isolate,
                                     const v8::String& description);
const v8::Symbol* v8__Symbol__GetToStringTag(v8::Isolate* isolate);
const v8::Symbol* v8__Symbol__GetIterator(v8::Isolate* isolate);
const v8::Symbol* v8__Symbol__GetHasInstance(v8::Isolate* isolate);
const v8::Value* v8__Symbol__Description(const v8::Symbol& self,
                                         v8::Isolate* isolate);

const v8::Private* v8__Private__New(v8::Isolate* isolate,
                                    const v8::String* name);
const v8::Private* v8__Private__ForApi(v8::Isolate* isolate,
                                       const v8::String* name);
const v8::Value* v8__Private__Name(const v8::Private& self);

const v8::Value* v8__Object__Get(const v8::Object& self,
                                 const v8::Context& context,
                                 const v8::Value& key);
int32_t v8__Object__Set(const v8::Object& self, const v8::Context& context,
                        const v8::Value& key, const v8::Value& value);
int32_t v8__Object__CreateDataProperty(const v8::Object& self,
                                       const v8::Context& context,
                                       const v8::Name& key,
                                       const v8::Value& value);
int32_t v8__Object__DefineOwnProperty(const v8::Object& self,
                                      const v8::Context& context,
                                      const v8::Name& key,
                                      const v8::Value& value,
                                      v8::PropertyAttribute attr);
void v8__Object__GetPropertyAttributes(
    const v8::Object& self, const v8::Context& context, const v8::Value& key,
    v8::Maybe<v8::PropertyAttribute>* out);
int32_t v8__Object__SetIntegrityLevel(const v8::Object& self,
                                      const v8::Context& context,
                                      v8::IntegrityLevel level);
int32_t v8__Object__DefineProperty(const v8::Object& self,
                                   const v8::Context& context,
                                   const v8::Name& key,
                                   v8::PropertyDescriptor& desc);
const v8::Value* v8__Object__GetOwnPropertyDescriptor(const v8::Object& self,
                                                      const v8::Context& context,
                                                      const v8::Name& key);
const v8::Array* v8__Object__GetPropertyNames(
    const v8::Object* self, const v8::Context* context,
    v8::KeyCollectionMode mode, v8::PropertyFilter property_filter,
    v8::IndexFilter index_filter, v8::KeyConversionMode key_conversion);
const v8::Value* v8__Object__GetPrivate(const v8::Object& self,
                                        const v8::Context& context,
                                        const v8::Private& key);
int32_t v8__Object__SetPrivate(const v8::Object& self,
                               const v8::Context& context,
                               const v8::Private& key, const v8::Value& value);
int32_t v8__Object__DeletePrivate(const v8::Object& self,
                                  const v8::Context& context,
                                  const v8::Private& key);
int32_t v8__Object__HasPrivate(const v8::Object& self,
                               const v8::Context& context,
                               const v8::Private& key);

// The CONSTRUCT bindings take an uninitialized-storage pointer in the
// artifact (support.h uninit_t<T>* there; void* here — ABI identical).
void v8__PropertyDescriptor__CONSTRUCT(void* buf);
void v8__PropertyDescriptor__CONSTRUCT__Value(void* buf,
                                              v8::Local<v8::Value> value);
void v8__PropertyDescriptor__CONSTRUCT__Value_Writable(
    void* buf, v8::Local<v8::Value> value, bool writable);
void v8__PropertyDescriptor__CONSTRUCT__Get_Set(void* buf,
                                                v8::Local<v8::Value> get,
                                                v8::Local<v8::Value> set);
void v8__PropertyDescriptor__DESTRUCT(v8::PropertyDescriptor* self);
bool v8__PropertyDescriptor__has_value(const v8::PropertyDescriptor* self);
bool v8__PropertyDescriptor__has_writable(const v8::PropertyDescriptor* self);
bool v8__PropertyDescriptor__has_enumerable(const v8::PropertyDescriptor* self);
bool v8__PropertyDescriptor__has_configurable(
    const v8::PropertyDescriptor* self);
bool v8__PropertyDescriptor__has_get(const v8::PropertyDescriptor* self);
bool v8__PropertyDescriptor__has_set(const v8::PropertyDescriptor* self);
bool v8__PropertyDescriptor__writable(const v8::PropertyDescriptor* self);
bool v8__PropertyDescriptor__enumerable(const v8::PropertyDescriptor* self);
bool v8__PropertyDescriptor__configurable(const v8::PropertyDescriptor* self);
const v8::Value* v8__PropertyDescriptor__value(const v8::PropertyDescriptor* self);
const v8::Value* v8__PropertyDescriptor__get(const v8::PropertyDescriptor* self);
const v8::Value* v8__PropertyDescriptor__set(const v8::PropertyDescriptor* self);
void v8__PropertyDescriptor__set_enumerable(v8::PropertyDescriptor* self,
                                            bool enumerable);
void v8__PropertyDescriptor__set_configurable(v8::PropertyDescriptor* self,
                                              bool configurable);
}

extern "C" {

using namespace gov8_rv_impl;

// --- shared validation macros -------------------------------------------------

#define GOV8_RV_NULL_WIRE(w)                                                   \
  if (w == nullptr) {                                                          \
    SetErr("null value wire");                                                 \
    return kErrBadArg;                                                         \
  }

#define GOV8_RV_END_CATCH(op)                                                  \
  catch (...) {                                                                \
    SetErr("C++ exception in " op);                                            \
    return kErrCpp;                                                            \
  }

// Direct-return NumberValue encoding: negative values retain the shim error
// convention, zero is an empty Maybe<double>, and a positive value addresses
// shim-owned thread-local double storage. Keeping the legacy output-pointer
// export in shim.cc preserves the prior C ABI entry point.
__declspec(dllexport) int64_t gov8_value_number_value_direct(
    v8::Isolate* iso, void* ctxw, void* v) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || v == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    double value;
    if (!FromWire<v8::Value>(v)->NumberValue(ctx).To(&value)) return 0;
    g_value_number_value_result = value;
    return static_cast<int64_t>(
        reinterpret_cast<intptr_t>(&g_value_number_value_result));
  } GOV8_RV_END_CATCH("value_number_value_direct")
}

// Direct-return Uint32Value encoding: every uint32 result occupies
// [0, 2^32), 2^32 means the Maybe conversion was empty, and negative int64
// values retain the shim's error-status convention. No host stack pointer is
// retained across a conversion that can re-enter JavaScript and Go callbacks.
__declspec(dllexport) int64_t gov8_value_uint32_value_direct(
    v8::Isolate* iso, void* ctxw, void* v) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || v == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    uint32_t value;
    if (!FromWire<v8::Value>(v)->Uint32Value(ctx).To(&value)) {
      return int64_t{1} << 32;
    }
    return static_cast<int64_t>(value);
  } GOV8_RV_END_CATCH("value_uint32_value_direct")
}

// --- predicates --------------------------------------------------------------

#define GOV8_RV_PREDICATE(name, artifact)                                      \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* v) {              \
    ClearErr();                                                                \
    GOV8_RV_NULL_WIRE(v)                                                       \
    try {                                                                      \
      v8::Isolate::Scope iso_scope(iso);                                       \
      v8::HandleScope hs(iso);                                                 \
      v8::Local<v8::Value> val = FromWire<v8::Value>(v);                       \
      return artifact(gov8_snap_impl::DerefLocal(val)) ? 1 : 0;                                           \
    } GOV8_RV_END_CATCH(#name)                                                 \
  }

GOV8_RV_PREDICATE(gov8_rv_is_date, v8__Value__IsDate)
GOV8_RV_PREDICATE(gov8_rv_is_reg_exp, v8__Value__IsRegExp)
GOV8_RV_PREDICATE(gov8_rv_is_map, v8__Value__IsMap)
GOV8_RV_PREDICATE(gov8_rv_is_set, v8__Value__IsSet)
GOV8_RV_PREDICATE(gov8_rv_is_proxy, v8__Value__IsProxy)
GOV8_RV_PREDICATE(gov8_rv_is_symbol, v8__Value__IsSymbol)
GOV8_RV_PREDICATE(gov8_rv_is_true, v8__Value__IsTrue)
GOV8_RV_PREDICATE(gov8_rv_is_name, v8__Value__IsName)
GOV8_RV_PREDICATE(gov8_rv_is_big_int, v8__Value__IsBigInt)
GOV8_RV_PREDICATE(gov8_rv_is_number_object, v8__Value__IsNumberObject)
GOV8_RV_PREDICATE(gov8_rv_is_boolean_object, v8__Value__IsBooleanObject)
GOV8_RV_PREDICATE(gov8_rv_is_string_object, v8__Value__IsStringObject)
GOV8_RV_PREDICATE(gov8_rv_is_big_int_object, v8__Value__IsBigIntObject)

#undef GOV8_RV_PREDICATE

// StrictEquals: ECMAScript strict equality.
__declspec(dllexport) int64_t gov8_rv_strict_equals(v8::Isolate* iso, void* a,
                                                    void* b) {
  ClearErr();
  GOV8_RV_NULL_WIRE(a)
  GOV8_RV_NULL_WIRE(b)
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> va = FromWire<v8::Value>(a);
    v8::Local<v8::Value> vb = FromWire<v8::Value>(b);
    return v8__Value__StrictEquals(gov8_snap_impl::DerefLocal(va),
                                   gov8_snap_impl::DerefLocal(vb))
               ? 1
               : 0;
  } GOV8_RV_END_CATCH("strict_equals")
}

// ToString with caller-owned TryCatch routing. Unlike the core value_text
// helper (which installs its own internal TryCatch and therefore swallows
// conversion failures), the thrown exception here is delivered to the
// caller's active TryCatch (or a fallback when tcw is null) — so the pinned
// "ToString of a symbol throws" behavior stays observable.
__declspec(dllexport) int64_t gov8_rv_value_to_string_utf8(
    v8::Isolate* iso, void* ctxw, void* scope, void* tcw, void* v, char* buf,
    int64_t cap, int64_t* out_len) {
  ClearErr();
  GOV8_RV_NULL_WIRE(v)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out_len == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::String> s;
    if (!gov8_snap_impl::DerefLocal(FromWire<v8::Value>(v))
             .ToString(ctx)
             .ToLocal(&s)) {
      return kErrException;
    }
    const size_t n = s->Utf8Length(iso);
    std::string text;
    text.resize(n);
    if (n > 0) {
      const size_t written = s->WriteUtf8(
          iso, text.data(), n, v8::String::WriteFlags::kReplaceInvalidUtf8);
      text.resize(written);
    }
    return BufferUtf8(text, buf, cap, out_len);
  } GOV8_RV_END_CATCH("value_to_string_utf8")
}

// TypeOf: the "typeof" string, created in the CALLER's open HandleScope.
__declspec(dllexport) int64_t gov8_rv_type_of(v8::Isolate* iso, void* scope,
                                              void* v, void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(v)
  if (out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Value> val = FromWire<v8::Value>(v);
    *out = const_cast<void*>(
        static_cast<const void*>(v8__Value__TypeOf(gov8_snap_impl::DerefLocal(val), iso)));
    return *out == nullptr ? kErrGeneric : kOk;
  } GOV8_RV_END_CATCH("type_of")
}

// --- Date --------------------------------------------------------------------

// Date::New. The double time argument sits in positional slot 3 so Go's
// SyscallN lands the IEEE-754 bits in an XMM register (Go's asmstdcall
// copies the first four integer args into XMM0-3 for exactly this case).
__declspec(dllexport) int64_t gov8_rv_date_new(v8::Isolate* iso, void* ctxw,
                                               double time, void* scope,
                                               void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Date* date = v8__Date__New(gov8_snap_impl::DerefLocal(ctx), time);
    if (date == nullptr) {
      SetErr("Date::New failed");
      return kErrGeneric;
    }
    *out = const_cast<void*>(static_cast<const void*>(date));
    return kOk;
  } GOV8_RV_END_CATCH("date_new")
}

// Date::ValueOf. Prevalidated with the artifact's IsDate before the typed
// binding is entered (a mis-typed wire would be undefined behavior).
__declspec(dllexport) int64_t gov8_rv_date_value_of(v8::Isolate* iso, void* v,
                                                    double* out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(v)
  if (out == nullptr) {
    SetErr("null out param");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> val = FromWire<v8::Value>(v);
    if (!v8__Value__IsDate(gov8_snap_impl::DerefLocal(val))) {
      SetErr("value is not a Date");
      return kErrBadArg;
    }
    *out = v8__Date__ValueOf(gov8_snap_impl::DerefLocal(val.As<v8::Date>()));
    return kOk;
  } GOV8_RV_END_CATCH("date_value_of")
}

// --- RegExp --------------------------------------------------------------------

__declspec(dllexport) int64_t gov8_rv_regexp_new(v8::Isolate* iso, void* ctxw,
                                                 void* scope, void* tcw,
                                                 void* pattern, int32_t flags,
                                                 void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(pattern)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Value> pat = FromWire<v8::Value>(pattern);
    if (!v8__Value__IsString(gov8_snap_impl::DerefLocal(pat))) {
      SetErr("pattern is not a String");
      return kErrBadArg;
    }
    // Reject flag bits the engine does not define before the cast.
    if ((flags & ~gov8_rv_impl::kAllRegExpFlags) != 0) {
      SetErr("invalid regexp flags");
      return kErrBadArg;
    }
    const v8::RegExp* re =
        v8__RegExp__New(gov8_snap_impl::DerefLocal(ctx), gov8_snap_impl::DerefLocal(pat.As<v8::String>()),
                        static_cast<v8::RegExp::Flags>(flags));
    return MaybeLocalStatus(re, out);
  } GOV8_RV_END_CATCH("regexp_new")
}

// Exec: no match resolves to the null VALUE (a Some result in the pinned
// crate); only a thrown exception yields kErrException.
__declspec(dllexport) int64_t gov8_rv_regexp_exec(v8::Isolate* iso,
                                                  void* ctxw, void* scope,
                                                  void* re, void* subject,
                                                  void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(re)
  GOV8_RV_NULL_WIRE(subject)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> rev = FromWire<v8::Value>(re);
    v8::Local<v8::Value> subj = FromWire<v8::Value>(subject);
    if (!v8__Value__IsRegExp(gov8_snap_impl::DerefLocal(rev))) {
      SetErr("value is not a RegExp");
      return kErrBadArg;
    }
    if (!v8__Value__IsString(gov8_snap_impl::DerefLocal(subj))) {
      SetErr("subject is not a String");
      return kErrBadArg;
    }
    const v8::Object* match =
        v8__RegExp__Exec(gov8_snap_impl::DerefLocal(rev.As<v8::RegExp>()), gov8_snap_impl::DerefLocal(ctx),
                         gov8_snap_impl::DerefLocal(subj.As<v8::String>()));
    return MaybeLocalStatus(match, out);
  } GOV8_RV_END_CATCH("regexp_exec")
}

// GetSource: the pattern source verbatim (never throws).
__declspec(dllexport) int64_t gov8_rv_regexp_get_source(v8::Isolate* iso,
                                                        void* scope, void* re,
                                                        void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(re)
  if (out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Value> rev = FromWire<v8::Value>(re);
    if (!v8__Value__IsRegExp(gov8_snap_impl::DerefLocal(rev))) {
      SetErr("value is not a RegExp");
      return kErrBadArg;
    }
    *out = const_cast<void*>(static_cast<const void*>(
        v8__RegExp__GetSource(gov8_snap_impl::DerefLocal(rev.As<v8::RegExp>()))));
    return *out == nullptr ? kErrGeneric : kOk;
  } GOV8_RV_END_CATCH("regexp_get_source")
}

// --- JSON ----------------------------------------------------------------------

__declspec(dllexport) int64_t gov8_rv_json_parse(v8::Isolate* iso,
                                                 void* ctxw, void* scope,
                                                 void* tcw, void* text,
                                                 void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(text)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Value> txt = FromWire<v8::Value>(text);
    if (!v8__Value__IsString(gov8_snap_impl::DerefLocal(txt))) {
      SetErr("json text is not a String");
      return kErrBadArg;
    }
    const v8::Value* parsed =
        v8__JSON__Parse(gov8_snap_impl::DerefLocal(ctx), gov8_snap_impl::DerefLocal(txt.As<v8::String>()));
    return MaybeLocalStatus(parsed, out);
  } GOV8_RV_END_CATCH("json_parse")
}

__declspec(dllexport) int64_t gov8_rv_json_stringify(v8::Isolate* iso,
                                                     void* ctxw, void* scope,
                                                     void* tcw, void* value,
                                                     void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(value)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Value> val = FromWire<v8::Value>(value);
    const v8::String* text =
        v8__JSON__Stringify(gov8_snap_impl::DerefLocal(ctx), gov8_snap_impl::DerefLocal(val));
    return MaybeLocalStatus(text, out);
  } GOV8_RV_END_CATCH("json_stringify")
}

// --- Array -----------------------------------------------------------------------

__declspec(dllexport) int64_t gov8_rv_array_new(v8::Isolate* iso, void* ctxw,
                                                void* scope, int32_t length,
                                                void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    // Negative lengths are forwarded verbatim: the engine collapses them to
    // an empty array (the pinned boundary; the JS constructor throws
    // instead, which the Go conformance observes through eval).
    *out = const_cast<void*>(static_cast<const void*>(
        v8__Array__New(iso, static_cast<int>(length))));
    return *out == nullptr ? kErrGeneric : kOk;
  } GOV8_RV_END_CATCH("array_new")
}

// wires points at `n` consecutive value wires (direct object words).
__declspec(dllexport) int64_t gov8_rv_array_new_with_elements(
    v8::Isolate* iso, void* ctxw, void* scope, void* wires, int64_t n,
    void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) || n < 0 ||
      (wires == nullptr && n > 0) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    *out = const_cast<void*>(static_cast<const void*>(
        v8__Array__New_with_elements(
            iso, static_cast<const v8::Value* const*>(wires),
            static_cast<size_t>(n))));
    return *out == nullptr ? kErrGeneric : kOk;
  } GOV8_RV_END_CATCH("array_new_with_elements")
}

__declspec(dllexport) int64_t gov8_rv_array_length(v8::Isolate* iso, void* a,
                                                   uint32_t* out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(a)
  if (out == nullptr) {
    SetErr("null out param");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> val = FromWire<v8::Value>(a);
    if (!v8__Value__IsArray(gov8_snap_impl::DerefLocal(val))) {
      SetErr("value is not an Array");
      return kErrBadArg;
    }
    *out = v8__Array__Length(gov8_snap_impl::DerefLocal(val.As<v8::Array>()));
    return kOk;
  } GOV8_RV_END_CATCH("array_length")
}

__declspec(dllexport) int64_t gov8_rv_array_get_index(v8::Isolate* iso,
                                                      void* ctxw, void* scope,
                                                      void* a, uint32_t index,
                                                      void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(a)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> val = FromWire<v8::Value>(a);
    if (!v8__Value__IsArray(gov8_snap_impl::DerefLocal(val))) {
      SetErr("value is not an Array");
      return kErrBadArg;
    }
    const v8::Value* got =
        v8__Object__GetIndex(gov8_snap_impl::DerefLocal(val.As<v8::Array>()), gov8_snap_impl::DerefLocal(ctx),
                             index);
    return MaybeLocalStatus(got, out);
  } GOV8_RV_END_CATCH("array_get_index")
}

__declspec(dllexport) int64_t gov8_rv_array_set_index(v8::Isolate* iso,
                                                      void* ctxw, void* scope,
                                                      void* a, uint32_t index,
                                                      void* v, int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(a)
  GOV8_RV_NULL_WIRE(v)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> val = FromWire<v8::Value>(a);
    if (!v8__Value__IsArray(gov8_snap_impl::DerefLocal(val))) {
      SetErr("value is not an Array");
      return kErrBadArg;
    }
    return MaybeBoolStatus(
        v8__Object__SetIndex(gov8_snap_impl::DerefLocal(val.As<v8::Array>()), gov8_snap_impl::DerefLocal(ctx),
                             index, gov8_snap_impl::DerefLocal(FromWire<v8::Value>(v))),
        ok);
  } GOV8_RV_END_CATCH("array_set_index")
}

__declspec(dllexport) int64_t gov8_rv_array_has_index(v8::Isolate* iso,
                                                      void* ctxw, void* scope,
                                                      void* a, uint32_t index,
                                                      int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(a)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> val = FromWire<v8::Value>(a);
    if (!v8__Value__IsArray(gov8_snap_impl::DerefLocal(val))) {
      SetErr("value is not an Array");
      return kErrBadArg;
    }
    return MaybeBoolStatus(
        v8__Object__HasIndex(gov8_snap_impl::DerefLocal(val.As<v8::Array>()), gov8_snap_impl::DerefLocal(ctx),
                             index),
        ok);
  } GOV8_RV_END_CATCH("array_has_index")
}

// --- Map / Set ---------------------------------------------------------------------

__declspec(dllexport) int64_t gov8_rv_map_new(v8::Isolate* iso, void* ctxw,
                                              void* scope, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    *out = const_cast<void*>(static_cast<const void*>(v8__Map__New(iso)));
    return *out == nullptr ? kErrGeneric : kOk;
  } GOV8_RV_END_CATCH("map_new")
}

// Set returns the collection itself (the Go side compares it for identity).
__declspec(dllexport) int64_t gov8_rv_map_set(v8::Isolate* iso, void* ctxw,
                                              void* scope, void* m, void* key,
                                              void* value, void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(m)
  GOV8_RV_NULL_WIRE(key)
  GOV8_RV_NULL_WIRE(value)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> mv = FromWire<v8::Value>(m);
    if (!v8__Value__IsMap(gov8_snap_impl::DerefLocal(mv))) {
      SetErr("value is not a Map");
      return kErrBadArg;
    }
    v8::Map* mapped =
        v8__Map__Set(gov8_snap_impl::DerefLocal(mv.As<v8::Map>()), gov8_snap_impl::DerefLocal(ctx),
                     gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key)),
                     gov8_snap_impl::DerefLocal(FromWire<v8::Value>(value)));
    return MaybeLocalStatus(mapped, out);
  } GOV8_RV_END_CATCH("map_set")
}

__declspec(dllexport) int64_t gov8_rv_map_get(v8::Isolate* iso, void* ctxw,
                                              void* scope, void* m, void* key,
                                              void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(m)
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> mv = FromWire<v8::Value>(m);
    if (!v8__Value__IsMap(gov8_snap_impl::DerefLocal(mv))) {
      SetErr("value is not a Map");
      return kErrBadArg;
    }
    const v8::Value* got =
        v8__Map__Get(gov8_snap_impl::DerefLocal(mv.As<v8::Map>()), gov8_snap_impl::DerefLocal(ctx),
                     gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key)));
    return MaybeLocalStatus(got, out);
  } GOV8_RV_END_CATCH("map_get")
}

__declspec(dllexport) int64_t gov8_rv_map_has(v8::Isolate* iso, void* ctxw,
                                              void* scope, void* m, void* key,
                                              int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(m)
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> mv = FromWire<v8::Value>(m);
    if (!v8__Value__IsMap(gov8_snap_impl::DerefLocal(mv))) {
      SetErr("value is not a Map");
      return kErrBadArg;
    }
    return MaybeBoolStatus(
        v8__Map__Has(gov8_snap_impl::DerefLocal(mv.As<v8::Map>()), gov8_snap_impl::DerefLocal(ctx),
                     gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key))),
        ok);
  } GOV8_RV_END_CATCH("map_has")
}

__declspec(dllexport) int64_t gov8_rv_map_delete(v8::Isolate* iso, void* ctxw,
                                                 void* scope, void* m,
                                                 void* key, int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(m)
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> mv = FromWire<v8::Value>(m);
    if (!v8__Value__IsMap(gov8_snap_impl::DerefLocal(mv))) {
      SetErr("value is not a Map");
      return kErrBadArg;
    }
    return MaybeBoolStatus(
        v8__Map__Delete(gov8_snap_impl::DerefLocal(mv.As<v8::Map>()), gov8_snap_impl::DerefLocal(ctx),
                        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key))),
        ok);
  } GOV8_RV_END_CATCH("map_delete")
}

__declspec(dllexport) int64_t gov8_rv_map_size(v8::Isolate* iso, void* m,
                                               int64_t* out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(m)
  if (out == nullptr) {
    SetErr("null out param");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> mv = FromWire<v8::Value>(m);
    if (!v8__Value__IsMap(gov8_snap_impl::DerefLocal(mv))) {
      SetErr("value is not a Map");
      return kErrBadArg;
    }
    *out = static_cast<int64_t>(v8__Map__Size(gov8_snap_impl::DerefLocal(mv.As<v8::Map>())));
    return kOk;
  } GOV8_RV_END_CATCH("map_size")
}

__declspec(dllexport) int64_t gov8_rv_map_clear(v8::Isolate* iso, void* m) {
  ClearErr();
  GOV8_RV_NULL_WIRE(m)
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> mv = FromWire<v8::Value>(m);
    if (!v8__Value__IsMap(gov8_snap_impl::DerefLocal(mv))) {
      SetErr("value is not a Map");
      return kErrBadArg;
    }
    v8__Map__Clear(gov8_snap_impl::DerefLocal(mv.As<v8::Map>()));
    return kOk;
  } GOV8_RV_END_CATCH("map_clear")
}

// AsArray renders the collection's entries (Map: k/v pairs, Set: values) in
// insertion order.
__declspec(dllexport) int64_t gov8_rv_map_as_array(v8::Isolate* iso,
                                                   void* ctxw, void* scope,
                                                   void* m, void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(m)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> mv = FromWire<v8::Value>(m);
    if (!v8__Value__IsMap(gov8_snap_impl::DerefLocal(mv))) {
      SetErr("value is not a Map");
      return kErrBadArg;
    }
    *out = const_cast<void*>(
        static_cast<const void*>(v8__Map__As__Array(gov8_snap_impl::DerefLocal(mv.As<v8::Map>()))));
    return *out == nullptr ? kErrGeneric : kOk;
  } GOV8_RV_END_CATCH("map_as_array")
}

__declspec(dllexport) int64_t gov8_rv_set_new(v8::Isolate* iso, void* ctxw,
                                              void* scope, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    *out = const_cast<void*>(static_cast<const void*>(v8__Set__New(iso)));
    return *out == nullptr ? kErrGeneric : kOk;
  } GOV8_RV_END_CATCH("set_new")
}

__declspec(dllexport) int64_t gov8_rv_set_add(v8::Isolate* iso, void* ctxw,
                                              void* scope, void* s, void* key,
                                              void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(s)
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> sv = FromWire<v8::Value>(s);
    if (!v8__Value__IsSet(gov8_snap_impl::DerefLocal(sv))) {
      SetErr("value is not a Set");
      return kErrBadArg;
    }
    v8::Set* added =
        v8__Set__Add(gov8_snap_impl::DerefLocal(sv.As<v8::Set>()), gov8_snap_impl::DerefLocal(ctx),
                     gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key)));
    return MaybeLocalStatus(added, out);
  } GOV8_RV_END_CATCH("set_add")
}

__declspec(dllexport) int64_t gov8_rv_set_has(v8::Isolate* iso, void* ctxw,
                                              void* scope, void* s, void* key,
                                              int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(s)
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> sv = FromWire<v8::Value>(s);
    if (!v8__Value__IsSet(gov8_snap_impl::DerefLocal(sv))) {
      SetErr("value is not a Set");
      return kErrBadArg;
    }
    return MaybeBoolStatus(
        v8__Set__Has(gov8_snap_impl::DerefLocal(sv.As<v8::Set>()), gov8_snap_impl::DerefLocal(ctx),
                     gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key))),
        ok);
  } GOV8_RV_END_CATCH("set_has")
}

__declspec(dllexport) int64_t gov8_rv_set_delete(v8::Isolate* iso, void* ctxw,
                                                 void* scope, void* s,
                                                 void* key, int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(s)
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> sv = FromWire<v8::Value>(s);
    if (!v8__Value__IsSet(gov8_snap_impl::DerefLocal(sv))) {
      SetErr("value is not a Set");
      return kErrBadArg;
    }
    return MaybeBoolStatus(
        v8__Set__Delete(gov8_snap_impl::DerefLocal(sv.As<v8::Set>()), gov8_snap_impl::DerefLocal(ctx),
                        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key))),
        ok);
  } GOV8_RV_END_CATCH("set_delete")
}

__declspec(dllexport) int64_t gov8_rv_set_size(v8::Isolate* iso, void* s,
                                               int64_t* out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(s)
  if (out == nullptr) {
    SetErr("null out param");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> sv = FromWire<v8::Value>(s);
    if (!v8__Value__IsSet(gov8_snap_impl::DerefLocal(sv))) {
      SetErr("value is not a Set");
      return kErrBadArg;
    }
    *out = static_cast<int64_t>(v8__Set__Size(gov8_snap_impl::DerefLocal(sv.As<v8::Set>())));
    return kOk;
  } GOV8_RV_END_CATCH("set_size")
}

__declspec(dllexport) int64_t gov8_rv_set_clear(v8::Isolate* iso, void* s) {
  ClearErr();
  GOV8_RV_NULL_WIRE(s)
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> sv = FromWire<v8::Value>(s);
    if (!v8__Value__IsSet(gov8_snap_impl::DerefLocal(sv))) {
      SetErr("value is not a Set");
      return kErrBadArg;
    }
    v8__Set__Clear(gov8_snap_impl::DerefLocal(sv.As<v8::Set>()));
    return kOk;
  } GOV8_RV_END_CATCH("set_clear")
}

__declspec(dllexport) int64_t gov8_rv_set_as_array(v8::Isolate* iso,
                                                   void* ctxw, void* scope,
                                                   void* s, void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(s)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> sv = FromWire<v8::Value>(s);
    if (!v8__Value__IsSet(gov8_snap_impl::DerefLocal(sv))) {
      SetErr("value is not a Set");
      return kErrBadArg;
    }
    *out = const_cast<void*>(
        static_cast<const void*>(v8__Set__As__Array(gov8_snap_impl::DerefLocal(sv.As<v8::Set>()))));
    return *out == nullptr ? kErrGeneric : kOk;
  } GOV8_RV_END_CATCH("set_as_array")
}

// --- Proxy --------------------------------------------------------------------------

__declspec(dllexport) int64_t gov8_rv_proxy_new(v8::Isolate* iso, void* ctxw,
                                                void* scope, void* target,
                                                void* handler, void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(target)
  GOV8_RV_NULL_WIRE(handler)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> tv = FromWire<v8::Value>(target);
    v8::Local<v8::Value> hv = FromWire<v8::Value>(handler);
    // Proxy::New dereferences both wires as Object; kind-check first.
    if (!v8__Value__IsObject(gov8_snap_impl::DerefLocal(tv)) || !v8__Value__IsObject(gov8_snap_impl::DerefLocal(hv))) {
      SetErr("proxy target and handler must be objects");
      return kErrBadArg;
    }
    const v8::Proxy* proxy =
        v8__Proxy__New(gov8_snap_impl::DerefLocal(ctx), gov8_snap_impl::DerefLocal(tv.As<v8::Object>()),
                       gov8_snap_impl::DerefLocal(hv.As<v8::Object>()));
    return MaybeLocalStatus(proxy, out);
  } GOV8_RV_END_CATCH("proxy_new")
}

#define GOV8_RV_PROXY_GETTER(name, artifact)                                   \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* scope, void* p,   \
                                     void** out) {                             \
    ClearErr();                                                                \
    GOV8_RV_NULL_WIRE(p)                                                       \
    if (out == nullptr || !ScopeIs(iso, scope)) {                              \
      SetErr("invalid argument");                                              \
      return kErrBadArg;                                                       \
    }                                                                          \
    try {                                                                      \
      v8::Isolate::Scope iso_scope(iso);                                       \
      v8::Local<v8::Value> pv = FromWire<v8::Value>(p);                        \
      if (!v8__Value__IsProxy(gov8_snap_impl::DerefLocal(pv))) {                                          \
        SetErr("value is not a Proxy");                                        \
        return kErrBadArg;                                                     \
      }                                                                        \
      *out = const_cast<void*>(                                                \
          static_cast<const void*>(artifact(gov8_snap_impl::DerefLocal(pv.As<v8::Proxy>())))); \
      return *out == nullptr ? kErrGeneric : kOk;                              \
    } GOV8_RV_END_CATCH(#name)                                                 \
  }

GOV8_RV_PROXY_GETTER(gov8_rv_proxy_get_target, v8__Proxy__GetTarget)
GOV8_RV_PROXY_GETTER(gov8_rv_proxy_get_handler, v8__Proxy__GetHandler)

#undef GOV8_RV_PROXY_GETTER

__declspec(dllexport) int64_t gov8_rv_proxy_is_revoked(v8::Isolate* iso,
                                                       void* p) {
  ClearErr();
  GOV8_RV_NULL_WIRE(p)
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> pv = FromWire<v8::Value>(p);
    if (!v8__Value__IsProxy(gov8_snap_impl::DerefLocal(pv))) {
      SetErr("value is not a Proxy");
      return kErrBadArg;
    }
    return v8__Proxy__IsRevoked(gov8_snap_impl::DerefLocal(pv.As<v8::Proxy>())) ? 1 : 0;
  } GOV8_RV_END_CATCH("proxy_is_revoked")
}

__declspec(dllexport) int64_t gov8_rv_proxy_revoke(v8::Isolate* iso, void* p) {
  ClearErr();
  GOV8_RV_NULL_WIRE(p)
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::HandleScope hs(iso);
    v8::Local<v8::Value> pv = FromWire<v8::Value>(p);
    if (!v8__Value__IsProxy(gov8_snap_impl::DerefLocal(pv))) {
      SetErr("value is not a Proxy");
      return kErrBadArg;
    }
    v8__Proxy__Revoke(gov8_snap_impl::DerefLocal(pv.As<v8::Proxy>()));
    return kOk;
  } GOV8_RV_END_CATCH("proxy_revoke")
}

// --- Symbol / Private -----------------------------------------------------------------

// Symbol::New with an optional description (null wire = anonymous).
__declspec(dllexport) int64_t gov8_rv_symbol_new(v8::Isolate* iso,
                                                 void* scope,
                                                 void* description,
                                                 void** out) {
  ClearErr();
  if (out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    const v8::String* desc = nullptr;
    if (description != nullptr) {
      v8::Local<v8::Value> dv = FromWire<v8::Value>(description);
      if (!v8__Value__IsString(gov8_snap_impl::DerefLocal(dv))) {
        SetErr("symbol description is not a String");
        return kErrBadArg;
      }
      desc = static_cast<const v8::String*>(description);
    }
    *out = const_cast<void*>(
        static_cast<const void*>(v8__Symbol__New(iso, desc)));
    return *out == nullptr ? kErrGeneric : kOk;
  } GOV8_RV_END_CATCH("symbol_new")
}

#define GOV8_RV_SYMBOL_FOR(name, artifact)                                     \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* scope,            \
                                     void* description, void** out) {          \
    ClearErr();                                                                \
    GOV8_RV_NULL_WIRE(description)                                             \
    if (out == nullptr || !ScopeIs(iso, scope)) {                              \
      SetErr("invalid argument");                                              \
      return kErrBadArg;                                                       \
    }                                                                          \
    try {                                                                      \
      v8::Isolate::Scope iso_scope(iso);                                       \
      v8::Local<v8::Value> dv = FromWire<v8::Value>(description);              \
      if (!v8__Value__IsString(gov8_snap_impl::DerefLocal(dv))) {                                         \
        SetErr("symbol description is not a String");                          \
        return kErrBadArg;                                                     \
      }                                                                        \
      *out = const_cast<void*>(static_cast<const void*>(                       \
          artifact(iso, gov8_snap_impl::DerefLocal(dv.As<v8::String>()))));                    \
      return *out == nullptr ? kErrGeneric : kOk;                              \
    } GOV8_RV_END_CATCH(#name)                                                 \
  }

GOV8_RV_SYMBOL_FOR(gov8_rv_symbol_for, v8__Symbol__For)
GOV8_RV_SYMBOL_FOR(gov8_rv_symbol_for_api, v8__Symbol__ForApi)

#undef GOV8_RV_SYMBOL_FOR

#define GOV8_RV_SYMBOL_WELLKNOWN(name, artifact)                               \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* scope,            \
                                     void** out) {                             \
    ClearErr();                                                                \
    if (out == nullptr || !ScopeIs(iso, scope)) {                              \
      SetErr("invalid argument");                                              \
      return kErrBadArg;                                                       \
    }                                                                          \
    try {                                                                      \
      v8::Isolate::Scope iso_scope(iso);                                       \
      *out = const_cast<void*>(static_cast<const void*>(artifact(iso)));       \
      return *out == nullptr ? kErrGeneric : kOk;                              \
    } GOV8_RV_END_CATCH(#name)                                                 \
  }

GOV8_RV_SYMBOL_WELLKNOWN(gov8_rv_symbol_get_to_string_tag,
                         v8__Symbol__GetToStringTag)
GOV8_RV_SYMBOL_WELLKNOWN(gov8_rv_symbol_get_iterator, v8__Symbol__GetIterator)
GOV8_RV_SYMBOL_WELLKNOWN(gov8_rv_symbol_get_has_instance,
                         v8__Symbol__GetHasInstance)

#undef GOV8_RV_SYMBOL_WELLKNOWN

__declspec(dllexport) int64_t gov8_rv_symbol_description(v8::Isolate* iso,
                                                         void* scope,
                                                         void* sym,
                                                         void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(sym)
  if (out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Value> sv = FromWire<v8::Value>(sym);
    if (!v8__Value__IsSymbol(gov8_snap_impl::DerefLocal(sv))) {
      SetErr("value is not a Symbol");
      return kErrBadArg;
    }
    *out = const_cast<void*>(static_cast<const void*>(
        v8__Symbol__Description(gov8_snap_impl::DerefLocal(sv.As<v8::Symbol>()), iso)));
    return *out == nullptr ? kErrGeneric : kOk;
  } GOV8_RV_END_CATCH("symbol_description")
}

#define GOV8_RV_PRIVATE_NEW(name, artifact)                                    \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* scope,            \
                                     void* pname, void** out) {                \
    ClearErr();                                                                \
    if (out == nullptr || !ScopeIs(iso, scope)) {                              \
      SetErr("invalid argument");                                              \
      return kErrBadArg;                                                       \
    }                                                                          \
    try {                                                                      \
      v8::Isolate::Scope iso_scope(iso);                                       \
      const v8::String* n = nullptr;                                           \
      if (pname != nullptr) {                                                  \
        v8::Local<v8::Value> nv = FromWire<v8::Value>(pname);                  \
        if (!v8__Value__IsString(gov8_snap_impl::DerefLocal(nv))) {                                       \
          SetErr("private name is not a String");                              \
          return kErrBadArg;                                                   \
        }                                                                      \
        n = static_cast<const v8::String*>(pname);                             \
      }                                                                        \
      *out = const_cast<void*>(                                                \
          static_cast<const void*>(artifact(iso, n)));                         \
      return *out == nullptr ? kErrGeneric : kOk;                              \
    } GOV8_RV_END_CATCH(#name)                                                 \
  }

GOV8_RV_PRIVATE_NEW(gov8_rv_private_new, v8__Private__New)
GOV8_RV_PRIVATE_NEW(gov8_rv_private_for_api, v8__Private__ForApi)

#undef GOV8_RV_PRIVATE_NEW

__declspec(dllexport) int64_t gov8_rv_private_name(v8::Isolate* iso,
                                                   void* scope, void* p,
                                                   void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(p)
  if (out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Private> pv = FromWire<v8::Private>(p);
    *out = const_cast<void*>(
        static_cast<const void*>(v8__Private__Name(gov8_snap_impl::DerefLocal(pv))));
    return *out == nullptr ? kErrGeneric : kOk;
  } GOV8_RV_END_CATCH("private_name")
}

// --- Object property surface -------------------------------------------------------

// Object::New: a fresh plain object (created in the CALLER's HandleScope).
__declspec(dllexport) void* gov8_rv_object_new(v8::Isolate* iso, void* ctxw,
                                               void* scope) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return nullptr;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return const_cast<void*>(static_cast<const void*>(v8__Object__New(iso)));
  } catch (...) {
    SetErr("C++ exception in object_new");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_rv_object_get(v8::Isolate* iso,
                                                 void* ctxw, void* scope,
                                                 void* obj, void* key,
                                                 void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Value* got =
        v8__Object__Get(gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)), gov8_snap_impl::DerefLocal(ctx),
                        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key)));
    return MaybeLocalStatus(got, out);
  } GOV8_RV_END_CATCH("object_get")
}

__declspec(dllexport) int64_t gov8_rv_object_set(v8::Isolate* iso,
                                                 void* ctxw, void* scope,
                                                 void* obj, void* key,
                                                 void* value, int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  GOV8_RV_NULL_WIRE(key)
  GOV8_RV_NULL_WIRE(value)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__Set(gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)), gov8_snap_impl::DerefLocal(ctx),
                        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key)),
                        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(value))),
        ok);
  } GOV8_RV_END_CATCH("object_set")
}

// Name-keyed data-property creation. The key wire is prevalidated with
// IsName (string or symbol) before the typed const Name& binding runs.
__declspec(dllexport) int64_t gov8_rv_object_create_data_property(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    void* value, int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  GOV8_RV_NULL_WIRE(key)
  GOV8_RV_NULL_WIRE(value)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> kv = FromWire<v8::Value>(key);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(kv))) {
      SetErr("key is not a Name");
      return kErrBadArg;
    }
    return MaybeBoolStatus(
        v8__Object__CreateDataProperty(gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
                                       gov8_snap_impl::DerefLocal(ctx),
                                       gov8_snap_impl::DerefLocal(kv.As<v8::Name>()),
                                       gov8_snap_impl::DerefLocal(FromWire<v8::Value>(value))),
        ok);
  } GOV8_RV_END_CATCH("object_create_data_property")
}

__declspec(dllexport) int64_t gov8_rv_object_define_own_property(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    void* value, int32_t attr, int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  GOV8_RV_NULL_WIRE(key)
  GOV8_RV_NULL_WIRE(value)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if ((attr & ~int32_t(0x7)) != 0) {  // PropertyAttribute uses bits 0..2
    SetErr("invalid property attribute bits");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> kv = FromWire<v8::Value>(key);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(kv))) {
      SetErr("key is not a Name");
      return kErrBadArg;
    }
    return MaybeBoolStatus(
        v8__Object__DefineOwnProperty(gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
                                      gov8_snap_impl::DerefLocal(ctx),
                                      gov8_snap_impl::DerefLocal(kv.As<v8::Name>()),
                                      gov8_snap_impl::DerefLocal(FromWire<v8::Value>(value)),
                                      static_cast<v8::PropertyAttribute>(attr)),
        ok);
  } GOV8_RV_END_CATCH("object_define_own_property")
}

// Property attributes: Maybe<PropertyAttribute> — a missing property is
// Just(NONE) (the pinned nuance); Nothing (exception) yields kErrException.
__declspec(dllexport) int64_t gov8_rv_object_get_property_attributes(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    int32_t* attr, int32_t* is_just) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || attr == nullptr || is_just == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Maybe<v8::PropertyAttribute> out{};
    v8__Object__GetPropertyAttributes(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)), gov8_snap_impl::DerefLocal(ctx),
        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key)), &out);
    if (out.IsNothing()) {
      return kErrException;
    }
    *attr = static_cast<int32_t>(out.FromJust());
    *is_just = 1;
    return kOk;
  } GOV8_RV_END_CATCH("object_get_property_attributes")
}

__declspec(dllexport) int64_t gov8_rv_object_set_integrity_level(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, int32_t level,
    int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (level < 0 || level > 1) {
    SetErr("invalid integrity level");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__SetIntegrityLevel(gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
                                      gov8_snap_impl::DerefLocal(ctx),
                                      static_cast<v8::IntegrityLevel>(level)),
        ok);
  } GOV8_RV_END_CATCH("object_set_integrity_level")
}

__declspec(dllexport) int64_t gov8_rv_object_define_property(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    void* pdw, int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  PdWrap* p = AsPd(pdw);
  if (w == nullptr || p == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso) || !OwnedBy(iso, p->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> kv = FromWire<v8::Value>(key);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(kv))) {
      SetErr("key is not a Name");
      return kErrBadArg;
    }
    return MaybeBoolStatus(
        v8__Object__DefineProperty(gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
                                   gov8_snap_impl::DerefLocal(ctx), gov8_snap_impl::DerefLocal(kv.As<v8::Name>()),
                                   *p->pd()),
        ok);
  } GOV8_RV_END_CATCH("object_define_property")
}

// The own descriptor for a missing key is the undefined VALUE (pinned
// nuance); only a thrown exception yields kErrException.
__declspec(dllexport) int64_t gov8_rv_object_get_own_property_descriptor(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> kv = FromWire<v8::Value>(key);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(kv))) {
      SetErr("key is not a Name");
      return kErrBadArg;
    }
    const v8::Value* desc = v8__Object__GetOwnPropertyDescriptor(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)), gov8_snap_impl::DerefLocal(ctx),
        gov8_snap_impl::DerefLocal(kv.As<v8::Name>()));
    return MaybeLocalStatus(desc, out);
  } GOV8_RV_END_CATCH("object_get_own_property_descriptor")
}

// Pointer-form binding: the object and context wires are passed as-is
// (ptr_to_local reinterprets them), matching the artifact's own signature.
__declspec(dllexport) int64_t gov8_rv_object_get_property_names(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, int32_t mode,
    int32_t property_filter, int32_t index_filter, int32_t key_conversion,
    void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (mode < 0 || mode > 1 || index_filter < 0 || index_filter > 1 ||
      key_conversion < 0 || key_conversion > 2) {
    SetErr("invalid property-name filter");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Array* names = v8__Object__GetPropertyNames(
        static_cast<const v8::Object*>(obj),
        // Pointer-form bindings receive the wire WORD itself (ptr_to_local
        // forms the Local from the pointer's value), so pass the context
        // local's first word, not the address of the local.
        *reinterpret_cast<const v8::Context* const*>(&ctx),
        static_cast<v8::KeyCollectionMode>(mode),
        static_cast<v8::PropertyFilter>(property_filter),
        static_cast<v8::IndexFilter>(index_filter),
        static_cast<v8::KeyConversionMode>(key_conversion));
    return MaybeLocalStatus(names, out);
  } GOV8_RV_END_CATCH("object_get_property_names")
}

// --- Object private-key family -------------------------------------------------------

__declspec(dllexport) int64_t gov8_rv_object_get_private(v8::Isolate* iso,
                                                        void* ctxw,
                                                        void* scope, void* obj,
                                                        void* priv,
                                                        void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  GOV8_RV_NULL_WIRE(priv)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Value* got = v8__Object__GetPrivate(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)), gov8_snap_impl::DerefLocal(ctx),
        gov8_snap_impl::DerefLocal(FromWire<v8::Private>(priv)));
    return MaybeLocalStatus(got, out);
  } GOV8_RV_END_CATCH("object_get_private")
}

__declspec(dllexport) int64_t gov8_rv_object_set_private(v8::Isolate* iso,
                                                         void* ctxw,
                                                         void* scope, void* obj,
                                                         void* priv,
                                                         void* value,
                                                         int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  GOV8_RV_NULL_WIRE(priv)
  GOV8_RV_NULL_WIRE(value)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__SetPrivate(gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
                               gov8_snap_impl::DerefLocal(ctx),
                               gov8_snap_impl::DerefLocal(FromWire<v8::Private>(priv)),
                               gov8_snap_impl::DerefLocal(FromWire<v8::Value>(value))),
        ok);
  } GOV8_RV_END_CATCH("object_set_private")
}

__declspec(dllexport) int64_t gov8_rv_object_delete_private(v8::Isolate* iso,
                                                            void* ctxw,
                                                            void* scope,
                                                            void* obj,
                                                            void* priv,
                                                            int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  GOV8_RV_NULL_WIRE(priv)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__DeletePrivate(gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
                                  gov8_snap_impl::DerefLocal(ctx),
                                  gov8_snap_impl::DerefLocal(FromWire<v8::Private>(priv))),
        ok);
  } GOV8_RV_END_CATCH("object_delete_private")
}

__declspec(dllexport) int64_t gov8_rv_object_has_private(v8::Isolate* iso,
                                                         void* ctxw,
                                                         void* scope, void* obj,
                                                         void* priv,
                                                         int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(obj)
  GOV8_RV_NULL_WIRE(priv)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    v8::Isolate::Scope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__HasPrivate(gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
                               gov8_snap_impl::DerefLocal(ctx),
                               gov8_snap_impl::DerefLocal(FromWire<v8::Private>(priv))),
        ok);
  } GOV8_RV_END_CATCH("object_has_private")
}

// --- PropertyDescriptor ---------------------------------------------------------------

// The four construction flavors allocate a magic-word wrap and placement-
// construct the descriptor into its storage. A failed engine-side
// construction frees the wrap before returning.

__declspec(dllexport) void* gov8_rv_pd_new(v8::Isolate* iso) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return nullptr;
  }
  PdWrap* w = nullptr;
  try {
    w = new PdWrap{};
    w->magic = kPdMagic;
    w->iso = iso;
    v8__PropertyDescriptor__CONSTRUCT(w->pd_storage);
    return w;
  } catch (...) {
    delete w;
    SetErr("C++ exception in pd_new");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_rv_pd_new_value(v8::Isolate* iso,
                                                 void* value) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return nullptr;
  }
  if (value == nullptr) {
    SetErr("null value wire");
    return nullptr;
  }
  PdWrap* w = nullptr;
  try {
    w = new PdWrap{};
    w->magic = kPdMagic;
    w->iso = iso;
    v8__PropertyDescriptor__CONSTRUCT__Value(w->pd_storage,
                                             FromWire<v8::Value>(value));
    return w;
  } catch (...) {
    delete w;
    SetErr("C++ exception in pd_new_value");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_rv_pd_new_value_writable(v8::Isolate* iso,
                                                          void* value,
                                                          int32_t writable) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return nullptr;
  }
  if (value == nullptr) {
    SetErr("null value wire");
    return nullptr;
  }
  PdWrap* w = nullptr;
  try {
    w = new PdWrap{};
    w->magic = kPdMagic;
    w->iso = iso;
    v8__PropertyDescriptor__CONSTRUCT__Value_Writable(
        w->pd_storage, FromWire<v8::Value>(value), writable != 0);
    return w;
  } catch (...) {
    delete w;
    SetErr("C++ exception in pd_new_value_writable");
    return nullptr;
  }
}

__declspec(dllexport) void* gov8_rv_pd_new_get_set(v8::Isolate* iso,
                                                   void* get, void* set) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("null isolate");
    return nullptr;
  }
  if (get == nullptr) {
    SetErr("null value wire");
    return nullptr;
  }
  if (set == nullptr) {
    SetErr("null value wire");
    return nullptr;
  }
  PdWrap* w = nullptr;
  try {
    w = new PdWrap{};
    w->magic = kPdMagic;
    w->iso = iso;
    v8__PropertyDescriptor__CONSTRUCT__Get_Set(w->pd_storage,
                                               FromWire<v8::Value>(get),
                                               FromWire<v8::Value>(set));
    return w;
  } catch (...) {
    delete w;
    SetErr("C++ exception in pd_new_get_set");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_rv_pd_dispose(void* pdw) {
  ClearErr();
  PdWrap* w = AsPd(pdw);
  if (w == nullptr) {
    SetErr("invalid descriptor handle");
    return kErrMagic;
  }
  try {
    w->pd()->~PropertyDescriptor();
    w->magic = 0;
    delete w;
    return kOk;
  } GOV8_RV_END_CATCH("pd_dispose")
}

#define GOV8_RV_PD_BOOL(name, artifact)                                        \
  __declspec(dllexport) int64_t name(void* pdw) {                              \
    ClearErr();                                                                \
    PdWrap* w = AsPd(pdw);                                                     \
    if (w == nullptr) {                                                        \
      SetErr("invalid descriptor handle");                                     \
      return kErrMagic;                                                        \
    }                                                                          \
    try {                                                                      \
      return artifact(w->pd()) ? 1 : 0;                                        \
    } GOV8_RV_END_CATCH(#name)                                                 \
  }

GOV8_RV_PD_BOOL(gov8_rv_pd_has_value, v8__PropertyDescriptor__has_value)
GOV8_RV_PD_BOOL(gov8_rv_pd_has_writable, v8__PropertyDescriptor__has_writable)
GOV8_RV_PD_BOOL(gov8_rv_pd_has_enumerable, v8__PropertyDescriptor__has_enumerable)
GOV8_RV_PD_BOOL(gov8_rv_pd_has_configurable,
                v8__PropertyDescriptor__has_configurable)
GOV8_RV_PD_BOOL(gov8_rv_pd_has_get, v8__PropertyDescriptor__has_get)
GOV8_RV_PD_BOOL(gov8_rv_pd_has_set, v8__PropertyDescriptor__has_set)
GOV8_RV_PD_BOOL(gov8_rv_pd_writable, v8__PropertyDescriptor__writable)
GOV8_RV_PD_BOOL(gov8_rv_pd_enumerable, v8__PropertyDescriptor__enumerable)
GOV8_RV_PD_BOOL(gov8_rv_pd_configurable, v8__PropertyDescriptor__configurable)

#undef GOV8_RV_PD_BOOL

#define GOV8_RV_PD_VALUE(name, artifact)                                       \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* scope, void* pdw, \
                                     void** out) {                             \
    ClearErr();                                                                \
    PdWrap* w = AsPd(pdw);                                                     \
    if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||              \
        !OwnedBy(iso, w->iso)) {                                               \
      SetErr("invalid argument");                                              \
      return kErrBadArg;                                                       \
    }                                                                          \
    try {                                                                      \
      *out = const_cast<void*>(static_cast<const void*>(artifact(w->pd())));   \
      return kOk;                                                              \
    } GOV8_RV_END_CATCH(#name)                                                 \
  }

GOV8_RV_PD_VALUE(gov8_rv_pd_value, v8__PropertyDescriptor__value)
GOV8_RV_PD_VALUE(gov8_rv_pd_get, v8__PropertyDescriptor__get)
GOV8_RV_PD_VALUE(gov8_rv_pd_set, v8__PropertyDescriptor__set)

#undef GOV8_RV_PD_VALUE

__declspec(dllexport) int64_t gov8_rv_pd_set_enumerable(void* pdw,
                                                        int32_t enumerable) {
  ClearErr();
  PdWrap* w = AsPd(pdw);
  if (w == nullptr) {
    SetErr("invalid descriptor handle");
    return kErrMagic;
  }
  try {
    v8__PropertyDescriptor__set_enumerable(w->pd(), enumerable != 0);
    return kOk;
  } GOV8_RV_END_CATCH("pd_set_enumerable")
}

__declspec(dllexport) int64_t gov8_rv_pd_set_configurable(void* pdw,
                                                          int32_t configurable) {
  ClearErr();
  PdWrap* w = AsPd(pdw);
  if (w == nullptr) {
    SetErr("invalid descriptor handle");
    return kErrMagic;
  }
  try {
    v8__PropertyDescriptor__set_configurable(w->pd(), configurable != 0);
    return kOk;
  } GOV8_RV_END_CATCH("pd_set_configurable")
}

}  // extern "C"
