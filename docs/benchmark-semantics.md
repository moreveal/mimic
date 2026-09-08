# Semantic defects found by the controlled benchmark

The 2026-09-08 correctness gate against pinned Chrome 152 found four failing
workloads before performance comparisons. The user authorized runtime fixes in
a separate commit; these are correctness changes, not benchmark optimizations.

- DocumentFragment append/insert now moves its children in order and empties the
  fragment. Synthetic parent links are detached and insertion cycles rejected.
- Node.ownerDocument returns the realm document (and null for Document), allowing
  ordinary client application DOM creation. Cross-document adoption remains outside
  this fix's scope.
- Element/fragment textContent excludes descendant comments while a Comment's own
  textContent still returns its data.
- Same-window postMessage uses the existing frame message scheduler and clone path,
  including the target-origin/options overload, instead of a missing semantic stub.
- V8 foreground tasks are pumped at explicit checkpoints. While native background
  work is pending, a bounded-delay control task requests another checkpoint. This
  lets asynchronous WebAssembly compilation settle without an unrelated timer or
  repeated evaluation; no background goroutine enters JavaScript.

Focused regressions cover node identity/order/ownership, comments, cycle rejection,
self-message delivery, and an awaited native WebAssembly Promise with no timers.
The full ordinary Go suite and focused race tests pass. The full race suite failed
inside Windows ThreadSanitizer with an address-allocation error (487), rather than
a race report; it is not recorded as passing.

The local React 18.3.1 fixture and the deterministic DOM, async, CPU, static and
WebAssembly workloads all pass the corrected gate. This does not imply complete
DOM, React, Worker, WebAssembly or cross-origin compatibility. Virtual performance
clocks, CDP serialization, object retention, allocation policy and production
optimization settings were not changed.
