// ===========================================================================
// Object-operations and value-conversion/predicate feature slice: the Object
// and Value APIs the pinned Rust oracle characterizes in
// rust-oracle/src/bin/conformance-object-ops.rs (crate v8 =152.2.0) that no
// other slice covers:
//
//   - Prototype: Object::GetPrototypeV2 / SetPrototypeV2 (null prototype,
//     cyclic rejection reported as an empty maybe WITHOUT a pending
//     exception).
//   - Has/delete family: Has (arbitrary key values), HasIndex,
//     HasOwnProperty (Name keys), Delete, DeleteIndex - each a Maybe<bool>.
//   - Real-named queries: GetRealNamedProperty (empty on miss, empty on
//     throw - discriminated via the caller's TryCatch), HasRealNamedProperty,
//     GetRealNamedPropertyAttributes (Maybe<PropertyAttribute>: Nothing on
//     miss, unlike GetPropertyAttributes which reports Just(NONE)).
//   - Identity: GetIdentityHash, creation-context identity against a Go
//     context wrapper (the engine's own context equality on the wires),
//     GetConstructorName.
//   - Receivers: GetWithReceiver / SetWithReceiver.
//   - Instance-level accessors: SetAccessor (AccessorNameGetter/Setter
//     through the shared template-callbacks trampolines) and
//     SetLazyDataProperty (getter-only, PropertyAttribute::NONE,
//     SideEffectType::kHasSideEffect - the pinned crate's defaults).
//   - Call-as-function/constructor (Object::CallAsFunction /
//     CallAsConstructor; the pinned crate's plain and *_with_context
//     variants share these bindings, differing only in which context local
//     is passed).
//   - Conversions: Value::ToObject / ToBoolean / ToInteger / ToBigInt /
//     ToDetailString, Value::InstanceOf.
//   - The missing-predicates inventory: every Value::is_* the object-ops
//     slice pins that no other slice exports (IsFalse, IsArgumentsObject,
//     IsSymbolObject, IsNativeError, IsAsyncFunction, IsGeneratorFunction,
//     IsPromise, IsMapIterator, IsSetIterator, IsGeneratorObject, IsWeakMap,
//     IsWeakSet). The typed-array predicates (IsTypedArray and the twelve
//     per-kind Is*Array) already have exports in the buffers-serialization
//     and typed-arrays slices (gov8_is_typed_array, gov8_is_*_array); the Go
//     layer of this slice reuses those exports instead of duplicating them.
//
// Receiver-kind defense in depth: the pinned crate performs NO runtime type
// checks on Object receivers (a confounded local is an access violation -
// rust-oracle/tests/object_ops_negative.rs), so every export below that
// binds a wire as Object& first validates the wire with the artifact's
// v8__Value__IsObject and fails with kErrBadArg instead of entering
// undefined behavior. The Go wrapper documents this contract as the reason
// its own *Object methods cannot skip validation.
//
// Cross-slice reuse (same translation unit; textually included after
// templates_callbacks.inc, snapshots_handles.inc and runtime_values.inc):
// gov8_snap_impl::DerefLocal, gov8_rv_impl::{MaybeBoolStatus,
// MaybeLocalStatus, RvResolveTc}, FallbackTryCatch, the GOV8_RV_NULL_WIRE /
// GOV8_RV_END_CATCH macros, the ToWire/FromWire helpers, and the accessor
// trampolines (HostAccessorGetTrampoline / HostAccessorSetTrampoline) with
// HostContextExternalWire and the GoCallbackContext type.
// ===========================================================================

#include <cstdint>

namespace gov8_oo_impl {

// Validates that a wire holds a JSReceiver-backed value before any typed
// Object& binding dereferences it. Pure object-header reads; no allocation.
inline bool IsReceiverWire(void* wire) {
  if (wire == nullptr) {
    return false;
  }
  v8::Local<v8::Value> v = FromWire<v8::Value>(wire);
  return v8__Value__IsObject(gov8_snap_impl::DerefLocal(v));
}

// SideEffectType (v8-object.h): kHasSideEffect = 0.
constexpr int32_t kSideEffectHasSideEffect = 0;

}  // namespace gov8_oo_impl

// --- artifact bindings used by this slice (compiled into the static lib) ----
//
// Already declared by earlier slices in this translation unit and NOT
// redeclared here: v8__Value__IsObject / v8__Value__IsName
// (runtime_values.inc, reference form), v8__Value__IsTypedArray
// (buffers_serialization.inc, pointer form).

