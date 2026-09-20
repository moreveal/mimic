# Wikipedia canonical-product replay (Part B, 2026-09-20)

## Purpose

Test whether replacing the owner style/layout producer can remove multiple
seconds from the complete unchanged Wikipedia E2E without merely moving work
from visibility into scrolling, input, hit testing or observer delivery.

This is a causal upper-bound experiment, not a production cache. It replays
products captured from the exact deterministic workload and intentionally
trusts the capture when its canonical identity key matches.

## Slice

The tape is demand-driven and reuses the existing owner observation stores. It
does not add a second DOM, style or layout model.

Captured products:

- resolved declaration arrays;
- computed scalar values;
- intrinsic widths;
- completed size/flow records, including child-position maps;
- final rectangles, widths, width boxes and height boxes.

Keys contain URL, canonical node ID, product/input, document mutation revision,
selector-target state, viewport/media environment, constructed stylesheet
revision and element-state revision. Repeated keys with different encoded
products are permanently marked ambiguous and fall back to normal production.
Provisional `computing` records are never captured.

On replay, decoded size records are published into the existing
`styleReadCache.boxSizes`; declarations and computed values enter their existing
maps. Geometry boundary products are returned from the existing `cssBoxModel`
boundary. IntersectionObserver, scrolling, hit testing, input, Playwright role
traversal, author scripts, navigation and mutations remain enabled.

## Capture cost and payload

Capture used one unchanged cold+warm E2E in the same server process. Recording
is intentionally slow because every new product crosses to the diagnostic
writer; record latency is not a candidate result.

The valid tape contains 418,048 JSONL records and is 121,951,630 bytes. Replay
loads the file once and transfers the current URL's rows into each owner realm.
All hydration and decoding cost remains inside the reported E2E.

## Full unchanged E2E

Three alternating fresh-process pairs used the same binary. Control had no tape
environment. Candidate used `MIMIC_PRODUCT_TAPE_MODE=replay` with the valid
tape. Every workload assertion passed.

| mode | control samples | replay samples | control median | replay median | delta |
| --- | --- | --- | ---: | ---: | ---: |
| cold | 10.281, 9.937, 9.939 s | 8.977, 9.289, 8.832 s | 9.939 s | 8.977 s | **-0.962 s / -9.7%** |
| warm | 6.422, 6.358, 6.341 s | 4.638, 5.290, 4.589 s | 6.358 s | 4.638 s | **-1.720 s / -27.1%** |

The warm result is the required multiple-second-scale mechanism signal even
though the median reduction is 1.72 s: the producer profile below shows that
2.65 s of named producer work was removed, with about 0.54 s returning as tape
lookup/decode/hydration overhead. The 5.290 s candidate sample shows remaining
variance; this is screening evidence, not a shipping benchmark.

## No displacement in warm stages

Stage medians come from the same three pairs. Stages not shown were effectively
flat or small.

| unchanged stage | control warm | replay warm | delta |
| --- | ---: | ---: | ---: |
| search fill | 322 ms | 35 ms | -287 ms |
| search navigation | 414 ms | 309 ms | -105 ms |
| JavaScript heading visible | 1,410 ms | 874 ms | -536 ms |
| ECMAScript scroll | 421 ms | 374 ms | -47 ms |
| ECMAScript innerText | 195 ms | 202 ms | +7 ms |
| ECMAScript href | 190 ms | 193 ms | +3 ms |
| ECMAScript click/navigation | 927 ms | 591 ms | -336 ms |
| ECMAScript heading visible | 826 ms | 318 ms | -508 ms |

Visibility did not become faster by pushing work into scroll or click. Both
later consumers also became faster, while scalar innerText/href remained flat.
The full warm wall reduction agrees with that shape.

Cold has a different shape. JavaScript heading visibility improves by about
1.02 s and the later ECMAScript heading by about 0.52 s, but click/navigation
regresses by about 0.71 s and search navigation by about 0.28 s. The first realm
for each URL pays large eager tape hydration. Cold still improves by 0.96 s,
but the payload format hides much of the producer upper bound.

## Producer budget destroyed

Part A measured 3.91-3.94 s of warm owner observations, including about 3.47 s
in named producer categories. Running the same exclusive profiler with replay
gave:

| warm owner budget | control | replay | removed |
| --- | ---: | ---: | ---: |
| complete owner observations | about 3.93 s | 1.81 s | **about 2.11 s** |
| named producer categories | about 3.47 s | 0.82 s | **about 2.65 s** |
| owner-observation unattributed | about 0.45 s | 0.99 s | **+0.54 s** |

Named replay-time residue was approximately:

| category | replay warm |
| --- | ---: |
| matching/cascade | 410 ms |
| style context | 144 ms |
| size/flow | 135 ms |
| computed value | 73 ms |
| intrinsic width | 38 ms |
| Taffy native bridge | 20 ms |
| placement | 1 ms |

The experiment therefore destroys a substantial majority of the covered
producer work. It does not merely rename or reschedule it. The difference
between 2.65 s of named work removed and 1.72 s median E2E improvement is mostly
the deliberately crude tape representation plus remaining non-producer Page
work.

## Rejected key reductions

Two attempts to reduce the 122 MB tape were rejected:

1. `(URL, node, product/input)` reduced the tape to 29 MB but made the heading
   permanently hidden and caused an unrelated table row to intercept the
   ECMAScript click.
2. Adding selector-target/environment/element-state while omitting document
   mutation revision produced a 53 MB tape and failed with the same visibility
   and hit-test errors.

Consequently document mutation identity is required for this replay boundary.
Product equality observed later in the capture cannot by itself prove that an
earlier state is valid. A production implementation needs dependency-aware
invalidation or retained canonical graph nodes; it cannot use a node/property
memo independent of the mutation graph.

## Conclusion

Part B confirms the producer-replacement hypothesis for warm browser automation:

- the covered producer contains enough removable work to materially change the
  full E2E;
- saved work survives visibility, scrolling, hit testing and click;
- existing owner observations and consumers are suitable integration points;
- the current algorithms and repeated reconstruction, rather than realm
  transport alone, create the dominant removable cost.

It does not justify shipping the tape or a parallel snapshot model. The natural
production direction is a retained canonical derived-state graph owned by the
document/owner and consumed by the existing observation APIs. It should retain
declaration/computed/style nodes and completed geometry products, with explicit
dependency invalidation driven by the existing mutation journal, resource and
environment epochs. The 122 MB tape and failed reduced keys show that compact
dependency identity—not serialization—is the central design problem.

