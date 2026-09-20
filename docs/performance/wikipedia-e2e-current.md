# Wikipedia Playwright E2E performance — current state (2026-09-20)

This is the canonical handoff for performance work on
`tools/runtimecheck/playwright_wikipedia_local.js`. Read this file before
changing runtime, DOM, style, geometry, scheduler, CDP, or Playwright-facing
code for this workload.

## Causal Chrome/Mimic result (2026-09-20)

Product-consumption follow-up: the
[partial audit](wikipedia-product-consumption-audit-2026-09-20.md) records cold/warm
production counts and a cursor-materialization counterfactual. It does **not**
establish a semantic dependency closure or an X/Y/Z split of baseline seconds.
Heavy instrumentation perturbed E2E; equal outputs are not proven avoidable work.
No production optimization remains from that audit. Independent source edits
appeared during cleanup, so further comparisons require a fixed source snapshot.

A synchronized Chrome Timeline and temporarily labelled Mimic trace now identify
the primary architectural difference. Playwright isolated-world reads in Mimic
invoke the main-world owner and reconstruct/serialize document-scale
style/visibility state; 24 warm projection misses occupied 3474 ms. Mimic also
executes IntersectionObserver sampling over its JavaScript geometry model: 16
warm deliveries occupied 679 ms and 59 cold deliveries occupied 3281 ms. Chrome
performed 219 native intersection computations in 1.3 ms total and its complete
journey used 273 ms inclusive in `UpdateStyleAndLayout`; post-navigation heading
visibility required no style or layout update. See
[the causal root-cause report](wikipedia-causal-root-cause-2026-09-20.md).

The concrete cause is not missing API coverage or CDP/network latency. Chrome's
realms, rendering lifecycle, observers and input consume one retained,
incrementally invalidated Blink style/layout graph. Mimic repeatedly builds
derived state in an owner realm, transfers document projections to an isolated
realm, and separately rebuilds geometry for observer and input consumers. The
remaining gap includes high-volume Playwright DOM/role traversal over Mimic's
JavaScript wrappers and Go-owned canonical DOM.

## Current direction: controlled whole-chain comparison (2026-09-20)

This section supersedes the historical decisions below. The user requests
diagnosis of the complete Chrome/Mimic observation and input chain, not further
polishing of individually small optimizations. Target remains <=4 s full warm
Wikipedia E2E, ideally Chrome-like latency, with cold approaching warm. No such
result has been established with preserved behavior.

The incoming workspace already contained an uncommitted production candidate.
Its source diff is preserved in
`tools/performance/pocs/chromium-starting-worktree.patch`; the freshly rebuilt
control SHA-256 is
`966091BED8ED5417296F1474CE626C1E67AE53BAC9BC9E4BC6963E96D99CA34D`.
All new experimental source edits were removed; the incoming source diff was
verified byte-for-byte unchanged. Historical statements below that production
sources are clean do not describe this incoming candidate.

Four Chromium-inspired removable PoCs were prepared: lazy CSS declaration
wrappers, observation-scoped sharing of equivalent specified cascades, an early
geometry cache hit before ancestor traversal, and direct stylesheet rule-program
projection with stable identity across unrelated DOM writes. The geometry-only
three-pair screen under concurrent game load was 16794/10142 -> 16600/10008 ms;
this is not a material result. The noisy series must not be compared with quiet
measurements.

After the user closed the game, three alternating pairs of the combined PoC
gave **10071/6062 -> 9632/5756 ms cold/warm** (-4.4%/-5.0%). All unchanged E2E
assertions passed. This combination does not achieve the target and does not
remove the cold/warm gap. Focused CSSOM, flat-tree/lifecycle, geometry recursion
and mutation tests passed. No shipping gate or production acceptance is claimed.
The combined binary is `.build/mimic-chromium-normalized-combined.exe`;
receipts are `.build/chromium-quiet-combined-paired-results.json`.

Exact rule/declaration verification rejected the initial direct-program variant:
the old stylesheet serialization/reparse path drops pending `border-color:var()`
longhands. The reverse shorthand index is built before the border grammar is
registered (`css_shorthands.js`, `css_value_grammar.js`, concatenation in
`surface.go`). Direct AST projection retains those entries and therefore changes
behavior. A separately named normalized variant deliberately retains the old
declaration normalization while eliminating whole-stylesheet serialization and
reparse. Its checked full cold/warm E2E passed exact selector/specificity/pseudo/
declaration comparison. This is parity with current Mimic, not a claim of correct
Chrome shorthand behavior. The serialization bug was not fixed in this task.

Pinned Chrome 152.0.7977.82 ran the unchanged frozen Wikipedia workload headless
in fresh processes at 1507, 1250 and 1395 ms in a quiet repeat. These are fresh
process launches, not warm measurements. Compact receipts are
`tools/performance/pocs/results/chromium-quiet-chrome152.log`.

A larger computed/cascade fixture-tape experiment was drafted but stopped before
build or execution when the user redirected the investigation. Its artifacts are
unvalidated and must not be represented as measured results. The next controlled
matrix compares first and repeated DOM/style/geometry/actionability/input reads,
different first-consumer orders, DOM-size scaling and subsequent mutations in
both browsers. Synthetic totals must not be substituted for Wikipedia E2E.

That matrix is now complete: all 48 cases passed. See
[the controlled whole-chain report](observation-chain-2026-09-20.md).
On 10000 divs, first synchronous rect was 349.3/11.4 ms Mimic/Chrome, but clean
repeats were ~0–0.1 ms in both. An irrelevant data attribute caused Mimic to
re-read 10005 style-observation records and cost 142.5 ms versus ~0 in Chrome.
The whole multi-action chain was 2760/305 ms; changing its first consumer did
not eliminate the work. Actual projection-argument tracing confirmed separate
cursor/content document batches and document-sized caller reconstruction even
when the owner cache hits. A fresh complete Wikipedia diagnostic found scheduler
task unions of 5420/1560 ms cold/warm versus foreground callback sums of
4640/4429 ms (not additive categories). Cold amplification is principally page
task work in this trace. This is diagnosis, not a validated 4 s implementation.

