# Wikipedia E2E causal Chrome/Mimic comparison — 2026-09-20

## Conclusion

The 6–10 second Mimic result is not caused by CDP latency, V8 startup, network,
or one slow CSS primitive. The primary cause is that Mimic does not have one
persistent derived style/layout graph shared by the main world, Playwright's
isolated world, rendering observations and input. It repeatedly reconstructs
and serializes document-scale derived state at those boundaries. Chrome keeps
Blink's style and layout objects alive, incrementally invalidates them, and all
worlds read the same clean graph.

Two measured mechanisms account for most of the difference:

1. Isolated-world reads invoke the main-world owner and create JSON
   document-wide style/visibility projections. On the measured warm Mimic run,
   24 projection calls were all misses and occupied 3,474 ms. The projection
   payloads totalled 1.14 MiB. Cold used 29 misses, 3,350 ms and 2.54 MiB.
2. Mimic samples `IntersectionObserver` in JavaScript over its reconstructed
   geometry. Cold used 59 deliveries totalling 3,281 ms; warm used 16 totalling
   679 ms. Chrome performed 219 native intersection computations over retained
   layout in 1.3 ms total.

The two categories are disjoint in this trace: isolated-world projections run
inside Playwright calls, while IntersectionObserver sampling runs in main-world
DOM tasks. In the warm stage comparison their excess over the corresponding
Chrome work explains approximately 3.88 seconds of the 5.73 second summed-stage
gap. The remainder is principally high-volume DOM emulation during repeated
Playwright selector/role traversal, input sequencing, and navigation/task work.

## Reproduction boundary

- Frozen workload: `tools/runtimecheck/playwright_wikipedia_profile.js` and the
  existing local Wikipedia capture.
- Browser reference: Chrome 152.0.7977.82.
- Mimic binary: freshly built from the incoming source.
- Chrome was launched fresh three times. Workload elapsed values written by the
  profile were 2,203, 1,494 and 1,419 ms. The corresponding process-level PASS
  times, which also include launch/setup/teardown, were 2,360, 1,628 and 1,587 ms.
- Mimic diagnostic cold/warm elapsed values varied with tracing overhead. The
  projection-labelled pair was 11,500/7,560 ms. These are diagnostic runs, not
  replacement benchmark results.
- Chrome tracing used `blink`, `blink.user_timing`, `devtools.timeline`,
  invalidation tracking and V8 categories. Every workload stage was bounded by
  `performance.mark` events.
- Mimic received two temporary `MIMIC_CHAIN_TRACE` hooks: one around
  `foreignComputedStyleFlatTree` and one around the separately queued
  IntersectionObserver callback. Both hooks were removed after capture.

Local ignored receipts:

- `.build/chrome-wikipedia-trace.json`
- `.build/chrome-traced-stage.json`
- `.build/wiki-projection-{cold,warm}.json`
- `.build/wiki-labeled-{cold,warm}.json`
- `.build/wiki-deep-{cold,warm}.json`

## Same workload, stage comparison

The table uses the projection-labelled warm Mimic diagnostic and the marked
Chrome trace. Tracing perturbs totals but the stage-local difference and event
attribution are stable enough for diagnosis.

| Stage | Mimic ms | Chrome ms | Difference ms |
| --- | ---: | ---: | ---: |
| Main navigation | 638.5 | 242.6 | +395.9 |
| Search fill | 310.4 | 15.1 | +295.3 |
| Search input value | 6.3 | 12.8 | -6.5 |
| Search navigation | 317.4 | 524.1 | -206.7 |
| JavaScript heading visible | 1,706.7 | 15.2 | +1,691.5 |
| Article inspection | 250.8 | 16.8 | +234.1 |
| ECMAScript scroll | 499.1 | 57.0 | +442.1 |
| ECMAScript inner text | 229.6 | 27.2 | +202.3 |
| ECMAScript href | 217.2 | 26.3 | +190.8 |
| ECMAScript click/navigation | 861.0 | 131.7 | +729.3 |
| ECMAScript heading visible | 810.6 | 13.9 | +796.6 |
| ECMAScript heading inner text | 125.9 | 1.9 | +124.0 |
| ECMAScript title | 32.4 | 0.6 | +31.8 |
| Back navigation | 1,023.4 | 216.6 | +806.7 |
| **Summed stages** | **7,029.2** | **1,301.9** | **+5,727.3** |

