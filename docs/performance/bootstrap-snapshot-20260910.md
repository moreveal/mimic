# Integrated bootstrap snapshots — 2026-09-10

## Scope and baseline

The implementation starts from restored commit `2a3f3626286f915d58dab7ce1946bba0d1ed59cb`. Each window realm still has its own V8 isolate and owner thread. Workers retain their existing path. The earlier approximately 54.9 ms result came from subsequently rolled-back shared-parent ownership experiments; it is not this baseline. Neither this change nor the memory probe removes ordinary cross-frame bridge round trips.

The snapshot contains the complete initialized window JavaScript surface. It does not use the deliberately incomplete, no-API iframe lower-bound surface. Frozen workloads and original baseline data are unchanged.

## Ownership and restoration

A browser Context owns a bounded snapshot cache: at most four entries and 32 MiB of accounted seed/snapshot storage. The key includes source, generated exposure/catalog, effective feature flags, graphics profile and security state. A second use admits background snapshot construction; a single first-use realm follows ordinary initialization. Cache closure is part of Context.Close.

Snapshot creation replays captured bootstrap host responses through a pure-JavaScript host facade. Native callbacks are installed anew in each consumer. The facade switches to the new host before the restoration hook runs, including for generated bindings which captured the facade during initialization. Replay validates call names, arguments and complete consumption. Construction uses four independently compiled stages: reply data, replay facade, actual surface, then finalization. Separating the reply-data source from surviving functions prevents each restored isolate retaining a large enclosing script and literal boilerplates.

V8 deliberately omits conditional intrinsics while constructing a snapshot and
installs them when restoring a Context. Publishing the complete global surface
inside the seed originally shadowed WebAssembly with a placeholder, exposed
SharedArrayBuffer in non-isolated realms and changed global property order.
The implementation therefore records the ordinary engine/global key order,
retains the initialized API graph privately in the seed, and publishes its global
descriptors only after V8 installs native intrinsics in the restored Context.
The final exposure and nonconfigurable flags then match ordinary bootstrap.
This uses the observed engine inventory, not a hardcoded list of native globals,
another realm's constructors, a replacement global proxy or an SDK fork.

Restoration updates token, Intl defaults, permissions-policy slots, security values, topology cells and both cached DOM root IDs. It clears seed-specific wrapper/reference caches and trace deduplication, re-registers the frame-reference, DOM-query, form-snapshot and shadow-snapshot callbacks, and signals readiness. Canonical Document, prototypes and initialization-time listeners survive. Browser installation hides the private restoration hook. The seed is captured before page script; this is not a mechanism for transplanting an already-used realm's DOM or user state.

Frozen Chrome 152.0.7977.83 confirmed that `top` is a nonconfigurable getter without a setter, while `parent` has a configurable accessor whose setter replaces it with a writable data property. Restore uses private topology cells so it does not redefine the unforgeable `top` property. Focused tests cover descriptor names/lengths, native function strings, assignments and deletion as well as restoration cells.

Each restored isolate owns a consumer snapshot copy until disposal. Page.Close releases these consumer copies together with its runtimes. The reusable Context cache intentionally remains after Page.Close and is released by Context.Close.

Snapshots are enabled by default for eligible V8 window realms. Set
`MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT=1` for ordinary-bootstrap A/B measurements.
Full source diagnostics (`MIMIC_DIAGNOSTICS=1`, without host-only profiling)
bypass snapshots so bootstrap execution remains observable. Construction or
restoration failures retain a trace diagnostic and use ordinary initialization;
a failed binding discards its runtime before retrying on the canonical DOM.
The second matching request starts construction asynchronously and itself uses
ordinary bootstrap. Requests arriving before completion also use that path.
The 32 MiB limit covers retained seeds and artifacts, not transient builder
memory or live consumer copies. There is no process-global snapshot cache.

`FunctionCodeKeep` is deliberate: the bounded Keep/Clear comparison saved only
about 57 KB (0.7%) with Clear, while first API execution increased from
0.141–0.156 ms to 0.196–0.221 ms. This was a separate PoC cohort, not an
end-to-end production comparison.