extern "C" {
const v8::Value* v8__Object__GetPrototype(const v8::Object& self);
int32_t v8__Object__SetPrototype(const v8::Object& self,
                                 const v8::Context& context,
                                 const v8::Value& prototype);
int32_t v8__Object__Has(const v8::Object& self, const v8::Context& context,
                        const v8::Value& key);
int32_t v8__Object__HasIndex(const v8::Object& self, const v8::Context& context,
                             uint32_t index);
int32_t v8__Object__HasOwnProperty(const v8::Object& self,
                                   const v8::Context& context,
                                   const v8::Name& key);
int32_t v8__Object__Delete(const v8::Object& self, const v8::Context& context,
                           const v8::Value& key);
int32_t v8__Object__DeleteIndex(const v8::Object& self,
                                const v8::Context& context, uint32_t index);
const v8::Value* v8__Object__GetRealNamedProperty(const v8::Object& self,
                                                  const v8::Context& context,
                                                  const v8::Name& key);
int32_t v8__Object__HasRealNamedProperty(const v8::Object& self,
                                         const v8::Context& context,
                                         const v8::Name& key);
void v8__Object__GetRealNamedPropertyAttributes(
    const v8::Object& self, const v8::Context& context, const v8::Name& key,
    v8::Maybe<v8::PropertyAttribute>* out);
int v8__Object__GetIdentityHash(const v8::Object& self);
const v8::Context* v8__Object__GetCreationContext(const v8::Object& self);
const v8::String* v8__Object__GetConstructorName(v8::Object& self);
const v8::Value* v8__Object__GetWithReceiver(const v8::Object& self,
                                             const v8::Context& context,
                                             const v8::Value& key,
                                             const v8::Object& receiver);
int32_t v8__Object__SetWithReceiver(const v8::Object& self,
                                    const v8::Context& context,
                                    const v8::Value& key,
                                    const v8::Value& value,
                                    const v8::Object& receiver);
int32_t v8__Object__SetAccessor(const v8::Object& self,
                                const v8::Context& context,
                                const v8::Name& key,
                                v8::AccessorNameGetterCallback getter,
                                v8::AccessorNameSetterCallback setter,
                                const v8::Value* data_or_null,
                                v8::PropertyAttribute attr);
int32_t v8__Object__SetLazyDataProperty(
    const v8::Object& self, const v8::Context& context, const v8::Name& key,
    v8::AccessorNameGetterCallback getter, const v8::Value* data_or_null,
    v8::PropertyAttribute attr, v8::SideEffectType getter_side_effect_type,
    v8::SideEffectType setter_side_effect_type);
bool v8__Object__IsCallable(const v8::Object& self);
bool v8__Object__IsConstructor(const v8::Object& self);
const v8::Value* v8__Object__CallAsFunction(const v8::Object& self,
                                            const v8::Context& context,
                                            const v8::Value& recv, int argc,
                                            const v8::Value* const argv[]);
const v8::Value* v8__Object__CallAsConstructor(const v8::Object& self,
                                               const v8::Context& context,
                                               int argc,
                                               const v8::Value* const argv[]);
const v8::Object* v8__Value__ToObject(const v8::Value& self,
                                      const v8::Context& context);
const v8::Boolean* v8__Value__ToBoolean(const v8::Value& self,
                                        v8::Isolate* isolate);
const v8::Integer* v8__Value__ToInteger(const v8::Value& self,
                                        const v8::Context& context);
const v8::BigInt* v8__Value__ToBigInt(const v8::Value& self,
                                      const v8::Context& context);
const v8::Number* v8__Value__ToNumber(const v8::Value& self,
                                      const v8::Context& context);
const v8::String* v8__Value__ToString(const v8::Value& self,
                                      const v8::Context& context);
const v8::String* v8__Value__ToDetailString(const v8::Value& self,
                                            const v8::Context& context);
const v8::Uint32* v8__Value__ToUint32(const v8::Value& self,
                                      const v8::Context& context);
const v8::Int32* v8__Value__ToInt32(const v8::Value& self,
                                    const v8::Context& context);
void v8__Value__InstanceOf(const v8::Value& self, const v8::Context& context,
                           const v8::Object& object, v8::Maybe<bool>* out);

bool v8__Data__EQ(const v8::Data* self, const v8::Data* other);
bool v8__Data__IsBigInt(const v8::Data& self);
bool v8__Data__IsBoolean(const v8::Data& self);
bool v8__Data__IsContext(const v8::Data& self);
bool v8__Data__IsFunctionTemplate(const v8::Data& self);
bool v8__Data__IsModule(const v8::Data& self);
bool v8__Data__IsName(const v8::Data& self);
bool v8__Data__IsNumber(const v8::Data& self);
bool v8__Data__IsObjectTemplate(const v8::Data& self);
bool v8__Data__IsPrivate(const v8::Data& self);
bool v8__Data__IsString(const v8::Data& self);
bool v8__Data__IsSymbol(const v8::Data& self);

bool v8__Value__IsFalse(const v8::Value& self);
bool v8__Value__IsArgumentsObject(const v8::Value& self);
bool v8__Value__IsSymbolObject(const v8::Value& self);
bool v8__Value__IsNativeError(const v8::Value& self);
bool v8__Value__IsAsyncFunction(const v8::Value& self);
bool v8__Value__IsGeneratorFunction(const v8::Value& self);
bool v8__Value__IsPromise(const v8::Value& self);
bool v8__Value__IsMapIterator(const v8::Value& self);
bool v8__Value__IsSetIterator(const v8::Value& self);
bool v8__Value__IsGeneratorObject(const v8::Value& self);
bool v8__Value__IsWeakMap(const v8::Value& self);
bool v8__Value__IsWeakSet(const v8::Value& self);
bool v8__Value__IsModuleNamespaceObject(const v8::Value& self);
}

