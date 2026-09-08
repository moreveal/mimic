# Performance architecture pass

## Current evaluation criteria

The frozen Chrome 152 measurements are the reference. Absolute millisecond milestones are retired and are not stopping criteria. Every workload must report Mimic/Chrome ratios for creation, navigation and execution, together with attributed remaining overhead. Startup and Page creation should beat Chrome; navigation, DOM and CPU work should approach or beat its measured costs. Fixed memory, marginal Page memory and density should provide a substantial advantage, not merely parity.

Continue a measured path until its gap is small, profiling demonstrates an intentional architectural tradeoff or unavoidable backend boundary, or diminishing returns make another measured bottleneck materially more important. An expensive current implementation is not by itself proof of an unavoidable cost. Correctness remains mandatory. The fast gate is the iteration tool; the full frozen matrix is a milestone gate after a substantial group of changes.

**CORRECTION: runs `01-page-concurrency` and `02-dom-host` are invalid for optimization attribution.** The invocation used `--skip-build` without `--mimic`, so the frozen harness selected the pre-existing `.build/mimic-benchmark.exe` rather than the newly built `.build/mimic.exe`. The former embeds revision 2d6ae126 with modified=true. Tables and aggregate statements below for those two runs describe that stale binary, NOT the optimization commits. Source HEAD alone was insufficient provenance. Preserve all raw observations. Subsequent fast and milestone gates use explicit freshly built executables with per-launch SHA-256 verification; the user superseded the earlier per-class full-rerun plan. Diagnostic Go test profiles were built from their intended source and remain usable.

Frozen harness SHA-256: `ce1fce42fa9b9e03f105900601db4d6d7fa9b0d9cda51d5096322357277673e7`. Original `benchmark/results` is unchanged.

## 01: Page concurrency

Page-owned command locks replace the server-wide lock. Context bootstrap runs outside the registry lock; shared localStorage and permission delivery have their own synchronization. A network-barrier regression proves an independent Page can evaluate while navigation is blocked.

Full harness: `benchmark/runs/01-page-concurrency`; machine memory pressure stopped density growth at N=1. No N=50 throughput or marginal RAM conclusion is possible. Chrome cold DOM contains one `net::ERR_ABORTED`, retained without retry or exclusion.

Mimic warm medians (ms), mechanically extracted from the unchanged comparator:

| Workload | Metric | Frozen | After | Change |
|---|---|---:|---:|---:|
| static | session_create_ms | 118.28 | 85.87 | -27.4% |
| static | execution_ms | 2.82 | 2.97 | 5.5% |
| static | completion_ms | 127.51 | 90.26 | -29.2% |
| dom | session_create_ms | 95.33 | 78.78 | -17.4% |
| dom | execution_ms | 1560.32 | 1271.19 | -18.5% |
| dom | completion_ms | 1661.46 | 1341.98 | -19.2% |
| react | session_create_ms | 121.60 | 69.43 | -42.9% |
| react | execution_ms | 166.03 | 89.06 | -46.4% |
| react | completion_ms | 302.94 | 168.72 | -44.3% |

These single-session deltas are observations, not causal claims for concurrency: memory pressure and background load differed. Correctness: all 12 browser/workload gates passed; Go suite and CDP race suite passed. Initial browser race run passed (396.302s); subsequent storage/permission changes require the final full rerun.

## 02: DOM host boundary

Measured before changes: 120,035 API trace crossings (301.6 ms inclusive host time), six queryAllWithin calls (149.8 ms), 27,000 attribute writes (91.9 ms), 18,006 parent projections (85.7 ms), 9,001 insertion calls (78.6 ms). The sampled full DOM phase was 1,252.9 ms; these nested inclusive costs must not be added to total CPU. Go CPU attributes 37% flat to cgocall and substantial scanning/GC; native stacks are not resolved by Go pprof. Full allocation sampling attributed 857.7 MiB to gov8 callTextFn (its fixed-capacity string buffer), not to live DOM text.

Changes: deduplicate API traces before crossing into Go; borrow arguments only for explicitly synchronous hosts; retain callbacks for asynchronous APIs; use exact-length reads for strings with ToString fallback for other types; batch plain record arrays through native JSONParse; perform canonical ancestry traversal in Go; pass only node IDs for non-resource insertion while preserving resource scheduling.

