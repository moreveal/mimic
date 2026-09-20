# Blitz producer migration checkpoint — 2026-09-20

## Status: performance leverage confirmed; compatibility gate remains RED

This is a production-migration checkpoint, **not a declaration that Blitz is a
Chrome-compatible replacement or safe default**. The batched native producer
reduces the unchanged Wikipedia E2E median by 3.377 seconds cold and 1.772
seconds warm. Focused frozen-Chrome compatibility still fails in several native
CSS/layout categories. Legacy remains the correctness oracle and migration
fallback, not a performance-optimization target.

The measured executable predates subsequent out-of-flow geometry edits. Do not
attribute these numbers to an arbitrary later source checkout or rebuild.

## Provenance and method

- Clean production baseline commit: `50e0f0c9e4673dde96b5df4ab1993f9f1b01f24e`.
- Baseline tree: `01f76e8d627149e6eb156ccd9ddcfad05494c87b`.
- Baseline SHA256: `fe635e939935dbe0406eb0fb31938c3607cb2bc174e87756183ff4d9d026322d`.
- Prior non-batched native SHA256: `b5313b43b8b26b6f3a34c8bea2d14511121afe16a81fcdb62e7d73c3af2f1d1a`.
- Successful batched native SHA256: `a1fbe32c4034096b0b9fd32a99384d0ae9644f1c630ac4712120583cc230db96`.
- Unchanged Wikipedia workload SHA256: `5b340d5c51d1ce7f291c1fb9f635df7e5a7d29cd12255ee2e09e59918325e444`.
- Runner: `tools/performance/pocs/blitz-matrix.cjs`, `BLITZ_PAIRS=5`.
- Each engine cold/warm pair owns a fresh process. Engine order alternates by
  iteration. Cold and warm each navigate through new Documents; warm is not a
  repeated observation of one retained Document.
- `MIMIC_PROFILE_BLITZ=0`; baseline `MIMIC_STYLE_ENGINE=legacy`, candidate `blitz`.
- Reported full times are controller wall time. Stage medians are separate
  distributions and must not be added to reconstruct the median total.

Baseline was built in a separate clean worktree with fresh locked native Taffy
compilation (`python tools/build_native_layout.py`) and `go build`. Build record:
`../blitz-baseline/.build/baseline-build-record.txt` relative to this worktree.

## First complete matrix: native computation alone did not win

Artifact directory: `.build/blitz-matrix-1789909733542`. All workloads passed,
but the performance result was NO-GO.

| Engine / phase | Five full E2E measurements (ms) | Median (ms) |
| --- | --- | ---: |
| Baseline cold | 9547, 9710, 9601, 9567, 9695 | 9601 |
| Native cold | 9592, 9118, 9570, 9331, 9730 | 9570 |
| Baseline warm | 6268, 6083, 6293, 6375, 6563 | 6293 |
| Native warm | 6565, 6753, 6748, 6615, 6662 | 6662 |

Warm regressed 369 ms (+5.9%). First heading visibility improved strongly, but
the native consumer/readback path paid downstream: scroll 417.39 → 1024.82 ms,
innerText 193.00 → 597.27 ms, href 176.67 → 553.78 ms, click/navigation
928.55 → 1780.58 ms. This matrix explicitly rejects accepting first-visible
latency as proof of a complete producer replacement.

The repair was not another selector leaf optimization. Publication now resolves
the required property collection from one native computed style per node,
including hidden nodes, and publishes shared native-derived snapshot products.
Previously, scalar hidden-style queries repeatedly resolved the hidden ancestry
for separate properties. Batch resolution preserves scalar CSSOM semantics
without fabricating primary styles on undisplayed nodes. A bounded C ABI
transfers length-prefixed strings, and packed readback avoids repeated
document-scale Go/V8 traffic. Focused tests verify hidden batch/scalar parity and
non-poisoning output-buffer retry.

## Batched producer matrix: substantial full-E2E improvement

Artifact directory: `.build/blitz-matrix-1789910308377`. Five alternating pairs
per engine; all 20 cold/warm workload executions passed.

| Engine / phase | Five full E2E measurements (ms) | Median (ms) |
| --- | --- | ---: |
| Baseline cold | 9830, 9292, 9779, 9334, 9387 | 9387 |
| Native cold | 5954, 6143, 6016, 6010, 5869 | 6010 |
| Baseline warm | 6202, 6173, 6507, 6250, 6142 | 6202 |
| Native warm | 4362, 4430, 4478, 4430, 4352 | 4430 |

- Cold: **−3377 ms / −35.98%**.
- Warm: **−1772 ms / −28.57%**.
- Part B previously established warm −1.72 seconds. This implementation reaches
  the same scale without replaying recorded producer outputs. This is evidence
  of practical leverage, not proof of equivalent semantics across all pages.

### Stage medians and displacement check