extern "C" {

using namespace gov8_rv_impl;

// Receiver-kind validation shared by every Object-receiver export below:
// the shim's own line of defense behind the Go wrapper's scope/isolate
// checks. The _P flavor is for exports returning a wire (nullptr on
// rejection) instead of a status. Like the other wire-producing slices,
// these exports do NOT open their own HandleScope: result wires must stay
// valid in the CALLER's open Go Scope after the export returns.
#define GOV8_OO_REQUIRE_RECEIVER(wire)                                         \
  if (!gov8_oo_impl::IsReceiverWire(wire)) {                                   \
    SetErr("receiver is not an object");                                       \
    return kErrBadArg;                                                         \
  }

#define GOV8_OO_REQUIRE_RECEIVER_P(wire)                                       \
  if (!gov8_oo_impl::IsReceiverWire(wire)) {                                   \
    SetErr("receiver is not an object");                                       \
    return nullptr;                                                            \
  }

// --- prototype -----------------------------------------------------------------

__declspec(dllexport) void* gov8_oo_object_get_prototype(v8::Isolate* iso,
                                                         void* scope,
                                                         void* obj) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER_P(obj)
    return const_cast<void*>(static_cast<const void*>(v8__Object__GetPrototype(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)))));
  } catch (...) {
    SetErr("C++ exception in oo_object_get_prototype");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_oo_object_set_prototype(v8::Isolate* iso,
                                                           void* ctxw,
                                                           void* scope,
                                                           void* obj,
                                                           void* proto,
                                                           int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(proto)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    // The engine's cyclic __proto__ rejection surfaces as Maybe::Nothing
    // WITHOUT a pending exception (the pinned crate's documented behavior);
    // MaybeBoolStatus reports kErrException and the caller's HasCaught stays
    // false.
    return MaybeBoolStatus(
        v8__Object__SetPrototype(
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
            gov8_snap_impl::DerefLocal(ctx),
            gov8_snap_impl::DerefLocal(FromWire<v8::Value>(proto))),
        ok);
  } GOV8_RV_END_CATCH("oo_object_set_prototype")
}

// --- has / delete family ---------------------------------------------------------

__declspec(dllexport) int64_t gov8_oo_object_has(v8::Isolate* iso, void* ctxw,
                                                 void* tcw, void* scope,
                                                 void* obj, void* key,
                                                 int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__Has(gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
                        gov8_snap_impl::DerefLocal(ctx),
                        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key))),
        ok);
  } GOV8_RV_END_CATCH("oo_object_has")
}

__declspec(dllexport) int64_t gov8_oo_object_has_index(v8::Isolate* iso,
                                                       void* ctxw, void* tcw,
                                                       void* scope, void* obj,
                                                       uint32_t index,
                                                       int32_t* ok) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__HasIndex(
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
            gov8_snap_impl::DerefLocal(ctx), index),
        ok);
  } GOV8_RV_END_CATCH("oo_object_has_index")
}

__declspec(dllexport) int64_t gov8_oo_object_has_own_property(
    v8::Isolate* iso, void* ctxw, void* tcw, void* scope, void* obj,
    void* key, int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    v8::Local<v8::Value> kv = FromWire<v8::Value>(key);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(kv))) {
      SetErr("key is not a Name (string or symbol)");
      return kErrBadArg;
    }
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__HasOwnProperty(
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
            gov8_snap_impl::DerefLocal(ctx),
            gov8_snap_impl::DerefLocal(kv.As<v8::Name>())),
        ok);
  } GOV8_RV_END_CATCH("oo_object_has_own_property")
}

__declspec(dllexport) int64_t gov8_oo_object_delete(v8::Isolate* iso,
                                                    void* ctxw, void* tcw,
                                                    void* scope, void* obj,
                                                    void* key, int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__Delete(
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
            gov8_snap_impl::DerefLocal(ctx),
            gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key))),
        ok);
  } GOV8_RV_END_CATCH("oo_object_delete")
}