Diagnostic replay: 1,252.9 ms -> 512–543 ms, persistent handles 664,483 -> 10,282, crossings 231,528 (including wrapper instrumentation) -> 99,523. The diagnostic wrapper counter itself creates 9,005 roots; ordinary builds do not install it. This is diagnostic attribution, not a frozen-harness result. Empty collections and byte arrays preserve recursive marshaling semantics. Baseline JSON/fixture correctness is unchanged.

Memory attribution before DOM changes: ten live static Pages use 362.1 MiB V8 physical heap, 10.1 MiB Go heap, 468.4 MiB process RSS; V8 external memory is zero and reported malloced memory totals 2.5 MiB. Static RSS slope from five to ten Pages is 37.8 MiB/Page in this diagnostic sequence. After close and Go scavenging, RSS remains 360.2 MiB despite Go heap 7.4 MiB and all isolates disposed. This is evidence of native/allocator retention, not evidence that Go retains all Page objects. RSS residual cannot be labeled exact native ownership: mapped code, stacks and allocator pools contribute. React follows static in this diagnostic and benefits from its allocator history; use the immutable density benchmark for comparative marginal RAM.

Full unchanged harness after class 02: `benchmark/runs/02-dom-host`, comparator `benchmark/runs/02-comparison.json`; all 12 correctness gates passed. Warm DOM execution median is 1123.71 ms; completion static 101.49 ms, DOM 1196.34 ms, React 173.36 ms. Static N=25 throughput 7.71/s; N=50 stopped on sustained system paging. React N=50 throughput 3.70/s. OLS marginal RSS: static 62.34 MiB, React 56.43 MiB. These stale-binary observations do not establish optimization progress. Chrome cold static has one retained ERR_ABORTED. Diagnostic replay lacks the CGO linkage of the production executable; its timings must not substitute for these measurements.

## 03: Initial Page and immutable bootstrap data

Production-linked diagnostic replay disproves CGO linkage as the cause of the benchmark/replay discrepancy: DOM remains approximately 492-496 ms when linked with QuickJS/CGO. The frozen CDP harness remains authoritative. Native code accounts for 60.45% flat CPU in this replay; unresolved native stacks prevent finer attribution from Go alone.

Before this class, warm blank creation takes 64 ms and immediately following navigation takes 71 ms. Compilation of the generated bootstrap takes 26-31 ms and execution 31-36 ms. Each initial blank document gets a separate full isolate which first navigation replaces. The initial document now owns its DOM, URL, identity and scheduler immediately; engine creation occurs on first engine observation. Navigation before observation avoids constructing the unused isolate. Independent mutable realms are preserved and tested.

Immutable source and exposure JSON are shared in a bounded Go cache. Generated metadata is parsed as JSON rather than compiled as JavaScript object literals; generated closures capture names instead of entire metadata records. Diagnostic warm navigation is 47.62 ms, bootstrap compilation 7.78 ms, execution 35.93 ms; initial creation is below 1 ms. Static live N=10 RSS falls from 466.8 to 399.1 MiB. Diagnostic V8 collection reduces those figures to 225.3 and 169.4 MiB respectively, showing much of the allocation footprint is collectible; this forced collection is NOT enabled in normal runtime or benchmark.

A compilation-byte cache experiment was discarded: pinned gov8 CreateCodeCache retries a native buffer after the read/delete API has freed it; its CompileCached also dereferences a nil Origin. No unsafe cache path or dependency replacement is included.

Validation: full ordinary Go suite passed (browser 58.043s); independent initial blank regression passed. Full race and frozen benchmark results are recorded after completion. Repository publication audit currently flags a machine-local executable path in the unmodified new benchmark raw shutdown exception; raw evidence has not been rewritten to conceal that failure.

## 04: One CDP event-loop pump per Page

Before: 16 WebSocket debugger connections advance one Page clock by 2309.238 ms during 150.387 ms wall time (15.36x). Mutex/block profiles are retained in `.build/pump-{mutex,block}.pprof`. The session-owned timer creates both redundant work and incorrect observable time.

After: Page-owned pumps shared by all connections, canceled when the target or server closes. Same probe: 150.352 ms Page time / 150.210 ms wall time (1.00x). Asynchronous navigation also holds its session binding lock, preventing a rebind from routing an ongoing navigation into another session. Ordinary and race CDP suites pass, including independent Page navigation and clock amplification regression. The full bootstrap race suite also passed; frozen benchmark evidence follows in explicitly version-verified runs.

## Verified rerun 01

