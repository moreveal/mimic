# Snapshot hydration and task-boundary capture, 2026-09-11

This investigation uses the separate `codex/youtube-twitch-hydration` worktree,
based on main at `037f8ff`. It does not change the original checkout. The runtime
and exporter checkpoint is `d03d353`; later documentation commits do not change
the executed binary. These changes do **not** establish faster-than-Chrome
navigation or full website compatibility.

## Causes and changes

- Navigation and background execution used the same 30-second deadline. A
  useful committed DOM could be interrupted during hydration, while waiting
  for unrelated frames or a continuously scheduled callback. Background tasks
  now have Page lifetime; the optional navigation deadline defaults to zero.
  Capture reserves a consistent task boundary, and an explicitly interrupted
  capture is marked partial. Closing a target cancels execution before waiting
  for the Page command lock, including an infinite parser script.
- CDP evaluation could drain unrelated ready timers before returning a simple
  result. CDP now returns at its own evaluation/checkpoint boundary. The direct
  Go embedding API retains its existing ready-task behavior. A Page still has
  ordered callbacks and checkpoints, not parallel author JavaScript.
- Hydration repeatedly crossed Go/V8 for individual child/parent reads and
  reparsed selectors/styles. One earlier YouTube parser sample took 16.5 s,
  including approximately 1.19 million `nodeChildAt` calls (10.2 s in those
  calls). A Twitch sample attributed 10.6 s to geometry reads and 3.15 s to
  435,000 parent reads. Candidate selection is now batched; immutable parsed
  selectors/styles have bounded caches. Computed observations are shared only
  within a synchronous read, with provisional cyclic boxes invalidated.
- Resolving width also resolved ancestors' content heights and revisited their
  descendants. Width resolution now has a separate read cache. This avoids that
  traversal without inventing a renderer or a global Page lock.
- Intersection observers were inert. The implementation now samples the same
  approximate DOM boxes as geometry APIs, after animation callbacks and their
  microtasks. Initial, threshold, clipping, hidden/detached and changed-state
  observations have regression coverage. Unchanged DOM/CSSOM/resource/viewport
  and selector state skips resampling; native shadow child lists conservatively
  disable that shortcut. A host-counter regression proves that unchanged ticks
  do not repeat geometry work.
- Stylesheets started too late and applied sheets could disappear when the
  bounded network-response history evicted them. Parser-discovered sheets now
  fetch concurrently, block subsequent parser scripts, and remain owned by the
  document. Percentage dimensions, limited variable-valued lengths, generated
  aspect-ratio blocks and indefinite percentage heights have focused fixtures.
- Real native shadow roots are exported as such. Public polyfill properties
  are not promoted into additional native roots; doing that duplicated composed
  light DOM and gave its styles a different scope.
- AVIF was advertised but failed image decoding, causing applications to replace
  valid images with transparent placeholders. Pinned `gav1d v0.2.5` now provides
  pure-Go decoding into the existing intrinsic pixel state. No GPU or native
  graphics backend is added. Its Go 1.26.4 requirement is explicit. A lossless
  2x2 fixture agrees exactly with Chrome 152 Image/Canvas readback; concurrent
  decode and malformed-data tests cover the resource boundary. This is not a
  claim of full ICC/HDR/color-management parity.
- Portable assets now reuse completed responses and fetch remaining resources
  with 32 workers, a 512-resource limit and an eight-second fetch budget.
  Workers are joined; cancellation also closes blocked response bodies.
  Export diagnostics distinguish clone, fetch, rewrite and serialization time.
  Missing or over-budget assets remain explicit warnings.

## Live-site observations

These are individual online runs, not frozen benchmarks. Responses, experiments,
localization, caches and background host load were not controlled. Intervening
code changes also change the amount of actual hydration, so shell-only captures
must not be used as a faster successful baseline.

| Capture | Navigation | Capture including boundary wait | Total | Offline check in Chrome 152 |
| --- | ---: | ---: | ---: | --- |
| YouTube homepage, `youtube-release-home-20260911` | 18.7 s | 1.4 s | 20.5 s | Normal logged-out search prompt and navigation, no home-page skeleton |
| YouTube search, `youtube-width-axis-20260911` | 20.5 s | 3.5 s | 24.2 s | Ordinary thumbnails and five first-row Shorts thumbnails decode locally; 13 loaded images |
| Twitch, `twitch-release-20260911` | 15.7 s | 0.85 s | 16.7 s | Channel cards and 34 locally decoded images, not the loading logo |
| Amazon, `amazon-width-axis-20260911` | 13.3 s | 1.5 s | 14.9 s | Storefront/navigation and campaign cards; 8 local images, no white screen |

The final homepage/Twitch run used the exact clean-build gate binary. Export's
own asset-fetch spans were 1.170 s, 1.154 s, 0.651 s and 0.488 s respectively;
the outer capture stage can additionally
wait for the currently running callback to finish. The original user-observed
58-second navigation and more than 37-second capture are not reproduced by these
runs, but no fixed upper latency or Chrome speed advantage is claimed.

