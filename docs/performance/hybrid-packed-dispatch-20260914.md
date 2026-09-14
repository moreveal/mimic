# Integrated hybrid Fast API decision

**Recommendation: reject promotion and stop this bounded dispatcher experiment.** The native path executed, but the complete frozen DOM workload did not become faster: execution **108.994 → 109.946 ms (+0.87%)**, navigation through completion **140.326 → 140.765 ms (+0.31%)**, and session creation + navigation/completion + teardown **146.394 → 146.560 ms (+0.11%)**. This provides no basis for expanding native DOM work and misses the required integrated 2× threshold.

The comparison used the same fresh Linux production executable, with the private hybrid flag disabled/enabled in **control, candidate, candidate, control** order. Each of six unchanged frozen workloads had one excluded warmup and three retained samples per process: **96/96 runs VALID; 72 retained samples; all four processes exited 0**. Full supported loading, tracing, cache-disabled resources and the ordinary canonical Go DOM remained enabled. Native counters were collected after the timed work and active-memory sample. No heavy work ran concurrently.

| Workload | Control execution, ms | Hybrid execution, ms | Change | Control complete operation, ms | Hybrid complete operation, ms |
|---|---:|---:|---:|---:|---:|
| Static | 3.720 | 4.081 | +9.71% | 42.085 | 42.426 |
| CPU | 35.619 | 33.789 | −5.14% | 74.544 | 71.353 |
| DOM | 108.994 | 109.946 | +0.87% | 146.394 | 146.560 |
| Async | 62.740 | 64.046 | +2.08% | 95.860 | 101.904 |
| React | 49.633 | 45.915 | −7.49% | 109.385 | 104.885 |
| Wasm | 4.426 | 4.457 | +0.68% | 40.757 | 41.590 |

Complete operation is the per-sample sum of session creation, navigation through completion and target teardown; its median is calculated before rounding. It excludes the fixed recovery wait and controller setup. No throughput improvement is claimed from the inverse of these numbers.

Only DOM invoked the candidate's new path. It took a median **3,360.5 native fast calls and 8,639.5 generic warmup calls per Page**, out of 12,000 token toggles, or approximately **28%** fast. Across all four DOM runs in each candidate process, independent native counters reported **13,464 and 13,334** calls, exactly equal to the sum of per-Page Go fast counters. The other five workloads took zero native fast calls; their differences, including the apparent CPU/React gains, are not evidence of accelerating their execution with the new path. Static execution and async complete-operation differences also cross 5% in this short series. The target itself provides enough evidence to reject promotion without additional timing runs.

DOM process CPU from before session creation through workload completion was **190 → 200 ms**. Incremental private memory from the same before-session sample was **36.68 → 39.71 MiB**. Absolute process private memory at DOM completion was **268.65 → 252.99 MiB**, and after teardown plus the fixed recovery wait **241.60 → 225.91 MiB**. These process-level memory figures include previous workloads, Go/V8 allocator state and background work; they do not establish a per-Page memory saving or a leak. The two-Page GC/disposal regression did independently prove that all native routing registrations return to their starting count. No additional allocation profile or 100-Page soak was run after the integrated latency result rejected the candidate.

The prototype keeps one authoritative Go DOM and adds an exact C++ V8 ABI thunk, a C-ABI Go dispatcher, numeric routing and an invocation-owned packed primitive frame. It uses no execution lock and never creates a V8 value or enters JavaScript from the fast body. Errors are delivered by a normal callback after return without repeating the mutation. It was tested for Unicode and ordinary fallback, error after mutation, nested tracing callbacks, MutationObserver delivery, Go/V8 GC, two concurrent Pages and disposal on Linux race and Windows. The [Windows validation](data/optimization-20260914/hybrid-packed-dispatch/windows-focused.txt) and [Linux race validation](data/optimization-20260914/hybrid-packed-dispatch/linux-race-focused.txt) preserve the focused evidence. Those functional test durations are not speed comparisons.

Implementation cost is a 10-file incremental Go patch with 601 added/32 removed lines, including 246 test lines, plus a 33-line C++ thunk and platform build recipes. The external experiment library is not packaged into the production shim. Supporting that additional native path is not justified by this result. The result bounds this packed Go-dispatcher design; it does not measure a native DOM core or every possible V8 Fast API use.

## Reproduction and evidence

- Frozen integrated control tree: `cf023020cc7292f64f4aebcfac601a2d12f778f9` over `f07bd2d643eebbadc28d582ef93f01c832a50106`. It contains improved DOM, v3 lifetimes, binary Fetch and shared bootstrap snapshot, and precedes later BodyStore sharing and final root cleanup. Both sides use the exact same base.
- Binary `mimic-hybrid-linux`: `aecbb2236640a77eaa83859b7c3261a1f557ec848302805ed8b10251a65d6c61`.
- Native library `libhybrid_thunk.so`: `0cdfc1cc5178a39282ebe32bb44b77c8715392e785ce3406c9c328a613957961`.
- Incremental unpromoted patch `hybrid-incremental.patch`: `a45fa79cb4ae60f3e21dfcd3e97e7a107d30cf1aea4275e6aed08b8dde83f51c`.
- Raw [observations](data/optimization-20260914/hybrid-packed-dispatch/raw.json): `3c58b4bd559481b7b36a6ed20fcd3b23aaa170b26cba78c4b317b762c84bc1b8`; includes launch commands, per-launch binary hashes, versions, source/harness fingerprints, all samples, counters, resource requests, CPU/memory and process exit receipts.
- Compact [summary](data/optimization-20260914/hybrid-packed-dispatch/summary.json), [analysis](data/optimization-20260914/hybrid-packed-dispatch/analysis.json) and [receipts](data/optimization-20260914/hybrid-packed-dispatch/receipts.json) are retained. The rejected runtime prototype and local orchestration/build files are not production changes.
- Runner imports the existing Linux adapter and untouched frozen harness. The first attempt stopped before launching an engine because Linux Git did not understand the Windows worktree's absolute `.git` reference; the retry supplied the corresponding Linux `--git-dir` explicitly. No sample was collected or excluded from that failed metadata attempt. This setup cost is outside all workload measurements.

Selected handle v2/v3 changes and the mandatory borrowed cached-module-error fix are independent of this rejected prototype and remain recommended. No main-checkout source or frozen benchmark file was modified by this experiment.