Source `f1a526fc134be4cc009638ee2d1206363132f68c`, executable SHA-256 `7f55ba9d2729dcf7dc439d942a8fb488b60a15c6866441485038826feb7bedb3`. Clean detached source checkout and Go module path verified before building; the exact executable was supplied with `--mimic`. Go 1.26.1 VCS discovery recognizes `.git` directories but not worktree `.git` files, so the source and output hash are verified separately. All 12 gates pass. Static N=50 throughput: 5.4072 -> 8.5934 sessions/s (+58.9%); CPU N=50: 2.9174 -> 7.9820 (+173.6%). Static N=50 RSS: 3091.7 -> 2810.2 MiB. See `benchmark/runs/01-verified-page-concurrency` and `01-verified-comparison.json`.

| Warm workload | Metric | Frozen ms | Verified ms |
|---|---|---:|---:|
| static | session_create_ms | 118.28 | 70.45 |
| static | execution_ms | 2.82 | 1.94 |
| static | completion_ms | 127.51 | 73.00 |
| dom | session_create_ms | 95.33 | 91.56 |
| dom | execution_ms | 1560.32 | 1703.89 |
| dom | completion_ms | 1661.46 | 1803.17 |
| react | session_create_ms | 121.60 | 70.88 |
| react | execution_ms | 166.03 | 96.41 |
| react | completion_ms | 302.94 | 178.77 |


## Fast iteration gate and class 05: DOM identity transport

User-directed loop change: full matrix runs are milestone gates, not per-change iterations. The in-progress second verified full run was stopped on request; its completed raw samples remain in `benchmark/runs/02-verified-dom-host`, without a full-run completion claim. Runs 03 and 04 were not launched. The frozen harness and original raw baseline remain unchanged.

`tools/performance/fast_gate.py` builds an executable into a fresh output directory and checks its SHA-256 immediately before every process launch. All six semantic workloads are mandatory; warm DOM/static/React use five measured iterations, static N=10/N=25 use three measured waves, with excluded warm-ups. Static and React memory are measured with ten live pages and after teardown/recovery. See `fast-gates/*.json` for all rows and executable provenance.

Before class 05, the attributed Go allocation profile contains nodeData record projection, recursive JSON conversion and per-property native calls; queryAllWithin takes 63.24 ms for six calls. Query results now transport canonical node IDs and reuse existing wrappers, loading a full record only for a new wrapper. Canonical attributes remain visible after external Go mutations. Immutable realm token reads are hoisted once; plain record conversion is batched while retaining rejection of unsupported Go values.

| Fast gate metric | Before class 05 | After class 05 |
|---|---:|---:|
| DOM execution median ms | 511.95 | 382.50 |
| DOM completion median ms | 564.92 | 436.57 |
| Static completion median ms | 54.45 | 54.19 |
| React completion median ms | 118.96 | 123.22 |
| Static marginal RSS MiB/page, N=10 | 39.16 | 39.40 |
| React marginal RSS MiB/page, N=10 | 42.67 | 43.08 |

Before executable SHA-256: `029caf9f8e83fde87e3531cd9b50c78d866ceb8603ead9545bbe290c682b0910`; after: `71bdc64069292b41d8f09503d5c0971fe5d9f0bb1adf98b6b042f0e231cc281c`. DOM execution improves 25.3%; no memory or React gain is claimed. Full ordinary correctness passed on retry. The first run timed out in TestPerformanceObserverReceivesFinalizedNavigationEntry; isolated replay passed, and the complete fresh retry passed. Both logs are retained locally.

The new V8 Inspector profile attributes about 120-128 ms of DOM execution to fragment removeChild processing; bootstrap applyTargetExposure takes about 21 ms of a 41 ms execution phase. These are the next measured architectural costs. Go allocation sampling after class 05 totals 314 MiB over three DOM replays plus diagnostics (previous comparable replay 624 MiB, with an additional three obsolete blank bootstraps). Do not attribute that entire difference to class 05. The V8 heap snapshot after collection has approximately 15.2 MiB of live nodes: arrays 4.34, strings 3.60 (bootstrap source 2.69), objects 3.29, closures 1.63 MiB. This is not resident process memory; the gap to physical/resident allocation remains material. Diagnostic replay timings include profiler overhead and are separate from gate timings.


## 06: Linear DocumentFragment membership transfer

