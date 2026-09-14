# Measured optimization campaign, 2026-09-14

Exploration is closed. The selected production package reduces DOM copying,
transient V8 roots, Fetch byte conversion, retained response copies and bootstrap
byte duplication. It keeps one authoritative Go DOM, independent Page isolates,
existing observation/tracing, and the current V8 ABI and 4 MiB nursery limit.
Rejected implementation prototypes are removed after retaining their evidence.
No further architecture candidate is required for this checkpoint.

The decision platform is Linux with all engines, server and controller together;
Windows is the frozen compatibility/performance checkpoint. Original workloads,
expectations and `benchmark/results` remain unchanged. Control is
`f07bd2d643eebbadc28d582ef93f01c832a50106`. An integrated 2x target gain is required
to expand a radical rewrite, and to accept up to 25% greater Page memory. A
confirmed control/throughput regression above 5% blocks promotion. Removing
lifetime accumulation is independently useful even without lower total latency.

## Selected implementation

| Area | Implemented behavior | Measured reason |
|---|---|---|
| DOM copying and crossings | Canonical script identity without copying child lists; cached/rebound document ID; create in the active document without redundant adoption; synchronous borrowed inputs; combined ordinary insertion validation/mutation | Baseline 96,088 host crossings and 60,030 additional roots; intermediate 57,080 crossings and about 26 MiB Go allocation versus 121 MiB. Final counters are recorded separately below. |
| Fragment insertion | Validate before mutation, splice eligible plain fragments once; keep callback-bearing resource/frame paths and observer ordering | At 6,000 nodes, full append slice 423.84 -> 139.23 ms, insertion-before slice 435.68 -> 140.75 ms. Transfer alone 113.97 -> 1.88 ms; this 60x phase gain is not a 60x whole-page claim. |
| Value lifetime | Borrow synchronous arguments; retain callbacks only for their owner; release exported Eval/Promise temporaries on the existing owner operation; release completed/canceled Window/Worker timers and structured-clone handoffs | v2 has zero temporary-root growth in repeated Eval, Promise, Fetch, listener and timer loops before Page.Close. 256 Eval operations 61.97 -> 51.44 ms; 1,024 completed timers 248.02 -> 204.52 ms. First extra-dispatch release design was rejected for 15-27% slowdown. |
| Fetch bytes | Explicit binary buffer projection through managed ArrayBuffer storage on all engine adapters; independent JS-owned bytes | 1 MiB response 246.43 -> 5.13 ms; 4 MiB 1003.77 -> 13.49 ms. 4 MiB Go allocation 320.90 -> 14.63 MB. The 1 KiB case is unchanged. |
| Body ownership | Immutable shared cache/history backing, independent mutable public Response.Body, direct CDP string/base64 projection, release history on Page close and all owners on Context close | Isolated no-store Fetch+CDP 17.363 -> 16.052 ms; CDP allocations fall by 8 MiB for a 4 MiB response. Repeated cache case retained Go heap 78.30 -> 32.31 MiB. |
| Bootstrap bytes | Independent native snapshot consumers share immutable Go bytes; independent heaps and disposal | Ten Pages: Go live heap 107.90 -> 20.71 MiB, private resident memory 486.58 -> 401.29 MiB; restored Page setup/use 28.05 -> 23.29 ms. |

These rows come from different bounded paired experiments. Their times and gains
must not be added together or substituted for the final integrated benchmark.
The larger fragment gain includes a different size/operation than the frozen
3,000-element DOM workload.

DOM insertion preserves Trusted Types, CSP, script/resource scheduling, canonical
identity and inert-document ownership. Chrome 152 oracles include invalid-reference
atomicity, cycles, self-insertion, fragments and MutationObserver records. Observer
wrappers force validation before any callback-visible preparation. A strict repeated
markup/mutation test detects zero invocation-root growth after warmup.

Final ownership review found and corrected an invalid cached-module error release:
an incoming graph error is borrowed, while a local EvalModule error is owned by the
completion. The repeated/concurrent invalid-import regression reproduces the old
timeout and checks shared SyntaxError identity, one fetch and stable roots. The
existing ordering between concurrent module-wait goroutines remains a documented
compatibility limitation; no new scheduler ordering is claimed.

Details and retained measurements:
[handles](handle-ownership-20260914.md),
[Worker lifetimes](worker-handle-ownership-20260914.md),
[module correction](module-error-ownership-20260914.md),
[binary Fetch](fetch-transfer-20260914.md),
[shared bodies](body-sharing-20260914.md),
[snapshot ownership](snapshot-byte-ownership-20260914.md).
The [DOM and ownership evidence manifest](data/optimization-20260914/manifest.json)
identifies exact original and compressed hashes. Diagnostic timing, allocation
profiling and explicit collection interventions are labeled separately from the
production latency runs.

## Rejected experiments and remaining boundaries

