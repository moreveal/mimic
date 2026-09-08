# One focused wrapper-path diagnostic — 2026-09-09

Source: production implementation at 7886b13 (code remains 52a2aa2), with the attached diagnostic-only patch. Exactly one process launch and one frozen DOM execution; no warm-up replay, second profile, benchmark matrix or optimization.

The result is correct: count=3000, active=3000, last=node-2999, children=3, round=3, textLength=25897. Instrumented execution wall time is 111.345 ms; it is not comparable to an uninstrumented frozen median.

## Top 10 wrapper operations by sampled self CPU

Calls are exact local counters. CPU is V8 sampled self time. V8 allocation bytes are estimates at a 1024-byte sampling interval, including allocations collected by minor/major GC; attribution uses the closest named wrapper ancestor and includes unnamed children. They are not exact object counts. Direct host crossings exclude indirect host-backed getters invoked through a Proxy.

| Operation | Calls | Self CPU ms | Inclusive CPU ms | Estimated V8 allocation KiB | Direct V8→Go host calls |
|---|---:|---:|---:|---:|---:|
| wrap | 24,008 | 9.672 | 16.981 | 3555.1 | 0 |
| observe | 9,004 | 4.754 | 5.742 | 1388.7 | 0 |
| freshCharacterData | 6,000 | 2.601 | 14.184 | 1294.1 | 0 |
| elementSlot | 96,028 | 2.593 | 2.593 | 0.0 | 0 |
| wrapperProxyGet | 63,026 | 2.082 | 7.448 | 1201.4 | 0 |
| freshNodeData | 9,001 | 1.552 | 1.552 | 241.9 | 0 |
| recordAPIAccess | 66,028 | 1.018 | 1.018 | 4.3 | 28 |
| observationHandler | 9,004 | 0.988 | 0.988 | 493.3 | 0 |
| qualified | 66,028 | 0.532 | 0.532 | 4.2 | 0 |
| wrapperProxySet | 3,002 | 0.093 | 2.202 | 602.1 | 0 |

Inclusive CPU rows overlap and must not be summed. Zero sampled bytes means no attributed sample, not proof of no allocation. `nodeList` was called seven times (no CPU sample; approximately 2.1 KiB attributed); `htmlCollection` was not called.

The wrapper group accounts for approximately 8.581 MiB of 19.160 MiB sampled V8 allocation. `observe` executed 9,004 Proxy-construction paths. `freshNodeData` executed 9,001 record-construction paths; this is a call count, not an exact physical allocation census.

## Go allocation and boundary scope

The navigation-to-execute diagnostic phase delta is 31,819,384 Go allocated bytes (30.345 MiB), 606,674 allocations, and 3 Go GC cycles. This includes diagnostic bookkeeping/profile serialization. Per-JS-operation Go bytes and allocations are unavailable; attributing this delta to each wrapper would be false precision.

The saved Go alloc-space profile covers the whole diagnostic process, including bootstrap: 77.15 MiB total. Examples are packed callback construction/dispatch (5.00 MiB flat, 14.65 MiB cumulative) and gov8 hostCallbackDispatch (4.50 MiB flat, 19.65 MiB cumulative). These are not execution-only or per-wrapper measurements, and cumulative values overlap.

There are exactly 54,055 V8→Go host calls in the workload. `wrap` makes zero direct nodeData calls; recordAPIAccess makes 28 direct apiAccess calls despite 66,028 JS invocations. The other listed wrapper helpers make no direct host calls, but Proxy getters/setters can reach hosts indirectly. A complete per-wrapper inclusive crossing census and all low-level Go→V8 SDK transitions were not measured. No second run was made to fill those gaps.

The native sample also contains 56.337 ms of unnamed/native frames and 9.092 ms of GC; 18.115 ms of idle samples are excluded from wrapper CPU interpretation. Counters and heap sampling perturb execution, and one execution is insufficient for stable timing/ranking conclusions.

## Artifacts and boundaries

The frozen harness and workload fingerprint matches the original baseline. Build and pre-launch SHA-256 receipts, raw CPU/allocation/host profiles, exact counters, diagnostic patch, validation output and Go allocation profiles are preserved here. The diagnostic patch is not active production code. No optimization is proposed or implemented in this report.