__declspec(dllexport) int64_t gov8_oo_object_delete_index(v8::Isolate* iso,
                                                          void* ctxw,
                                                          void* tcw,
                                                          void* scope,
                                                          void* obj,
                                                          uint32_t index,
                                                          int32_t* ok) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__DeleteIndex(
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
            gov8_snap_impl::DerefLocal(ctx), index),
        ok);
  } GOV8_RV_END_CATCH("oo_object_delete_index")
}

// --- real-named queries -----------------------------------------------------------
//
// GetRealNamedProperty resolves to an EMPTY maybe both on a throw and on a
// plain miss; the caller's TryCatch discriminates (HasCaught => exception,
// else a genuine miss reported as a null wire with kOk).

__declspec(dllexport) int64_t gov8_oo_object_get_real_named_property(
    v8::Isolate* iso, void* ctxw, void* tcw, void* scope, void* obj,
    void* key, void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    v8::Local<v8::Value> kv = FromWire<v8::Value>(key);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(kv))) {
      SetErr("key is not a Name (string or symbol)");
      return kErrBadArg;
    }
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Value* got = v8__Object__GetRealNamedProperty(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
        gov8_snap_impl::DerefLocal(ctx),
        gov8_snap_impl::DerefLocal(kv.As<v8::Name>()));
    if (got == nullptr) {
      if (tc->HasCaught()) {
        return kErrException;
      }
      // A real miss: no exception, null wire, kOk.
      *out = nullptr;
      return kOk;
    }
    *out = const_cast<void*>(static_cast<const void*>(got));
    return kOk;
  } GOV8_RV_END_CATCH("oo_object_get_real_named_property")
}

__declspec(dllexport) int64_t gov8_oo_object_has_real_named_property(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    v8::Local<v8::Value> kv = FromWire<v8::Value>(key);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(kv))) {
      SetErr("key is not a Name (string or symbol)");
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__HasRealNamedProperty(
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
            gov8_snap_impl::DerefLocal(ctx),
            gov8_snap_impl::DerefLocal(kv.As<v8::Name>())),
        ok);
  } GOV8_RV_END_CATCH("oo_object_has_real_named_property")
}

// Maybe<PropertyAttribute>: *present receives 1 for Just(attr) and 0 for
// Nothing (the engine's miss report - unlike GetPropertyAttributes, which
// reports Just(NONE) for a missing property). A throw surfaces as
// kErrException with *present untouched.
__declspec(dllexport) int64_t gov8_oo_object_get_real_named_property_attributes(
    v8::Isolate* iso, void* ctxw, void* tcw, void* scope, void* obj,
    void* key, int32_t* present, int32_t* attr) {
  ClearErr();
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || present == nullptr || attr == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    v8::Local<v8::Value> kv = FromWire<v8::Value>(key);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(kv))) {
      SetErr("key is not a Name (string or symbol)");
      return kErrBadArg;
    }
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Maybe<v8::PropertyAttribute> out;
    v8__Object__GetRealNamedPropertyAttributes(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
        gov8_snap_impl::DerefLocal(ctx),
        gov8_snap_impl::DerefLocal(kv.As<v8::Name>()), &out);
    if (out.IsNothing()) {
      if (tc->HasCaught()) {
        return kErrException;
      }
      *present = 0;
      *attr = 0;
      return kOk;
    }
    *present = 1;
    *attr = static_cast<int32_t>(out.FromJust());
    return kOk;
  } GOV8_RV_END_CATCH("oo_object_get_real_named_property_attributes")
}

// --- identity -------------------------------------------------------------------

__declspec(dllexport) int64_t gov8_oo_object_identity_hash(v8::Isolate* iso,
                                                           void* scope,
                                                           void* obj,
                                                           int32_t* out) {
  ClearErr();
  if (out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    *out = v8__Object__GetIdentityHash(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)));
    return kOk;
  } GOV8_RV_END_CATCH("oo_object_identity_hash")
}

// Returns the object's creation Context as a local wire in the caller's open
// Scope. A null wire is the pinned crate's empty Option and is not an error.
__declspec(dllexport) int64_t gov8_oo_object_creation_context(
    v8::Isolate* iso, void* scope, void* obj, void** out) {
  ClearErr();
  if (out == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  *out = nullptr;
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    const v8::Context* created = v8__Object__GetCreationContext(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)));
    *out = const_cast<void*>(static_cast<const void*>(created));
    return kOk;
  } GOV8_RV_END_CATCH("oo_object_creation_context")
}

