// ===========================================================================
// Promise / PromiseResolver native API (feature slice).
//
// Exports (all invoked from Go through the dynamic proc lookup):
//   - resolver construction and promise retrieval, state, result,
//     has-handler, mark-as-handled
//   - resolve/reject with call-success semantics (the returned bool reports
//     the success of the call, NOT a settlement change; a second resolve or
//     a reject after settlement is silently ignored and still succeeds)
//   - then / then2 / catch returning the derived native promise
//   - the isolate promise-reject callback with event ordering
//
// Native reaction handlers (Go callbacks)
// ---------------------------------------
// Go pointers must not be retained across the ABI, so native functions are
// backed by an INTEGER callback registry on the Go side:
//
//   1. Go registers its two dispatch trampolines (created once per process
//      with syscall.NewCallback, whose entries the Go runtime pins for the
//      process lifetime) via gov8_promise_fn_set_entry and
//      gov8_promise_reject_set_entry. These are the only function pointers
//      that ever cross the boundary; everything else is an int64 registry id.
//   2. gov8_promise_fn_new creates a v8::Function whose callback data is the
//      registry id (a v8::Integer).
//   3. When the engine invokes such a function (e.g. from a promise reaction
//      job during a microtask checkpoint), PromiseNativeTrampoline reads the
//      id and calls the Go trampoline with (id, argc, argv wires, rv out).
//   4. The isolate promise-reject callback works the same way with a
//      per-isolate id map; V8 gives the callback no data parameter, so the
//      entered isolate (Isolate::TryGetCurrent) resolves the id.
//
// Wire validity: argument/result/reject-message handles are slot words
// created in the engine-owned handle scope this trampoline opens; they are
// valid exactly for the duration of the synchronous Go callback. No C++
// memory escapes to Go, and no Go memory is retained by the shim.
//
// Lifecycle notes: g_reject_ids entries are removed by
// gov8_isolate_clear_promise_reject_callback. An entry left behind for an
// isolate that is disposed without clearing is inert: a recycled isolate
// pointer only reaches the trampoline through SetPromiseRejectCallback,
// which overwrites the map entry first.
// ===========================================================================

#include <mutex>
#include <unordered_map>
#include <vector>

namespace gov8_promise_impl {

// Go trampoline for native function invocation: (id, argc, argv, rv out) ->
// 1 when a return value was written to *rv, 0 otherwise.
using NativeEntry = uintptr_t (*)(int64_t id, int64_t argc, void** argv,
                                  void** rv);

// Go trampoline for the promise-reject callback:
// (id, event, value wire or null, promise wire) -> 1 (return value unused).
using RejectEntry = uintptr_t (*)(int64_t id, int64_t event, void* value_wire,
                                  void* promise_wire);

NativeEntry g_native_entry = nullptr;
RejectEntry g_reject_entry = nullptr;

// Per-isolate registry id for the promise-reject callback. The map is
// mutated from the isolate owner threads (registration) and read on the
// owning thread during reject dispatch; the mutex makes registration from
// different isolates' threads safe.
std::mutex g_reject_mu;
std::unordered_map<v8::Isolate*, int64_t> g_reject_ids;

void PromiseNativeTrampoline(const v8::FunctionCallbackInfo<v8::Value>& info) {
  NativeEntry entry = g_native_entry;
  if (entry == nullptr) {
    SetErr("promise native callback entry not registered");
    return;
  }
  try {
    v8::HandleScope hs(info.GetIsolate());
    int64_t id = -1;
    v8::Local<v8::Value> data = info.Data();
    if (!data.IsEmpty() && data->IsInt32()) {
      id = data.As<v8::Integer>()->Value();
    }
    if (id <= 0) {
      SetErr("promise native callback has no registry id");
      return;
    }
    const int argc = info.Length();
    std::vector<void*> argv;
    argv.reserve(argc > 0 ? static_cast<size_t>(argc) : 0);
    for (int i = 0; i < argc; ++i) {
      argv.push_back(ToWire(info[i]));
    }
    void* rv = nullptr;
    const uintptr_t handled = entry(id, static_cast<int64_t>(argc),
                                    argv.empty() ? nullptr : argv.data(), &rv);
    if (handled == 1 && rv != nullptr) {
      info.GetReturnValue().Set(FromWire<v8::Value>(rv));
    }
  } catch (...) {
    SetErr("C++ exception in promise native trampoline");
  }
}

void PromiseRejectTrampoline(v8::PromiseRejectMessage message) {
  RejectEntry entry = g_reject_entry;
  if (entry == nullptr) {
    return;
  }
  v8::Isolate* iso = v8::Isolate::TryGetCurrent();
  if (iso == nullptr) {
    return;
  }
  int64_t id = 0;
  {
    std::lock_guard<std::mutex> lock(g_reject_mu);
    auto it = g_reject_ids.find(iso);
    if (it == g_reject_ids.end()) {
      return;
    }
    id = it->second;
  }
  try {
    v8::HandleScope hs(iso);
    void* promise_wire = ToWire(message.GetPromise());
    // GetValue() is empty for HandlerAddedAfterReject; ToWire maps empty
    // locals to null and Go models that as "no value".
    void* value_wire = ToWire(message.GetValue());
    entry(id, static_cast<int64_t>(message.GetEvent()), value_wire,
          promise_wire);
  } catch (...) {
    SetErr("C++ exception in promise reject trampoline");
  }
}

}  // namespace gov8_promise_impl