Search navigation is faster in Mimic only because it does not finish the same
derived-state work before returning. The immediately following first visibility
read pays it. Chrome completes lifecycle work during navigation/rendering and
the visibility read observes clean retained state.

## Warm differential accounting

The following is an exclusive wall-time accounting, not a list of overlapping
profile stacks. For each stage:

- **foreign projection/reconstruction** is the directly timed
  `foreignComputedStyleFlatTree` owner-realm work. Chrome has no equivalent
  serialized cross-realm projection;
- **same work, slower** is Mimic command-active time after subtracting that
  projection, minus Chrome renderer-main-thread `RunTask` time in the same
  marked interval. It includes analogous selector/accessibility traversal,
  DOM reads, input and lifecycle work, although the implementations differ;
- **waiting/scheduling** is the difference between wall time outside those
  active intervals. This includes Page task ordering, Playwright polling/frame
  cadence and navigation waits;
- **unattributed** is the residual after the first three categories.

Signed values are retained so that the rows and total close exactly. A negative
`same work` value means Chrome did more active lifecycle work in that stage; a
small negative waiting value is trace-boundary noise.

| Warm stage | Mimic excess | Foreign projection/reconstruction | Same work, slower | Waiting/scheduling | Chrome equivalent in this stage |
| --- | ---: | ---: | ---: | ---: | --- |
| Main navigation | +395.9 ms | 421.6 ms | -107.3 ms | 81.6 ms | navigation lifecycle; Chrome has more active renderer work |
| Search fill | +295.3 ms | 197.9 ms | 90.3 ms | 7.1 ms | input/actionability and DOM event work |
| Search input value | -6.5 ms | 0 | -9.5 ms | 2.9 ms | native DOM value read |
| Search navigation | -206.7 ms | 72.9 ms | -184.3 ms | -95.3 ms | navigation performs lifecycle work before returning |
| JavaScript heading visible | +1,691.5 ms | 1,590.3 ms | 99.3 ms | 1.9 ms | 10.3 ms `RunTask`; no style recalculation or layout |
| Article inspection | +234.1 ms | 144.4 ms | 88.6 ms | 1.0 ms | text/visibility reads; no style recalculation or layout |
| ECMAScript scroll | +442.1 ms | 67.8 ms | 381.4 ms | -7.1 ms | 45.9 ms `RunTask`, about 0.3 ms rAF; no style/layout |
| ECMAScript innerText | +202.3 ms | 0 | 201.3 ms | 1.0 ms | 26.3 ms `RunTask`; no style/layout |
| ECMAScript href | +190.8 ms | 0 | 189.7 ms | 1.1 ms | 26.2 ms `RunTask`; no style/layout |
| ECMAScript click/navigation | +729.3 ms | 1.0 ms | 608.7 ms | 119.6 ms | 121.8 ms `RunTask`, 18.1 ms style-tree update, 8.2 ms layout |
| ECMAScript heading visible | +796.6 ms | 644.9 ms | 55.0 ms | 96.8 ms | 10.9 ms `RunTask`; no style recalculation or layout |
| ECMAScript heading innerText | +124.0 ms | 112.1 ms | 7.2 ms | 4.6 ms | 0.9 ms `RunTask`; no style/layout |
| ECMAScript title | +31.8 ms | 0 | 0.9 ms | 30.9 ms | 0.1 ms renderer work; almost entirely scheduling in Mimic |
| Back navigation | +806.7 ms | 220.9 ms | -76.7 ms | 662.5 ms | 198.2 ms `RunTask`, 39.5 ms style-tree update, 19.5 ms layout |
| **Measured marked stages** | **+5,727.3 ms** | **3,474.0 ms** | **1,344.6 ms** | **908.8 ms** | **residual below rounding precision** |

Thus the exactly matched warm-stage differential is explained as:

| Category | Time | Share of measured 5.727 s |
| --- | ---: | ---: |
| Extra work: foreign projection/reconstruction | 3.474 s | 60.7% |
| Same work, slower | 1.345 s | 23.5% |
| Waiting/scheduling | 0.909 s | 15.9% |
| Unattributed inside marked stages | approximately 0 | approximately 0% |