// Creation-context identity: reports whether the object was created in the
// context wrapper ctxw (the engine's own context equality on the wires).
// An object with NO creation context (empty maybe) is kErrState, not a
// mismatch - the pinned crate maps that case to None.
__declspec(dllexport) int64_t gov8_oo_object_creation_context_eq(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, int32_t* same) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || same == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    const v8::Context* created = v8__Object__GetCreationContext(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)));
    if (created == nullptr) {
      SetErr("object has no creation context");
      return kErrState;
    }
    void* created_wire = const_cast<void*>(static_cast<const void*>(created));
    void* target_wire = ToWire(w->ctx->Get(iso));
    *same = gov8_ca_context_eq(created_wire, target_wire) == 1 ? 1 : 0;
    return kOk;
  } GOV8_RV_END_CATCH("oo_object_creation_context_eq")
}

__declspec(dllexport) void* gov8_oo_object_constructor_name(v8::Isolate* iso,
                                                            void* scope,
                                                            void* obj) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER_P(obj)
    return const_cast<void*>(static_cast<const void*>(
        v8__Object__GetConstructorName(
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)))));
  } catch (...) {
    SetErr("C++ exception in oo_object_constructor_name");
    return nullptr;
  }
}

// --- receivers -------------------------------------------------------------------

__declspec(dllexport) int64_t gov8_oo_object_get_with_receiver(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    void* receiver, void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(key)
  GOV8_RV_NULL_WIRE(receiver)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    GOV8_OO_REQUIRE_RECEIVER(receiver)
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Value* got = v8__Object__GetWithReceiver(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
        gov8_snap_impl::DerefLocal(ctx),
        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key)),
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(receiver)));
    return MaybeLocalStatus(got, out);
  } GOV8_RV_END_CATCH("oo_object_get_with_receiver")
}

__declspec(dllexport) int64_t gov8_oo_object_set_with_receiver(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    void* value, void* receiver, int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(key)
  GOV8_RV_NULL_WIRE(value)
  GOV8_RV_NULL_WIRE(receiver)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    GOV8_OO_REQUIRE_RECEIVER(receiver)
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__SetWithReceiver(
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
            gov8_snap_impl::DerefLocal(ctx),
            gov8_snap_impl::DerefLocal(FromWire<v8::Value>(key)),
            gov8_snap_impl::DerefLocal(FromWire<v8::Value>(value)),
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(receiver))),
        ok);
  } GOV8_RV_END_CATCH("oo_object_set_with_receiver")
}

// --- instance-level accessors and lazy properties ---------------------------------
//
// The getter/setter dispatch through the shared accessor trampolines of the
// template-callbacks slice; host_ctx is the shim dispatch context produced
// by gov8_host_context_new for the Go-registered callback pair.

__declspec(dllexport) int64_t gov8_oo_object_set_accessor(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    void* host_ctx, int32_t with_setter, int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  GoCallbackContext* cbctx = static_cast<GoCallbackContext*>(host_ctx);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      cbctx == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    v8::Local<v8::Value> kv = FromWire<v8::Value>(key);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(kv))) {
      SetErr("key is not a Name (string or symbol)");
      return kErrBadArg;
    }
    void* data_external = HostContextExternalWire(iso, cbctx);
    if (data_external == nullptr) {
      return kErrNoMemory;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::AccessorNameGetterCallback getter = HostAccessorGetTrampoline;
    v8::AccessorNameSetterCallback setter =
        with_setter != 0 ? HostAccessorSetTrampoline : nullptr;
    return MaybeBoolStatus(
        v8__Object__SetAccessor(
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
            gov8_snap_impl::DerefLocal(ctx),
            gov8_snap_impl::DerefLocal(kv.As<v8::Name>()), getter, setter,
            *FromWire<v8::Value>(data_external),
            static_cast<v8::PropertyAttribute>(0)),
        ok);
  } GOV8_RV_END_CATCH("oo_object_set_accessor")
}

__declspec(dllexport) int64_t gov8_oo_object_set_lazy_data_property(
    v8::Isolate* iso, void* ctxw, void* scope, void* obj, void* key,
    void* host_ctx, int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(key)
  CtxWrap* w = AsCtx(ctxw);
  GoCallbackContext* cbctx = static_cast<GoCallbackContext*>(host_ctx);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      cbctx == nullptr || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    v8::Local<v8::Value> kv = FromWire<v8::Value>(key);
    if (!v8__Value__IsName(gov8_snap_impl::DerefLocal(kv))) {
      SetErr("key is not a Name (string or symbol)");
      return kErrBadArg;
    }
    void* data_external = HostContextExternalWire(iso, cbctx);
    if (data_external == nullptr) {
      return kErrNoMemory;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    return MaybeBoolStatus(
        v8__Object__SetLazyDataProperty(
            gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
            gov8_snap_impl::DerefLocal(ctx),
            gov8_snap_impl::DerefLocal(kv.As<v8::Name>()),
            HostAccessorGetTrampoline, *FromWire<v8::Value>(data_external),
            static_cast<v8::PropertyAttribute>(0),
            static_cast<v8::SideEffectType>(
                gov8_oo_impl::kSideEffectHasSideEffect),
            static_cast<v8::SideEffectType>(
                gov8_oo_impl::kSideEffectHasSideEffect)),
        ok);
  } GOV8_RV_END_CATCH("oo_object_set_lazy_data_property")
}