## Current decision: stop diagnosis, plan real fixes (2026-09-19)

The user has stopped diagnostic experiments and requested a production
implementation plan. See [wikipedia-production-plan.md](wikipedia-production-plan.md).
No production implementation is applied. The last combined destructive PoC
reached **2251 / 2147 ms cold / warm**, medians of five paired runs against a
2717 / 2664 ms destructive control. This supersedes the earlier 4608 / 4172 ms
PoC result below, not the production baseline. The 2/2 s target is not achieved.

The final combination additionally fabricates geometry, skips scroll work,
routes input to a recent target without correct occlusion checks, retains stale
attributes/topology and memoizes positive Playwright selector results. Passing
all frozen E2E assertions does not make it browser-compatible. Its timing is
not a proven attainable bound for a semantics-preserving implementation.

| Increment after prior destructive combination | pairs | paired control cold / warm ms | candidate cold / warm ms | conclusion |
| --- | ---: | ---: | ---: | --- |
| Read plans plus unsafe geometry/input hints | 3 | 4564 / 3934 | 4280 / 3733 | Smaller computed reads are actionable; blind hints are not |
| Joint fabricated geometry, scroll bypass and target routing | 3 | 4307 / 3772 | 3090 / 2534 | Joint sensitivity; not a pure layout or duplicate-work attribution |
| Lazy font-size state | 1 | 3046 / 2509 | 3079 / 2508 | Neutral in this synthetic configuration |
| Stale attribute snapshot | 3 | 3048 / 2500 | 3013 / 2452 | Secondary read cost; requires correct mutation visibility |
| Stale parent and owner-document snapshot | 3 | 2984 / 2436 | 2740 / 2234 | Secondary structural-read cost; requires adoption/reparent correctness |
| Positive querySelector/querySelectorAll memoization | 5 | 2717 / 2664 | 2251 / 2147 | Workload-specific destructive shortcut, never ship |

Later control warm timings drifted relative to the topology series. Only compare
within each paired row; do not subtract drift from the best result. The earlier
querySelector-only memo attempt had no actual cache hits because the relevant
path used querySelectorAll, so its flat timing is not a negative causal result.

All temporary production edits were removed after saving
`tools/performance/pocs/wikipedia-joint-upper-bound.patch`; its applicability to
the clean production sources was checked without rerunning diagnostics.
Raw receipts, including hashes, are in `tools/performance/pocs/results`.

## Earlier scope and combination experiments (2026-09-19, evening)

The user changed the task to **removable PoCs only**, prioritizing warm latency,
with an aspirational 2 s cold / 2 s warm target. No production fix is authorized
in this phase. The earlier 20% acceptance threshold is not the finish line.
The following experiments have been removed from production sources and saved
as diagnostic patches. The frozen workload hash remains
`5B340D5C51D1CE7F291C1FB9F635DF7E5A7D29CD12255EE2E09E59918325E444`.

Every row compares fresh processes, cold then warm in each process, with
alternating control/candidate order. Medians below are milliseconds; different
rows have different paired controls and must not be treated as one experiment.

| hypothesis / intervention | pairs | control cold / warm | PoC cold / warm | decision |
| --- | ---: | ---: | ---: | --- |
| Bulk exact growing-prefix line shaping in one owner host call | 3 | 10349 / 6889 | 10019 / 6851 | Warm -0.6%; close direction |
| Author classic-script suppression + no author stylesheet cascade | 3 | 10138 / 6688 | 4821 / 4383 | Cold -52.4%, warm -34.5%; destructive combined bound, not a fix |
| Add split derived-state freeze to the preceding combination | 3 | 4790 / 4376 | 4608 / 4172 | Incremental cold -3.8%, warm -4.7%; little additional leverage in this state |
| Taffy-first leaf sizing with recursive-root guard, otherwise normal workload | 1 | 10441 / 6954 | 10640 / 6704 | Cold +1.9%, warm -3.6%; no material screen win, not validated |

All completed samples in the table passed the frozen E2E assertions. The
unguarded Taffy-first + bulk-lines combination failed at Search fill with
`RangeError: Maximum call stack size exceeded`; no speedup is claimed for it.
The cycle was `append -> size(leaf) -> containingWidth -> layoutWidthFor(parent)
-> observedWidth -> taffyBox -> append`. Initial legacy layout was also a cycle
breaker, not simply redundant work. A per-observation root-construction guard
allowed completion, but did not establish a large E2E gain. This was a limited
static top-root intervention, not a bound on all possible Taffy redesigns.

At this earlier checkpoint the best combined destructive PoC was
**4.608 s cold / 4.172 s warm**, not 2/2.
Cold is within 0.436 s of warm in that modified workload. It removes author
execution and stylesheet effects and freezes invalid derived results: it does
**not** preserve Chrome behavior, despite passing this workload's assertions.
These interventions change downstream demand and thus are not additive CPU
attributions or rigorous upper bounds for a semantics-preserving implementation.
Freeze retains only maps already present at its first capture, not every future
derived object; it cannot disprove all dependency-aware architectures.

Work still moves between stages. In the two-ablation warm run, first-visible
JavaScript falls from median 1462.50 to 720.53 ms, while role resolution plus
`scrollIntoViewIfNeeded` rises from 469.22 to 756.12 ms. The following role-based
innerText/href operations remain around 186/170 ms each. ECMAScript first-visible
falls from 834.91 to 319.93 ms. First construction and repeated structural reads
therefore remain important even after removing author-driven changes.

