# Goja observer profiling and deduplication checkpoint

Baseline: 78eb990, following [callable metadata](callable-metadata-2026-09-11.md).
The changed tree contains only owned Goja runtime/test edits at measurement time.
[Receipts](goja-observer-20260911/) record revisions, dirty state, executable and
probe hashes, commands, failures and corpus results. This is a small engine
performance correction, not a newly fixed Chrome semantic root cause.

| ID | Cause / hypothesis | Repro | Chrome | Mimic | Priority | Status | Fix | Limits |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| GOJA-OBS | Redundant support queries after trace deduplication | TestGlobalAccessObserverDeduplicatesSupportQueriesWithoutCachingHas | Internal instrumentation, no Chrome analogue | Repeated Reflect.has calls for discarded observations | P2 | Corrected | Check the existing seen map before support query | Membership still queries live state |
| GOJA-RACE | Retained-srcdoc setup exceeds its original deadline with race instrumentation | TestIframeSrcdocRetainsExportedDescendant/goja | Not a wall-clock oracle | Both paired builds time out | P2 | Open performance failure | None claimed | Not a native V8 crash |
| DOC-GET | Invalid receiver checks and foreign active document projection | Local document getter discovery | Strict except measured lenient attributes; borrowed projections agree | 644 differing leaves, including seven foreign projections | P1 | Confirmed backlog; multiple causes | Pending | 644 leaves are not 644 defects |

The Get trap now skips Reflect.has when the observer has already recorded the
property. The Has trap still reads live state after mutation. Captured Reflect.get
continues to preserve accessor receivers and exact thrown values. The strengthened
regression addresses the actual installed global proxy. Its earlier draft used
the old globalThis binding and bypassed interception: the two initial failed
fixture runs are retained privately and are not defect evidence. The corrected
regression fails on the baseline and passes with this change.

Full browser suite PASS (461.695 s test output, 468.014 s package completion).
Goja engine tests PASS; CDP and WebAPI checks reused cached passing results.
Full Goja engine race PASS (2.050 s); full browser race was not run.
A predeclared paired retained-srcdoc race/CPU profile FAILS on both builds:
baseline 25.59 s, changed 23.53 s, at the unchanged setup deadline. No retry,
deadline extension or selected-workload exclusion converts these into passes.
The earlier 706fc15/78eb990 profile pair also failed both sides. These timings
are not a causal performance estimate because both executions are incomplete.
The changed profile records 20 ms in support queries and 990 ms cumulative in
the observer Get trap; the prior profile exposed 880 ms in redundant support
queries. This supports removing that overhead, not closing the timeout.

Fresh V8 binary replay: fixed corpus stays 205/236; original representatives
stay 19/28. No new differing leaves, regressed probes or Chrome drift. Fresh
Chrome-to-Chrome replay is 236/236 and 28/28. Independent corpus records remain
40 general, 609 brand, 2 relations; all corresponding controls have zero diffs.
These overlapping counts must not be added. This Goja-only edit does not change
V8 semantics; V8 replay is not evidence of complete Goja corpus parity. New
fuzzer discovery and sweep were not repeated: their prior 9 observational groups
and 114/130 sweep remain historical rather than fresh measurements.

The unchanged V8 fast gate was not repeated for this small Goja-only edit. The
previous paired gates and the now-completed separate native/density profiles
remain the performance evidence for V8; no Goja concurrency or teardown-memory
improvement is claimed. Native-source profile jobs on clean ed4261b and 706fc15
all passed. After diagnostic Go GC/scavenging, static/cpu/react private memory
was 120.89/127.90/128.82 MiB before and 120.79/127.03/128.19 MiB after. Ten live
Pages and forced Go reclamation differ from the gate topology. Native workload
profiles begin after navigation, and static has only 34/37 samples. Neither
profile explains the earlier 25-Page throughput drop or proves no retention bug.

[P0 read-only snapshot lineage](snapshot-readonly-lineage-2026-09-11.md) remains
corrected only within its demonstrated scope. Other native signatures, especially
CreateBlob, remain open and are not merged with Goja timeout failures. No target
site was run; raw profiles, native dumps and session data remain ignored locally.