// --- call-as-function / constructor ----------------------------------------------
//
// argv is a Go-filled array of wire words (const v8::Value*), argc the
// element count; the pinned crate's plain and _with_context variants share
// these bindings, differing only in which context local is passed.

__declspec(dllexport) int64_t gov8_oo_object_is_callable(v8::Isolate* iso,
                                                         void* scope,
                                                         void* obj) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    return v8__Object__IsCallable(
               gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)))
               ? 1
               : 0;
  } GOV8_RV_END_CATCH("oo_object_is_callable")
}

__declspec(dllexport) int64_t gov8_oo_object_is_constructor(v8::Isolate* iso,
                                                            void* scope,
                                                            void* obj) {
  ClearErr();
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::HandleScope hs(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    return v8__Object__IsConstructor(
               gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)))
               ? 1
               : 0;
  } GOV8_RV_END_CATCH("oo_object_is_constructor")
}

__declspec(dllexport) int64_t gov8_oo_object_call_as_function(
    v8::Isolate* iso, void* ctxw, void* tcw, void* scope, void* obj,
    void* recv, int32_t argc, void** argv, void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(recv)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Value* const* args =
        reinterpret_cast<const v8::Value* const*>(argv);
    const v8::Value* got = v8__Object__CallAsFunction(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
        gov8_snap_impl::DerefLocal(ctx),
        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(recv)), argc, args);
    return MaybeLocalStatus(got, out);
  } GOV8_RV_END_CATCH("oo_object_call_as_function")
}

__declspec(dllexport) int64_t gov8_oo_object_call_as_constructor(
    v8::Isolate* iso, void* ctxw, void* tcw, void* scope, void* obj,
    int32_t argc, void** argv, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Value* const* args =
        reinterpret_cast<const v8::Value* const*>(argv);
    const v8::Value* got = v8__Object__CallAsConstructor(
        gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
        gov8_snap_impl::DerefLocal(ctx), argc, args);
    return MaybeLocalStatus(got, out);
  } GOV8_RV_END_CATCH("oo_object_call_as_constructor")
}

// --- value conversions ------------------------------------------------------------

__declspec(dllexport) int64_t gov8_oo_value_to_object(v8::Isolate* iso,
                                                      void* ctxw, void* tcw,
                                                      void* scope, void* v,
                                                      void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(v)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Object* got = v8__Value__ToObject(
        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(v)),
        gov8_snap_impl::DerefLocal(ctx));
    return MaybeLocalStatus(got, out);
  } GOV8_RV_END_CATCH("oo_value_to_object")
}

__declspec(dllexport) void* gov8_oo_value_to_boolean(v8::Isolate* iso,
                                                     void* scope, void* v) {
  ClearErr();
  if (v == nullptr) {
    SetErr("null value wire");
    return nullptr;
  }
  if (!ScopeIs(iso, scope)) {
    SetErr("invalid scope handle for this isolate");
    return nullptr;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return const_cast<void*>(static_cast<const void*>(v8__Value__ToBoolean(
        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(v)), iso)));
  } catch (...) {
    SetErr("C++ exception in oo_value_to_boolean");
    return nullptr;
  }
}

__declspec(dllexport) int64_t gov8_oo_value_to_integer(v8::Isolate* iso,
                                                       void* ctxw, void* tcw,
                                                       void* scope, void* v,
                                                       void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(v)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::Integer* got = v8__Value__ToInteger(
        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(v)),
        gov8_snap_impl::DerefLocal(ctx));
    return MaybeLocalStatus(got, out);
  } GOV8_RV_END_CATCH("oo_value_to_integer")
}

__declspec(dllexport) int64_t gov8_oo_value_to_big_int(v8::Isolate* iso,
                                                       void* ctxw, void* tcw,
                                                       void* scope, void* v,
                                                       void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(v)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::BigInt* got = v8__Value__ToBigInt(
        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(v)),
        gov8_snap_impl::DerefLocal(ctx));
    return MaybeLocalStatus(got, out);
  } GOV8_RV_END_CATCH("oo_value_to_big_int")
}

