# Location ownership and cross-realm property mutation

This package follows [owner bindings](owner-bindings-2026-09-11.md), baseline
`87805927ebc578be7518c28ef07d81cb24c95382`, on the same isolated cleanup branch.
Historical reports are not rewritten. Only local fixtures and retained repros
were used; no target site was launched.

## Findings and scope

| ID | Root cause or hypothesis | Repro | Chrome | Mimic before | Priority | Status | Fix | Limitations |
|---|---|---|---|---|---|---|---|---|
| LR01 | Handwritten Location installation loses unforgeable descriptors and owner binding | `location_reflection_oracle.js` | Own nonconfigurable accessors and readonly methods; branded receiver | Prototype methods, writable methods, permissive getters | P1 | Confirmed semantic defect, fixed | Owner binding dispatch and Location descriptor installation | Does not establish every navigation URL edge case |
| LR02 | ancestorOrigins has no semantic implementation | `ancestor_origins_oracle.js` | Realm-owned DOMStringList with indexed legacy behavior | Probe aborts on missing list | P2 | Incomplete implementation, implemented | Snapshot ancestor origin values and canonical branded list | No general browsing-context sandbox/opaque-origin completeness claim |
| LR03 | Window.origin is absent | `ancestor_origins_oracle.js` | Serialized origin; branded replaceable getter | undefined | P2 | Incomplete implementation, implemented | Authoritative Realm origin and Window receiver validation | Origin follows existing Realm security state |
| LR04 | Imported object reflection refuses owner mutations | `realm_property_mutation_oracle.js` | Define/delete affect owner; original exceptions and identities survive | Explicit unsupported exception | P1 | Incomplete implementation, implemented | Owner Reflect operations with descriptor and exception transport | General preventExtensions and setPrototypeOf remain unsupported; imported HTMLDDA prototype replacement remains limited |
| LR05 | goja global observation proxy drops accessor receiver | `TestGlobalAccessObserverPreservesAccessorReceiverAndException` | Accessor receives actual receiver | Underlying target passed to getter | P1 | Confirmed semantic defect, fixed | Captured Reflect.get forwards receiver and original exception | This is an engine observation fix, not a relaxed Window brand check |

The DOMStringList indexed set/define operations report success without changing
the indexed values in the measured Chrome behavior; deleting an existing index
and preventing extensions report false. Named expandos retain ordinary object
behavior. The list stores strings, not references to ancestor DOM arenas. Retained
lists survive child navigation and detach; snapshot restoration clears the
seed's lazy list cache.

Cross-realm descriptors travel through the existing private reference bridge.
The owner remains authoritative, including partial descriptors and thrown
objects. The importing proxy materializes descriptors only when required by its
native invariants. Iterator caches invalidate on these mutations. This does not
establish weak collection of the strong bridge caches or old DOM arena nodes.

The earlier srcdoc oracle compared child.origin with parent.origin without
checking their types. Both could be undefined. Its passing equality did not
prove origin serialization. The new oracle explicitly checks type, URL origin,
descriptor, forged receiver, borrowed getter and replaceable setter; the old
srcdoc lifecycle evidence remains useful only for the observations it actually
established.

## Fixed comparison

| Independent set | Before | After | Chrome control |
|---|---:|---:|---:|
| Original fixed probes | 198/236 | 202/236 | 236/236 |
| Original representatives | 13/28 | 17/28 | 28/28 |
| Official observational groups | 15 | 11 | Not a root-cause count |
| General corpus | 12/15; 41 diff records | 12/15; 41 diff records | No differences |
| Brand matrix | 1319 diff records | 1319 diff records | No differences |
| State relations | 2 diff records | 2 diff records | No differences |
| Sweep | 112/130 | 113/130 | Headful/headless sensitivity recorded separately |

The four newly matching fixed probes are surface.location,
descriptor.location.ancestorOrigins, property.location.ancestorOrigins and
descriptor.location.assign. There are no new differing leaves, regressions of
previously matching probes or Chrome drift. All normalized leaves are compared;
overlapping sets and leaves are not added into an error count.

Official discovery remains 3533 probes with a 100 surface-probe limit:
236 checks, 202 matches, 34 divergent probes, 11 observational groups, zero
unstable/errors. New focused probes are separate. All three have independent
matching Chrome controls. Baseline ancestorOrigins and property-mutation probes
abort; their later fields were not executed and are not separately counted as
fixed leaves.

The sweep increase is Chrome's opportunistic favicon entry disappearing; Mimic
returned the same empty resource list in both runs. The earlier intermediate
112/130 run remains recorded. This is not an additional semantic fix.

Navigation replay still has two native globalThis-retargeting and three
saved-eval call/apply/bind differences. Reflection, nested srcdoc and lifecycle
match. These architectural boundaries and native snapshot instability remain
open.

## Validation

- Full browser suite PASS, 415.113 s (test driver elapsed 422.093 s); related
  webapi/CDP/DOM/network/scheduler and engine packages PASS. The engine contract package has no
  tests. Full JSON events retain the opt-in diagnostic skips, five native
  lifecycle stress skips and the goja native-microtask boundary skip.
- Targeted race PASS: browser 61.094 s, goja 1.572 s. It includes reflection,
  owner references/iteration and the new Location/ancestor/mutation oracles.
  Full browser race was not run.
- Final fixed comparison, representatives, general/brand/state corpus,
  navigation replay, focused controls, sweep and official fuzzer completed.
  [Results and skips](location-owner-reflection-20260911/results.json) retain
  the independent sets and all differing leaves.
- First full suite/race failed at the goja receiver. The next full suite
  exposed an old test requiring define/delete to remain unsupported. That
  implementation-boundary test now checks actual owner mutations while still
  requiring unsupported preventExtensions/setPrototypeOf to fail. A focused
  rerun then caught goja's Proxy descriptor-presence problem; the owner now
  materializes only present descriptor fields. One intermediate missing
  Object.create capture was corrected before final validation. All failures
  remain recorded; no frozen Chrome expectation was weakened.
- The successful full suite still emits the existing circular-JSON diagnostic
  on stderr. It is preserved in the log, not silently removed.
- Baseline full fast gate FAILED at the first measured 10-Page static wave
  after a valid warmup: native access violation, Mimic exit 2, gate exit 1.
  First-chance capture retained a 389,882,223-byte dump and native stderr.
  Changed full gate PASS with every mandatory workload, 10/25-Page concurrency
  and teardown-memory wave; no changed-run native exception was captured.
  This is not a passing pair or evidence that P0 is repaired. Failed baseline
  concurrency and unexecuted baseline memory coverage remain unavailable.

Warm completion medians (ms) before/after are DOM 491.32/489.90, static
26.89/26.94 and React 94.65/86.41. Changed-run measured throughput medians are
66.08 and 86.64 sessions/s at 10 and 25 Pages. Changed post-recovery private
memory is 166.94 MiB static and 172.37 MiB React. The interrupted baseline
cannot establish performance neutrality or resolve the previous package's
throughput/retained-memory concerns. No workload was excluded or failed run
retried into a pass. [Gate receipts](location-owner-reflection-20260911/gates.json).

Evidence root: `compatibility/private-captures/cleanup-location-20260911/`.
The final tested binary SHA256 is
`cc97570c06cf010f6525cd41c946a935f603bfab1d71061105eb1403724ee88d`.
Its source dirty state is recorded in `validated-identity.json`; the Chrome headful session uses frozen
152.0.7977.82 and the dedicated cleanup-bindings profile. Raw protocol data and
native dumps remain ignored local evidence.