**Causal conclusion:** the claim that the whole gap is caused by the absence of
a persistent dependency-aware style/layout tree is not established. Previous
split-freeze alone improved warm by only 8.1%; adding it after these ablations
improves warm by 4.7%. The supported model has at least two contributors:
author-triggered deferred work (especially cold), and substantial first-read /
structural observation work that remains without it. A permanent retained tree
is not yet a demonstrated solution to the latter.

Reproduction, unapplied three-switch patch, and durable raw receipts are in
[`tools/performance/pocs`](../../tools/performance/pocs/README.md). Receipts
include binary hashes and every measured stage. The incremental freeze control
is the earlier author-ablation binary, not production. Production files are
clean; the experiment patch applies cleanly to `eac6b5c`. Bulk-shaping focused
browser tests passed, but no PoC is accepted for production. Full frozen/race/
memory shipping gates were not run for these deliberately incompatible PoCs.

## Primary acceptance criterion

The complete unchanged Wikipedia workflow is the primary metric. A faster
microbenchmark, CDP command, locator, geometry read, or synthetic click is
evidence only. Do not expand an optimization unless a fresh matched Wikipedia
E2E run improves materially and correctness remains intact.

Historical production behavior during this investigation was roughly 10 s cold
and 6 s warm, versus roughly 2 s Chrome. Individual runs vary with the local
fixture/controller and process state, so compare fresh before/after binaries
against the same workload and environment.

The latest matched local run around commit `58ca5f6` was:

| build | Wikipedia E2E |
| --- | ---: |
| pre-fix control | 9344 ms |
| `58ca5f6` | 9382 ms |

That is noise/regression, not an E2E speedup. The same change improved one
measured `ECMAScript click + navigation` step from about 1546 ms to 1228 ms,
but it did not move the complete workflow. Do not present that local win as the
solution to the Wikipedia gap.

## Causal attribution follow-up (2026-09-19)

A fresh `eac6b5c` control binary, SHA-256
`D8FBDCF2798DD3730B91E54331CAE093FA404C54CBB254D26C46873AB762A7D5`, was
measured in five clean-process pairs. Each pair ran the unchanged workload once
cold and then once warm in the same process:

| mode | samples (ms, sorted) | median |
| --- | --- | ---: |
| cold | 9630, 9734, 10004, 10626, 10674 | 10004 ms |
| warm | 6422, 6438, 6547, 6562, 6910 | 6547 ms |

`tools/runtimecheck/playwright_wikipedia_profile.js` is a diagnostic companion to
the frozen workload. It brackets every action with `Mimic.getTrace`, records
the commands and correlated call IDs for that stage, and supports
`MIMIC_PROFILE_STOP_AFTER`. Early stop preserves the active realm long enough
for `MIMIC_PROFILE_CF_DEEP=1` and `MIMIC_PROFILE_CF_NATIVE=1` to return the
selected V8 profiles. Its trace traffic makes it unsuitable for latency claims;
only the unchanged local workload supplies the acceptance result.

The warm attributed run identified these representative calls:

| operation | correlated call | profiled callback wall |
| --- | --- | ---: |
| JavaScript first visible heading | `CF#60` | 1635 ms |
| article inspection | `CF#66` | 136 ms |
| ECMAScript first visible heading | `CF#84` | 806 ms |

The two first-visible calls materialized 6584 and 3639 wrappers respectively.
Their measured `nodeData` host bodies accounted for about 185 ms and 95 ms,
while the caller profiles spent most of their interval in native callback /
value-conversion frames below `wrap` and `foreignCSSObservation`.
**Correction:** the previously reported 44 ms / 74 ms owner intervals were
preparatory calls, not the nested style/geometry execution. The foreign bridge
passes `context.Background()` to `RunNested`, losing the outer correlation ID;
packed host callbacks also do not appear in the deep-host event list. Those
profiles therefore did not rule out owner CSS/layout computation.

Direct nested-owner profiling and a separate lightweight `MIMIC_PROFILE_HOSTS=1`
run resolve the missing time. In the latter warm first-visible sample, isolated
`v8:call.fnCall` totaled 1530 ms and its two `foreignComputedStyleFlatTree` calls
1441 ms, versus 52 ms in isolated `nodeData`. Owner host costs included 232 ms
for 8705 `shapeTextMetrics` calls, 98 ms in `nodeData`, and 51 ms in `layoutTaffy`.
These nested intervals overlap; do not add owner and caller totals.

The Playwright source explains the wrapper scan: `_queryCSS` enumerates `*` to
find shadow roots even for `#firstHeading`. Its `computeBox` then reads cursor,
display, visibility and the bounding rectangle. A simultaneously profiled cold
callback attributed 472 ms to owner `documentValues`, 278 ms to owner `rect`,
and 231 ms to DOM-data materialization during the Playwright scan. Native V8
`toString` callback frames are not evidence of expensive JavaScript string
conversion: the same intervals cover Go callbacks and nested owner execution.

The same stage trace found independent work outside those callbacks. The warm
back navigation contained about 429 ms of DOM tasks, 207 ms of navigation work
and 123 ms of network/resource tasks. Click navigation and document startup
likewise had separate navigation and DOM work. Therefore eliminating only the
visible-heading callback cannot close the complete warm gap.

### Destructive upper bounds

Every experiment below used a separate diagnostic binary and was removed after
measurement:

