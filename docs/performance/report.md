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


## 12: Separate immutable catalog data from retained executable source

Before this class, V8 retains the approximately 1 MiB escaped generated catalog literal as part of every bootstrap script. Detailed static profiling attributes bootstrap costs to generated binding installation (~11 ms), exposure JSON/global setup (~5 ms), prototype normalization (~6.2 ms), native marking (~5.3 ms) and prototype ownership reconciliation (~5.3 ms), plus ~9 ms compilation. The generated catalog is now an explicit, deterministic JSON artifact embedded once in Go and delivered through a private host during bootstrap. Window and Worker use the selected bundle's catalog; JavaScript parses independent per-realm objects. Mutable prototypes/objects and isolate boundaries remain unchanged.

Static diagnostic navigation median falls 65.24 → 58.65 ms. At navigation, V8 used heap falls 18,151,632 → 15,350,320 bytes and physical heap 23,228,416 → 20,193,280 bytes. This is live heap evidence, without a forced collection in the benchmark. Fast gate marginal RSS decreases 28.81 → 26.44 MiB/static Page and 35.75 → 33.02 MiB/React Page. Immediate residual RSS remains 64.18 / 89.41 MiB; allocator retention is not solved by separating source data.

Fast gate DOM execution 162.74 → 142.89 ms (4.71× frozen Chrome), static completion 59.11 → 56.78 ms (2.03×), React completion 106.80 → 100.86 ms (2.05×). Static N=25 throughput 74.97 → 69.54 sessions/s, N=10 42.50 → 52.15; no general throughput gain is asserted. The full ordinary correctness suite passes (browser 85.584 s), as does a Goja/V8 regression proving nested catalog objects do not cross Page boundaries. Offline generation/hashes pass; the retained upstream IDL, CDP and exposure inputs are unchanged. Executable SHA-256: `4956f8dbfb00f5856fafbf4348a5d12f946cdc367bfe1d531a7e0014657d0cac`. Raw gate is `fast-gates/fast-gate-catalog.json`.

A separate safe FunctionCodeCache experiment reduced bootstrap compile time roughly 9 → 3 ms but did not reduce installation cost (43–52 ms); end-to-end navigation only improved a few milliseconds and cold cache production added 5.7 ms. It was removed, prioritizing the larger retained-source and installation costs. This used the safe Function cache API, not the unsafe Script cache retry described earlier. No cache implementation remains in production.

The N=100 static close diagnostic (`.build/profile-close-100/raw.json`, executable SHA-256 `54ca9e42680689566543533535afcf20a1a2b8db9bdbb5216829c27ce9a27291`) did not reproduce the full milestone's ten-second tail: three waves completed in 1.48/1.25/1.26 s; maximum observed individual CDP close was 39.84 ms. The tail's root cause remains unproven and must be monitored in later milestones, not erased from the previous result.


## Class 13: bounded transient callback argument storage

Fresh profiles replace the preceding attribution. Before this change, the 20-Page DOM replay without diagnostic hooks costs 155.16 ms and allocates 48.45 MiB Go bytes per execution. The detailed replay costs 182.90 ms: instrumentation is material and its wall time is not a benchmark result. The allocation profile assigns 41.49% of total allocated bytes to adapter callback argument construction. Conversion is called 153,112 times per diagnostic DOM execution, taking 48.46 ms inclusive of its diagnostic timer. These costs must not be added to inclusive host timings.

Synchronous transient callbacks now reuse bounded, per-adapter scratch frames (up to eight frames, eight arguments each). Nested calls check out distinct frames; every frame is cleared after result marshalling. Larger calls and persistent callbacks keep their original allocation/lifetime contract. No DOM wrapper, mutable prototype or isolate is shared. Reentrant callbacks, Unicode return values, persistent callbacks, and cleared scratch roots have regression coverage. The ordinary suite passes, including browser tests in 69.774 s.

The matched 20-Page replay drops to 139.72 ms (-10.0%) and 29.82 MiB allocated Go bytes (-38.5%). The unchanged fast gate gives DOM 140.11 ms versus the preceding gate's 142.89 ms (-1.9%, **not** an order-of-magnitude latency improvement). Its value is the large reduction in allocation traffic without a density regression. Static completion 53.92 ms (1.93× frozen Chrome); React execution 41.34 ms (1.67×), completion 102.48 ms (2.09×); DOM execution remains 4.62× Chrome. Live marginal RSS is 26.33 MiB/static Page and 32.82 MiB/React Page versus 26.44/33.02 before. N=10/N=25 static median throughput is 54.06/71.24 sessions/s versus 52.15/69.54. All six semantic gates pass; every launch is hash verified. Gate executable SHA-256: `1fc3bcabe51492c62bc46c750f73fbec2d62a5b002ea544bc49263337263aed8`.

Fresh evidence is retained under `dom-current` and `fast-gates/fast-gate-transient-frames.json`. Local Go CPU/allocation/mutex/block/goroutine and native CPU profiles are `.build/dom-fresh-*.pprof`, `.build/perf-dom-fresh-detailed`, `.build/dom-frames-*.pprof`, `.build/perf-dom-attribution`, and `.build/perf-dom-go-only`.

### Current DOM attribution and limitations

