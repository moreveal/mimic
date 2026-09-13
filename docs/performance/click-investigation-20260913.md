# Click latency investigation, 2026-09-13

Subsequent mutation-heavy click work and newer release measurements are in
[the follow-up investigation](click-followup-20260913.md). Results below describe
the first, cross-checkpoint reuse change rather than the final combined release.

Starting source: `ab5033a` (clean tracked files). Existing untracked user
directories were preserved. The before executable is
`.build/mimic-click-investigation-before.exe`, SHA-256
`8cfedb434de18a2db6466053060402aad90e75d1477047db53796bca6b255503`.

## Reproduced delay and attribution

A real GitHub header click took **10.31 seconds**. Client CDP timings and
`MIMIC_PROFILE_CDP` show five successive action commands each waiting
approximately 0.7–0.9 seconds for the Page, followed by 0.7–1.5 seconds of work.
The page subsequently navigated to signup instead of opening the selected menu.
This is a failed interaction, not a successful latency sample. The harness now
checks the projected hit before activating a control; the README disclosure
also fails this guard. No form was filled or submitted on GitHub.

Separate Go CPU/allocation/heap/block/mutex and native V8 profiles identify
`IntersectionObserver` sampling → box sizing → CSS selector matching as the
expensive background task. The Page lock wait reflects that work; it is not
evidence that independent Pages share a global execution lock. Native sampling
also shows substantial allocation/GC and selector-attribute memo work. A Go CPU
profile alone mostly reports native execution and does not identify the JS cause.

Evidence is local in `.build/click-github-before/result.json`,
`.build/click-profile-github-before/`, and `.build/click-profile-github-pump/`.
Profiles are attribution runs, not unprofiled latency baselines.

## Production change

Top-main-realm style and geometry observations already had a canonical epoch
covering DOM writes, resources, viewport, CSSOM, media, element state and scroll.
They were nevertheless discarded at every microtask checkpoint. An observer
sample and the next input command therefore repeatedly rebuilt the same graph.
The graph now survives checkpoints while that epoch is unchanged. A changed
epoch replaces it, and a failed observation discards provisional results.
Child, isolated and foreign-realm observations retain their conservative
synchronous boundary. Restore clears the retained graph. No global cache or
Page lock change is introduced.

Shadow attachment and synthetic membership already invalidate the canonical
epoch. Unchanged shadow trees now use the same IntersectionObserver no-change
check instead of forcing layout every rendering opportunity.

The cache retains at most the last derived observation graph per eligible
realm. A graph may remain until the next geometry read or realm teardown after
a mutation; this is not an unbounded history of document revisions. New
geometry inputs must participate in the epoch before using this retention.

Experiments with selector indexes, memo records, ancestor traversal and compiled
selector caches were removed. The retained production change addresses repeated
work, not individual sites or selector spellings.

## Measurements

The measured candidate is `.build/mimic-click-investigation-after.exe`, SHA-256
`c5527cdb4aeb73ad931fffb8085cff96a27284c87974a28b915d6754e2c203fa`.
It predates the final defensive discard-on-exception clause, which does not
affect successful observations. Final-source verification is recorded below.

| Observation | Before ms | Candidate ms |
| --- | ---: | ---: |
| Local real click, median of three trial medians | 109.97 | 27.29 |
| Local geometry read, same aggregation | 29.79 | 2.00 |
| Local geometry mutation/read, same aggregation | 142.89 | 112.72 |
| ChatGPT login click → verified modal, three trials | 1176.65 | 1013.39 |
| TodoMVC checkbox click → verified state, six actions | 49.15 | 47.98 |
| GitHub warm hit test, five diagnostic observations | 3653.27 | 32.20 |
| GitHub warm mouse move, five diagnostic observations | 780.05 | 7.86 |

The local series contains one excluded warmup and 30 measured rounds per trial,
90 rounds per binary. The frozen workload is unchanged. All geometry values,
mutation results and click counts passed. ChatGPT's three candidate modal
observations were 1049.96, 968.04 and 1013.39 ms; the before series includes a
1972.34 ms sample. No stable p95 is inferred from three live trials.

GitHub's first candidate hit test took 639.89 ms and first mouse move 78.47 ms.
Both binaries found the same element over all six observations. Server timing
attributes the before warm mouse moves to 762–802 ms of Page lock wait and
6–7 ms of handler work; candidate Page lock wait was zero in those samples.
These runs used CDP timing instrumentation and are explicitly diagnostic.

Pinned Chrome 152 completed the six TodoMVC actions at median 12.71 ms.
Mimic is still about 3.8× slower on that interaction. GitHub's coordinate
projection remains incorrect for the attempted controls, and cold geometry and
mutation-heavy workloads remain expensive. No general faster-than-Chrome or
complete GitHub compatibility claim is made.