## Bounded memory measurement

A fresh stock-dependency test binary used an external overlay adding one diagnostic browser test; no engine or runtime files were replaced. Control disables snapshots with `MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT=1`; enabled uses the current integrated implementation. Ten Pages run sequentially in one browser Context, each creating and removing ten iframes. The first Page also creates/removes a warm-up iframe and waits for snapshot construction. Existing lifecycle ownership retains detached frame runtimes until Page.Close, so the last live sample contains eleven isolates.

Live samples follow V8 LowMemoryNotification on every observed runtime, then Go GC and FreeOSMemory. After-close samples follow Go GC and FreeOSMemory. These interventions characterize memory and must not be interpreted as benchmark latency. Values are MiB (2^20 bytes).

| Last-cycle phase | Control private | Snapshot private | Control Go heap | Snapshot Go heap |
| --- | ---: | ---: | ---: | ---: |
| Ten iframes removed, after GC | 268.664 | 340.816 | 11.723 | 71.071 |
| Page.Close, after GC | 137.191 | 128.441 | 10.568 | 15.540 |
| Context.Close, after GC | 137.176 | 123.406 | 10.570 | 10.557 |

| Eleven live isolates, after GC | Control | Snapshot |
| --- | ---: | ---: |
| V8 used heap | 108.044 MiB | 87.157 MiB |
| V8 physical heap | 124.355 MiB | 96.121 MiB |
| Native contexts | 11 | 11 |
| Persistent adapter handles | 13,957 | 1,076 |

The current cache entry is exactly 5,220,080 bytes (4.978 MiB). The extra Go heap after Page.Close is consistent with that cache; after Context.Close the difference is effectively zero. Every one of 111 observed runtime closes passed synchronous assertions: adapter closed, zero retained globals, zero owner contexts, and no consumer snapshot root. Cache population and the last restored main runtime's 46 persistent handles versus 1,217 in the control confirm actual snapshot use.

The initial single-script seed produced a 13,954,264-byte cache entry and much higher live memory: 626.707 MiB private and 171.016 MiB Go heap in the same last-cycle scenario. The intermediate four-stage seed reduced its payload to 5,198,312 bytes and live memory to 334.637 MiB private / 70.801 MiB Go heap. The final correctness-frozen build, including native-global publication and ordinary Error.stack source URLs, is the 5,220,080-byte result shown in the tables. The disabled control is unchanged from the original paired memory run. The original results and binary are preserved separately. The exact payload reduction is established; cross-process private-memory differences also include allocator variation.

This bounded comparison finds no additional snapshot-specific Go retention after complete Context closure. It does establish a material live-memory cost from cache and per-isolate consumer copies. Private bytes include native heaps, native copies and process allocator pools; this test does not uniquely attribute every native allocation or prove that all memory returns to the OS. It is not a long-duration leak stress test or a concurrency/many-cache-entry memory result.

Raw artifacts are local diagnostics in `compatibility/private-captures/bootstrap-snapshot-memory-20260910/`: `control.json`, original `enabled.json`, intermediate `enabled-split.json`, final `enabled-final.json`, `result.md`, additive overlay and test source. Current memory binary SHA256: `1648fbca73568d43337432cb061ffefee4da81cfe273eb7eb14b1d0b5bc71bd6`.

## Remaining iframe creation cost

After the correctness-frozen source passed focused equivalence tests, freshly built baseline and instrumented binaries ran sequentially A/B/B/A with no concurrent heavy tests. The baseline overlay adds only the identical diagnostic helper and test; the instrumented overlay inserts QPC spans in production paths. Each process runs 24 rounds and excludes the first four. Every round requires an actually restored child and checksum 6050 from the existing mixed operation sequence.

The timed setup creates/appends an iframe, obtains contentWindow and evaluates the child definitions. Mixed operations and removal are outside this interval. It uses separate Page.Evaluate calls, so it is not the unchanged single-expression end-to-end benchmark.

