# Bootstrap compilation and exposure transport checkpoint

Production baseline is bda67db. cee1376 and 7dd923b add snapshot documentation
and a regression test; the pending production batch changes Goja bootstrap
compilation and the private exposure transport. Semantic Node/DOMException
commits are validated in a separate worktree, not included in these numbers.

Goja caches at most eight immutable runtime-independent Programs, keyed by
source hash and name. No runtime, Page, host or JavaScript object is cached.
Compilation and execution occur outside the cache mutex. Ordinary Eval does
not use this cache. The limit bounds entry count, not bytes; compilation and
waiting for the first compilation are not interruptible. Execution retains the
existing context interrupt/join/clear behavior.

The exposure transport encodes repetitive field names as tuples and flag bits.
The public frozen SurfaceProperty schema is unchanged. The production decoder
retains the legacy object input, null/empty distinctions, writable tri-state,
Unicode and own __proto__ keys. Selected metadata shrinks from 1,815,439 to
472,688 bytes; the separate 623,151-byte catalog is unchanged.

## Completed validation

Full ordinary browser suite passes (419.428 s), as do affected WebAPI checks.
Fixed 236 probes retain Goja 154 matches and V8 205; the 28 original
representatives retain Goja 17 and V8 19. Every normalized leaf was compared:
zero changed observations, zero harness errors and zero Chrome drift. Discovery
was not rerun or used to inflate the fixed set. Chrome 152.0.7977.82 is headful
with the reused controlled profile explicitly marked unverified-reused-controlled.

The full race run completed, but FAILED the browser package at 2004.465 s:
six Goja deadline failures (partition cookies, synchronous frame entry,
document-scoped navigation timing, child load observer, fragment insertion,
resource timing). No WARNING: DATA RACE was reported. Goja, V8, WebAPI, CDP,
network and scheduler race packages pass. Opt-in snapshot/profiling diagnostics
and Goja native-microtask cases have explicit skips; they are not covered by
this run. An earlier cache-only full race reached the default ten-minute suite
timeout; it is not a passing result or evidence of a deadlock. The separate
deferred-child experiment is not included here and must not erase these failures.

## Performance and retained state

Fresh five-Page Goja cold/warm waves improve 527.12/443.03 to 485.05/333.42 ms.
After ordinary teardown plus 250 ms, warm private memory is 551.71 to 483.23 MiB.
After a separately labelled diagnostic Go GC, live Go heap is 13.72 to 21.57 MiB:
the code cache has a retained-memory cost. Neither forced GC nor retries are part
of the production path. These are single paired diagnostics, not confidence bounds.

Both full unchanged V8 fast gates pass. Median static/React/DOM completion time
is 29.32/81.96/485.39 to 31.99/84.00/501.86 ms. Median throughput at 10/25 Pages
is 69.42/71.48 to 72.38/71.58 Pages/s. Static/React private memory after recovery
is 159.73/170.54 to 150.38/162.50 MiB. Results are mixed; no blanket latency or
performance-neutrality claim is made. V8 does not use the Goja Program cache.
No native crash was reported in these gates; the separate historical snapshot
P0 remains open despite these successful runs.

Compact receipts are in [bootstrap-transport-20260911](bootstrap-transport-20260911/).
Ignored raw logs are in bootstrap-transport-fixed-final-20260911,
bootstrap-transport-memory-final-20260911, cleanup-validation-20260911,
exposure-transport-prototype-20260911 and the paired bootstrap-transport-gate
directories under compatibility/private-captures. No target site was run.