V8 CPU sampling identified 120-128 ms in removeChild when moving 3,000 fragment children: repeated front removal shifted the remainder each time. Fragment insertion now drains membership once and clears synthetic parent links before inserting the ordered children. Existing append/insert-before, identity, ownerDocument, comment and cycle regressions pass for Goja and V8. Full ordinary correctness passes (browser 58.867 s).

The fast gate records DOM execution 382.50 -> 333.43 ms (-12.8%). This run had substantial variance (DOM completion 301.60 to 494.85 ms across five iterations), and static also slowed from 54.19 to 69.54 ms; therefore a larger causal speedup is not claimed. V8 sampling confirms the former removeChild hotspot disappears. All raw samples remain in `fast-gates/fast-gate-linear-fragment.json`; native samples are in `linear-fragment-native-phases.json`. SHA-256 is in each gate's build receipt and launch records.


## 07: Per-isolate nursery sizing

Per-space V8 diagnostics identify 16 MiB physical new-space allocation on static Pages with only 1.71 MiB used; React has the same 16 MiB footprint with 1.35 MiB used. The engine's nursery growth is multiplied by the independent Page isolates. A diagnostic collection costs approximately 7-8 ms/page and releases about 23 MiB/page, but no forced collection is inserted into benchmark measurement.

Isolate creation now caps the young generation at 4 MiB through V8 CreateParams. Old-generation limits retain V8 defaults; Page isolation and event-loop ownership are unchanged. All ordinary correctness tests pass (browser 77.609 s), and all six frozen semantic workloads pass in the fast gate.

Static marginal RSS N=10: 39.40 -> 28.84 MiB/page; React: 43.08 -> 35.67. Corresponding private bytes: 28.90 and 36.01 MiB/page. These are measured reductions; Chrome-relative memory advantage still requires the full milestone comparison. Residual RSS after ten closed pages is still 207.50 MiB static / 261.76 MiB React above process readiness, so teardown remains unresolved. DOM execution median is 281.16 ms, static completion 57.72 ms and React completion 113.38 ms. Host-machine variance precludes attributing all timing changes to nursery sizing. Evidence is in `fast-gates/fast-gate-page-nursery.json` and `nursery-{before,after}-spaces.json`.


## Correctness gate finding: zero-duration navigation completion

The earlier intermittent observer timeout reproduced in 20 isolated repetitions. Trace shows a fast local navigation with zero transport duration; the observer deduplication key used duration to distinguish initial and finalized entries, so a zero-duration completed load was indistinguishable from its initial entry. This is a lifecycle-state defect, not a reason to inflate measured time. The private observer notifier now receives explicit load-finalized state, and its navigation key includes that state. No public Web API is added. The test accepts a nonnegative duration (zero is legitimate at timer resolution), has a bounded wait, and a deterministic zero-duration finalization regression covers the failure. Twenty repetitions pass after the fix.


## 08: Release unused V8 pages at isolate teardown

Before disposal, after all adapter roots and contexts have been released, a V8 low-memory notification lets the engine reclaim empty heap pages. The measured residual RSS above readiness after ten closed Pages falls from 207.50 to 68.66 MiB (static) and 261.76 to 88.10 MiB (React). This residual includes shared process/allocator state and Go memory, not just live Page objects. Live marginal memory stays at 29.10 / 35.45 MiB per Page.

The tradeoff is explicit: median teardown within the N=10 memory wave rises from 6.42 to 20.46 ms static and 7.54 to 26.40 ms React; static N=25 median throughput falls from 80.39 to 62.37 sessions/s in these samples. Warm execution/completion excludes teardown by the unchanged harness definition, while wave throughput includes it. There is no collection added by the benchmark: this is the production isolate lifecycle policy and its cost is included where applicable. Fast gate DOM execution is 258.06 ms, static completion 58.00 ms, React completion 109.01 ms. See `fast-gates/fast-gate-release-isolate.json` for SHA-256 and all raw rows.

After repairing the zero-duration observer correctness defect, the full race-enabled correctness suite passes (browser 377.989 s, CDP 2.797 s). This validation precedes the separate classList host batching change.


## 09: Canonical DOMTokenList toggle in one host operation

Before: classList toggle repeatedly exported and parsed the same attribute via getAttribute/setAttribute; the profile attributed 51.39 ms to all attribute writes, 31.99 ms to reads, and 33-47 ms of V8 samples to domTokens. The private toggle host now performs one canonical read/modify/write under the DOM mutex. There is no stale JS value cache. Existing force behavior and whitespace/deduplication semantics are retained, with explicit Goja/V8 tests including external Go attribute mutation and Unicode whitespace.

