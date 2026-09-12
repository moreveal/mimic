# Native function compatibility, 2026-09-12

`Function.prototype.toString` was a JS Proxy around the original intrinsic.
Setting its prototype to itself changed the hidden target's prototype and allowed
a cycle. Chrome's ordinary native callable instead rejects that operation with
TypeError (or false via Reflect.setPrototypeOf). The Proxy apply frame also
contaminated invalid-receiver error stacks.

V8 now installs a real, nonconstructible, length-zero native callback after ordinary
bootstrap or snapshot restoration. It consults the realm's private source registry
for explicitly registered browser functions. Unmarked author functions/proxies and
invalid receivers go to the original V8 intrinsic. A native TryCatch/ReThrow
preserves the real pending JS exception: gov8 can report ok=false with no Go error.
Internal binding failures are propagated separately, rather than returning
undefined. No stack text is rewritten or filtered.

The registry contains binding names or immutable intrinsic source imported with a
foreign callable. It uses null-prototype records and captured WeakMap operations;
source observation never reads public name/source/toString properties or triggers
author proxy traps. The existing frame encoder transports author-function source
from its owning realm, avoiding the imported bridge Proxy's anonymous source.
The callback itself is deliberately not serialized into the bootstrap snapshot:
only its pure-JS registry/resolver and original intrinsic are restored, then native
state is installed for the new realm.

Platform operation normalization now also covers async implementations, so fetch
has Function.prototype rather than AsyncFunction.prototype while retaining its
promise behavior. Generic global-accessor registration supplies native getter
source/name metadata (including crypto, isSecureContext, speechSynthesis and
representative neighboring getters). WorkerGlobalScope/DedicatedWorkerGlobalScope
operations use the same regular function shape and private registration. Console
implementation and performance/Trusted Types behavior are untouched.

## Frozen validation

30 top-level observations against frozen headful Chrome **152.0.7977.82 / V8
15.2.124.21**: **0 A/B differences, 0 Chrome/Mimic differences**. The fixture checks
name/length/descriptors/prototype/constructability, call/apply/bind, invalid receiver
error name/message/first native frame, cycle rejection, native name mutation,
prototype pollution, author/proxy/revoked-proxy source, foreign native and author
callables, plus worker fetch/toString/receiver behavior. The capture records the
actual hidden startup window and measured geometry; no presentation-dependent
behavior is used as an expectation.

`TestNativeFunctionSourcesMatchFrozenChrome` repeats the retained fixture in
ordinary and restored-bootstrap V8 realms. Existing function-source tests across
V8/Goja/QuickJS, callable metadata, execution source, frame reflection/constructors/
encoder, worker tests, and snapshot callback/concurrent teardown/exception-stack
and page-lifecycle tests pass. Full internal/engine/v8 and internal/webapi packages
also pass. The JSON receipt retains probe and binary hashes. Full raw observations,
runner, probe and measured binary are retained outside the disposable worktree at
`E:/GitHub/mimic/.build/native-function-compat-delegated`.

This is a bounded native-function fix, not a rewrite of every handwritten API.
Goja/QuickJS retain a concise-method fallback and their own intrinsic limitations;
the complete frozen stack/Proxy result is claimed only for V8. Binding-name/source
registration does not establish full WebIDL receiver semantics for every getter,
or native stack equivalence for failures inside every JS-backed API. Performance,
CSP/Trusted Types, console and unrelated VM payload groups are separate work.