#define GOV8_OO_LOCAL_CONVERSION(name, binding, label)                         \
  __declspec(dllexport) int64_t name(                                          \
      v8::Isolate* iso, void* ctxw, void* tcw, void* scope, void* v,           \
      void** out) {                                                             \
    ClearErr();                                                                 \
    GOV8_RV_NULL_WIRE(v)                                                        \
    CtxWrap* w = AsCtx(ctxw);                                                   \
    if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||              \
        !OwnedBy(iso, w->iso)) {                                                \
      SetErr("invalid argument");                                              \
      return kErrBadArg;                                                        \
    }                                                                           \
    try {                                                                       \
      Gov8IsolateScope iso_scope(iso);                                          \
      FallbackTryCatch fallback;                                                \
      v8::TryCatch* tc = nullptr;                                               \
      if (!RvResolveTc(iso, tcw, &fallback, &tc)) return kErrBadArg;            \
      v8::Local<v8::Context> ctx = w->ctx->Get(iso);                            \
      v8::Context::Scope ctx_scope(ctx);                                        \
      const void* got = binding(                                                \
          gov8_snap_impl::DerefLocal(FromWire<v8::Value>(v)),                  \
          gov8_snap_impl::DerefLocal(ctx));                                     \
      return MaybeLocalStatus(got, out);                                        \
    } GOV8_RV_END_CATCH(label)                                                  \
  }

GOV8_OO_LOCAL_CONVERSION(gov8_oo_value_to_number, v8__Value__ToNumber,
                         "oo_value_to_number")
GOV8_OO_LOCAL_CONVERSION(gov8_oo_value_to_string, v8__Value__ToString,
                         "oo_value_to_string")
GOV8_OO_LOCAL_CONVERSION(gov8_oo_value_to_uint32, v8__Value__ToUint32,
                         "oo_value_to_uint32")
GOV8_OO_LOCAL_CONVERSION(gov8_oo_value_to_int32, v8__Value__ToInt32,
                         "oo_value_to_int32")

#undef GOV8_OO_LOCAL_CONVERSION

__declspec(dllexport) int64_t gov8_oo_value_to_detail_string(
    v8::Isolate* iso, void* ctxw, void* tcw, void* scope, void* v,
    void** out) {
  ClearErr();
  GOV8_RV_NULL_WIRE(v)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    const v8::String* got = v8__Value__ToDetailString(
        gov8_snap_impl::DerefLocal(FromWire<v8::Value>(v)),
        gov8_snap_impl::DerefLocal(ctx));
    return MaybeLocalStatus(got, out);
  } GOV8_RV_END_CATCH("oo_value_to_detail_string")
}

__declspec(dllexport) int64_t gov8_oo_value_instance_of(v8::Isolate* iso,
                                                        void* ctxw, void* tcw,
                                                        void* scope, void* v,
                                                        void* obj,
                                                        int32_t* ok) {
  ClearErr();
  GOV8_RV_NULL_WIRE(v)
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || ok == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    GOV8_OO_REQUIRE_RECEIVER(obj)
    FallbackTryCatch fallback;
    v8::TryCatch* tc = nullptr;
    if (!RvResolveTc(iso, tcw, &fallback, &tc)) {
      return kErrBadArg;
    }
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Maybe<bool> out;
    v8__Value__InstanceOf(gov8_snap_impl::DerefLocal(FromWire<v8::Value>(v)),
                          gov8_snap_impl::DerefLocal(ctx),
                          gov8_snap_impl::DerefLocal(FromWire<v8::Object>(obj)),
                          &out);
    if (out.IsNothing()) {
      if (tc->HasCaught()) {
        return kErrException;
      }
      SetErr("instance_of produced an empty maybe without an exception");
      return kErrState;
    }
    *ok = out.FromJust() ? 1 : 0;
    return kOk;
  } GOV8_RV_END_CATCH("oo_value_instance_of")
}

// --- predicates (missing inventory) ------------------------------------------------

#define GOV8_OO_PREDICATE(name, artifact)                                      \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* v) {              \
    ClearErr();                                                                \
    GOV8_RV_NULL_WIRE(v)                                                       \
    try {                                                                      \
      v8::Isolate::Scope iso_scope(iso);                                       \
      v8::HandleScope hs(iso);                                                 \
      v8::Local<v8::Value> val = FromWire<v8::Value>(v);                       \
      return artifact(gov8_snap_impl::DerefLocal(val)) ? 1 : 0;                \
    } GOV8_RV_END_CATCH(#name)                                                 \
  }

