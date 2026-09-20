# Native browser-semantics synthetic vertical — 2026-09-20

## Question and boundary

This experiment tested the architecture
`canonical DOM + CSS -> compact native state -> V8 observations`. A standalone
arithmetic benchmark was deliberately excluded: the controlled test exercises
the same classes of mechanisms as Wikipedia.

Implementation branch: `experiment/native-semantics-synthetic`, commit
`9aa40cf`, based on `27e4561`. The candidate is enabled only by
`MIMIC_NATIVE_SEMANTICS_SYNTHETIC=1` and is not production code.

The candidate combines a Document-owned compact style arena, native selector
matching/cascade/inheritance, a native ordinary-block layout transaction,
packed style and geometry snapshots, mutation-journal reuse, and direct main/
isolated-world reads without main-world JavaScript document projection. The
existing visibility, IntersectionObserver, scrolling, hit testing and input
consumers read the packed state.

The chain retains author CSS and uses public browser/CDP boundaries: direct
computed style and rectangles, Playwright visibility, role traversal, a real
IntersectionObserver delivery, scroll, DOM/mouse/locator clicks, clean repeats,
irrelevant attribute mutation, ancestor width mutation and unrelated text
mutation.

The diagnostic native layout covers ordinary block flow, px/percentage sizes
and simple box edges. It does not cover tables, flex/grid, full shaping,
shadow/slot layout, positioned geometry or the complete CSS value model.

## Binaries and runs

| binary | SHA-256 |
| --- | --- |
| clean control | `22E17096D8D19C85708882ED6CE637B04A99041ADCBE21998FC543A8654B2E79` |
| native-semantics PoC | `C09E089019B3C0266A3B9D05664F7433B12597B7B9E082C7FA301FBBEEC3B188` |

Three fresh-process control/PoC pairs were alternated. Each pair ran all four
first-consumer orders on a new 10,000-row Page. All assertions passed.

Median over all 12 complete chains:

| metric | control | PoC | delta |
| --- | ---: | ---: | ---: |
| complete chain | 3024.81 ms | 2048.33 ms | -976.49 ms (-32.3%) |

| first consumer | control | PoC | delta |
| --- | ---: | ---: | ---: |
| rectangle | 3087.09 ms | 2122.02 ms | -31.3% |
| scalar style | 3064.69 ms | 2015.81 ms | -34.2% |
| Playwright visibility | 3024.56 ms | 1980.69 ms | -34.5% |
| Playwright click | 2985.21 ms | 2139.80 ms | -28.3% |

Selected stage medians:

| stage | control | PoC | delta |
| --- | ---: | ---: | ---: |
| first Playwright visibility | 569.77 ms | 164.99 ms | -71.0% |
| first Playwright click | 846.98 ms | 507.10 ms | -40.1% |
| settled visibility | 120.46 ms | 79.31 ms | -34.2% |
| role lookup | 73.30 ms | 24.50 ms | -66.6% |
| scroll | 58.38 ms | 54.70 ms | -6.3% |
| initial IO delivery | 25.56 ms | 30.03 ms | +17.5% |
| locator click | 162.70 ms | 261.92 ms | +61.0% |
| rect after irrelevant attribute | 156.60 ms | 3.81 ms | -97.6% |
| rect after ancestor width | 255.28 ms | 35.92 ms | -85.9% |
| rect after unrelated text | 154.29 ms | 34.63 ms | -77.6% |

Complete later phase totals confirm that work was not merely displaced:

| phase | control | PoC | delta |
| --- | ---: | ---: | ---: |
| settled | 552.63 ms | 472.17 ms | -14.6% |
| after input | 207.86 ms | 169.90 ms | -18.3% |
| irrelevant attribute | 464.83 ms | 308.13 ms | -33.7% |
| ancestor width | 589.09 ms | 371.27 ms | -37.0% |
| unrelated text | 474.92 ms | 354.37 ms | -25.4% |

For 100 rows, rect/style/visible/click totals changed from
1128.9/768.2/725.7/1031.1 ms to 1111.1/758.6/697.5/883.0 ms. Fixed setup cost
therefore matters; the large benefit appears when document-scale JavaScript
production/projection would dominate.

## Interpretation

This is positive evidence for native ownership. Moving style/layout production
out of JavaScript and letting both worlds consume packed canonical products
removed about one second, or 32%, from a complete 10k chain. The strongest
effects were first construction, isolated-world reads and post-mutation work.

It is not yet a Wikipedia forecast or production result. Locator click
regressed and initial IO was slightly slower. The PoC is single-threaded and its
layout semantics intentionally cover only the controlled block subset. The
next useful vertical is to extend native ownership to Wikipedia's actual text,
table and formatting-context dependencies and run unchanged cold/warm E2E
early. Parallelism should be measured after that single-thread critical path is
known.

Raw JSON remains under the experimental worktree `.build/` as
`native-semantics-io-*`, `native-semantics-pair2-*`,
`native-semantics-pair3-*`, and `native-semantics-small-*`.