| hypothesis / destructive change | local result | complete E2E result | conclusion |
| --- | --- | --- | --- |
| Avoid isolated-world wrapper creation when only counting `query('*')` results | removed one redundant wrapper path | cold median 10091 ms; warm median 6663 ms | no gain; owner materialization remains |
| Do not eagerly batch visibility with scalar document styles | forced per-element visibility reads | one pair 10636 / 7166 ms | worse; many foreign reads recreate the cost |
| Return a free foreign rectangle | first-visible steps improved locally | one pair 9539 / 6433 ms; warm scroll rose to 1058 ms | moved layout work into scrolling |
| Suppress IntersectionObserver sampling | removed the long sampled DOM tasks | one pair 8925 / 6758 ms | cold-only gain; warm regressed |

The first four experiments did not meet the acceptance threshold. The earlier
conclusion that no independently removable layer could explain the gap was
premature because the nested-owner profile was missing.

A subsequent diagnostic disabled author stylesheet matching/cascade globally
while retaining inline/default declarations. Five alternating, fresh-process
cold/warm pairs ran the unchanged E2E with every assertion passing:

| build | cold samples (ms) | cold median | warm samples (ms) | warm median |
| --- | --- | ---: | --- | ---: |
| control | 10624, 11094, 11446, 11412, 10932 | 11094 | 7491, 6964, 7061, 7273, 7017 | 7061 |
| no author stylesheet cascade | 8286, 8387, 8004, 8196, 7433 | 8196 | 5756, 4949, 6475, 5188, 4827 | 5188 |

Median deltas are **-26.1% cold / -26.5% warm** against this experiment's paired
control, not the earlier baseline. This establishes that the author-CSS path
affects complete E2E, but is **not** a pure matching CPU upper bound: removing
styles changes geometry, fonts, visibility and downstream work. A recorded
matching-result replay experiment separates those effects.
No production fix is retained from this experiment because its semantics change
and later production-shaped candidates did not meet the joint acceptance gate.

### Output-checked selector-matching replay

The second PoC records matching rule ordinals, keyed by document URL, node ID,
pseudo and a rule-program fingerprint, then replays the recorded result without
running the matcher. Ambiguous keys are excluded, and missing keys execute the
original implementation. It leaves stylesheet declarations and geometry intact.
This is deliberately a fixture tape, not a correct general-purpose cache.

A separate cold/warm verification pair recomputed every eligible result instead
of replaying it and found **zero mismatches**. At warm JavaScript first-visible,
coverage was 13471 matching calls hit / 1 miss; at warm ECMAScript first-visible,
5578 hit / 0 miss. Coverage is not universal: one cold ECMAScript stage had no
eligible tape entries. Thus this is a measured intervention, not a proven hard
upper bound on all matching work.

Five alternating fresh-process pairs of the unchanged E2E, all assertions passing:

| build | cold samples (ms) | cold median | warm samples (ms) | warm median |
| --- | --- | ---: | --- | ---: |
| paired control | 10177, 10411, 10697, 10319, 10483 | 10411 | 6820, 6942, 7962, 7077, 7126 | 7077 |
| matching replay | 9210, 8952, 9817, 9302, 9642 | 9302 | 6145, 6221, 6336, 6356, 6155 | 6221 |

The measured savings are **10.7% cold / 12.1% warm**. Warm JavaScript visibility
median fell from 1566 to 1225 ms, ECMAScript visibility from 870 to 640 ms, and
scroll from 524 to 496 ms. Unlike the no-stylesheet experiment, this did not
produce the large scroll regression caused by changing the layout itself.

### Output-checked declaration replay

A third PoC replays the complete `uncachedCSSDeclarations` output, preserving
the declared values rather than deleting styles. Its independent cold/warm
verification also found zero mismatches at eligible keys, with the same warm
visibility coverage as the matching tape. Missing/ambiguous keys still execute
the original computation. Five new paired runs all passed the unchanged E2E:

| build | cold samples (ms) | cold median | warm samples (ms) | warm median |
| --- | --- | ---: | --- | ---: |
| paired control | 10630, 10287, 10449, 10463, 10636 | 10463 | 6961, 7094, 7025, 7047, 7038 | 7038 |
| declaration replay | 9946, 9805, 9658, 9768, 9452 | 9768 | 6382, 6388, 6445, 6314, 6455 | 6388 |

Measured savings are **6.6% cold / 9.2% warm**. This replay is not free: its tape
is ~19.9 MB versus ~4.5 MB for matching and it decodes declaration arrays at
eligible reads, whereas the original code can reuse immutable arrays. Neither
these deltas nor their difference from matching replay are exclusive CPU-time
measurements or hard upper bounds. They establish an E2E effect while preserving
checked outputs, but do not justify attributing the full 26% no-stylesheet gain
to selector/cascade computation. Remaining first-use geometry/text, computed
property resolution, initialization and scheduling costs are not eliminated.

### Stage-complete attribution and follow-up PoCs

The diagnostic companion now has a minimal mode which records stage wall time
without trace or diagnostics traffic. Five warm Mimic Pages in one process were
compared with five fresh launches of installed Chrome 153.0.8010.48. This is an
apples-to-apples stage decomposition, but Chrome 153 is still not the frozen
Chrome 152 acceptance reference:

| stage | Mimic median (ms) | Chrome 153 median (ms) | Mimic excess (ms) |
| --- | ---: | ---: | ---: |
| main navigation and inspection | 662 | 175 | 487 |
| search fill | 340 | 11 | 328 |
| search navigation | 407 | 604 | -197 |
| JavaScript heading visible | 1543 | 14 | 1529 |
| article inspection | 207 | 15 | 192 |
| role link scroll | 508 | 47 | 461 |
| role link innerText | 188 | 20 | 168 |
| role link href | 178 | 19 | 159 |
| click and navigation | 953 | 103 | 849 |
| ECMAScript heading visible | 868 | 24 | 844 |
| back navigation and heading read | 1022 | 171 | 851 |