The often quoted approximately 6.5 second warm difference is not one exact
matched pair. Depending on the diagnostic run, launch/setup/teardown and the
unmarked gaps add roughly 0.4–0.8 seconds. If 6.5 seconds is used as the fixed
denominator, the conservative statement is therefore 3.474 seconds extra work,
1.345 seconds same work but slower, 0.909 seconds waiting/scheduling, and
0.773 seconds unattributed outside the marked-stage comparison. That last value
must not be assigned to a subsystem without a single matched whole-run trace.

### Controlled role/scroll split

The production workload resolves the role locator as part of later operations,
so a second opt-in diagnostic run inserted `link.count()` and split article
inspection into individual reads. These rows are explanatory and are **not
additive** to the unchanged-workload table above.

| Controlled warm stage | Mimic excess | Foreign projection/reconstruction | Same work, slower | Waiting/scheduling | Chrome equivalent |
| --- | ---: | ---: | ---: | ---: | --- |
| Article heading `innerText` | +9.7 ms | 1.2 ms | 7.5 ms | 1.0 ms | 1.8 ms `RunTask`; no style/layout |
| Paragraph visible | +44.5 ms | 0 | 41.4 ms | 3.1 ms | 4.4 ms `RunTask`; no style/layout |
| Paragraph `innerText` | +25.7 ms | 1.6 ms | 22.8 ms | 1.3 ms | 2.8 ms `RunTask`; no style/layout |
| Body `innerText` | +111.1 ms | 102.5 ms | 8.1 ms | 0.5 ms | 3.2 ms `RunTask`; no style/layout |
| Role lookup (`link.count`) | +276.0 ms | 55.1 ms | 219.8 ms | 1.2 ms | 35.6 ms `RunTask`; no style/layout |
| Scroll after role cache warm-up | +295.4 ms | 0 | approximately 295 ms | approximately 0 | 29.6 ms `RunTask`; no style/layout |

This split shows that `role lookup` is not synonymous with the large foreign
projection: only 55.1 ms of its 276.0 ms excess is projection. Approximately
220 ms is analogous traversal performed much more slowly through Mimic's
JavaScript wrappers and Go-backed DOM. It also shows that scroll remains about
295 ms slower after role lookup and projection have already been paid, so that
cost belongs to the active actionability/geometry path rather than to waiting.

## Cause 1: isolated worlds reconstruct a document projection

Playwright locators execute in an isolated world. Mimic's
`foreignComputedStyleFlatTree` locates the canonical owner realm and calls an
owner-realm JavaScript callback. For `documentValues`, that callback enumerates
every element, resolves requested properties, calls
`observeElementVisibility(candidate)` for every row, serializes all rows to
JSON, exports the string through Go, then parses it in the isolated world.

Important measured examples from the warm run:

| Stage/projection | Time | Payload |
| --- | ---: | ---: |
| Main navigation: cursor/display/visibility | 421.1 ms | 168.2 KiB |
| First JavaScript heading visibility: cursor/display/visibility | 968.2 ms | 464.2 KiB |
| Same visibility call: separate rectangle | 622.2 ms | scalar object |
| ECMAScript scroll: content column | 66.3 ms | 214.6 KiB |
| ECMAScript heading visibility: cursor/display/visibility | 474.6 ms | 256.0 KiB |
| Same visibility call: separate rectangle | 170.2 ms | scalar object |

There were no owner-cache hits among the 24 recorded warm calls. Some kinds,
including rectangles and rendered inner text, are intentionally non-cacheable.
Document batches also miss after document/selector/resource epochs change or
when a new property set is requested.

Chrome has no corresponding cross-realm projection. Isolated worlds contain
different JavaScript wrappers but access the same native DOM, computed style,
layout and paint state. Across the complete traced Chrome journey:

- `LocalFrameView::UpdateStyleAndLayout`: 273.0 ms inclusive;
- `Layout`: 181.1 ms inclusive;
- `UpdateLayoutTree`: 130.3 ms inclusive;
- `Document::recalcStyle`: 76.5 ms inclusive.

These rows overlap and must not be added. More importantly, after search
navigation the 15.8 ms heading-visibility stage contained zero
`UpdateLayoutTree`, zero `Document::recalcStyle`, and zero `Layout` events.
Mimic spent roughly 1.59 seconds in its two owner projections at that boundary.

