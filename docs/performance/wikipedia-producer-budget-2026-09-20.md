# Wikipedia producer budget (Part A, 2026-09-20)

## Question

Before building a causal replay, measure how much of the unchanged Wikipedia
E2E is actually spent producing canonical style and geometry products. The
accounting must be exclusive: nested style, intrinsic, size, placement and
Taffy intervals must not be added twice.

This is attribution only. No producer output is skipped or replayed.

## Instrumentation

`MIMIC_PROFILE_PRODUCER=1` enables diagnostic instrumentation in the canonical
main-world owner only. Each outer `withStyleReadCache` observation owns a small
stack profiler. Nested categories charge their elapsed time to the child and
subtract it from the parent, so category totals are mutually exclusive.

The measured categories are:

- `matching-cascade`: an uncached declaration-array build;
- `computed-value`: an uncached computed-value resolution;
- `style-context`: an uncached geometry style-context build;
- `intrinsic`: an uncached intrinsic-width build;
- `size-flow`: an uncached box-size/flow build;
- `placement`: an uncached document-coordinate rectangle build;
- `taffy-native-bridge`: request serialization, Go/Rust Taffy execution and
  response decoding inside the existing host call;
- `unattributed`: outer-observation time not enclosed by one of those scopes.

The foreign bridge separately records projection kind, hit/miss and wall time.
Foreign projection time overlaps owner producer time and is never added to it.
The Playwright profile harness assigns completed events to its current stage.

Instrumentation is environment-gated. Empty isolated-world observations are
not timed or emitted. Cache-hit geometry helpers are not timed.

## Observer effect

All runs used the same freshly built binary and split-stage profile harness.
The only server difference was `MIMIC_PROFILE_PRODUCER`.

| mode | control samples | instrumented samples | added wall |
| --- | --- | --- | --- |
| cold | 10.714 s, 10.858 s | 11.574 s, 11.763 s | 0.72-1.05 s |
| warm | 6.702 s, 6.737 s | 6.967 s, 7.038 s | 0.23-0.34 s |

The instrumented wall values are therefore not acceptance numbers. The stable
shape and exclusive category totals are the result. Exact low-millisecond
subcategories should not be interpreted more finely than the observer effect.

## Whole-E2E owner producer budget

The two fresh-process samples agree closely:

| category | cold samples | warm samples |
| --- | ---: | ---: |
| matching/cascade | 2.826 s, 2.734 s | 1.736 s, 1.706 s |
| size/flow | 1.600 s, 1.545 s | 0.831 s, 0.826 s |
| style context | 0.557 s, 0.530 s | 0.186 s, 0.187 s |
| intrinsic width | 0.479 s, 0.473 s | 0.290 s, 0.294 s |
| computed value | 0.426 s, 0.432 s | 0.248 s, 0.243 s |
| Taffy native bridge | 0.334 s, 0.342 s | 0.134 s, 0.135 s |
| placement | 0.098 s, 0.095 s | 0.054 s, 0.073 s |
| unattributed inside owner observations | 0.797 s, 0.827 s | 0.464 s, 0.444 s |
| **owner producer total** | **7.117 s, 6.977 s** | **3.944 s, 3.907 s** |

The warm result is the most decision-relevant: about **3.93 s** of a 6.97-7.04
s instrumented E2E is inside canonical owner observations. About **3.47 s** is
inside named producer categories, and about **0.45 s** is still unclassified
inside those observations. The corresponding foreign projection misses occupy
3.09-3.12 s, but that is an overlapping view of the same work.

Warm owner observations by requested consumer are also stable:

| consumer | owner producer wall (representative run) |
| --- | ---: |
| document-wide computed values/visibility | 1.784 s |
| rectangles | 0.921 s |
| local main-world consumers, chiefly observer/input lifecycle | 0.939 s |
| inner text | 0.300 s |

This distinction matters: eliminating cross-realm serialization alone cannot
remove the local 0.94 s, and replaying only the scalar foreign return cannot
satisfy later geometry consumers.

## Warm stage shape

Two-run averages are rounded to milliseconds. Producer time is owner work that
completed while the stage was being observed; background lifecycle work can be
scheduled behind a foreground getter. Projection is an overlapping bridge-wall
view and must not be added to producer.

| stage | stage wall | owner producer | matching/cascade | size/flow | other named | owner unattributed | foreign projection |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| main navigation | 594 | 390 | 287 | 0 | 56 | 47 | 391 |
| search fill | 309 | 282 | 18 | 125 | 135 | 4 | 196 |
| search navigation | 312 | 83 | 17 | 32 | 33 | 1 | 82 |
| JavaScript heading visible | 1,418 | 1,310 | 535 | 324 | 286 | 165 | 1,311 |
| ECMAScript role count | 312 | 80 | 0 | 0 | 42 | 38 | 80 |
| ECMAScript scroll | 352 | 76 | 0 | 16 | 25 | 35 | approximately 0 |
| ECMAScript click/navigation | 890 | 482 | 64 | 169 | 162 | 88 | approximately 1 |
| ECMAScript heading visible | 800 | 643 | 365 | 101 | 120 | 57 | 643 |
| back navigation | 846 | 497 | 353 | 62 | 63 | 19 | 200 |

Small inner-text stages often consume a retained product and contain little or
no new owner production. Their foreign bridge time is mostly serialization or
other uninstrumented owner work. Cold `article paragraph visible` and later
`ECMAScript innerText` do rebuild substantial state; warm retention removes
those particular builds. This is exactly why a per-stage local optimization is
not a reliable E2E claim.

## Decision

Part A passes the budget gate for a causal replay.

- The covered warm producer budget is multiple seconds, not a 5-10% tail.
- The shape is repeatable across fresh processes.
- The budget spans both foreign and local lifecycle consumers, so the next
  experiment must publish the completed internal products into the existing
  owner observation stores. Replaying only returned projections would test the
  already-disproved transfer path again.
- Matching/cascade plus size/flow dominate. Taffy native execution and final
  placement are not large enough to be standalone explanations.
- The replay should be judged by how much of this approximately 3.9 s warm
  owner budget disappears and whether the unchanged E2E improves by multiple
  seconds without later scroll/click/observer debt. There is no symmetric
  30-percent cold/warm cutoff.

The next step is therefore Part B, but it should start with the minimum product
set that covers declaration/computed/style context, intrinsic/size records and
rectangles for one captured canonical revision. It must leave the existing
owner, invalidation epochs and consumers in place.