extern "C" {

// --- callback registry entry points -----------------------------------------

__declspec(dllexport) int64_t gov8_promise_fn_set_entry(uintptr_t entry) {
  ClearErr();
  gov8_promise_impl::g_native_entry =
      reinterpret_cast<gov8_promise_impl::NativeEntry>(entry);
  return kOk;
}

__declspec(dllexport) int64_t gov8_promise_reject_set_entry(uintptr_t entry) {
  ClearErr();
  gov8_promise_impl::g_reject_entry =
      reinterpret_cast<gov8_promise_impl::RejectEntry>(entry);
  return kOk;
}

// --- native functions backed by the Go integer registry ----------------------

// Creates a native function whose callback data is the Go registry id. The
// function and data handles are created in the CALLER's open HandleScope
// (the Go-owned scope), matching the primitive-constructor contract.
__declspec(dllexport) int64_t gov8_promise_fn_new(v8::Isolate* iso,
                                                  void* ctxw, void* scope,
                                                  int64_t id, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  if (id <= 0 || id > 0x7fffffffLL) {
    SetErr("callback id out of range");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Function> fn;
    if (!v8::Function::New(ctx, gov8_promise_impl::PromiseNativeTrampoline,
                           v8::Integer::New(iso, static_cast<int32_t>(id)))
             .ToLocal(&fn)) {
      SetErr("Function::New failed");
      return kErrGeneric;
    }
    *out = ToWire(fn);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in promise_fn_new");
    return kErrCpp;
  }
}

// --- resolver -----------------------------------------------------------------

__declspec(dllexport) int64_t gov8_promise_resolver_new(v8::Isolate* iso,
                                                        void* ctxw,
                                                        void* scope,
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
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Promise::Resolver> resolver;
    if (!v8::Promise::Resolver::New(ctx).ToLocal(&resolver)) {
      SetErr("PromiseResolver::New failed");
      return kErrGeneric;
    }
    *out = ToWire(resolver);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in promise_resolver_new");
    return kErrCpp;
  }
}

// Allocation-free Go hot path: successful local handles are user-space
// addresses (positive intptr_t values); negative values remain the exact shim
// status codes returned by the compatibility export above.
__declspec(dllexport) int64_t gov8_promise_resolver_new_direct(
    v8::Isolate* iso, void* ctxw, void* scope) {
  void* out = nullptr;
  const int64_t status = gov8_promise_resolver_new(iso, ctxw, scope, &out);
  if (status < 0) return status;
  return static_cast<int64_t>(reinterpret_cast<intptr_t>(out));
}