Mimic/Chrome totals in this diagnostic were 7002/1440 ms. The first visible,
click/navigation, second visible and back stages alone contribute **4073 ms** of
excess wall time. The gap is therefore not an unobserved CDP pause: it is visible
in ordinary Playwright actions that force first-use style/layout, accessibility,
hit testing and document reconstruction.

Splitting the back stage gave Mimic medians of about 723 ms for navigation to
`DOMContentLoaded` and 310 ms for the first heading `innerText`, versus about
117/52 ms in Chrome 153. A realm marker was absent after back in both browsers,
so Chrome's result does not support the hypothesis that its advantage is simply
BFCache realm preservation. Chrome reported navigation type `back_forward` and
Mimic `reload`; that semantic difference is separate from the absent marker.

Host snapshots of each repeated `getByRole('link', {name:'ECMAScript'})`
resolution found roughly 19k `observationVersion`, 21k `getAttribute`, 31k
`parentNode`, 25k `nodeOwnerDocument`, 17k `documentActive` and 2.4k
`isConnected` calls: more than 110k host crossings per operation. CPU profiles
place these calls under Playwright `queryRole`, accessible-name computation and
`isElementHiddenForAria`. A callback-scoped read-snapshot PoC reduced
`observationVersion` to 1--3, `parentNode` to about 7k, `nodeOwnerDocument` to
about 4.3k and `documentActive` to 1. Five unchanged warm E2E runs improved only
6751 -> 6567 ms median (-2.7%). This makes repeated accessibility traversal a
real secondary cost, not the missing four seconds by itself.

The click-only diagnostic measured 738 ms Mimic versus 29 ms Chrome 153. Its
Mimic trace contains about 170--190 ms of role resolution, 160--180 ms of
`elementFromPoint`/hit-target checking and about 350 ms dispatching the three
mouse events. Owner CPU profiles show that `mousemove` executes hover handlers;
the subsequent `mousedown` revalidates the protocol target by rebuilding
geometry/hit-test state. Two deliberately unsafe PoCs trusted (a) the recent
geometry-read hint for hit testing and (b) the protocol input target hint for
mouse dispatch. Together they changed the unchanged E2E click-navigation median
from 924 to 450 ms. Five paired warm runs all passed assertions:

| build | samples (ms) | median | delta |
| --- | --- | ---: | ---: |
| paired control | 7343, 7009, 6876, 6940, 6766 | 6940 | — |
| trust both actionability hints | 6548, 6577, 6335, 6650, 6645 | 6577 | -5.2% |

This is a causal upper bound, not a production fix: blindly trusting a hint can
miss a newly introduced occluder or movement between actionability checking and
dispatch.

An exact cross-realm output tape captured two full consecutive runs and replayed
all 58 observed owner projections (16 `documentValues`, 16 `innerText`, 26
rectangles) with unchanged assertions. One replay pair changed 10637/6749 ms
cold/warm to 8747/5697 ms. Warm JavaScript visibility fell to 104 ms, but scroll
rose to 1627 ms because replay no longer primed owner geometry. This proves
roughly 1--2 seconds of E2E sensitivity while also demonstrating that projection
and geometry construction are coupled and that removing one consumer moves work.

Two further screened hypotheses did not justify expansion. Adaptive per-element
style reads before switching to `documentValues` produced an unstable three-run
warm median change of 6654 -> 6495 ms (-2.4%) and moved work to later stages. A
Page-lifetime intrinsic text-shape cache was flat in three paired warm runs
(7119 -> 7117 ms median). Neither is the missing mechanism.

Additional production-shaped PoCs reinforced that boundary. Sharing one
geometry binding program per realm instead of allocating per-element operation
closures passed the geometry/foreign-realm oracle, but moved paired medians only
9826 -> 9451 ms cold and 6361 -> 6274 ms warm (-3.8% / -1.4%). Removing the
element observation Proxy entirely improved warm median by about 3.3% and did
not move first-visible materially; it also correctly failed the API feature
trace regression. Returning complete node records from one bulk selector host
call was substantially worse (+1.3--1.5 s cold, +0.7--0.8 s warm), disproving
the theory that the missing time is the per-node `nodeData` transport boundary.

A callback-scoped demand projection was then tested: the first distinct element
used a scalar owner projection and a second distinct element switched to the
existing document batch. This cut the two first-visible stages by roughly
0.35--0.55 s combined, but fill/scroll/navigation paid the deferred work and
warm E2E stayed flat (6758 -> 6724 ms in one pair and 6748 -> 6826 ms in the
other). A threshold of eight showed the same displacement. The experiment was
removed. This is direct evidence that first-visible latency is partly movable
projection priming, not independently removable E2E work.

Finally, intrinsic width was given a separate Page-lifetime weak cache keyed by
the exact retained style object, font-collection epoch, direct text, ordered
child identities and recursively validated child widths. Font invalidation and
foreign-realm geometry tests passed after the font epoch was added. Three
alternating pairs nevertheless changed cold median 10300 -> 10681 ms and warm
median 6988 -> 6957 ms. Recursive exact-key validation costs as much as the
intrinsic work it avoids, so this implementation was removed. Efficient reuse
across connected mutations would require canonical subtree dependency epochs;
adding them is the broader incremental-state design already outside a local
cache fix.

The current causal conclusion is therefore **not one four-second stall**. The
dominant mechanism is repeated first-use JS implementation work across newly
created documents: stylesheet matching/cascade, computed-value projection,
text/geometry construction and paint/hit testing. Playwright accessibility and
actionability scans multiply that work. Chrome performs analogous observations
in its native retained style/layout/AX machinery; Mimic repeatedly constructs
them in JavaScript and across realm boundaries. Navigation/bootstrap contributes
an additional independent approximately 0.5--0.7 seconds on main/back document
creation. No tested isolated production-safe layer reached the required 20%
warm and cold E2E threshold.