Raw observations: `.build/click-verified-pairs/`, `.build/click-todo-{before,after,chrome}/`,
`.build/click-github-probe-{before,after}/`; compact local aggregation:
`.build/click-investigation-summary.json`. The standalone repeatable harness is
`tools/performance/click_latency.py`.

## Verification

### Final-source live interaction and internal profile

Three complete blast.hk login-menu cycles passed on the original before binary,
the final tested binary, and pinned Chrome 152. Each cycle clicks the actual
login link, verifies `aria-expanded`, closes with Escape, and verifies closure.
No form fields were filled or submitted. Direct repeated link clicks navigate
to login in Chrome too, so they are not a valid open/close toggle workload.

| Verified operation, ms | Before | Final | Chrome 152 |
| --- | ---: | ---: | ---: |
| First mouse open | 2724.98 | 1386.93 | 32.17 |
| Second mouse open | 3272.40 | 3114.63 | 45.25 |
| Third mouse open | 2707.69 | 2096.63 | 38.53 |
| Escape closes, range | 27.68–41.01 | 23.44–33.70 | 5.19–24.12 |
| Sampled peak process-tree RSS, MiB | 567.19 | 551.71 | 808.58 |
| Sampled peak process-tree private memory, MiB | 595.53 | 580.63 | 512.78 |

All three DOMs had 2616 elements. These are single live sequences, not stable
percentile estimates. First open improved about 2×, but later mutation-driven
opens still cost 2.10–3.11 seconds. This is a major remaining bottleneck, not a
solved faster-than-Chrome interaction. Memory samples are taken every 100 ms;
CPU snapshots cover currently live processes, not accumulated departed children.
Raw successful sequences: `.build/click-blast-cycles-{before,final,chrome}/`.
Earlier `click-blast-*` attempts with sibling-based menu detection or repeated
link-click toggling failed the same harness assumptions in Chrome and are
excluded from successful sequence comparisons.

The final source also passed a combined native/Go CPU, allocations, heap,
mutex, block and goroutine profile with host counters and a 15-second live
GitHub task window: `.build/click-profile-final/` and its sibling `.log`.
Independent hit tests cost 735.38, 70.76, 32.70 and 36.16 ms, with consistent
results and 2267 DOM elements. Mutation-driven pump tasks still reach about
0.9 seconds. This final-source run confirms warm reuse but does not replace
unprofiled measurements or eliminate the remaining CSS/geometry rebuild cost.

### Tests and fast gate

The focused geometry/style/input/shadow/font/viewport/animation suite passed
(browser 76.916 s, CDP 0.549 s). New deterministic host-count regressions prove
that unchanged checkpoints and shadow samples avoid geometry while class,
CSSOM, visibility, synthetic removal and reinsertion still invalidate results.
Full `go test ./... -count=1 -timeout=15m` passed the browser package (446.431 s)
and every tested package except CDP. Its one failing test,
`TestInterruptAndCloseChildParserContinuation/external=false/close=false`,
reported missing interrupted parser state. Focused repetitions failed 4/10 on
the changed source and 2/5 on a clean detached `ab5033a` checkout, with the same
assertion. This is a pre-existing failure; the complete suite is not claimed
exit-zero. Logs: `.build/click-all-tests.log`,
`.build/click-cdp-interrupt-{recheck,baseline}.log`.

Both fresh fast gates completed all six mandatory semantic workloads, warm
workloads, N=10/N=25 waves and memory phases. They used the repository's
`.build/benchmark-venv/Scripts/python.exe`; the first attempted system-Python
launch failed before building because it lacked `websockets.sync`.
Receipts/raw samples are in `.build/click-fast-{before,final}/`.

| Fast gate metric | Fresh before | Final |
| --- | ---: | ---: |
| DOM completion median ms | 223.16 | 193.51 |
| Static completion median ms | 44.11 | 36.00 |
| React completion median ms | 96.11 | 83.20 |
| Static N=10 median sessions/s | 52.20 | 76.95 |
| Static N=25 median sessions/s | 59.18 | 88.47 |
| Static marginal RSS MiB/page, N=10 | 44.98 | 45.18 |
| React marginal RSS MiB/page, N=10 | 47.95 | 47.93 |
| Static residual RSS MiB after teardown/recovery | 95.80 | 99.44 |
| React residual RSS MiB after teardown/recovery | 111.89 | 110.49 |

These short gates are regression controls. Their timing variance, especially
early warm samples, does not justify attributing all throughput improvements
to this change. No material live per-page memory increase is evident in these
samples; allocator residual memory remains. No full frozen matrix or broad race
rerun was performed.

The final tested executable is `.build/click-fast-final/mimic.exe`, SHA-256
`035eff9e864902fed0653dde8bf74aefa239dd29aad3ca5305fe550633ec4ddf`.
The fresh clean baseline gate's SHA-256 is
`088872eac02d3764408e548ac993303efa0441960cbf8ba7a73d284ffd579d71`;
its source is the detached checkout `.build/click-before-source`.