__declspec(dllexport) int64_t gov8_promise_resolver_get_promise(
    v8::Isolate* iso, void* scope, void* resolver, void** out) {
  ClearErr();
  if (out == nullptr || resolver == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    *out = ToWire(FromWire<v8::Promise::Resolver>(resolver)->GetPromise());
    return kOk;
  } catch (...) {
    SetErr("C++ exception in promise_resolver_get_promise");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_promise_resolver_get_promise_direct(
    v8::Isolate* iso, void* scope, void* resolver) {
  void* out = nullptr;
  const int64_t status =
      gov8_promise_resolver_get_promise(iso, scope, resolver, &out);
  if (status < 0) return status;
  return static_cast<int64_t>(reinterpret_cast<intptr_t>(out));
}

// Resolve/reject the associated promise. *out_ok reports the success of the
// CALL (Maybe<bool>); settling an already-settled promise is silently
// ignored by the engine and still reports success, matching the oracle.
__declspec(dllexport) int64_t gov8_promise_resolver_resolve(
    v8::Isolate* iso, void* ctxw, void* scope, void* resolver, void* value,
    int32_t* out_ok) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out_ok == nullptr || resolver == nullptr ||
      value == nullptr || !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Maybe<bool> ok = FromWire<v8::Promise::Resolver>(resolver)->Resolve(
        ctx, FromWire<v8::Value>(value));
    if (ok.IsNothing()) {
      // Resolving ran user code (a thenable) that threw; the exception is
      // observable through a TryCatch by the caller.
      return kErrException;
    }
    *out_ok = ok.FromJust() ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in promise_resolver_resolve");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_promise_resolver_resolve_direct(
    v8::Isolate* iso, void* ctxw, void* scope, void* resolver, void* value) {
  int32_t out_ok = 0;
  const int64_t status = gov8_promise_resolver_resolve(
      iso, ctxw, scope, resolver, value, &out_ok);
  if (status < 0) return status;
  return out_ok;
}

__declspec(dllexport) int64_t gov8_promise_resolver_reject(
    v8::Isolate* iso, void* ctxw, void* scope, void* resolver, void* value,
    int32_t* out_ok) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out_ok == nullptr || resolver == nullptr ||
      value == nullptr || !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Maybe<bool> ok = FromWire<v8::Promise::Resolver>(resolver)->Reject(
        ctx, FromWire<v8::Value>(value));
    if (ok.IsNothing()) {
      return kErrException;
    }
    *out_ok = ok.FromJust() ? 1 : 0;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in promise_resolver_reject");
    return kErrCpp;
  }
}

// --- promise state ------------------------------------------------------------

// Returns 0 = kPending, 1 = kFulfilled, 2 = kRejected, negative on error.
__declspec(dllexport) int64_t gov8_promise_state(v8::Isolate* iso,
                                                 void* promise) {
  ClearErr();
  if (promise == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    switch (FromWire<v8::Promise>(promise)->State()) {
      case v8::Promise::kPending:
        return 0;
      case v8::Promise::kFulfilled:
        return 1;
      case v8::Promise::kRejected:
        return 2;
    }
    SetErr("unknown promise state");
    return kErrState;
  } catch (...) {
    SetErr("C++ exception in promise_state");
    return kErrCpp;
  }
}

