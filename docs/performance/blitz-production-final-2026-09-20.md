# Blitz production producer — final migration checkpoint (2026-09-20)

## Outcome

Commit `fc1e9202f7e9cb36380e336ecbef41bd8f044281` makes the retained
`blitz-dom` style/geometry producer unconditional for admitted Documents.
`MIMIC_STYLE_ENGINE` is not consulted. The JavaScript producer is reachable
only through an explicit semantic-admission fallback and remains a temporary
correctness oracle; it is no longer an alternative production mode.

The unchanged Wikipedia E2E gate realizes the Part B leverage in production:

| phase | clean production baseline median (range) | Blitz median (range) | paired median delta |
| --- | ---: | ---: | ---: |
| cold | 9648 ms (9275–10054) | 6144 ms (6046–6423) | −3489 ms / −36.2% |
| warm | 5879 ms (5777–6038) | 3977 ms (3841–4187) | −1854 ms / −31.7% |

All five alternating fresh-process pairs completed both cold and warm phases:
20/20 successful unchanged workload executions. The candidate binary SHA-256
is `ee963029438606ba3814bb477b25ee8375980e7d451f17dcaf6660a62e95b990`;
the clean baseline is
`fe635e939935dbe0406eb0fb31938c3607cb2bc174e87756183ff4d9d026322d`.
The frozen workload SHA-256 remains
`5b340d5c51d1ce7f291c1fb9f635df7e5a7d29cd12255ee2e09e59918325e444`.

## Displacement check

Warm named-stage medians show the expected producer savings and the remaining
cost, rather than hiding it in a later action:

| stage | baseline | Blitz | delta |
| --- | ---: | ---: | ---: |
| Search fill | 297.51 ms | 59.03 ms | −238.48 ms |
| Search press + navigation | 309.84 ms | 261.79 ms | −48.05 ms |
| JavaScript heading visible | 1358.67 ms | 324.20 ms | −1034.47 ms |
| ECMAScript scrollIntoViewIfNeeded | 402.76 ms | 653.05 ms | **+250.29 ms** |
| ECMAScript innerText | 194.71 ms | 190.89 ms | −3.82 ms |
| ECMAScript getAttribute href | 182.80 ms | 205.65 ms | +22.85 ms |
| ECMAScript click + navigation | 828.12 ms | 604.95 ms | −223.17 ms |
| ECMAScript heading visible | 805.69 ms | 359.26 ms | −446.43 ms |

Scroll is a real remaining regression, but click and the post-navigation
observable boundary are also faster; the 1.85 s warm wall reduction is not work
merely shifted into click/navigation. This checkpoint does not claim Chrome
parity yet.

The separately instrumented cold/warm run recorded eight first builds (median
171.70 ms), twenty rebuilds (median 19.37 ms), and 4125 preparation reuses
(median 0 ms, 16.54 ms total). It recorded zero whole-document fallback and
zero producer errors on Wikipedia. Four selective legacy scalar traces remain;
they are property-level CSSOM serialization, not execution of the legacy
document producer. Profile envelopes, native phase records, calls and snapshots
overlap and are not summed.

## Published state and hot-path fallback removal

The Go DOM remains canonical. One main-world native owner receives a
snapshot-and-diff projection with stable canonical IDs. It publishes one packed
generation containing geometry, eligibility and the common computed values used
by visibility, scroll, IntersectionObserver and input. Isolated worlds consume
that authoritative packet through the owner boundary rather than reconstructing
document state.

The final compatibility pass added products that otherwise would have forced
Wikipedia back to the old producer:

- parsed canonical inline declarations are transferred independently of their
  serialized `style` attribute, preserving precision and attribute selectors;
- geometry consumes an unrounded native transform matrix and origin;
- dirty text/search/email/url/tel/password values are independent native form
  state, preserving `[value]` semantics, reset and exact Parley scroll extent;
- SVG root intrinsic dimensions/viewBox ratios use native Stylo/Taffy inputs;
- tables, captions, closed details, controls, disclosure markers, font struts,
  absolute/fixed containing blocks and fractional CSSOM used values are native
  producer products, not readback patches.

Fallback remains explicit for unsupported semantic domains: shadow/slot and
adopted-sheet integration, quirks mode, active animation lifecycle, child-frame
viewport integration, reduced motion, non-1 device scale, authored
`content-visibility`, connected code-unit `TextJSON`, nonempty textarea/live
textarea content, select listbox modes, video intrinsic metadata, and unsupported
font descriptors/indexes or resource bounds. These are migration debt to remove;
there is no environment fallback switch. Native transaction failures remain
errors and are never converted into fallback success.

## Memory, lifetime and correctness gates

Warm post-Page-close medians fell with the native producer:

| metric | baseline | Blitz |
| --- | ---: | ---: |
| working set | 563.99 MiB | 312.58 MiB |
| private memory | 603.37 MiB | 340.63 MiB |
| peak working set | 925.41 MiB | 611.74 MiB |

These process-level values are not a proof of zero retained native state.
Teardown tests additionally cover owner disposal and same-transaction retired
node eviction by count and estimated 16 MiB input bound.

Final gates on the committed source:

- combined focused browser/native/DOM gate: PASS (`63.600 s`);
- Go race gate for native owner, DOM, input/observer and browser integration:
  PASS (`browser 71.591 s`);
- complete `internal/browser` suite: PASS (`567.580 s`);
- controlled observation/input chain: 12/12 PASS. At 10k rows the four orders
  completed in 1726.62 / 1703.38 / 1697.63 / 1843.62 ms;
- exact frozen Chrome receipts cover corrected hand-written geometry
  expectations, select baseline, 18 SVG intrinsic cases and live input extent.

Raw matrix samples, per-process logs, profile accounting, controlled-chain
receipt, build hashes and gate logs are archived under
`data/blitz-production-2026-09-20/final/`.

## Remaining work

Do not optimize the old JavaScript producer. Next work should remove explicit
fallback domains and attack the now-visible residual native/consumer costs,
starting with the warm scroll regression and the remaining non-native scalar
properties. Any change still gates on the full unchanged Wikipedia E2E and must
not trade visibility savings for scroll, IO, input or navigation work.