| Run | Median setup | Mean setup |
| --- | ---: | ---: |
| A1, baseline | 16.237 ms | 16.625 ms |
| B1, instrumented | 14.801 ms | 14.946 ms |
| B2, instrumented | 15.226 ms | 15.349 ms |
| A2, baseline | 16.181 ms | 15.667 ms |

The following means cover the same forty instrumented rounds. Start/end timestamps were checked for nonoverlap in every round; component means plus residual equal the mean total. Component medians must not be summed.

| Nonoverlapping component | Mean | Share |
| --- | ---: | ---: |
| Profile key and cache selection | 1.661 ms | 11.0% |
| Go snapshot clone | 1.118 ms | 7.4% |
| Native snapshot parameter preparation | 0.006 ms | <0.1% |
| Native isolate creation | 4.006 ms | 26.4% |
| Context deserialization | 4.419 ms | 29.2% |
| Adapter initialization after Context creation | 0.126 ms | 0.8% |
| Go host map/function preparation | 0.272 ms | 1.8% |
| Host object publication | 0.636 ms | 4.2% |
| Restore lookup, global publication and rebind | 0.867 ms | 5.7% |
| Resolver, wrapper capture and hiding internals | 0.337 ms | 2.2% |
| Unattributed remainder | 1.701 ms | 11.2% |
| **Total** | **15.148 ms** | **100%** |

Native isolate creation and Context deserialization account for 8.425 ms (55.6%); restoration/publication itself is below 1 ms. This supports investigating isolate/context creation and profile-key/clone overhead before making snapshot bootstrap more elaborate. It does not establish that those native costs are unavoidable or quantify a possible ownership redesign's gain.

Native copying performed inside NewIsolateWithSnapshotParams remains in the isolate-creation interval; the separate parameter span is not a measurement of all native blob copying. Context timing covers the SDK call on the owner thread. Owner startup/dispatch, clock setup, frame/DOM work, contentWindow lookup, child eval and other uninstrumented work remain in the residual. Publication and JavaScript rebind share one existing runtime Call; no extra host calls were introduced to split them.

The uninstrumented median is roughly 1 ms above the instrumented median, demonstrating process/code-layout/allocator variation. No instrumentation speedup is claimed. The ranking of large costs is useful; tiny differences should not be treated as stable optimization targets. These setup-only results do not measure cold snapshot construction, concurrency, network intervals or full workloads. Exact aggregates, source hashes, one-match replacement receipts, binary hashes and raw rounds are in `compatibility/private-captures/bootstrap-snapshot-breakdown-20260910/` (`summary.json`, `receipt.json`, `result.md`, `A1.json`, `B1.json`, `B2.json`, `A2.json`).

## Correctness requirements and evidence

Restoration must preserve observations of ordinary bootstrap, including property
order and descriptors, constructor/prototype identity relationships, native
function strings, exceptions, callbacks and realm-specific state. Objects remain
independent between realms. A finite corpus is evidence for this contract, not
a proof of all possible JavaScript observations.

The differential corpus compares ordinary and confirmed-restored main frames and
iframes in insecure, secure and isolated profiles. It traverses global, interface,
prototype and namespace descriptors and identity relationships. Separate checks
change locale, timezone, viewport, screen/DPR and preferences between seed and
consumer while requiring reuse of the same snapshot profile. Exception tests
compare complete Error.stack strings; seed compilation preserves the ordinary
`mimic:webapi-surface` source URL.

Other focused tests cover DOM/form/shadow callbacks, same/cross-origin navigation,
WindowProxy identity, re-entry and microtask ordering, pending-timer teardown,
cache eviction/cancellation, private intrinsic capture and fallback after an
injected restoration failure. Warm concurrency uses three waves of ten Pages;
cold concurrency exercises capture, background construction and restoration
across navigation and Page closure.

The existing cross-frame bridge still rejects reading one child's object with
another child's local Symbol (`cross-realm symbol belongs to another realm`).
The same operation fails with snapshots disabled; this change does not claim to
fix that Chrome compatibility gap. Snapshot tests also check distinct child
Symbols through a shared parent object to detect accidental token collisions.