### Whole-chain attribution and derived-generation upper bound

A later work-conservation pass paired complete scheduler task intervals with
foreground `Runtime.callFunctionOn` intervals. The cold/warm diagnostic runs
were 12851/9135 ms. Scheduler tasks accounted for 6070/2749 ms, a 3321 ms
difference, while foreground Runtime callbacks differed by only 183 ms. CDP
showed the same split: command work differed by 345 ms, but accumulated
`pageWait` differed by 2951 ms. The pump already yields after one task and to a
waiting protocol command, so this is real non-preemptible page work, not unfair
queue draining or transport latency.

DOM tasks contributed 3361/775 ms of scheduler work. Nine cold tasks with the
exact IntersectionObserver geometry signature (`style,link` plus 9--10 table
row scans) totaled 2778 ms; four warm tasks totaled 615 ms. Author script start
intervals themselves were only about 68/38 ms. A parse-only PoC retained script
parsing but skipped execution and changed paired medians 10041/6404 ms to
6419/5804 ms (-36.1%/-9.4%). The cold chain is therefore:

```text
author ResourceLoader execution
  -> mutations, timers and observers
  -> repeated IO style/table geometry tasks
  -> single Page event-loop blocking
  -> Playwright command pageWait
```

The following additional upper-bound experiments all ran the unchanged E2E and
were removed afterward:

| destructive experiment | control cold/warm median | candidate cold/warm median | conclusion |
| --- | ---: | ---: | --- |
| suppress image-completion resource epoch | 9569/6409 ms | 9833/6415 ms | image invalidation is not causal |
| keep only the first realm-level IO sample unless targets change | 10057/6427 ms | 9653/6341 ms | broad epoch resampling is only about 4%/1% |
| sample each observer only once | 9920/6456 ms | 9471/6089 ms | observer-membership fan-out is about 5%, not the root |
| additive word shaping instead of growing-prefix shaping | 9902/6460 ms | 9577/6211 ms | quadratic-looking shaping is only about 3%/4% |
| freeze post-load derived maps but keep a fresh DOM read view | 9743/6418 ms | 7946/5900 ms | coarse derived invalidation is a large cold amplifier, only 8.1% warm |
| matching replay plus frozen derived maps | 9746/6479 ms | 7275/5258 ms | combined upper bound is 25.4% cold but still only 18.8% warm |

The split freeze is materially different from the earlier unsafe whole-cache
freeze: parent/child/attribute/node-state caches were recreated for every DOM
revision, while declarations, computed values, intrinsic sizes, boxes, table
layouts and Taffy results were deliberately retained. It passed every workload
assertion but failed focused invalidation tests as expected. This is strong
evidence that a dependency-aware derived generation would help cold execution,
but it disproves the claim that missing incremental invalidation alone explains
the warm four-second gap. Even combining it with exact fixture matching replay
did not pass the joint 20% acceptance gate.

Pinned Chrome 152 also rules out BFCache as the hidden explanation. Three fresh
back-navigation runs had a 146.93 ms navigation median and 22.73 ms first
`innerText` median, reported `back_forward`, did not retain a realm marker and
did not report persisted `pageshow`. Mimic's equivalent 723/311 ms split is
slower and incorrectly labels the navigation `reload`, but even deleting the
entire 1034 ms Mimic back stage is only a 15% warm upper bound. About 418 ms of
that stage is old ECMAScript IO work queued before the navigation task.

The corrected architectural conclusion is narrower than “there is no shared
tree.” Main-world IO and isolated Playwright projections already enter the same
canonical owner observation, and document-wide scalar projections are shared.
The missing capability is an efficient maintained derived representation:
first construction is expensive, new documents rebuild it, and coarse epochs
discard useful parts. A viable redesign must improve both first construction
and dependency-scoped reuse; an invalidation journal alone cannot meet the gate.

### Interpretation and reproducibility

- The expensive first-visible callback is primarily executing owner CSS/layout,
  not waiting for an idle owner or spending its whole interval materializing
  isolated wrappers. The missing nested correlation caused the earlier error.
- A warm workload creates a **new Page** and navigates through new documents;
  it is not a second read of the previous Page's already-computed DOM/layout.
  First-use projection/layout construction remains substantial. The evidence
  does not establish broad mutation invalidation as the primary cause.
- `documentValues` expands one requested property into per-element values and
  visibility for the document. Geometry then builds formatting contexts and
  measures text. Removing only a later consumer can shift this work, as the
  earlier free-rectangle test demonstrated.
- The complete stage comparison used installed Chrome 153.0.8010.48; bundled
  Playwright Chromium is 151.0.7922.34. The later pinned Chrome 152 measurement
  covered the back-navigation semantic question only, not the complete E2E.

Local evidence is under `.build/`: `foreign-hosts-warm.json`,
`foreign-owner-fullwarm/`, `cascade-paired-results.json`,
`matching-paired-results.json`, `cascadeReplay-paired-results.json`,
`matching-verify-{cold,warm}.json`, `cascade-verify-{cold,warm}.json`, and their
individual workload logs. New stage/actionability evidence is in
`minimal-{mimic,chrome153}-*.json`, `split-back-{mimic,chrome}-*.json`,
`click-only-{mimic,chrome}-*.json`, `click-actionability-warm.json`,
`owner-{style,input}-profiles.jsonl`, `callback-dom-scope4-*`,
`actionability-{control,poc}-*.log` and `foreign-sequence-*`. Binary hashes are
in each paired-results JSON.
`wikipedia-matching-replay-experiment.patch` and
`wikipedia-cascade-replay-experiment.patch` preserve the removable experiment
sources; both pass `git apply --check` against the unchanged production tree.
Do not ship either patch. `MIMIC_PROFILE_EXPRESSION` in the diagnostic companion
optionally records a main-world diagnostic expression after each stage; it is
not enabled in acceptance or timing runs.

