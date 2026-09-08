# Performance architecture pass

**CORRECTION: runs `01-page-concurrency` and `02-dom-host` are invalid for optimization attribution.** The invocation used `--skip-build` without `--mimic`, so the frozen harness selected the pre-existing `.build/mimic-benchmark.exe` rather than the newly built `.build/mimic.exe`. The former embeds revision 2d6ae126 with modified=true. Tables and aggregate statements below for those two runs describe that stale binary, NOT the optimization commits. Source HEAD alone was insufficient provenance. Preserve all raw observations; rerun each class from its detached source revision with an explicit executable and verify embedded revision plus SHA-256. Diagnostic Go test profiles were built from their intended source and remain usable.

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

Diagnostic replay: 1,252.9 ms -> 512–543 ms, persistent handles 664,483 -> 10,282, crossings 231,528 (including wrapper instrumentation) -> 99,523. The diagnostic wrapper counter itself creates 9,005 roots; ordinary builds do not install it. This is not a frozen-harness result and is still above the 150 ms target. Empty collections and byte arrays preserve recursive marshaling semantics. Baseline JSON/fixture correctness is unchanged.

Memory attribution before DOM changes: ten live static Pages use 362.1 MiB V8 physical heap, 10.1 MiB Go heap, 468.4 MiB process RSS; V8 external memory is zero and reported malloced memory totals 2.5 MiB. Static RSS slope from five to ten Pages is 37.8 MiB/Page in this diagnostic sequence. After close and Go scavenging, RSS remains 360.2 MiB despite Go heap 7.4 MiB and all isolates disposed. This is evidence of native/allocator retention, not evidence that Go retains all Page objects. RSS residual cannot be labeled exact native ownership: mapped code, stacks and allocator pools contribute. React follows static in this diagnostic and benefits from its allocator history; use the immutable density benchmark for comparative marginal RAM.

Full unchanged harness after class 02: `benchmark/runs/02-dom-host`, comparator `benchmark/runs/02-comparison.json`; all 12 correctness gates passed. Warm DOM execution median is 1123.71 ms; completion static 101.49 ms, DOM 1196.34 ms, React 173.36 ms. Static N=25 throughput 7.71/s; N=50 stopped on sustained system paging. React N=50 throughput 3.70/s. OLS marginal RSS: static 62.34 MiB, React 56.43 MiB. Targets remain unmet. Chrome cold static has one retained ERR_ABORTED. Diagnostic replay lacks the CGO linkage of the production executable; its timings must not substitute for these measurements.

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