One corrected-gate run exited during its first measured ten-session concurrency
wave: the process-tree samples fell to zero active processes and all WebSockets
closed. That frozen harness deleted its temporary process log, so the cause is
undetermined. A complete retry using the identical executable SHA256
`0fcca2c2aaf33d89479932655fa5519fcc1b9bdd82a7279857aae9673595d7c6`
passed, with process logs preserved by an external wrapper. A further 300 CDP
sessions across three processes and 180 cold-admission test Pages, including
race-detector runs, did not reproduce the exit. This is an unresolved diagnostic
limitation, not an established fix or evidence that the original exit was harmless.
Receipts are in `compatibility/private-captures/bootstrap-snapshot-final-20260910/`.

## Final workload checkpoint

The unchanged `BenchmarkFrameBridgeOperations` ran sequentially A/B/B/A with
40 iterations per process, one fresh binary and checksum 6050 throughout.
Ordinary bootstrap measured 130.348 / 139.823 ms per operation; snapshots
measured 97.983 / 105.236 ms. The two-run means are 135.085 versus 101.610 ms,
approximately 24.8% less elapsed time. These are complete-probe timings on the
restored separate-isolate architecture, not the historical 54.9 ms experiment.
The benchmark was not changed to wait for cache readiness or exclude admission;
initial bootstrap and background construction contribute to its measurements.
It closes its Page, while the dedicated memory test explicitly closes Contexts.

The setup-only final diagnostic above measures warm restored consumers. An earlier
paired setup cohort measured ordinary 61.37–66.47 ms versus restored
18.80–19.65 ms, with the mixed cross-frame portion unchanged at 88–91 ms.
The final setup diagnostic's 16.2 ms uninstrumented medians are a separate cohort,
not a further claimed optimization. Frozen Chrome 152.0.7977.83 measured a
2.0 ms median setup in 40 browser-side samples; its mixed phase was below the
timer resolution. Mimic's QPC setup includes Evaluate entry, while Chrome's
timer is inside JavaScript, so this is not an exact tiny-interval ratio.
Cold admission remains real work: the earlier integrated cohort measured
204–218 ms for background image construction. There is no guaranteed benefit
for the first iframe in a cold browser Context.

Both final `fast_gate.py` runs pass all six frozen semantic workloads, warm
workloads, ten/twenty-five-session concurrency and memory waves (184 valid rows
per mode). Each run freshly built and verified the identical executable SHA256
`020d1dbec85fc335deac47465d92a522980ad57a46707eb4919a867ed0ebe34d`.
The harness fingerprint remains
`ce1fce42fa9b9e03f105900601db4d6d7fa9b0d9cda51d5096322357277673e7`.
An external wrapper preserves process logs before the frozen runner's cleanup;
workloads, expectations and timed operations are unchanged.

| Warm workload median completion | Ordinary | Snapshot |
| --- | ---: | ---: |
| DOM | 599.260 ms | 573.236 ms |
| Static | 72.294 ms | 25.377 ms |
| React | 150.244 ms | 106.677 ms |

The three measured 25-session waves delivered 47.77 / 48.79 / 48.64 sessions/s
with ordinary bootstrap and 46.26 / 86.05 / 74.04 with snapshots. The first
measured wave is retained; background cache readiness is not forced by the gate.
DOM and concurrency variation argue against extrapolating these bounded runs
into universal speedup or Chrome parity.

`go test -json ./...` completed with 1,071 passing test/subtest events and no
failures after the publication correction. The subsequent source-URL-only fix
passed its exact-stack regression; the complete snapshot suite then passed again
under `-race` on the final source (browser 18.473 s, engine 1.316 s). The final
paired gates and memory/profile binaries also include that correction.

Private receipts: `bootstrap-snapshot-final-20260910/benchmark-release.json`,
`release-receipt.json` (source hashes, Go/module version and binary hash),
`release-gate-summary.json`, `full-tests-corrected.jsonl` and
`snapshot-race-release.log`, under `compatibility/private-captures/`.
Complete gate data/build receipts are in
`.build/bootstrap-snapshot-release-gate-disabled/` and
`.build/bootstrap-snapshot-release-gate-enabled/`.
