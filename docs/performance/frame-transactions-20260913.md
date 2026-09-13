# Cross-realm bridge transactions — 2026-09-13

The previous bridge repeatedly entered the owning runtime to materialize
arguments, read reflection result fields, classify values and marshal those
records back through Go. CPU sampling of a real account page identified this
path as substantial work inside long synchronous callbacks. A loopback
fixture reproduced the overhead independently of that application.

## Design

Imported object/function proxies now use one owner transaction for get, set,
call, construction, has, prototype, descriptor, ownKeys and property mutation.
The private JavaScript operation materializes arguments, uses the existing
captured reflection intrinsics and canonical value encoder, then returns an
encoded result. Go forwards that result without re-exporting intermediate
JavaScript records. Reference-bearing outcomes still update the existing
realm-retention graph, including third-realm and Window references.

V8 implements the optional engine-neutral `StringCallRuntime` boundary. It
marshals primitive/native-value arguments and reads the serialized result in
one native handle scope, without creating persistent scratch roots. Other
engines use ordinary Call and the same transaction semantics. The source
callback borrows its arguments for the synchronous call; it does not retain
those borrowed handles across the transaction.

Captured JSON functions and null-prototype transport records prevent author
`toJSON` hooks and replaced JSON methods from observing or corrupting private
metadata. Symbols/references keep their canonical decoders; NaN, infinities,
negative zero, undefined and BigInt retain explicit wire representations.
No author objects are copied or serialized by value. Property values, getters,
Proxy results and access decisions are never cached.

The existing Page event loop, origin checks, document-entry tracking, nested
actor calls and checkpoints remain authoritative. This is deliberately a
bridge correction, not a change to isolate sharing, snapshot restoration or
the concurrency model. That keeps the optimization within the measured hot
path while preserving the existing navigation/lifetime and realm boundaries.

## Validation and measurements

New regressions cover nested callbacks, receiver/return/exception identity,
special numbers, BigInt, symbols, live getters, reflection/mutation and a
poisoned public JSON environment. Engine regressions check Unicode including
embedded NUL, borrowed callback arguments, pending microtasks, interruption
and recovery, non-string results, and unchanged persistent-root counts over
repeated private string calls.

The reusable local probe is `tools/performance/frame_bridge_probe.py`. It
checks every arithmetic result and records executable and probe hashes.
Full validation receipts and final measurements are recorded in
`docs/performance/report.md`: the full Go suite and unchanged fast gate passed.
The local five-sample medians improved across all seven foreign operations,
including calls from 117.3 to 48.3 ms and descriptors from 102.2 to 49.3 ms per
1000 operations. Independent controls and limitations remain in that report.

## Limits

Distinct V8 actors and native call crossings still have a cost. The change
does not claim Chrome-level latency, a new garbage-collection model, or
completion of the bridge's existing unsupported exotic-object operations.
Event-listener dispatch and Window-specific operations retain their existing
specialized semantics. There are no domain checks, application identifiers,
altered clocks, fabricated observations or site payload modifications.

The earlier intermittent Google admission result is a separate observation.
Reducing runtime overhead does not establish or control Google's decision.
The final live check had no JS exceptions/network failures and reached
account-not-found, but two approximately five-second callbacks remained.
Child-realm diagnostics found 155,100 nested bridge transactions; the residual
actor/native-call cost and receipts are recorded in the performance report.
