# Detached browsing-context lifetime

Baseline 9d6de91, following [Attr/Node bindings](attr-node-2026-09-11.md).

| ID | Root cause | Repro | Chrome | Mimic before | Priority | Status | Fix | Limits |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| FRAME-DETACH-LIFETIME | Detached frame lookup was traced as ownership; descendant Page entries survived detach | TestUnobservedDetachedFramesAreReleased | No JS reference requires these contexts | Eight unobserved removals retain nine realms and eight lookup entries, zero exported Window references | P1 memory | Corrected | Remove descendant active-tree entries; trace exported references and prune unreachable lookup entries | Strong exported bridge caches and DOM arena collection remain bounded separately |
| FRAME-INACTIVE-RELATIONS | Inactive contexts still exposed active parent/top/frameElement and child browsing contexts | detached_frame_oracle.js | Null relations and no child contentWindow | Stale relations/contexts | P1 semantic | Corrected | Project inactive state for own/borrowed window relations and child creation | Native global-proxy retargeting and saved-eval call/apply/bind limits remain |

Focused final normalized leaves improve **six to zero**, with zero Chrome control
differences. The initial five-leaf probe lacked own-realm eval; the expanded probe
is recorded separately, not summed. The retained Window remains usable after its
ancestor is removed; this is not simulated by dropping all exported references.
Targeted tests cover retained descendants through navigation and removal and the
original owner-retention checks. The direct logical regression fails on baseline
(owners=9, retained=8, windowReferences=0) and passes after the change.

Original fixed corpus stays 205/236 and representatives 19/28, with no regressions,
new differing leaves or Chrome drift. Brand/general/relations stay 558/40/2 with
no added, removed or changed records. Full unchanged Chrome controls, fuzzer
discovery and sweep are not repeated. [Evidence](detached-frame-lifetime-20260911/)
records source/build identity, profile metadata, observations and transitions.

An initial exploratory expression accessed a null Chrome parent and aborted;
only the completed null-safe probe is counted. The isolated Window descriptor
unit fixture also initially lacked the newly required documentActive host callback.
It now supplies that callback and additionally asserts null inactive relations;
all prior descriptor/assignment assertions remain. The initial full-run failure
is preserved, and the complete WebAPI race rerun passes (2.598 s).

All browser probes use loopback fixtures and frozen Chrome 152.0.7977.82 headful
with the same controlled, reused profile. No target site ran. No snapshot disabling,
global runtime lock, retry, or forced collection is introduced.
P0 remains separately documented in [serialization](snapshot-serialization-2026-09-11.md)
and [historical map-read analysis](snapshot-sizefrommap-2026-09-11.md).

Full browser suite PASS (473.680 s output / 480.635 s completion); engine V8/Goja,
CDP, network and scheduler pass (unchanged engines eligible for cache). Initial
WebAPI fixture failure and its complete race correction are reported above.
Targeted non-race retention tests pass (7.371 s), focused snapshot checks pass
(2.116 s). The expanded browser race FAILS solely at the existing Goja
TestIframeSrcdocRetainsExportedDescendant setup deadline (21.88 s Goja subtest,
46.160 s package). It reports no data race. This is not considered a passing race
run or a resolved timeout. New lifetime/relation regressions also pass in a
separate focused race run; full browser race remains outstanding.

Both complete monitored fast gates pass, no native dumps and no retry/excluded
workloads. Throughput 10/25 Pages is 51.50/87.00 to 72.14/72.35 sessions/s.
DOM execution/completion 447.63/470.09 to 460.33/501.25 ms; React 60.15/94.29
to 48.95/87.48 ms. Static/React private memory after recovery is 162.79/169.36
to 161.05/171.82 MiB. These mixed measurements leave the broader performance
concern open; they do not establish neutrality or close historical native faults.

The separate identical 32-removal memory workload shows baseline realms growing
1/9/17/25/33 versus a constant one after each batch. Detached lookup entries grow
to 32 before and stay zero after. Private memory at removal 32 improves 1051.06
to 287.50 MiB; after Page.Close and 250 ms it improves 533.83 to 250.56 MiB.
Both measurements pass, using ordinary reclamation with no forced GC/scavenging.
Context snapshot caches and allocator high-water state remain in this diagnostic;
these values are not live-object counts. Raw records include Go heap, RSS,
private bytes and goroutines. This addresses unobserved detached contexts, not
all strong bridge-cache retention or incremental DOM arena reclamation.