// The result handle is created in the CALLER's open HandleScope (Go-owned),
// so no shim-local HandleScope is opened here.
__declspec(dllexport) int64_t gov8_promise_result(v8::Isolate* iso,
                                                  void* scope, void* promise,
                                                  void** out) {
  ClearErr();
  if (out == nullptr || promise == nullptr || !ScopeIs(iso, scope)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    *out = ToWire(FromWire<v8::Promise>(promise)->Result());
    return kOk;
  } catch (...) {
    SetErr("C++ exception in promise_result");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_promise_result_direct(v8::Isolate* iso,
                                                         void* scope,
                                                         void* promise) {
  void* out = nullptr;
  const int64_t status = gov8_promise_result(iso, scope, promise, &out);
  if (status < 0) return status;
  return static_cast<int64_t>(reinterpret_cast<intptr_t>(out));
}

__declspec(dllexport) int64_t gov8_promise_has_handler(v8::Isolate* iso,
                                                       void* promise) {
  ClearErr();
  if (promise == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return FromWire<v8::Promise>(promise)->HasHandler() ? 1 : 0;
  } catch (...) {
    SetErr("C++ exception in promise_has_handler");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_promise_mark_as_handled(v8::Isolate* iso,
                                                           void* promise) {
  ClearErr();
  if (promise == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    FromWire<v8::Promise>(promise)->MarkAsHandled();
    return kOk;
  } catch (...) {
    SetErr("C++ exception in promise_mark_as_handled");
    return kErrCpp;
  }
}

// --- then / then2 / catch -------------------------------------------------------

__declspec(dllexport) int64_t gov8_promise_then(v8::Isolate* iso, void* ctxw,
                                                void* scope, void* promise,
                                                void* handler, void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || promise == nullptr ||
      handler == nullptr || !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> hv = FromWire<v8::Value>(handler);
    if (!hv->IsFunction()) {
      SetErr("promise handler is not a function");
      return kErrBadArg;
    }
    v8::Local<v8::Promise> derived;
    if (!FromWire<v8::Promise>(promise)
             ->Then(ctx, hv.As<v8::Function>())
             .ToLocal(&derived)) {
      SetErr("Promise::Then failed");
      return kErrException;
    }
    *out = ToWire(derived);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in promise_then");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_promise_then_direct(
    v8::Isolate* iso, void* ctxw, void* scope, void* promise,
    void* handler) {
  void* out = nullptr;
  const int64_t status =
      gov8_promise_then(iso, ctxw, scope, promise, handler, &out);
  if (status < 0) return status;
  return static_cast<int64_t>(reinterpret_cast<intptr_t>(out));
}

__declspec(dllexport) int64_t gov8_promise_then2(v8::Isolate* iso, void* ctxw,
                                                 void* scope, void* promise,
                                                 void* on_fulfilled,
                                                 void* on_rejected,
                                                 void** out) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || promise == nullptr ||
      on_fulfilled == nullptr || on_rejected == nullptr ||
      !ScopeIs(iso, scope) || !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> fv = FromWire<v8::Value>(on_fulfilled);
    v8::Local<v8::Value> rv = FromWire<v8::Value>(on_rejected);
    if (!fv->IsFunction() || !rv->IsFunction()) {
      SetErr("promise handler is not a function");
      return kErrBadArg;
    }
    v8::Local<v8::Promise> derived;
    if (!FromWire<v8::Promise>(promise)
             ->Then(ctx, fv.As<v8::Function>(), rv.As<v8::Function>())
             .ToLocal(&derived)) {
      SetErr("Promise::Then2 failed");
      return kErrException;
    }
    *out = ToWire(derived);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in promise_then2");
    return kErrCpp;
  }
}

// catch maps to Option in the oracle (MaybeLocal -> is_some), so an empty
// result is reported through *out_ok instead of an error status.
__declspec(dllexport) int64_t gov8_promise_catch(v8::Isolate* iso, void* ctxw,
                                                 void* scope, void* promise,
                                                 void* handler, void** out,
                                                 int32_t* out_ok) {
  ClearErr();
  CtxWrap* w = AsCtx(ctxw);
  if (w == nullptr || out == nullptr || out_ok == nullptr ||
      promise == nullptr || handler == nullptr || !ScopeIs(iso, scope) ||
      !OwnedBy(iso, w->iso)) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    v8::Local<v8::Context> ctx = w->ctx->Get(iso);
    v8::Context::Scope ctx_scope(ctx);
    v8::Local<v8::Value> hv = FromWire<v8::Value>(handler);
    if (!hv->IsFunction()) {
      SetErr("promise handler is not a function");
      return kErrBadArg;
    }
    v8::Local<v8::Promise> derived;
    if (!FromWire<v8::Promise>(promise)
             ->Catch(ctx, hv.As<v8::Function>())
             .ToLocal(&derived)) {
      *out = nullptr;
      *out_ok = 0;
      return kOk;
    }
    *out = ToWire(derived);
    *out_ok = 1;
    return kOk;
  } catch (...) {
    SetErr("C++ exception in promise_catch");
    return kErrCpp;
  }
}

// Strict equality of two values in the same isolate (used to prove derived
// promises are distinct objects). Scoped here because the promise checks are
// its first consumers; a general Value API will absorb it later.
__declspec(dllexport) int64_t gov8_promise_strict_equals(v8::Isolate* iso,
                                                         void* a, void* b) {
  ClearErr();
  if (a == nullptr || b == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    Gov8IsolateScope iso_scope(iso);
    return FromWire<v8::Value>(a)->StrictEquals(FromWire<v8::Value>(b)) ? 1
                                                                        : 0;
  } catch (...) {
    SetErr("C++ exception in promise_strict_equals");
    return kErrCpp;
  }
}

// --- promise-reject callback -----------------------------------------------------

// Installs the reject trampoline for the isolate and binds it to the Go
// registry id. Installing again replaces the previous binding (V8 keeps one
// reject callback per isolate); the Go layer drops the superseded entry.
__declspec(dllexport) int64_t gov8_isolate_set_promise_reject_callback(
    v8::Isolate* iso, int64_t id) {
  ClearErr();
  if (iso == nullptr || id <= 0) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    {
      std::lock_guard<std::mutex> lock(gov8_promise_impl::g_reject_mu);
      gov8_promise_impl::g_reject_ids[iso] = id;
    }
    iso->SetPromiseRejectCallback(gov8_promise_impl::PromiseRejectTrampoline);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in isolate_set_promise_reject_callback");
    return kErrCpp;
  }
}

__declspec(dllexport) int64_t gov8_isolate_clear_promise_reject_callback(
    v8::Isolate* iso) {
  ClearErr();
  if (iso == nullptr) {
    SetErr("invalid argument");
    return kErrBadArg;
  }
  try {
    iso->SetPromiseRejectCallback(nullptr);
    std::lock_guard<std::mutex> lock(gov8_promise_impl::g_reject_mu);
    gov8_promise_impl::g_reject_ids.erase(iso);
    return kOk;
  } catch (...) {
    SetErr("C++ exception in isolate_clear_promise_reject_callback");
    return kErrCpp;
  }
}

}  // extern "C"