All exported-page checks disabled network access in frozen headful
Chrome **152.0.7977.82**, and inspected DOM, styles and intrinsic image dimensions.
The same geometry/intersection regression scripts ran unchanged in that Chrome
and Mimic. A separate clean Chrome homepage run showed YouTube's logged-out search
prompt with no video cards; the absence of a personalized feed is not necessarily
a runtime failure. An Amazon "continue shopping" response was recorded separately,
not counted as a successfully hydrated storefront.

Local receipts are in `.build/*-20260911/snapshot.json`, with each exported
`index.html` and assets beside them. These captures contain live-site content and
are intentionally not committed as frozen reference data.

## Correctness and performance checkpoint

The complete browser, CDP, Web API, DOM, networking, scheduler and image-resource
package suites passed. Browser suite wall time was 304.9 s with unrelated host
work also active; that duration is not a runtime benchmark. Focused tests cover
asset concurrency, cancelled body reads, preserved task boundaries, target close,
CDP evaluation fairness, stylesheet retention, geometry/cache invalidation,
reflection, media's explicit unsupported boundary, and AVIF pixels. The Python
tool's four tests cover redirect/main-frame status, request reuse, lifecycle
reset and output path safety.
Focused browser/CDP race tests also passed (including observation invalidation,
snapshot interruption and infinite-parser teardown); networking, scheduler, DOM
and image-resource packages passed their complete race suites.

The repository publication audit is not clean on this inherited baseline: it
flags historical benchmark paths, archived binaries/profiles and credential-like
patterns in existing CSS/documentation. The original main report already triggers
the latter pattern. No new snapshot source/report file was flagged by the rerun;
unrelated historical artifacts were not removed or altered to make the audit pass.

The unchanged fast gate rebuilt from clean `d03d353` using Go 1.26.4. Executable
SHA-256: `20af11eae414229256302b73b59fcd9c90b8d799bd8d1a330558396bf29b3094`.
The frozen harness digest was verified before launches and after the run.

The first gate failed during the measured 25-Page wave: all connections closed
abruptly, and the process disappeared. Its standard cleanup removed the process
log, so the cause is **unresolved**; neither memory pressure nor a semantic
regression has been established as its cause. Two diagnostic runs of ten 25-Page
waves each (500 sessions, including held-until-complete teardown waves) passed.
A complete fresh-build repeat also passed all mandatory workloads, concurrency
waves and memory waves. A local wrapper retained process logs before cleanup;
it did not modify the frozen runner or workloads. The initial failure must not
be hidden by the successful repeat.

Local gate receipts:

- `.build/snapshot-final-fastgate-20260911/{build,raw}.json`: failed gate.
- `.build/snapshot-final-fastgate-repeat-20260911/{build,raw}.json`: complete repeat,
  plus retained process logs.

Warm medians from the complete repeat (milliseconds):

| Workload | Page creation | Execution | Completion |
| --- | ---: | ---: | ---: |
| DOM | 2.95 | 161.51 | 193.44 |
| Static | 12.42 | 3.18 | 27.91 |
| React | 3.19 | 61.81 | 93.02 |

Static throughput in measured waves was 39.18 / 73.59 / 75.29 sessions/s at
10 Pages and 53.72 / 93.71 / 89.98 at 25 Pages. Timing variability and background
host activity rule out a controlled speedup claim. No Chrome ratios are derived
from this Mimic-only iteration gate.

Ten-Page memory waves used approximately 35.69 MiB marginal private memory for
static and 40.53 MiB for React. Private memory after teardown/recovery was
159.55 MiB and 168.55 MiB versus ready levels of 77.48 MiB and 77.88 MiB.
These are process/V8/cache retention observations, not evidence that all memory
returns immediately to baseline. The earlier in-worktree gate measured
36.03 / 40.31 MiB marginal private memory and 165.12 / 166.30 MiB after recovery;
these samples do not indicate a large new memory regression, but are not an
isolated allocation attribution.

## Remaining limits

Network quiet plus lifecycle/task status is a heuristic, not application-level
readiness. A snapshot freezes the current DOM; it does not scroll an infinite
feed or load every later lazy row. Some offscreen YouTube rows still have no
image source. Native boxes remain approximate: full layout, scrolling,
transforms, paint visibility and all CSS length expressions are not implemented.
Scripts, embedded frames, canvas rendering and video playback are not exported.
Backend challenges and changing website responses remain possible.

Remaining measured costs include author hydration, CSS selector matching,
Go/V8 observation crossings, image work and waiting for a long Page callback
before capture. Parallel resource transfers do not make those callbacks parallel.
The unresolved first concurrency-gate exit also needs retained-log evidence if it
recurs. Do not describe this checkpoint as full Chrome compatibility, a robustly
proven concurrent speedup, or faster-than-Chrome execution.