Fast gate DOM execution improves 258.06 -> 197.37 ms (-23.5%); static completion is 59.02 ms and React 111.23 ms. All six workload result gates and the full ordinary correctness suite pass (browser 62.332 s). Native attribution now shows 18,000 setAttribute calls / 25.20 ms and 12,000 toggleToken calls / 22.07 ms; the former 21,000 repeated attribute reads disappear. The next measured cost is 9,001 node creation hosts / 50.16 ms. Raw timing and source/executable hashes are in `fast-gates/fast-gate-token-toggle.json`; the diagnostic profile is `token-toggle-native-phases.json`.


## 10: Return canonical identity for newly created nodes

Before this class, create/createText/createComment consume 50.16 ms across 9,001 calls in the attributed DOM replay. The JS caller already owns the initial text/type/empty attribute state, but each call serializes that state back through a full Go record. New text/comment nodes and ASCII HTML names now return canonical IDs and construct their known initial JS record locally. Non-ASCII names preserve Go's original normalization path; UTF-16 surrogate text takes the canonical record path to preserve bridge string conversion. Go owns each node immediately, including detached nodes. Goja/V8 Unicode, identity and insertion regressions pass, as does the full ordinary suite (browser 53.165 s).

Fast gate DOM execution: 197.37 -> 156.11 ms (-20.9%); static completion 58.05 ms; React completion 103.97 ms. A slow DOM sample remains in the raw series (completion 325.42 ms); no sample was removed. Compared with frozen Mimic DOM execution 1560.32 ms this is approximately 10x; compared with frozen Chrome execution 30.326 ms it remains about 5.15x slower and is not a stopping point. The final full milestone matrix is required to confirm results.

The final diagnostic replay records 69,503 crossings including 9,005 diagnostic wrapper hooks, versus 231,528 originally with the same hooks. Persistent handles after DOM are 10,284 including those diagnostic hooks (approximately 1,279 without them), versus over 655,000 normal retained roots originally. Go sampled allocation totals 216 MiB over three DOM replays plus bootstrap and profiling; native/FFI execution remains dominant at 55% flat Go CPU samples. These profiling numbers are attribution, not benchmark timings.

The N=50 parallel creation/navigation diagnostic also collected CPU/allocation/mutex/block profiles. The principal aggregate mutex waits are initial immutable Surface OnceValue publication (~1.98 s across all waiters) and gov8 isolate construction bookkeeping (~0.78 s), not a lock held across independent Page CDP evaluations. Aggregate waits must not be read as wall-clock delay. Per-Page phase records are in `final-parallel-phases.json`; local profiles are `.build/final-parallel-*.pprof`. No isolation boundary was changed.

## Milestone 03: frozen full matrix, classes 01–10

Source `d69cff971c9aba22f28889ded904caff392ed8f1`; executable SHA-256 `967df4b8677f545b38ff9f5c74e7c607e0c6b7deadbc6fbceff0fda557461d68`. The external milestone wrapper built the executable and verified both executable hashes before every launch. The immutable comparator accepted harness, fixture, Chrome binary and recorded environment equality. Raw evidence: `benchmark/runs/03-architecture-milestone`; deltas: `benchmark/runs/03-architecture-comparison.json`.

All twelve system/workload correctness gates pass. React N=100 was stopped during the excluded warmup for sustained system paging in both systems; it is not a successful density observation. All other recorded levels complete. This matrix predates the callback-registry teardown correction.

Warm medians (20 measured samples each); ratios use the original frozen Chrome 152, not this run’s Chrome:

| Workload | Mimic navigation ms / Chrome ratio | Mimic execution ms / Chrome ratio | Mimic completion ms / Chrome ratio |
|---|---:|---:|---:|
| static | 53.10 / 2.31× | 1.04 / 0.21× | 54.22 / 1.94× |
| cpu | 53.53 / 2.25× | 45.38 / 1.56× | 98.79 / 1.88× |
| dom | 54.08 / 2.41× | 153.50 / 5.06× | 207.32 / 3.88× |
| async | 53.98 / 2.07× | 48.37 / 1.61× | 102.03 / 1.82× |
| react | 106.17 / 4.31× | 87.83 / 3.56× | 194.87 / 3.97× |
| wasm | 64.49 / 2.71× | 9.28 / 1.81× | 73.88 / 2.58× |