| Stage | Cold baseline | Cold native | Warm baseline | Warm native |
| --- | ---: | ---: | ---: | ---: |
| Search fill | 435.91 | 101.73 | 293.90 | 59.93 |
| Search inputValue | 6.38 | 17.12 | 5.31 | 5.59 |
| Search press + navigation | 893.25 | 705.18 | 473.79 | 351.75 |
| JavaScript heading visible | 1802.22 | 703.48 | 1290.54 | 332.81 |
| ECMAScript scrollIntoViewIfNeeded | 481.42 | 684.72 | 411.73 | 682.42 |
| ECMAScript innerText | 567.31 | 181.00 | 191.17 | 178.40 |
| ECMAScript href | 377.54 | 203.68 | 174.85 | 192.66 |
| ECMAScript click + navigation | 1338.61 | 970.15 | 892.61 | 668.78 |
| ECMAScript heading visible | 783.95 | 369.70 | 790.34 | 365.79 |
| ECMAScript heading innerText | 7.12 | 7.88 | 11.92 | 7.88 |
| ECMAScript page.title | 4.50 | 4.71 | 36.78 | 5.35 |

All values are milliseconds. Scroll remains **slower**, by 203.30 ms cold and
270.69 ms warm; warm href is 17.81 ms slower. Therefore this is not a claim of
zero downstream overhead. However, click/navigation improves by 368.46 ms cold
and 223.83 ms warm, and the full E2E falls by seconds. The earlier large
downstream displacement has been removed, not merely hidden by a faster getter.
These stage data do not independently isolate all IO/input lifecycle CPU; a
separate causal accounting trace is still required for that attribution.

### Process memory after page close

Medians, MiB. These are OS process measurements, **not exact live native heap or
retained object measurements**. Peak working set is process cumulative. A
post-close working set does not prove that allocators returned every freed page
to the OS or that teardown has no leak.

| Metric | Cold baseline | Cold native | Warm baseline | Warm native |
| --- | ---: | ---: | ---: | ---: |
| Working set | 538.84 | 286.31 | 555.39 | 309.34 |
| Private bytes | 574.61 | 312.04 | 592.43 | 336.07 |
| Peak working set | 933.79 | 582.76 | 933.79 | 582.76 |

Native Page/Document teardown closes the native owner; detached-node retention
has a live-relative bound. Long-running repeated navigation/teardown plateau
testing and detailed native retained-state attribution remain separate gates.

## Unchanged controlled observation/input chain

Ran `tools/runtimecheck/chromium_observation_chain.cjs` unchanged with
`BROWSERS=mimic`, `COUNTS=100,1000,10000`,
`ORDERS=rect,style,visible,click`, `TRIALS=1`, default `POSITION=before`, no
diagnostic traffic. Each engine passed all twelve cases, including mutation
dependency checks and subsequent real input/click assertions. Binary hashes are
recorded in each JSON and match the successful matrix pair.

Artifacts:

- `.build/blitz-chain-baseline-checkpoint.json`
- `.build/blitz-chain-native-checkpoint.json`

| Nodes | First operation | Baseline total ms | Native total ms |
| ---: | --- | ---: | ---: |
| 100 | rect | 1118.82 | 1118.31 |
| 100 | style | 768.02 | 738.68 |
| 100 | visible | 703.52 | 687.86 |
| 100 | click | 746.45 | 748.67 |
| 1000 | rect | 910.11 | 747.30 |
| 1000 | style | 909.71 | 778.67 |
| 1000 | visible | 911.44 | 769.03 |
| 1000 | click | 950.49 | 816.84 |
| 10000 | rect | 2624.90 | 1633.10 |
| 10000 | style | 2624.21 | 1607.12 |
| 10000 | visible | 2652.86 | 1654.85 |
| 10000 | click | 2724.19 | 1797.73 |

This is a single-trial causal correctness/scaling check, not a confidence study
or replacement for Wikipedia. The harness deliberately does not require exact
cross-browser pixels. Its first process page includes different startup work;
do not call that row Wikipedia cold. Counts/orders within one engine are not
independent fresh-process trials.

## Correctness and race evidence

- `go test -race ./internal/layoutblitz ./internal/dom`: PASS.
- `go test -race ./internal/browser -run TestBlitz -count=1 -timeout=180s`: PASS.
- Full focused browser test selection with `MIMIC_STYLE_ENGINE=blitz`: FAIL.
  Selection: `Test.*(Style|Geometry|Input|IntersectionObserver|ResizeObserver|Scrolling|Computed|Isolated)`.
- Log: `.build/blitz-focused-postbatch.log`; race logs:
  `.build/blitz-race-native-dom.log`, `.build/blitz-race-browser.log`.

The null computed-style-context crash, font collection/same-task invalidation,
pseudo-state invalidation, and default-control geometry tests now pass. Table
row/group rectangles and fractional column widths were repaired. Remaining
failures at this checkpoint include missing caption layout, closed-details
query geometry, positioned/iframe offsets, wrapper flow, inline baseline and
progress geometry, 1/64-pixel text width rounding, computed property
support/defaults/serialization and logical-property mappings, auto-height cycle
expectations, and dependency accounting across navigation.

These failures are blocking compatibility regressions, not cosmetic precision
waivers. Keep the fallback explicit and retain useful diagnostics. Performance
success does not authorize stale geometry, fake rectangles, observer suppression,
or benchmark-specific admission rules.

## Scope of what this checkpoint proves

The native architecture can deliver the measured Part B scale on the real
workflow, with lower process memory, once complete producer results are cheaply
published. It does not yet prove general Chrome semantic parity, a default-on
migration, zero fallback, bounded long-term resident memory, or a complete
first-build/rebuild/reuse/invalidation CPU decomposition. Close these gates
against the actual candidate build rather than treating the E2E win as permission
to weaken them. Do not resume optimization of the old producer.