* **Host boundary / conversion:** dominant. The current backend calibration performs 60,000 empty transient host calls in 19.82 ms, and the same calls exporting a number and two strings in 71.02 ms. This is an independent boundary calibration, not a frozen workload measurement or a subtraction-based prediction. Go CPU samples also place native calls and callback dispatch first. The SDK's string extraction checks type, computes UTF-8 length, writes UTF-8, and copies bytes into a Go string; generic export adds another type check. Windows DLL calls and Go callback transitions remain part of this backend boundary. Canonical Go DOM requires synchronous host mutation; moving DOM ownership would change an intentional architecture boundary. Neither all 140 ms nor every SDK call is proven unavoidable.
* **Wrapper identity / mutation bookkeeping:** the fresh native baseline samples average about 7.04 ms in `wrap`, 2.96 ms in `observe`, 5.06 ms in Proxy `get`, and 3.92 ms in insertion steps per replay. These are sampled self times, not inclusive cost or exact uninstrumented wall time. Canonical wrapper identity remains intact. The earlier insertion experiment was saved outside production and reverted before this new baseline; it is not an accepted optimization.
* **Selector/tree traversal:** six `queryAllWithin` calls total roughly 4.3–4.5 ms inclusive in the detailed current replay. `elementChildren` executes 6,002 times. This is a smaller cost than conversion/host dispatch, despite the traversal count.
* **Go mutation bodies:** detailed callback bodies total 78.25 ms, including 56.68 ms of conversion instrumentation; this is not 78 ms of tree mutation. Argument framing and return handling measure 11.84/14.20 ms with detailed timers. Subtracting these overlapping, perturbed medians does not yield a precise additive decomposition.
* **Tracing/diagnostics:** native `recordAPIAccess` self time is about 6.99 ms per fresh baseline replay; Proxy work overlaps the tracing path. Optional wrapper counting itself adds 9,005 host calls. The control/detailed replay difference (~28 ms before scratch reuse) demonstrates substantial profiling perturbation; normal gates disable it.
* **V8 GC:** native baseline sampling attributes about 7.29 ms/replay to GC. **Go GC:** execution phase counters record a median 0 ms stop-the-world pause; this does not imply zero concurrent marking or allocation-assist CPU. CPU samples include allocation and GC work; the allocation traffic reduction is independently measured.
* **Scheduler/tasks:** aggregate block time is mostly the calling goroutine waiting for its Page's owner to finish execution, not an extra delay to add to DOM time. Mutex contention is primarily Go runtime/GC, with no evidence of a DOM-wide Page serialization bottleneck in this replay. Native profiler idle/setup samples are excluded from hotspot interpretation.

A 100-Page sequential diagnostic (no forced GC between Pages) keeps six goroutines after each close and two after the final collection. Static closed RSS grows 60.61 → 78.51 MiB, React 67.31 → 81.49 MiB; Go live heap fluctuates rather than accumulating one Page per close. This is allocator/process retention, not zero retention. React execution nevertheless rises from a first-ten median 32.44 ms to last-ten 75.92 ms; that timing regression requires fresh edge profiles and must not be dismissed as solved by the earlier callback-root fix.


## Class 14: explicit latency QoS for Page owner threads

The 100-Page React regression is now attributed with fresh measurements. Native edge profiles show the same heap (~20.0 MiB used), external memory (zero), allocator usage (524,432 bytes), and similar work, but approximately doubled costs across unrelated JS and native functions. OS topology identifies logical CPUs 0–15 as performance cores and 16–27 as efficiency cores on this i7-14700KF. Sampling the current processor every 1,024 host calls shows 60/60 samples on performance cores for each first-ten group; after ~30 Pages, 60/60 samples are on efficiency cores. Execution changes 32–33 → 75–82 ms. These samples are inside execution, not just the core on which the final diagnostic happened.

An opt-in diagnostic setting the thread's execution-speed QoS to latency-sensitive restores 32–36 ms across all 100 Pages, with 600/600 performance-core samples. The production change applies the same Windows scheduling hint to Page owner threads. It neither pins CPUs nor raises scheduler priority, does not change process-wide policy, honors an existing explicit thread policy, and restores the original state before V8 unlocks the OS thread. Unsupported OS queries leave the default behavior intact. Idle Page threads remain blocked. This intentionally prioritizes latency over Windows' automatic background energy-saving classification; energy consumption was not measured and no energy-efficiency claim is made.

