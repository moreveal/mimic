//go:build (windows || linux) && amd64

// Package gov8 provides Go bindings for Google's V8 JavaScript and WebAssembly
// engine.
//
// Applications can embed JavaScript in Go using typed APIs for isolates,
// contexts, handle scopes, scripts, callbacks, promises, modules, WebAssembly,
// snapshots, and the V8 Inspector. Packaged Windows and Linux amd64 engines
// require no compiler or Rust installation for application developers.
//
// Both builds wrap rusty_v8 152.2.0 (V8 15.2.124.1-rusty) with the same C ABI
// shim. Windows uses native system-call trampolines; Linux uses the System V
// ABI through purego and a pointer-word bridge for unusually wide exports.
//
// # Supported platforms
//
// Windows amd64 and Linux amd64 (glibc 2.39+). The native library is verified
// and extracted to a content-addressed per-user cache. GOV8_SHIM_LIBRARY can
// select a trusted ABI-compatible developer build; GOV8_SHIM_DLL remains a
// legacy alias. The cache must permit executable mappings.
//
// # Ownership and lifetime rules
//
//   - Process: Initialize/Dispose/DisposePlatform follow the pinned crate's
//     strict one-shot state machine; violations return errors (the crate
//     panics — see the parity notes on gov8.Initialize). Dispose is
//     additionally refused while any isolate is still live, and isolate
//     creation is synchronized against teardown, so an isolate can never be
//     created across or after Dispose.
//   - Isolate: V8 isolates are thread-affine. Creating an Isolate locks the
//     creating goroutine to its OS thread for the isolate's lifetime; every
//     operation validates the owning thread ID and every Close must happen on
//     that thread. There is no hidden cross-thread marshaling. The thread-id
//     check is a misuse guard, not a capability: only the goroutine that
//     created the isolate may call its APIs, even when another goroutine
//     happens to be scheduled on the same OS thread.
//   - Ownership: every wrapper validates that the resources it passes to the
//     engine (scopes, contexts, TryCatches, queues, values) belong to the
//     same isolate before crossing the ABI; cross-isolate misuse returns an
//     error instead of reaching V8.
//   - Scope: v8 local handles live in a HandleScope owned by gov8.Scope. All
//     Values are valid only while their Scope is open; using one after
//     Scope.Close returns an error instead of touching the engine.
//   - Context/Script/MicrotaskQueue/TryCatch are engine-persistent objects
//     with explicit Close; no finalizers are used anywhere — Close is the
//     only correctness mechanism and leaks are visible in tests.
//   - No Go pointers cross the C boundary; string/byte traffic uses pinned
//     buffers copied during the call, and no C++ exception is ever allowed to
//     unwind across the ABI (the shim converts them to status codes).
package gov8