That attribution-only checkpoint left production unchanged. The production
tree has since changed as described in "Production causal fixes -- 2026-09-20"
below.
The frozen workload SHA-256 remains
`5B340D5C51D1CE7F291C1FB9F635DF7E5A7D29CD12255EE2E09E59918325E444`.
The diagnostic companion passes Node syntax and Prettier checks; both experiment
patches pass apply checks. Production race, memory and frozen benchmark gates
were not run because this is an attribution experiment, not a production fix.

## Production causal fixes -- 2026-09-20

The follow-up implementation retains four generic mechanisms whose effects were
measured on the complete unchanged workload rather than inferred from a
microbenchmark:

1. ready navigation work overtakes queued work belonging to the document it is
   about to replace;
2. selector reuse is driven by a bounded connected-mutation journal;
3. style/layout projections use a per-active-document connected observation
   revision, so detached fragments, inert documents and template construction
   no longer invalidate the visible document;
4. discovery of the Taffy formatting root is memoized and path-compressed for
   one observation, with custom-element and authored-width boundaries retained.

Cross-root mutation, reparenting, navigation, teardown and journal-overflow
regressions cover the new lifetime boundaries. Arena revision remains the
authority for DOM identity and detached reads; the observation revision is not
a second DOM model.

A fresh three-pair combined run compared the pre-investigation control binary
with a binary built from the combined working tree. Every unchanged Wikipedia
assertion passed:

| build | cold samples (ms) | cold median | warm samples (ms) | warm median |
| --- | --- | ---: | --- | ---: |
| control | 10129, 10075, 9881 | 10075 | 6728, 6635, 6720 | 6720 |
| combined | 9500, 9206, 9792 | 9500 | 5791, 5949, 5813 | 5813 |

The combined median delta is **-575 ms (-5.7%) cold** and **-907 ms (-13.5%)
warm**. This is a real combined checkpoint, not the <=4 s goal. Cold remains
3.69 s slower than warm and the remaining cold differential is still dominated
by author-scheduled DOM/IntersectionObserver tasks. An exact-result postorder
Taffy experiment was tested separately because its avoided legacy prepass was a
measured 520 ms. Although its height and root-origin verification matched
exactly, its first candidate cold run regressed by 805 ms; it is not part of
production unless the complete paired run reverses that result.

Raw combined stages and executable hashes are in
`.build/combined-current-paired-results.json`.

A fresh chain trace of this exact combined source measured 10997 ms cold and
6970 ms warm (diagnostic traffic makes these absolute values unsuitable as a
benchmark). Of the 4027 ms cold/warm difference, mutually exclusive scheduler
intervals explain **3462 ms (86%)**. The source breakdown is DOM +2465 ms,
timer +799 ms, navigation +162 ms and network +31 ms. Eight long cold DOM tasks
with the repeated IntersectionObserver/table-geometry signature total 2440 ms;
the corresponding three warm tasks total 379 ms. Thus IO-layout tasks plus
timer work explain **2860 ms (71%)** of the current diagnostic cold gap. This is
the measured target for the next structural change: preserve canonical layout
products across the connected mutations that do not invalidate their inputs.
Suppressing samples or computing an isolated substitute is already disproved;
it either saves only about 5% or moves the same work into scroll/navigation.

An isolated mutation-complete trace then split the eight heavy cold samples
(1885.3 ms total) by what a correct retained graph could recover:

| class | measured wall | conclusion |
| --- | ---: | --- |
| first build in the relevant document | 1045.9 ms | no prior product exists; retention cannot remove it |
| post-initialization structural/state batches touching html/body/head | 107.7 ms | conservatively global |
| remote later-sibling style/title mutation after the observed heading | 215.0 ms | directly recoverable with target/order-aware invalidation |
| body/class and pseudo-state gaps | 516.7 ms | recoverable only after selector/pseudo dependency proof |

The credible dependency-aware retention ceiling for these samples is therefore
about **731.7 ms (38.8%)**, not 1.9 s. The remaining cold work requires a lower
first-build cost as well as incremental invalidation. Raw isolated receipts are
`.build/cold-io-diagnostic-complete.json` and
`.build/cold-io-diagnostic-targets.json`; production source was untouched by
that instrumentation.

## Established facts

### CDP / scheduler attribution

Expensive `Runtime.callFunctionOn` commands were correlated through
`Debugger.CallFunction`, `runtime.Call`, owner execution and gov8. For the
expensive calls, `ownerWait` was effectively zero and the wall interval was
inside the executed utility callback. CDP serialization, JSON conversion,
persistent/local conversion and value release were negligible in the measured
calls. Earlier cold runs did contain separate Page/queue waits, but those do not
explain the warm E2E gap.

Deep host tracing found thousands of generic DOM callbacks (for example
`nodeData`), but their measured callback time explained only a minority of the
expensive gov8 wall interval. Do not restart from “Go/V8 crossings must be the
whole root cause” without new E2E evidence.

### Full-document computed-style materialization

A first derived-style observation has a large cold/warm cliff. Full-document
`documentValues` can move that work earlier, but disabling it or prewarming it
does not solve the complete workflow. Per-element foreign reads can be worse for
role scans. A Document-owned derived-state rewrite improved settled scalar
probes but regressed Wikipedia from about 10/6 s to about 16/10 s and was
rejected.

The failed rewrite is historical evidence only. Do not restore broad retained
state or shared execution ownership merely because a settled read gets faster.