Production validation, without the diagnostic policy override: React first/last-ten execution 33.44/36.25 ms, overall median 33.67 ms over 100 Pages (1.36× frozen Chrome's 24.692 ms, noting this is a diagnostic replay). DOM first/last-ten 142.57/138.76 ms, overall 144.38 ms (4.76× Chrome); 5,883 performance-core and 17 efficiency-core samples. The large React long-run regression is eliminated without claiming the DOM backend boundary is eliminated.

The unchanged short gate passes all semantic workloads. Static marginal RSS 26.24 MiB, React 32.74 MiB; static N=10/N=25 throughput 53.67/70.64 sessions/s, essentially unchanged from 54.06/71.24. Short-gate DOM execution regresses 140.11 → 152.11 ms; retain this result, do not replace it with the longer diagnostic. Static completion 52.63 ms, React execution 39.84 ms, completion 104.49 ms. Source and raw launch hashes are in `fast-gates/fast-gate-page-qos.json`; executable SHA-256 `8b42fc83cb03488ad8de95244b7788faaab004cc09ff3f0bcd5a26a76740c73c`.

All ordinary tests pass (browser 87.283 s); engine race checks pass. A Windows regression verifies active QoS and exact restoration on the same OS thread. Long-run evidence and CPU topology are preserved in `dom-current`. A single new full frozen matrix follows this measured subsystem milestone. Startup remains deprioritized: internal readiness and the frozen external readiness probe measure different phases; no readiness workaround was added.


## Milestone 04: full frozen matrix after classes 11–14

The full run completed once, including every N=100 level for both systems, with all twelve semantic gates and every measured wave successful. Source is clean commit `082a22657dd7117affcb4fa3ab8b57a9c7b04359`; executable SHA-256 `3d5c896c4f710e93ab876a40f46b49c6ac3b6ac85246e192907e02dc9e708a46`. Every process launch verifies the recorded executable hash. The immutable comparator accepts the original harness, fixture and Chrome provenance. Raw evidence is `benchmark/runs/04-dom-long-run-milestone`; comparison is `benchmark/runs/04-dom-long-run-comparison.json`. The subsequent edits only extend diagnostic test serving/collection, not production behavior.

Warm medians, 20 samples each. Ratios use the **original frozen Chrome 152**, not this run's Chrome. Each cell is Mimic milliseconds / Mimic-to-Chrome ratio.

| Workload | Page creation | Navigation | Execution | Completion |
|---|---:|---:|---:|---:|
| static | 8.28 / 0.22× | 54.45 / 2.37× | 1.10 / 0.22× | 55.53 / 1.99× |
| cpu | 12.96 / 0.30× | 55.92 / 2.35× | 46.91 / 1.61× | 103.14 / 1.96× |
| dom | 12.04 / 0.32× | 52.86 / 2.35× | 148.62 / 4.90× | 201.91 / 3.77× |
| async | 13.34 / 0.34× | 56.21 / 2.15× | 52.13 / 1.73× | 104.47 / 1.87× |
| react | 13.50 / 0.35× | 64.45 / 2.62× | 44.33 / 1.80× | 107.05 / 2.18× |
| wasm | 8.43 / 0.22× | 58.22 / 2.45× | 9.25 / 1.80× | 67.63 / 2.36× |

Relative to milestone 03: DOM execution 153.50 → 148.62 ms (3.2%); React execution 87.83 → 44.33 ms (49.5%), completion 194.87 → 107.05 ms (45.1%). DOM remains expensive: its original frozen 1560.32 ms is reduced 10.5× overall, but current Chrome-relative 4.90× is not parity or an evidence-based claim of an absolute performance limit.

| Workload | N=50 throughput / frozen Chrome | N=100 throughput / frozen Chrome | Fitted marginal RSS / frozen Chrome |
|---|---:|---:|---:|
| static | 76.72/s / 2.96× | 77.01/s / 3.68× | 21.19 / 59.46 MiB (0.36×) |
| cpu | 45.07/s / 1.63× | 37.72/s / 2.14× | 35.99 / 73.87 MiB (0.49×) |
| react | 44.42/s / 3.06× | 47.54/s / 3.43× | 27.99 / 72.27 MiB (0.39×) |

These slopes fit all six valid levels through N=100. They differ from the short gate's direct ten-Page 26.24/32.74 MiB measurements; do not interchange estimators. Cold ready RSS across measured workloads has median 24.36 MiB versus frozen Chrome 380.18 MiB. Model intercepts are not the ready-process footprint. N=100 static waves last 1.26–1.37 s, so the previous ten-second connection tail did not recur; its original root cause remains unproven.

**CPU memory regression:** compared with milestone 03, CPU marginal RSS rises 26.12 → 35.99 MiB (+37.8%) while N=50 throughput rises 32.52 → 45.07/s (+38.6%). That comparison spans classes 11–14, not QoS alone; it does not establish QoS as the sole cause. The prescribed static/React memory baselines are preserved and improved. This CPU memory regression is recorded, not hidden behind those improvements.

### Final-build profiles and residual costs

Fresh final-production profiles, captured after the full matrix, are summarized in `dom-current/final/native-hotspots.json`. Exact build and launch hashes accompany them. Native profiles and Go allocation/mutex/block/goroutine files remain in `.build/profiles-final-native-complete`. The diagnostic runner `tools/performance/profile_gate.py` rebuilds and verifies the executable before each replay, checks the frozen fixture fingerprint, and separates native sampling from execution-only Go CPU sampling. Its first async attempt found a missing `/worker.js` route in the diagnostic server, not a product regression; the server now serves the frozen runner's exact worker bytes. The full benchmark's async gates had already passed.

* **DOM:** exactly 60,060 host crossings per uninstrumented-wrapper replay. Final native sampling averages 98.72 ms in anonymous native/host frames, 6.00 ms in API-access tracing, 5.78 ms in `wrap`, 4.79 ms in Proxy `get`, 2.96 ms in `observe`, and 5.16 ms in V8 GC. Inspector idle/setup is excluded from attribution. The earlier detailed conversion/host counters explain the native family; their inclusive timings are not added to these sampled self times. Synchronous canonical Go DOM and the present typed-value/UTF-8 SDK boundary explain the dominant overhead. Further large reduction requires changing that bridge's representation/call granularity, while preserving immediate canonical mutations; small selector optimizations cannot close this gap. It is **not** proven that every current bridge operation is unavoidable.
* **CPU:** native samples predominantly execute `__benchRun` (37.63 ms/replay) and V8 GC (8.25 ms). This workload is not dominated by DOM host traffic. The four-MiB young generation remains an intentional density/GC trade-off; the exact 1.61× Chrome gap is not wholly explained by GC alone.
* **React:** native/host samples ~15.81 ms, V8 GC ~1.47 ms, plus React reconciliation and wrapper/Proxy/insertion work. The earlier long-run doubling is independently reproduced and removed by the thread QoS experiment and production 100-Page validation. Remaining 1.80× full execution includes canonical DOM bridge and task delivery.
* **Async:** the isolated replay is mostly waiting/task delivery; the CPU sampler cannot charge idle samples as CPU or equate them to the timed window. Its diagnostic median 24.70 ms is well below full CDP 52.13 ms, so the full gap is not solely JS execution. Exact CDP/scheduler/worker contributions remain unresolved; no false additive attribution is asserted.
* **Wasm:** isolated execution is 2.76 ms with ~1.07 ms sampled in the workload loop, ~0.51 ms in export lookup, ~0.41 ms in JS→Wasm and ~0.10 ms in the Wasm body. Full CDP execution remains 9.25 ms. Thus the full 1.80× gap cannot be called an inherent slow Wasm backend; surrounding task/CDP completion costs remain to be isolated.
* **Navigation/static:** the installed per-isolate compatibility surface and prototype/exposure work remain the principal measured navigation overhead. Static workload execution itself is already cheaper than Chrome. Startup probing was left untouched.

Ten held Pages, independent diagnostic (not the benchmark's RSS slope):

| Workload | Process RSS MiB | Go live heap MiB | Sum V8 used heap MiB | Sum V8 physical heap MiB | V8 reported malloc MiB |
|---|---:|---:|---:|---:|---:|
| static | 269.17 | 13.83 | 146.77 | 194.11 | 2.50 |
| cpu | 351.64 | 15.20 | 165.97 | 255.88 | 3.59 |
| react | 335.80 | 25.38 | 189.71 | 233.68 | 5.00 |

V8 reported external memory is 0/480/0 bytes respectively. Used/physical heap and process RSS are overlapping measures, not additive components; mapped code, stacks, Go reservations and native allocator capacity are not fully attributed by V8 statistics. After close, ordinary 250-ms recovery RSS is 71.91/72.44/94.16 MiB. Explicit Go GC/scavenging (diagnostic only) lowers it to 63.50/67.04/71.03 MiB and ~8.4–8.5 MiB Go live heap.

A separate **forced V8 collection diagnostic**, never substituted for benchmark results, lowers held ten-Page RSS from 267.04 → 145.59 MiB static, 355.09 → 154.44 MiB CPU, and 342.07 → 202.09 MiB React. Median per-Page collection costs are 5.06/6.00/7.90 ms. This proves considerable capacity/garbage is reclaimable and helps attribute CPU's footprint; it does not establish which class caused the full-fit regression. No collection was inserted into the workload or normal navigation to manufacture a smaller official memory number. Choosing an automatic reclamation policy would require a new measured latency/throughput/density trade-off, rather than treating forced-GC memory as the current product result.

## DOM crossing census — 2026-09-08 (diagnostics only)

No optimization or workload/harness/baseline changes. A new `profile_gate.py --hosts` mode enables existing inclusive host timers without wrapper hooks, bootstrap JS instrumentation, or conversion timers. Ten fresh DOM Pages each execute the unchanged workload once. Subtract navigation counters from execution counters; sum of named host counts equals the independent callback sequence delta in every replay: **60,060**, across 20 host names. These are JS-to-Go host invocations, not a count of all bidirectional SDK/FFI operations. All ten complete with the expected DOM result.

Top ten ordered by aggregate callback time. Counts are exact per replay; mean summed time is the aggregate over ten replays divided by ten (not mean duration of one call).

| Host function | Calls/replay | Summed ms/replay (mean) | Summed ms, all 10 |
|---|---:|---:|---:|
| setAttribute | 18,000 | 22.756 | 227.565 |
| toggleToken | 12,000 | 20.797 | 207.967 |
| insertPlain | 9,001 | 11.510 | 115.101 |
| elementChildren | 6,002 | 8.898 | 88.981 |
| queryAllWithin | 6 | 4.455 | 44.547 |
| create | 3,001 | 3.192 | 31.916 |
| createText | 3,000 | 2.888 | 28.884 |
| createComment | 3,000 | 2.415 | 24.151 |
| parentNode | 3,003 | 2.370 | 23.703 |
| contains | 3,001 | 2.315 | 23.148 |

Top ten cover 60,014 calls (99.923%). All host timers total 82.788 ms/replay. Remaining names/counts: apiAccess 31; queryWithin, textContent, query, performanceNow, getAttribute, setInnerHTML each 2; nodeChildren, storageGet, storageSet each 1. By call count, apiAccess replaces queryAllWithin in the top ten.

**Timing boundary and limitations:** timers begin inside the Go callback and end on callback return; they include argument wrapping, conversions, host body, result handling and nested work. They exclude transition/dispatch before callback entry and after return to V8. These are inclusive instrumented elapsed times, not isolated transition overhead or an additive CPU decomposition. Existing Go time.Now/time.Since counters exhibit zero/quantized measurements for short operations; zero does not establish zero cost, and close rankings should not be overinterpreted. Instrumented execution median is 147.449 ms; the separate subsequent control is 156.836 ms. Their -6.0% difference is run variation, not an optimization or a negative instrumentation overhead estimate.

Evidence: `dom-current/crossings-20260908/` contains both raw phase series, build/launch receipts, complete per-replay/per-type counts and times in `summary.json`, and fast-gate results. Reproduce aggregation with `tools/performance/summarize_crossings.py`. Both diagnostic launches rebuilt and verified SHA-256 immediately before execution: `995452e25e294a660d8f410da4f4f8f6da8e2b162597d342fcca25041ed57916`. Frozen fingerprint verification passed. Fast gate passes all semantic gates, prescribed warm runs, N=10/25 waves and ten-Page memory collection. That supplementary gate overlapped correctness tests, so its latency/throughput/memory numbers must not be used to claim a performance delta; the two attribution replays preceded these checks and did not overlap them.
Correctness: go test ./internal/engine/v8 ./internal/browser passes (browser 63.030 s). Full matrix is not rerun: no production optimization milestone occurred.

## Typed attribute argument experiment — 2026-09-08 (not adopted)

The user narrowed this work to one tested experiment before any full implementation. Scope: export hints for only setAttribute (number/string/string) and toggleToken (number/string/string/number). Synchronous Go DOM ownership and the workload remain unchanged. The prototype is isolated at `.build/cross-opt-typed`, detached commit `0b4c43c`; no runtime/browser change is applied to the main checkout. A complete patch and regression tests are retained in `dom-current/typed-attributes-experiment/prototype.patch`.

The first variant used NumberValueRaw. Its DOM fast gate regressed 146.833 -> 149.923 ms (+2.1%). SDK inspection showed that existing NumberValue already uses a direct-return fast path. The revised variant preserves that path and skips only redundant type probes. StringValue itself validates strings; mismatched hints fall back to ordinary Export. No unchecked native casts or changed coercion rules were introduced.

Fresh detailed profiles (ten executions each) show setAttribute inclusive mean 25.586 -> 21.911 ms and toggleToken 25.363 -> 21.179 ms; combined -15.4%. Export instrumentation falls 48.145 -> 42.069 ms. These overlapping instrumented times are attribution, not additive savings.

Uninstrumented execution replays were then run sequentially in A/B/B/A order, 20 fresh Pages per block, rebuilding and verifying SHA-256 before every launch. A is the unchanged checkout and B the revised isolated prototype. No correctness tests or other agent benchmark jobs overlapped these measurements.

| Block | DOM execution median ms | Mean Go allocated MiB/execution | Host calls/execution |
|---|---:|---:|---:|
| A1 | 183.239 | 29.829 | 60,060 |
| B1 | 166.422 | 28.773 | 60,060 |
| B2 | 171.822 | 28.784 | 60,060 |
| A2 | 175.688 | 29.824 | 60,060 |

Pooled medians across 40 executions per variant: 181.307 -> 170.608 ms (-5.9%, 1.063x); allocation traffic improves about 3.5%. System/run variation remains visible, including between these replays and the earlier gate. This supports a modest local effect, not a multiple-fold speedup or a precise universal 5.9% gain.

**Fast-gate regressions are retained:** revised prototype DOM 146.833 -> 153.567 ms (+4.6%), static completion 55.641 -> 65.619 ms (+17.9%), React execution 48.737 -> 55.280 ms (+13.4%). Static N=10 throughput 53.47 -> 42.64/s (-20.2%), N=25 61.58 -> 58.33/s (-5.3%). These sequential gates do not isolate environmental drift from product effects, so the A/B/B/A diagnostic does not erase them or establish gate acceptance. Marginal RSS/static Page 26.347 -> 26.442 MiB; React 32.835 -> 33.026 MiB. After ordinary recovery, process RSS static 88.953 -> 90.590 MiB, React 111.895 -> 114.512 MiB. Retention is not improved by this experiment.

All three fast gates completed the prescribed warm runs, N=10/25 waves, memory collection and all six semantic gates. Both detailed replays and all 80 uninstrumented DOM replays pass workload checks. Prototype engine/browser tests pass (browser 67.846 s, V8 0.363 s), including generic-vs-typed export equivalence, Unicode/lone-surrogate/NUL values, wrong-type fallback, non-finite numbers, large argument lists, nested callbacks and scratch-frame clearing. Frozen fingerprints and executable SHA-256 receipts are preserved beside raw phase/gate data and `summary.json` in `dom-current/typed-attributes-experiment/`.

Decision: retain the prototype for review, do not merge or expand it. The measured opportunity in these type probes is modest, while the requested multiple-fold win would require removing substantially more work/crossings. No full matrix is warranted for an unadopted, limited experiment. No C++/Rust, DOM ownership migration, selector changes or insertion/collection optimizations were attempted.

## Class 15: synchronous packed primitive bridge

Fresh host profiles precede this class (`dom-current/packed-bridge/before-phases.json`). Instead of exporting each primitive through several SDK calls, thirteen synchronous DOM hosts now use one private 2-KiB argument/result ArrayBuffer per V8 adapter. Go owns and pins its pointer-free allocation until the SDK backing-store deleter releases the final native reference. Only the owning Page thread touches it. Arguments are fully decoded into separate bounded Go frames before host execution, preserving nested calls. Short strings use UTF-16 with an ASCII decoding path; long strings and mismatched arguments retain the original callback path. Null/boolean/numeric/undefined results return through private slots; other results retain ordinary marshaling. Mutation remains immediate in the canonical Go DOM. No new Web API, global runtime lock, C++ or Rust implementation was added.

The initial copied-block experiment reduced diagnostic DOM execution 177.813 -> 122.788 ms; removing the per-call SDK copy and primitive result roundtrips reduced it further to 98.817 ms before the final ASCII decoding refinement. These diagnostic medians are not gate numbers. Native buffer ownership uses the SDK's explicit caller-owned backing-store API; no borrowed engine pointer is dereferenced by Go. The final buffer remains live across Go/V8 collections and is released on isolate teardown.

The unchanged fast gate on the final class build gives DOM execution 147.749 -> 95.692 ms (-35.2%, 1.54x), DOM completion 201.422 -> 148.722 ms. React execution 46.619 -> 39.922 ms (-14.4%), completion 109.211 -> 102.199 ms. Static completion regresses 54.062 -> 55.357 ms (+2.4%); no startup improvement is claimed. Static N=10 throughput 54.80 -> 53.90/s (-1.6%), N=25 61.36 -> 61.97/s (+1.0%). Marginal RSS/static Page 26.105 -> 26.383 MiB; React 32.853 -> 33.312 MiB. Post-recovery process RSS static 87.887 -> 88.633 MiB, React 112.840 -> 114.703 MiB. Memory/retention is slightly worse, not hidden by the latency gain. The number of host calls remains 60,060 per DOM execution.

All six semantic gates pass, as do the ordinary repository tests (browser 83.691 s). Focused packed-bridge tests also pass after the final ASCII refinement: primitive/wrong-type/long-string equivalence, Unicode including lone surrogates and NUL, captured string intrinsics, reentrant arguments/results, exceptions, negative zero, Go and V8 collections, repeated close and frame clearing. Build and per-launch SHA-256 receipts, frozen fingerprints, both complete gate files and raw profiles are retained in `dom-current/packed-bridge/`. Full matrix follows the next substantial class rather than replacing the immutable baseline.

## Class 16: skip frame insertion traversal when no iframe can exist

Fresh class-15 host profiles still show 6,002 elementChildren projections during DOM insertion. A conservative document flag now records whether any iframe has been parsed or created, including innerHTML and namespace creation. It never resets on detach: detached canonical nodes can be reused. Non-resource insertion returns that flag through the existing host call, allowing JS to skip the iframe-only traversal when it cannot do work. Ordinary detached DocumentFragments do not run connection steps; ShadowRoots preserve them. Pages that contain iframe elements retain the original traversal, including closed shadow trees. This is not a mirror DOM or a deferred mutation scheme.

Fast gate: DOM execution 95.692 -> 79.340 ms (-17.1%), overall 147.749 -> 79.340 ms (1.86x) from the fresh pre-bridge gate; completion 148.722 -> 134.418 ms. React execution 39.922 -> 36.365 ms, completion 102.199 -> 98.294 ms. Static completion 55.357 -> 54.910 ms. Crossings fall 60,060 -> 54,055 per replay; the diagnostic median is 84.065 ms. Canonical node identity is unchanged. Static N=10/N=25 throughput 54.55/61.02 sessions/s. Marginal RSS static/React is 26.445/32.355 MiB per Page; post-recovery process RSS is 90.293/114.945 MiB. Static density and residual RSS have not improved.

All six gate workloads pass. Focused Goja/V8 tests cover fragment identity and moves, iframe creation from innerHTML followed by fragment insertion, connected and newly connected shadow iframe navigation/load, and all canonical frame-creation paths. Broader tests follow the final group. Raw host/native profiles, gate data and per-launch hash receipts are in `dom-current/insertion-steps/`. The new native profile is the input for the next class; no claim of a full-matrix result is made yet.

## Class 17: share observation handlers and avoid repeated trace-key allocation

The fresh post-insertion native profile attributes about 5.41 ms/replay to recordAPIAccess, 4.43 ms to Proxy get, 2.51 ms to observe and 6.34 ms to V8 GC (sampled self times, excluding 18.59 ms idle/setup). Each DOM wrapper previously allocated fresh get/set closures and repeatedly concatenated the same qualified property and support keys.

Observation handlers are now shared by interface label within one realm. A bounded 128-entry property-name cache per label avoids repeated qualified-name construction; a two-bit state per name preserves once-per-supported/unsupported reporting without appending a boolean to every trace key. Support is still checked against the current receiver/prototype on every access. Wrapper identity and the actual getter/setter receiver remain unchanged; nothing is shared across Pages.

Fast gate DOM execution 79.340 -> 70.283 ms (-11.4%), overall fresh baseline 147.749 -> 70.283 ms (2.10x). Completion is 126.466 ms versus original 201.422 ms (1.59x, not a 2x completion claim). React execution 36.365 -> 33.002 ms; overall 46.619 -> 33.002 ms (1.41x). Static completion 54.196 ms. Static N=10/N=25 throughput is 52.80/62.64 sessions/s versus the original 54.80/61.36; N=10 regresses 3.6%, N=25 improves 2.1%. Marginal RSS/static Page 26.195 MiB and React 31.371 MiB, versus original 26.105/32.853. Post-recovery process RSS 86.953/109.812 MiB, versus original 87.887/112.840. These results do not establish that all workloads or throughput doubled.

All six semantic gates pass. Focused Goja/V8 regressions verify one trace per supported/unsupported state per Page, mutations of own properties and prototypes, and correct getter receivers for different wrappers sharing a label. Full suite/race checks and a single final frozen matrix follow. Gate raw data and launch hashes are `dom-current/observation/gate.json`; native pre-profile is retained under `insertion-steps/`.

## Milestone 05: packed bridge, insertion and observation — final validation

Final production source is clean commit `0abfa05` (classes 15–17); executable SHA-256 `ea237da15757c8f1a11e8a8632cc5a9495040cd64d4373e923a746e1833988c6`. The original frozen Chrome 152 executable hash remains `ea36dd818a90176f1a70616f0363d9be527229389a6c073a0b1688b9e73f67e9`. Every benchmark process launch verifies the just-built executable hash; the frozen harness/fixture fingerprint remains unchanged. Raw full-matrix evidence is `benchmark/runs/05-packed-dom-milestone`, with the immutable comparison against milestone 04 in `benchmark/runs/05-packed-dom-comparison.json`.

**The requested multiple-fold DOM gain is confirmed:** warm execution 148.618 -> 73.406 ms, **2.02x** on the full frozen workload, 20 samples per version. The fresh short gate independently measured 147.749 -> 70.283 ms, 2.10x. Full DOM completion (including navigation) is 201.910 -> 131.728 ms, only 1.53x. Relative to the original frozen Chrome 152 execution median, Mimic DOM improves from 4.90x to 2.42x Chrome; it has not reached parity.

| Workload | Previous / final execution ms | Previous / final completion ms |
|---|---:|---:|
| static | 1.097 / 1.420 | 55.531 / 59.097 |
| cpu | 46.907 / 50.255 | 103.142 / 107.403 |
| dom | 148.618 / 73.406 | 201.910 / 131.728 |
| async | 52.133 / 49.393 | 104.466 / 109.779 |
| react | 44.332 / 38.089 | 107.051 / 109.506 |
| wasm | 9.253 / 9.359 | 67.627 / 71.678 |

These full-run regressions are not erased by the short gate: CPU execution +7.1%; static/CPU/async/React/Wasm completion +6.4/+4.1/+5.1/+2.3/+6.0%. Navigation medians increased by roughly 3–6 ms across workloads. The bridge adds private wrapper setup, but environmental drift and setup cost have not been separately attributed; no startup speedup or all-workload improvement is claimed. React execution improves 14.1% in the full run, not twofold.

All twelve semantic gates and all measured cold/warm workload rows are valid. Concurrency waves through N=50 are valid for all three workloads and both systems. **N=100 is not certified:** each level stopped during its excluded warm-up because the unchanged harness detected memory pressure; Chrome React additionally reports sustained paging. No measured N=100 waves exist in this run. Any numeric N=100 throughput in the generated summary belongs to the stopped level and must not be used as a result. No memory guard was bypassed or workload modified to make these levels pass. Current fitted marginal RSS uses five levels through N=50: static 22.047, CPU 36.415, React 27.897 MiB/Page. Previous fits used six levels through N=100; they are not identical estimators. The ten-Page short-gate measurements remain in the class-17 section.

**Static N=50 teardown regression:** aggregate throughput drops 76.72 -> 18.42 sessions/s. One measured wave has 10.642 s elapsed despite a 0.632 s execution/completion window; its longest teardown is 10,006 ms. The other measured waves take 0.709–0.768 s. This reproduces the kind of intermittent CDP close tail previously recorded before these changes, but does not prove an unchanged cause. All samples and the aggregate regression are retained. Fresh hash-verified close diagnostics on both source versions (three 50-Page waves each) do not reproduce the tail: final elapsed 0.834/0.750/0.661 s, original 0.908/0.723/0.627 s; maximum individual CDP close 25.26/26.18 ms. The tail remains unresolved, and no general static throughput improvement is claimed. Full N=50 CPU throughput is 45.07 -> 45.19/s; React is 44.42 -> 46.15/s.

The measurement phase completed and saved its finished raw data. The wrapper initially exited nonzero only because its default Python lacked matplotlib for report generation. The unchanged `benchmark/report.py` was then successfully run with the existing `.build/benchmark-venv/Scripts/python.exe`; no benchmark samples were rerun or replaced. The frozen comparator accepted the resulting artifacts.

Final host replay records exactly 54,055 calls per execution, versus 60,060 before: 6,005 fewer. Mean inclusive host timer sum is 25.754 ms; this now excludes JS-side argument packing, so it is not an additive or like-for-like estimate of total bridge CPU. Final native sampling attributes about 44.10 ms/replay to anonymous/native frames, 5.63 ms to wrap, 5.03 ms to GC, 3.12 ms to observe and 2.97 ms to Proxy get. Idle/setup (19.56 ms) is excluded. These sampled and inclusive measures overlap; the full workload wall times above establish the actual gain.

The final ordinary suite passes: 222 test/subtest passes, zero failures; browser 82.879 s. V8 engine race checks pass (1.478 s). Lifecycle, Unicode, fallback, reentrancy, DOM identity, iframe/shadow insertion and dynamic trace-state regressions are included. Final raw host/native profiles, build/launch receipts, crossing census, validation summary, and both close-tail diagnostics are under `docs/performance/dom-current/packed-final/`. No production code from the earlier typed-argument experiment was merged; no C++/Rust implementation or DOM ownership migration was introduced.


## 2026-09-09 — isolated JS-owned DOM kernel, not production migration

User-authorized architecture spike is isolated in `.build/jsdom-spike`, commit
`93c1c4a`; its portable patch, receipts and raw evidence are retained in
`docs/performance/dom-current/jsdom-spike/`. Production code is unchanged.

Fresh native profiling and the required production fast gate preceded the
experiment. All fast-gate correctness rows passed (DOM/static/React warm x5,
static N10/25 x3, ten-Page memory). Current control DOM execution is 117.093 ms,
so historical 73.406 ms is not used as the experiment denominator.

The kernel owns its tree in JS and retains identity within that tree. The frozen
workload source is read verbatim and evaluated with a lexical document argument;
no frozen source, expected output, baseline or harness file changes. Native HTML
parsing is retained. This is an explicitly incomplete diagnostic, not an alternate
implementation that passes the full frozen matrix. Child collections are
snapshots; general selectors, live collection branding, events, resource steps,
shadow DOM, complete WebIDL and API access observation are missing. Go does not
observe JS tree mutations. Initial import/compilation is measured separately.

The naive prototype was slower: 227.923 vs 112.633 ms. A fresh prototype CPU
profile identified quadratic fragment removal (143.862 ms sampled self time)
and repeated class token parsing (46.449 ms). Bulk fragment transfer and coherent
token caching bring the in-process comparison to 56.162 vs 112.829 ms. All exact
result fields and independent insertion/identity/error atomicity/attribute/token/
Unicode/comment/HTML/query assertions pass on both variants.

Final CDP diagnostic, 20 samples per variant after one warmup each, ABBA order
within each runtime (runtime blocks are sequential):

| Runtime / DOM | Execution ms | Setup ms | Navigation ms | Close ms |
|---|---:|---:|---:|---:|
| Mimic / current | 111.271 | 0.731 | 73.617 | 6.392 |
| Mimic / JS kernel | 56.298 | 2.993 | 75.292 | 13.007 |
| frozen Chrome 152 / native | 45.210 | 4.298 | 27.542 | 1.698 |
| frozen Chrome 152 / JS kernel | 26.635 | 4.874 | 27.257 | 1.486 |

This is a 1.98x kernel execution improvement inside Mimic, but still 1.25x slower
than Chrome native DOM. Setup and teardown regress. Identical kernel code is
2.11x slower inside Mimic than Chrome in this series; the cause is not isolated.
Investigate runtime/embedding/scheduling before assuming DOM migration alone can
create a strong Chrome lead. No production 2x speedup is claimed.

One diagnostic ten-Page wave per variant, fresh processes, serial creation and
execution: Mimic active RSS increase 402.82 -> 319.60 MiB; after close +250 ms,
116.38 -> 57.92 MiB above ready baseline. Serial throughput including setup and
teardown 4.79 -> 6.31 Pages/s. These are not concurrent-capacity or long-run leak
results. Full matrix is deliberately not claimed for this incomplete kernel.
All benchmark executables were rebuilt and SHA-256 verified before launch;
frozen Chrome digest and harness fingerprint were checked and recorded.


## 18: Preserve engine-owned ECMAScript intrinsic prototypes (2026-09-09)

Fresh paired micro-diagnostics isolated a Page bootstrap defect: generic WebIDL
exposure normalization rewrote built-in ECMAScript prototypes, added spurious
Symbol.toStringTag properties, and invalidated V8 species/prototype guards.
Pure-V8 string/token microcode takes about 8.4 ms after warmup; Page takes about
51 ms before this fix and 9.3 ms after. V8 optimization traces show active Maglev
and TurboFan compilation; a disabled JIT was not the cause. Merely skipping the
final tag loop was insufficient: both exposure prototype passes also modified
intrinsics. The fix excludes engine-supplied globals from these WebIDL passes.
A regression compares intrinsic descriptors against the pristine engine, and
checks subclass species behavior plus native/browser brands, on Goja and V8.
The full ordinary Go suite passes; exact validation is under `intrinsics/`.

The independent JS DOM kernel now takes 16.414 ms inside Mimic versus 16.480 ms
inside frozen Chrome 152, in the same diagnostic protocol (20 measured samples
each). Chrome native DOM takes 28.902 ms. Current production DOM in that series
is still 65.855 ms. Thus the partial kernel demonstrates an execution-only lead,
not a production/browser lead; its missing semantics remain as previously listed.

The production fast gate control -> fixed: DOM execution 65.405 -> 67.519 ms,
completion 125.200 -> 118.425; static completion 55.510 -> 53.622; React execution
35.144 -> 37.684, completion 96.133 -> 99.066 ms. N10 throughput medians
49.55 -> 53.61/s; N25 72.79 -> 62.53/s. All correctness rows valid. Short samples
are noisy: do not present the micro-kernel gain as an across-workload gain.
An earlier exploratory control and CDP run overlapped; they are not the clean
control used here. All executable launches were freshly rebuilt and hash checked.
Production DOM identity, synchronous Go DOM, event loops and API observation
remain intact. No native code, runtime flag or JS DOM prototype is merged.


Timing-boundary clarification: in that diagnostic Chrome native DOM reports
8.6 ms on its in-page timer, versus 14.7 ms for the JS kernel. Its CDP wall times
are 28.902 and 16.480 ms respectively. Thus the apparent kernel lead includes
scheduling/protocol/browser work around script execution; it does not prove a
faster DOM algorithm. Mimic's deterministic in-page timer reports zero here and
must not be compared. Navigation + setup + execution medians are 68.714 ms for
Mimic's kernel and 47.653 ms for Chrome native DOM: end-to-end parity is NOT
achieved. The reproducible diagnostic additions are saved as `diagnostic.patch`
(commit b631da0), separate from the production fix.

## 19: Intrinsics full matrix and bootstrap catalog filtering (2026-09-09)

The completed full matrix `benchmark/runs/06-intrinsics-milestone` tests
4d0f0a1, before catalog filtering. All 12 gates and 396 cold/warm rows are valid;
all 36 concurrency groups completed, including five measured waves at N100.
Fresh executable hashes and the unchanged harness fingerprint are recorded in
`build.json` and per-launch `launches.jsonl`. Reaching N100 depends on available
workstation memory and is not attributed solely to the intrinsic fix.

| Warm workload | Mimic execution / completion ms | Chrome execution / completion ms |
|---|---:|---:|
| static | 1.401 / 54.699 | 5.015 / 29.526 |
| CPU | 37.365 / 93.800 | 29.121 / 53.068 |
| DOM | 74.142 / 131.286 | 34.034 / 60.368 |
| async | 47.495 / 101.933 | 30.288 / 55.251 |
| React | 33.284 / 96.160 | 28.330 / 58.233 |
| wasm | 9.416 / 64.933 | 5.443 / 32.085 |

Against full05, Mimic CPU execution changes 50.255 -> 37.365 ms and React
38.089 -> 33.284 ms; DOM is effectively unchanged (73.406 -> 74.142 ms).
These are separate run cohorts with uncontrolled background applications.

At N100, aggregate throughput across all five waves, including teardown, is
29.48 vs 18.85 Pages/s for static (1.56x), 45.29 vs 14.71 for CPU (3.08x),
and 57.82 vs 21.22 for React (2.72x), Mimic vs Chrome. Static's median-wave
throughput suggests 3.6x but hides a 10008.74 ms teardown tail; use the aggregate
ratio. Median active RSS is 2183.6 / 7112.1 MiB (static), 3316.2 / 8774.0
(CPU), and 2715.9 / 8566.0 (React). Median recovered RSS after 250 ms is
194.8 / 1257.4, 209.0 / 1432.3, and 395.7 / 1422.0 MiB respectively.
These are process-tree measurements, not a long-run leak proof.

The close diagnostic now accepts multiple waves and uses one shared observer
instead of one timer thread per close. Three separate ten-wave, 100-Page static
probes did not reproduce the tail (maximum individual closes 56.224, 66.60,
and 39.703 ms). Both Python environments use Python 3.14.2 / websockets 13.1.
Unread WebSocket events are a hypothesis, not an established cause. No
production teardown fix is claimed. Compressed evidence is under `close-tail/`.

The next production change, 52a2aa2, filters generated fallback bindings before
bootstrap instead of constructing bindings which exposure normalization removes.
The immutable bundle/profile cache stores only source and JSON. Ancestors,
aliases, static members, constants, and members affecting descendant lookups
are retained. Missing prototype capture data is not treated as an empty capture.
Exact exposed descriptor snapshots match the original catalog; the full Go suite
passes (241 test/package pass events, zero failures), including DOM identity.

Clean fast-gate control -> catalog filter:

| Metric | Control | Filter |
|---|---:|---:|
| DOM execution ms | 67.519 | 66.678 |
| DOM completion ms | 118.425 | 109.859 |
| static completion ms | 53.622 | 45.931 |
| React execution ms | 37.684 | 34.524 |
| React completion ms | 99.066 | 84.443 |
| static N10 median Pages/s | 53.61 | 72.37 |
| static N25 median Pages/s | 62.53 | 74.41 |
| static marginal RSS MiB/Page | 26.345 | 24.836 |
| React marginal RSS MiB/Page | 31.736 | 29.082 |
| static recovered RSS MiB | 90.023 | 98.285 |
| React recovered RSS MiB | 114.914 | 114.691 |

The static recovered-memory sample regresses by 8.262 MiB. The fresh detailed
profile measures warm navigation at 42.28 ms, generated bootstrap ending at
9.15 ms, and exposure ending at 27.20 ms, versus 57.63 / 13.84 / 40.44 before.
Fast-gate raw data, build receipts, profile, and validation are under `catalog/`.
The full matrix for the catalog change is a separate run; full06 does not
certify that change. Single-Page Chrome parity remains unachieved.

## 20: Catalog full matrix and refreshed crossing counts (2026-09-09)

`benchmark/runs/07-catalog-milestone` now validates production 52a2aa2.
All 12 correctness gates, 396 cold/warm rows, and 36 concurrency groups pass,
including five measured waves at N100 for static, CPU, and React. All 204
launch receipts match the freshly built executable or pinned Chrome digest;
the frozen harness fingerprint is unchanged. `audit.json` records the checks.

| Warm workload | Mimic execution / completion ms | Chrome execution / completion ms |
|---|---:|---:|
| static | 1.265 / 44.177 | 3.618 / 20.651 |
| CPU | 34.713 / 77.093 | 28.233 / 45.326 |
| DOM | 68.929 / 111.149 | 30.747 / 48.129 |
| async | 49.214 / 91.776 | 27.544 / 44.444 |
| React | 36.307 / 85.881 | 22.412 / 41.219 |
| wasm | 9.065 / 51.443 | 5.165 / 22.480 |

Here completion is the frozen harness's navigation + workload interval;
it excludes session creation and teardown. Against full06, Mimic completion
falls 19.2% for static, 15.3% for DOM, and 10.7% for React. React execution
regresses 9.1%, and async execution regresses 3.6%. Chrome also improves
materially between cohorts, so these changes cannot all be attributed to code.
There is still no single-Page DOM latency win over Chrome.

| N100, aggregate all measured waves | Mimic Pages/s | Chrome Pages/s | Ratio | Active RSS MiB, Mimic / Chrome | Recovered RSS MiB, Mimic / Chrome |
|---|---:|---:|---:|---:|---:|
| static | 91.53 | 28.63 | 3.20x | 1942.4 / 7075.8 | 186.6 / 1254.4 |
| CPU | 52.11 | 24.56 | 2.12x | 3056.2 / 8737.8 | 207.0 / 1411.8 |
| React | 61.26 | 23.07 | 2.66x | 2440.0 / 8339.9 | 394.0 / 1375.9 |

Throughput includes session creation and teardown. Maximum observed measured
concurrency teardown is 257.95 ms for Mimic and 572.33 ms for Chrome. The
previous 10-second tail did not recur; no causal teardown fix was made.
Memory figures are medians across five waves; recovery is sampled after 250 ms.

Fresh detailed profile counts, subtracting navigation from execution and
excluding the first iteration and diagnostic wrapper callbacks, remain 54055
production host calls per DOM iteration. `catalog/crossings.json` contains all
counts and measured host-inclusive time for four warm iterations. Top ten:

| Host call | Calls per iteration | Mean measured host-inclusive ms per iteration |
|---|---:|---:|
| setAttribute | 18000 | 7.527 |
| toggleToken | 12000 | 8.661 |
| insertPlain | 9001 | 5.085 |
| parentNode | 3003 | 0.542 |
| contains | 3001 | 0.935 |
| create | 3001 | 1.173 |
| createComment | 3000 | 1.313 |
| createText | 3000 | 1.451 |
| apiAccess | 28 | below timer resolution |
| queryAllWithin | 6 | 4.019 |

Host-inclusive counters do not include the entire V8/Go dispatch latency and
have instrumentation overhead; do not sum them into a prediction of unprofiled
wall time. The next ownership experiment is scoped in
`dom-current/detached-ownership-next.md`; it is not implemented or merged.

## 21: Detached ownership bridge foundation (2026-09-09)

Detached worktree `.build/detached-dom-spike`, commit b7dc510, now implements
reserved node IDs, atomic import of a plain detached forest, and private host
functions for the bridge. It is not merged into production. Normal WebAPI
creation still uses the eager Go path: no speedup or full ownership migration
is claimed. The preceding full07 matrix remains the production evidence.

The DOM package tests pass, including invalid-forest atomicity, reservation
retry, and isolation from input slice/map mutations. A browser integration test
passes on V8 and Goja using the existing canonical WebAPI wrappers created
before import: identity survives materialization, insertion, parent/child/query
lookup, retained classList use, and subsequent Go attribute mutation. This is
an explicit test-only probe of the bridge, not a substituted workload.

Patch and validation are in `dom-current/detached-bridge/`. Next are the private
JS pending store and complete entry/exit, microtask, error and reentrant-host
synchronization; benchmark only when their cost is included in normal execution.