DOM execution improves 1560.32 → 153.50 ms (10.17×) but remains 5.06× Chrome. Profiles attribute the remaining cost primarily to synchronous DOM host crossings/native calls; attribute mutation is the largest remaining host family. Navigation spends about 8 ms compiling and 35–49 ms installing per-isolate bootstrap; exposure/prototype normalization is its largest measured phase. These are implementation costs still eligible for optimization. Static execution is already cheaper than Chrome. CPU and Wasm need dedicated native workload profiles to separate engine work from embedder overhead; async includes timers/worker/event-loop work and requires phase attribution.

React’s full warm median is worse than the short gate: execution rises from roughly 40 ms to 85–99 ms after the first few iterations, while RSS grows. A later isolated diagnostic reproduces memory growth but not the sustained doubling in execution cost; therefore memory retention alone must not be asserted to explain the timing shift. The raw slow samples remain included.

Static N=50 throughput is 75.11 sessions/s versus frozen Chrome 25.93 (2.90×); CPU 32.52 versus 27.72 (1.17×); React 30.78 versus 14.50 (2.12×). Static N=100 falls to 10.28 versus Chrome 20.95 (0.49×): four measured waves have about 10 extra seconds after the workload completion window, indicating a teardown/connection-tail investigation rather than slower DOM execution. This remains unresolved.

Fitted marginal RSS: static 24.47 MiB/Page versus frozen Chrome 59.46 (0.41×), CPU 26.12 versus 73.87 (0.35×), React 37.38 versus 72.27 (0.52×; current fit only through N=50). Linear-fit intercepts are 82.30/145.12/90.66 MiB respectively; these are modeled intercepts, not directly measured process readiness. Warm Page creation medians span 2.12–14.04 ms, all below the corresponding frozen Chrome 37.86–43.60 ms. Calibrated CDP readiness is 231.84 ms versus frozen Chrome 279.37 (0.83×); startup remains an optimization candidate because that advantage is modest.


## 11: Release Go host callback registrations at isolate disposal

A 20-Page React replay in one Context reproduces monotonic retained memory. The post-GC heap contains closed DOM nodes, response bodies and semantic trace records. gov8 explicitly requires `ReleaseIsolateHostState` on the owner thread before `Isolate.Close`; Mimic omitted it. The global callback registry therefore rooted every callback closure and its closed Realm. A weak-reference regression fails before the fix and passes after it, directly proving callback capture release. Cleanup now follows final native collection and precedes native isolate disposal.

The same diagnostic replay reduces post-GC live Go heap 32.33 → 12.45 MiB and RSS 119.91 → 84.23 MiB. First/last post-close RSS changes from 76.30/119.74 to 75.25/84.74 MiB. Ordinary execution median is 41.76 → 35.85 ms in this diagnostic, but the full-matrix sustained React timing doubling was not reproduced and is not declared solved. The fast gate is essentially unchanged for single-workload timing: DOM 156.11 → 162.74 ms, static completion 58.05 → 59.11 ms, React completion 103.97 → 106.80 ms. Relative to frozen Chrome, execution is 5.37× DOM / 0.23× static / 1.69× React. N=25 static throughput is 62.71 → 74.97 sessions/s; N=10 is 53.46 → 42.50, so a blanket throughput gain is not claimed.

Immediate ten-Page RSS retention in the fast gate does not improve: static residual 64.37 → 65.68 MiB, React 90.21 → 93.21 MiB. The benchmark does not force Go GC, and freed Go objects need not return allocator pages immediately. This change repairs indefinite ownership retention, not all allocator high-water memory. Further teardown/allocator attribution remains necessary.

All ordinary correctness passes (browser 87.783 s), including repeated owner-thread disposal/cancellation tests and the capture regression. All six frozen workload semantic gates pass. Fast executable SHA-256: `bfbac8a2a0db5b13fd663109c529bec3f3f2cbc5998513b634c21fdbc4f15d2d`. Diagnostic executable hashes are `31a2e6b5e75e0e091530da8e0202ed64e2d96ef3dd50a322f9ca416b4fb2f757` before and `400926c52d61fad9c790195ff089501c3b10003ef765e07d0c01a9dd78ef7b5d` after; each was verified immediately before launch. Phase evidence is `react-long-{before,after}-phases.json`; CPU/heap/allocation/mutex/block/goroutine profiles remain in `.build/perf-react-long` and `.build/react-long-*.pprof`.
