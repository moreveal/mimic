# Compact canonical style/geometry vertical — 2026-09-20

## Question and implemented vertical

This experiment tested whether the Part B upper bound could be approached by
stopping the production and publication of required style/geometry state as
large JavaScript object/Map graphs. It covered computed style, Flow/Taffy
geometry, visibility, IntersectionObserver, scrolling, hit testing and input.

The isolated candidate used a Document-owned Go `CompactStyleArena`, dense
canonical node IDs, interned property/string/set columns, separate inheritance
and sparse overlays, and one `CST1` binary snapshot per observation. JavaScript
decoded typed views locally. Main-world computed style and all geometry/action
consumers read the same authoritative snapshot. A safe rule filter retained
only declarations intersecting the required closure while preserving `all`,
original rule indices and dynamic selector fallback.

This was a vertical compact-representation/shared-publication test, not a
demand-only getter shortcut. The code lived in `.build/compact-producer` and
was not merged after the E2E gate rejected it.

## Correctness

The candidate passed:

```powershell
go test ./internal/engine ./internal/dom -count=1
go test ./internal/browser -run "Test(ComputedStyleFlatTreeMatchesFrozenChrome|CSSFlowRootFlexItemMeasuresNestedInlineContent|CSSDefaultIframeReplacedGeometryAndHitTesting|GeometryReadCachesObserveSameTaskMutations|StyleRuleIndexRevalidatesCanonicalState|SelectorMatchCacheUsesCanonicalMutationJournal|Input|Hit|Intersection|Visibility|Scroll)" -count=1
git diff --check
```

The unchanged Wikipedia workload also completed successfully.

## Causal counters

Final instrumented unchanged Wikipedia run:

| counter | result |
| --- | ---: |
| snapshot crossings | 17 |
| compact native records consumed | 58,453 |
| legacy/hard fallback records | 0 |
| aggregate input / retained rules | 68,863 / 47,655 |
| rules removed by closure filter | about 31% |
| arena events | 17 |
| first builds / rebuilds / reuses | 6 / 10 / 1 |
| arena record builds | 90,275 |
| arena build wall | 1,564.6 ms |

The transport shape changed materially: approximately 113,455 host crossings
became 17 and all migrated consumers had zero legacy/hard fallback. Filtering
reduced the preceding candidate's warm penalty by about 367 ms.

## Full-E2E gate

One fresh-process screening pair rejected expansion to five pairs:

| mode | clean control | compact candidate | delta |
| --- | ---: | ---: | ---: |
| cold | 8,843 ms | 11,005 ms | +2,162 ms (+24.4%) |
| warm | 5,517 ms | 6,494 ms | +977 ms (+17.7%) |

The earlier unfiltered screening was also negative (11,381/13,534 ms total),
and the computed-style-complete candidate had about a 1.34 s warm penalty
before filtering. The direction was stable and too far from GO to justify five
repetitions.

## Interpretation

Compact publication is viable: one snapshot served the whole consumer chain
without per-node crossings, JavaScript `Map` construction or downstream
reconstruction, while preserving the tested semantics. But compact storage did
not make production cheap. Real structural/selector-relevant mutations caused
six first builds and ten complete rebuilds. Producing 90k records and repeating
matching/cascade/inheritance for the whole arena cost 1.565 s.

Part B supplied already-produced intermediate state and removed representation
cost **and** producer work. This vertical removed the former but repeatedly
performed the latter, so it could not reproduce Part B's `-1.72 s` warm E2E.

This does not justify more getter, selector-leaf or transport tuning: those are
no longer the measured bottleneck. It also does not reject compact canonical
storage as a component. It rejects **whole-arena rebuild plus compact
publication** as the complete mechanism.

## Required next architectural step

Continuation requires changing the production unit from a whole Document arena
to an invalidation-derived delta: retain unchanged records; derive affected
nodes from the canonical mutation journal; expand through selector,
inheritance/custom-property and layout dependency edges; rebuild affected
subtrees/formatting contexts; publish changed ranges into the same arena; and
instrument late expansion so scroll/click/IO/input cannot silently pay deferred
work. The unchanged cold/warm Wikipedia E2E remains the gate.

Raw artifacts remain in the rejected isolated worktree:

- `.build/compact-producer/.build/compact-style-filter-profile.json`
- `.build/compact-producer/.build/mimic-compact-style.exe`
- `.build/compact-producer/.build/mimic-compact-clean.exe`