## Cause 2: IntersectionObserver repeatedly runs the geometry model

Mimic's rendering timer queues a DOM callback that samples every observer and
target with `withStyleReadCache`, `intersectionSample`, `clientRectFor` and
ancestor clipping. The callback is implemented in the authored Web API layer,
so a sample pays the same style/table/text/geometry construction costs as other
JavaScript observations.

| Runtime | Deliveries/computations | Total | Maximum single delivery |
| --- | ---: | ---: | ---: |
| Mimic cold | 59 | 3,280.8 ms | 796.3 ms |
| Mimic warm | 16 | 678.7 ms | 232.8 ms |
| Chrome 152 | 219 | 1.3 ms | below trace significance here |

This is not evidence that Chrome suppresses observers. It computes more samples
but reads retained native layout. Mimic coalesces callbacks, yet a remaining
sample can reconstruct large table and ancestor geometry.

The cold/warm scheduler difference confirms the same mechanism:

| Task source | Cold | Warm | Cold excess |
| --- | ---: | ---: | ---: |
| DOM | 3,470 ms | 494 ms | +2,976 ms |
| Timer | 1,063 ms | 380 ms | +683 ms |
| Navigation | 377 ms | 185 ms | +192 ms |
| Network | 418 ms | 366 ms | +52 ms |

The 3,903 ms task-source increase matches the approximately 3.94 second
diagnostic cold/warm increase. Direct IntersectionObserver labelling explains
2,602 ms of that increase. The rest is author timer/navigation/resource work
and the mutations it publishes.

## Cause 3: Playwright traversal is implemented over expensive projected DOM

After the first large projection, Playwright still repeats selector and
accessible-name traversal for each locator operation. Chrome runs that injected
JavaScript against native DOM wrappers. Mimic runs it against JavaScript
compatibility wrappers whose getters repeatedly consult the Go-owned canonical
DOM.

The warm `ECMAScript href` stage is the clean example: it had no foreign style
projection, yet took 208–217 ms versus 26–28 ms in Chrome. During one captured
stage Mimic performed approximately:

- 19,150 `observationVersion` reads;
- 16,516 `documentActive` reads;
- 30,830 parent reads;
- 24,771 owner-document reads;
- 20,727 attribute reads.

The individually measured Go bodies are cheap; the cost is the repeated
JavaScript traversal, wrapper/brand work, conversions and boundary calls. This
is why optimizing one host getter or dumping one larger node array did not close
the gap in earlier experiments.

## What this rules out

- **CDP/network as the primary cause:** the largest intervals are inside
  `Runtime.callFunctionOn` owner work and main-world DOM tasks. Network task time
  is similar cold/warm and far smaller than the difference.
- **V8 startup/compilation:** warm creates new pages but the measured long calls
  are style/visibility/geometry observations, not compilation. Fresh Chrome also
  launches a process and remains near 1.5 seconds.
- **Chrome skipping correctness work:** its trace contains style resolution,
  layout, invalidation, hit testing and 219 intersection computations. They are
  cheap because they consume persistent incrementally maintained state.
- **A missing leaf cache:** clean repeated rectangles are already near zero in
  both engines. The expensive boundary is first construction and reconstruction
  after coarse invalidation or across consumers/worlds.
- **Frame cadence alone:** rAF/stability waits account for tens to low hundreds
  of milliseconds, not multi-second owner projections and observer callbacks.

## Architectural implication

The concrete fix boundary is now narrower than “copy Blink”:

1. One Page-owned derived style/layout state must be readable from every realm;
   an isolated world must not request a serialized document copy.
2. Navigation/render lifecycle and script observations must consume the same
   results, so work completed before DOMContentLoaded remains available to
   visibility, scrolling, hit testing and IntersectionObserver.
3. Style, intrinsic measurement, constrained sizing, placement and viewport
   projection need separate dependency generations. A mutation must not replace
   the whole observation merely because one consumer could be affected.
4. IntersectionObserver must sample retained geometry and schedule delivery;
   it must not itself become another full geometry builder.
5. Repeated Playwright traversal still needs cheaper canonical DOM reads, but
   that is secondary to removing the document projection/reconstruction path.

The result is a shared Blink-shaped derived-state spine over Mimic's existing
canonical DOM, not a second DOM and not a website-specific cache.
