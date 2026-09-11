# Captured owner bindings, 2026-09-11

Baseline: clean `09f45a6`, after [srcdoc navigation](iframe-srcdoc-2026-09-11.md).
This is a semantic package, separate from the still-open
[P0 native investigation](snapshot-firstchance-2026-09-11.md).

## Confirmed defects

| ID | Cause | Repro | Chrome | Before Mimic | Priority | Status | Fix | Limit |
|---|---|---|---|---|---|---|---|---|
| IDL-PERF | Missing private receiver checks, arity/conversion ordering and owner dispatch in implemented Performance reads | performance_binding_oracle.js; fixed getEntriesByName probes | Rejects forged/Proxy receivers before conversion; uses receiver's timeline; ignores public getEntries override | Accepts wrong receivers, converts too late, consults public method, uses wrong timeline | P1 | Fixed | Private captured binding operations | Other Performance operations are not claimed complete |
| DOM-ALL-BORROW | HTMLAllCollection methods recognize only local WeakMap entries | html_all_binding_oracle.js; state-relations-audit | Genuine foreign collection accepted; owner identity preserved | Illegal invocation for genuine foreign collection | P1 | Fixed | Same owner-binding mechanism | Imported native prototype mutation still differs |
| DOM-ALL-CONVERT | item/call reused one DOMString conversion for named lookup | html_all_binding_oracle.js conversionPaths | One conversion for index, two for named branch, including changed return value | One conversion in both branches | P1 | Fixed | Classify index, then convert original argument for named lookup | New supplemental discovery, outside fixed denominator |

Private bindings carry a brand and a captured dispatcher with the canonical
cross-realm reference. They reuse existing owner calls and retention checks;
no public property, prototype identity, `instanceof`, or overridable method is
used to authenticate an object. The receiver check and argument conversions
execute in the borrowed method's realm. The operation executes at the receiver's
owner and returns canonical values through the existing bridge. Chrome measures
the returned Performance array and entries as belonging to that owner realm.

Performance coverage here is `now`, `timeOrigin`, `getEntries`, `getEntriesByType`
and `getEntriesByName`. The latter converts name then optional type exactly once,
including when no entries match; its required-argument length is one.
HTMLAllCollection coverage is item, namedItem and length, plus the callable
object's index/named conversion branch. Duplicate-name collections preserve
owner identity and remain live, including through borrowed methods and retained
references after frame removal.

## Fixed sets and controls

| Set | Before | After | Chrome→Chrome |
|---|---:|---:|---:|
| Fixed probes, complete normalized match | 178/236 | 198/236 | 236/236 |
| Original representatives, complete match | 12/28 | 13/28 | 28/28 |
| Official observational groups | 17 | 15 | 0 |
| General corpus | 12/15, 41 records | 12/15, 41 records | 15/15, 0 records |
| Brand matrix differing records | 1319 | 1319 | 0 |
| State-relations differing records | 3 | 2 | 0 |
| Standalone sweep | 113/130 | 112/130 | mode comparison; see below |

The fixed comparison has zero new differing leaves, regressed matching probes
or Chrome drift. The official fuzzer keeps 236 checks, 3533 discoveries and the
100 surface-probe budget: 198 matches, 38 divergent probes, 15 observational
groups, zero unstable/errors. Groups and diff records are not root-cause counts;
overlapping sets are not added. New focused conversion/ownership probes remain
separate from the unchanged fixed set.

The sweep's extra record is Chrome's opportunistic `/favicon.ico` resource
entry. Its Mimic before/after observation is identically empty; Chrome headless
was also empty. Three separate same-source headful controls and both Mimic
builds subsequently returned empty arrays. The original 112/130 result remains
recorded as an environment/timing observation, not silently replaced by 113.
No renderer, network protocol or resource-timing completeness claim follows.

Both new focused Chrome oracles have matching independent controls. The old
HTMLAll borrowed probe terminates at its initial Illegal invocation; later
fields were not executed and are not counted as separately fixed leaves. The
first private control runner stopped on that protocol-projected JavaScript
exception; the completed runner explicitly records it and continues. The
aborted runner is not counted as a pass.

Fresh navigation replay retains exactly two native globalThis-retargeting and
three saved-eval call/apply/bind leaves. Cross-origin reflection, srcdoc nested
ownership and lifecycle remain matched. The two state-relations records left
are live prototype replacement on imported HTMLDDA. Strong bridge caches and
old DOM arena retention remain architectural limitations; the new dispatch
mechanism does not establish weak-reachability-based reclamation.

## Validation

- Full browser suite PASS, 342.362 s. Webapi and CDP PASS; unchanged DOM,
  network, scheduler and engine suites returned cached passes.
- Affected browser race PASS, 43.673 s, including Performance/HTMLAll bindings,
  HTMLDDA, iterator/reference and srcdoc tests. Full browser race not run.
- Full fixed comparison, original representative replay, general corpus,
  brand/state-relations, navigation supplemental, sweep and official fuzzer
  completed; divergence exits remain recorded.
- Complete fast gates PASS on both baseline and changed builds, with a second
  identical pair to investigate timing differences. Every mandatory workload,
  concurrency and memory wave completed; no first-chance dump was produced.
  [All gate receipts](owner-bindings-20260911/gates.json) retain both pairs.
  No native-crash repair is claimed.

The latest paired warm completion medians (ms) are DOM 497.91→534.20,
static 26.65→26.86, React 89.35→84.39. The DOM execution median itself is
468.17→463.99 ms; the longer completion is not a demonstrated DOM-operation
slowdown. Median measured throughput is 73.71→66.41 sessions/s at 10 Pages and
71.98→71.14 at 25. The first pair's 25-Page result was 89.15→69.62, exposing
substantial baseline variation. Post-recovery private memory is static
158.29→162.10 MiB and React 169.94→170.50 MiB in the latest pair.

These monitored samples do not prove performance neutrality: the 10-Page
throughput decrease and static retained-memory increment remain visible costs
to investigate. Two pairs with debugger overhead are insufficient to assign
their cause. No workload was excluded, failed run retried into a pass, or
semantic check weakened to improve the measurements.

Tested binary SHA256:
`462d5254e6b52a8267d16488c9dc25b639c469bdc26094be6216f118dcd80fc5`.
The dedicated Chrome 152.0.7977.82 headful profile is the same session-isolated
profile used for the preceding package, not a new profile for each probe.
All input hashes, observations and build receipts remain in the local
`compatibility/private-captures/cleanup-owner-bindings-20260911/` directory;
reviewable summaries and test logs accompany this report.