| Direction | Decision at this checkpoint | Evidence / cost of expansion |
|---|---|---|
| Hybrid C++ Fast API -> Go primitive dispatcher | Stop and delete prototype | Real native path handled 28% of token toggles, but integrated DOM 108.99 -> 109.95 ms, whole operation 146.39 -> 146.56 ms. 96/96 runs valid. Ten Go files plus native thunk and platform build support buy no measured target gain. [Report](hybrid-packed-dispatch-20260914.md). |
| Full prebuilt bootstrap | Stop this candidate | Process through first useful result 395.68 -> 307.21 ms (1.29x), below 2x, and a later race variant failed restoration. Moving verification outside the measured startup was rejected. Only immutable-byte sharing is selected. Worker prebuilt bootstrap is unmeasured. |
| CSS source epoch | Stop this candidate | Removing repeated stylesheet discovery does not remove cascade/geometry recomputation; whole-operation results vary from small gains to >5% slowdown. Existing computed-var mismatch remains visible in the oracle. Dependency-specific invalidation, shaping and hit testing have not been independently integrated. [Report](css-source-epoch-20260914.md). |
| Nursery 2/8/16 MiB | Keep 4 MiB | 8 MiB CPU 1.12x faster but DOM essentially unchanged, marginal USS +2.7% and throughput -4.8%; 16 MiB worsens DOM throughput 10.5%; 2 MiB worsens CPU latency 19.4%. No adaptive heuristic introduced. [Report](nursery-20260914.md). |
| Synchronous file spill | Stop and delete prototype | Bounded memory worked, but unique no-store Fetch slowed 22.6%. Only the memory-only shared body store is selected. [Rejected variant](body-retention-20260914.md). |
| JS/C++/Rust authoritative DOM migration | Do not expand in this checkpoint | Historical JS dual-state synchronization yielded only 1.32x DOM and +28.9% React slowdown. The kernel excludes browser bindings/observers/resources/tracing/CDP and cannot authorize migration. Neither an integrated native DOM nor equal-ABI Rust DOM is measured. The rejected hybrid dispatcher is not a native DOM core. |
| Shared contexts in one Page isolate | Keep existing Page/realm ownership | Historical 153.126 -> 0.147 ms cross-context kernel excludes WindowProxy, origin/navigation changes, full DOM, bootstrap and retained-reference teardown. It permits a future bounded browser experiment only. [Boundary](frame-bridge-ownership-20260910.md). |
| V8 native rebuild, pointer compression, PGO | No build change | Frozen CPU kernel uses zero host calls; plain V8 24.52 ms and Page 25.03 ms versus Chrome 17.47 ms. Page overhead is only about 2%; remaining engine/build/GC differences are not individually isolated. ABI settings must change together with native V8. [Report](v8-environment-20260914.md). |
| Wrappers, event loop and CDP | Keep observation and task semantics | Previously disabling observation broke compatibility; current dispatcher experiment supplies no integrated gain. Existing owner batching and selected direct body projection remove measured overhead. A complete queue/resource/wakeup/serialization cost partition has not been measured here. |

Fetch still waits for body EOF. There is no headers-first browser Fetch, streaming
navigation, new byte budget or asynchronous disk spill in production. The old HTTP
cache policy can retain unbounded bytes; response history remains bounded by count
(128), not bytes. The transport-only pull/cancellation proof does not establish the
Window/Worker/CORS/cache/CDP lifecycle required for real streaming. The mixed
shared-body trial's +11.8% no-store Fetch result is retained; its isolated repeat
showed -3.3% Fetch and -7.5% Fetch+CDP, with unchanged Fetch allocation. Prior-mode
cleanup and allocator history are plausible contributors, not a proven unique cause.

One native root is not one distinct JavaScript object. Eliminating invocation
roots does not prove that all listener, debugger, cross-realm or canonical DOM
owners are collectible at every point. The long-session checks distinguish the
fixed reusable DOM from retained canonical nodes and intentionally retained request
history. Independent Pages remain concurrent; there is no global runtime lock.

## Final integration evidence

The final fresh Windows build passed all 12 correctness gates and all 360 measured
single-page attempts against Chrome 152. The [published checkpoint](../../benchmark/runs/09-optimized-20260914/public-summary.md)
records 229.57 versus 236.68 ms CDP readiness and 28.84 versus 376.26 MiB ready RSS.
At 50 static Pages Mimic used 1,568.62 versus 4,079.95 MiB process-tree RSS and
delivered 69.64 versus 21.00 sessions/s. The Windows safety guard stopped Mimic's
100-Page measured waves after Chrome left less than 2 GiB or 15% available memory;
the stopped warmup observations remain in the raw checkpoint and are not headlines.

Warm single-Page execution medians were 3.61/4.79 ms (Mimic/Chrome) for static,
39.63/27.80 CPU, 96.58/29.63 DOM, 67.90/28.92 async, 42.14/23.43 React and
4.09/4.91 WebAssembly. Against the matched pre-change Linux control, the selected
package reduced the frozen DOM execution median from 221.59 to 102.20 ms (2.17x)
and completion from 252.00 to 131.37 ms (1.92x). React execution improved 1.20x.

The separate [Linux 100-Page run](../../benchmark/runs/09-mimic-chrome-linux-100-pages-20260914/raw.json)
completed three measured waves per workload and both runtimes. Mimic throughput was
5.33x static, 4.49x CPU and 6.88x React, but marginal USS per Page was higher:
34.90 versus 13.40 MiB static, 45.08 versus 25.72 MiB CPU, and 45.68 versus
18.64 MiB React. Active USS at 100 Pages was likewise higher for Mimic in this Linux
configuration (3.43/4.42/4.48 GiB versus Chrome 1.40/2.55/1.86 GiB). This is the
remaining density bottleneck and prevents a general lower-memory-per-Page claim.

The long-running Page test keeps the native root count at 1,342 through 2,048
iterations; explicitly collected V8 used memory stays within 0.13 MiB, the bounded
body store stays at 128 entries/8 MiB and reaches zero after Page close. Process
private memory returns to 112.62 MiB after caller-visible trace data and the Context
are released. The full Windows suite, focused ownership/race suites and benchmark
harness tests pass; the CDP race suite passed after enforcing Page command ownership.

Implementation effort is reported as the concrete scope and maintenance cost of
each variant; human engineering hours were not tracked. Profiling counters and
nested host/cgocall times overlap and are never summed as independent CPU costs.