### 13k-node actionability fixture

A deterministic fixture with one target button and 13,000 unrelated nodes
proved a real document-size-dependent actionability cost. Representative clean
measurements were approximately:

- `locator.isVisible()`: 776–845 ms;
- `locator.boundingBox()`: 18–23 ms after materialization;
- `locator.click()`: 615–678 ms.

Important destructive experiments:

- returning `null` from `getComputedStyle(target)` made Playwright return
  early and reduced visibility dramatically, but this did **not** prove that
  computed-style calculation itself was the cause;
- synthetic/free `display`, `visibility`, `cursor` and
  `checkVisibility()` did not remove the main scaling cost;
- removing scalar `documentValues` / `query('*')` did not remove the main
  scaling cost;
- the first direct `getBoundingClientRect()` could cost roughly 560–600 ms,
  while subsequent reads were near a few milliseconds;
- for a definite positioned target, both `rect(parent)` and `size(parent)`
  could independently force parent/normal-flow work;
- hit testing scanned the whole document and computed geometry/style/paint
  ordering for candidates;
- protocol scroll-to-visible work was also expensive even when no scroll was
  required.

These findings are real local bottlenecks, but the complete Wikipedia run proved
that eliminating them is insufficient by itself.

## Production actionability fix at 58ca5f6

Commit `58ca5f6 perf: reduce actionability layout work` contains generic
production changes, not target-ID or Playwright-specific shortcuts:

1. definite `absolute`/`fixed` positioned geometry avoids unnecessary
   parent normal-flow materialization where the position is independent;
2. recent measured geometry is used as a conservative paint-order lower bound
   for hit testing; possible occluders above the hint are still fully checked;
3. `DOM.scrollIntoViewIfNeeded` now actually sends `ifNeeded: true`;
4. already-visible elements can take a cheap no-op scroll path when no
   intermediate clipping/scroll context requires more work.

On the 13k fixture, a fresh final run was approximately:

| operation | clean | production fix |
| --- | ---: | ---: |
| visible | 845 ms | 476–531 ms |
| box | 23 ms | 22 ms |
| click | 671 ms | 184–196 ms |

Targeted browser regressions passed, including an occluder above a measured
target and protocol no-op scrolling. Wikipedia also passed, but total E2E did
not improve.

## Experiments that must not be repeated blindly

- Broad Document-owned derived-state architecture: rejected by real E2E
  (approximately 10/6 -> 16/10 s).
- Sharing V8 execution owners across lifecycle boundaries: caused navigation /
  teardown problems and did not establish an E2E win.
- Treating `documentValues` as the standalone root cause.
- Treating `query('*')` alone as the root cause.
- Optimizing only `getAttribute`, `nodeData`, JSON/CDP serialization,
  ownerWait, or gov8 wrapper conversion based on aggregate counts.
- NOPing `checkVisibility`, target computed-style properties, NodeList
  wrappers, parent/children wrappers, or local attribute reads and claiming a
  root cause from a small local delta.
- Persisting the existing geometry caches more aggressively without changing
  the work required to build the initial state; the tested version did not
  improve the target workload.
- Moving the same CPU work into navigation/prewarm and calling it a speedup.
- Rebuilding the current style/geometry semantics as a native matcher plus a
  shared Flow/Taffy record graph. The complete experiment removed fallback DOM
  walks and duplicate traversals but regressed five-pair medians by 9.8% cold
  and 4.8% warm. See
  [the full producer replacement no-go](wikipedia-full-producer-replacement-no-go-2026-09-20.md).

## Rules for the next E2E investigation

1. Build a fresh control binary and a fresh candidate binary from explicit
   commits. Kill unused `mimic-*` processes before each matched run.
2. Keep `tools/runtimecheck/playwright_wikipedia_local.js` unchanged.
3. Use the full Wikipedia E2E as the go/no-go metric from the beginning, not
   only after hours of microbenchmark work.
4. A synthetic/destructive PoC is useful for causal localization. Once it shows
   a large local delta, immediately test whether the corresponding generic
   candidate changes Wikipedia E2E.
5. If Wikipedia does not move materially, record the negative result and leave
   that area. Do not spend hours polishing a local optimization as the E2E
   solution.
6. Separate cold-specific queue/navigation costs from warm runtime costs.
7. Preserve navigation, cross-realm ownership, teardown, memory and observable
   browser semantics.
8. Prefer a table of hypothesis -> destructive experiment -> local delta -> E2E
   delta -> conclusion. “Looks expensive” is not attribution.

## Reproduction

Build Mimic:

```powershell
go build -o .build/mimic-e2e.exe ./cmd/mimic
.\.build\mimic-e2e.exe -listen 127.0.0.1:9222
```

Run the unchanged local workload in another process:

```powershell
$env:PW_MIMIC_ENDPOINT='http://127.0.0.1:9222'
node tools/runtimecheck/playwright_wikipedia_local.js
```

For attributed diagnostics only, start Mimic with `MIMIC_PROFILE_CDP=1`, set
`MIMIC_PROFILE_OUTPUT` and run
`node tools/runtimecheck/playwright_wikipedia_profile.js`. Set
`MIMIC_PROFILE_STOP_AFTER` to a printed stage name when profiling a call that
would otherwise belong to a retired navigation realm.

For Chrome, use the existing Chrome branch of the same script and the pinned
Chrome executable. Match headful/headless and viewport conditions before making
fine-grained operation comparisons.

## Historical material

This file supersedes the dated Wikipedia-specific handoffs from 2026-09-15 and
2026-09-18. Git history retains their detailed intermediate evidence. General
performance architecture and benchmark history remain in
`docs/performance/report.md` and the other subsystem-specific reports.
