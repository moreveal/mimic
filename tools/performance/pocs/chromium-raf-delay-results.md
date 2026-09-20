# Rendering-opportunity delay PoC (2026-09-20)

Destructive diagnostic, removed after measurement. No production recommendation.

The flag changes the rendering timer from 16 ms to 0 only when an animation-frame
callback is pending. IO-only opportunities keep 16 ms. It does not modify the
frozen workload or skip its stability checks; callback timing is intentionally
not browser-compatible. Existing already-scheduled opportunities are unchanged.

The same binary was used for both sides with `MIMIC_POC_ZERO_FRAME_DELAY=0/1`:
SHA-256 `849207797F3B40E241AD15C23AF608BB07D46402CD3E4F800A36509DE5146D97`.
Base commit `eac6b5c30028800eceeed9c557e59487771c9d6e`, plus the incoming tracked
runtime changes. `chromium-raf-full-build.patch` records that complete build
delta; `chromium-raf-delay.patch` is the isolated removable intervention.

Three alternating fresh-process pairs per workload, all assertions passed:

| workload | control median ms | candidate median ms | delta |
| --- | ---: | ---: | ---: |
| 10k controlled chain, rect-first | 2995.73 | 2569.74 | -14.22% |
| Wikipedia cold | 10099 | 9857 | -2.40% |
| Wikipedia warm | 6371 | 6331 | -0.63% |

Controlled sum of ten scroll/locator-click stages: 786.10 -> 409.41 ms.
First full rect/build: 350.02 -> 350.17 ms. Geometry after mutations remains
approximately unchanged, so the controlled improvement is real removed waiting,
not displaced layout. Its many repeated stability waits overrepresent this
mechanism relative to Wikipedia.

Wikipedia warm scroll: 471.31 -> 454.51 ms; click/navigation: 878.81 -> 842.91 ms.
Warm first JavaScript visibility: 1270.42 -> 1280.07 ms. Cold click/navigation
regresses in the median, 1366.72 -> 1600.31 ms; these stage deltas are not additive
CPU attribution. The complete E2E result is the acceptance metric.

Raw receipts: `results/raf-chain-{1,2,3}-{0,1}.json` and
`results/raf-delay-paired-results.json`. All cold/warm samples and stage results
are retained. Frozen workload SHA-256 is unchanged:
`5B340D5C51D1CE7F291C1FB9F635DF7E5A7D29CD12255EE2E09E59918325E444`.

Decision: close this as an explanation of the multi-second Wikipedia gap. It
does not reach the 10% E2E threshold, so do not develop a production scheduler
change from this experiment. Main production tree was never modified; the
isolated worktree's experimental hunks were reverted after saving the patch.