GOV8_OO_PREDICATE(gov8_oo_is_false, v8__Value__IsFalse)
GOV8_OO_PREDICATE(gov8_oo_is_arguments_object, v8__Value__IsArgumentsObject)
GOV8_OO_PREDICATE(gov8_oo_is_symbol_object, v8__Value__IsSymbolObject)
GOV8_OO_PREDICATE(gov8_oo_is_native_error, v8__Value__IsNativeError)
GOV8_OO_PREDICATE(gov8_oo_is_async_function, v8__Value__IsAsyncFunction)
GOV8_OO_PREDICATE(gov8_oo_is_generator_function, v8__Value__IsGeneratorFunction)
GOV8_OO_PREDICATE(gov8_oo_is_promise, v8__Value__IsPromise)
GOV8_OO_PREDICATE(gov8_oo_is_map_iterator, v8__Value__IsMapIterator)
GOV8_OO_PREDICATE(gov8_oo_is_set_iterator, v8__Value__IsSetIterator)
GOV8_OO_PREDICATE(gov8_oo_is_generator_object, v8__Value__IsGeneratorObject)
GOV8_OO_PREDICATE(gov8_oo_is_weak_map, v8__Value__IsWeakMap)
GOV8_OO_PREDICATE(gov8_oo_is_weak_set, v8__Value__IsWeakSet)
GOV8_OO_PREDICATE(gov8_oo_is_module_namespace_object,
                  v8__Value__IsModuleNamespaceObject)

// Data locals include metadata objects which are not Values. These endpoints
// therefore bind as Data from the start and use the pinned artifact's Data
// predicates rather than downcasting through Value.
#define GOV8_OO_DATA_PREDICATE(name, artifact)                                  \
  __declspec(dllexport) int64_t name(v8::Isolate* iso, void* wire) {            \
    ClearErr();                                                                 \
    if (iso == nullptr || wire == nullptr) {                                    \
      SetErr("invalid data predicate argument");                               \
      return kErrBadArg;                                                        \
    }                                                                           \
    try {                                                                       \
      Gov8IsolateScope iso_scope(iso);                                          \
      return artifact(                                                          \
                 gov8_snap_impl::DerefLocal(FromWire<v8::Data>(wire)))         \
                 ? 1                                                            \
                 : 0;                                                           \
    } GOV8_RV_END_CATCH(#name)                                                  \
  }

GOV8_OO_DATA_PREDICATE(gov8_oo_data_is_big_int, v8__Data__IsBigInt)
GOV8_OO_DATA_PREDICATE(gov8_oo_data_is_boolean, v8__Data__IsBoolean)
GOV8_OO_DATA_PREDICATE(gov8_oo_data_is_context, v8__Data__IsContext)
GOV8_OO_DATA_PREDICATE(gov8_oo_data_is_function_template,
                       v8__Data__IsFunctionTemplate)
GOV8_OO_DATA_PREDICATE(gov8_oo_data_is_module, v8__Data__IsModule)
GOV8_OO_DATA_PREDICATE(gov8_oo_data_is_name, v8__Data__IsName)
GOV8_OO_DATA_PREDICATE(gov8_oo_data_is_number, v8__Data__IsNumber)
GOV8_OO_DATA_PREDICATE(gov8_oo_data_is_object_template,
                       v8__Data__IsObjectTemplate)
GOV8_OO_DATA_PREDICATE(gov8_oo_data_is_private, v8__Data__IsPrivate)
GOV8_OO_DATA_PREDICATE(gov8_oo_data_is_string, v8__Data__IsString)
GOV8_OO_DATA_PREDICATE(gov8_oo_data_is_symbol, v8__Data__IsSymbol)

#undef GOV8_OO_DATA_PREDICATE

__declspec(dllexport) int64_t gov8_oo_data_equal(v8::Isolate* iso, void* a,
                                                  void* b) {
  ClearErr();
  if (iso == nullptr || a == nullptr || b == nullptr) {
    SetErr("invalid data equality argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return v8__Data__EQ(
               &gov8_snap_impl::DerefLocal(FromWire<v8::Data>(a)),
               &gov8_snap_impl::DerefLocal(FromWire<v8::Data>(b)))
               ? 1
               : 0;
  } GOV8_RV_END_CATCH("oo_data_equal")
}

__declspec(dllexport) int64_t gov8_oo_context_data(
    v8::Isolate* iso, void* ctxw, void* scope, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid context data argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    *out = ToWire(w->ctx->Get(iso));
    return kOk;
  } GOV8_RV_END_CATCH("oo_context_data")
}

__declspec(dllexport) int64_t gov8_oo_module_data(
    v8::Isolate* iso, void* modulew, void* scope, void** out) {
  ClearErr();
  ModuleWrap* w = AsModule(modulew);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid module data argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    *out = ToWire(w->module->Get(iso));
    return kOk;
  } GOV8_RV_END_CATCH("oo_module_data")
}

}  // extern "C"
