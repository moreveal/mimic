# Mutation-aware click latency follow-up, 2026-09-13

This continues [the initial investigation](click-investigation-20260913.md).
The comparison baseline for this follow-up is that investigation's tested
executable, `.build/click-fast-final/mimic.exe`, SHA-256
`035eff9e864902fed0653dde8bf74aefa239dd29aad3ca5305fe550633ec4ddf`.
It already contains cross-checkpoint geometry reuse. The original unmodified
`ab5033a` remains available separately in `.build/click-before-source`.

## Attribution and implementation

The production input replay in `.build/click2-profile-blast-input/` reproduced
1.46-second mouse-release handlers and a 531 ms second mouse move. Both menu
opens were checked as expanded. Unlike a synthetic `element.click()`, this
replay uses `Page.DispatchProtocolInput` and pumps asynchronous Page work.
It contains native V8 and Go CPU, allocation, heap, mutex, block and goroutine
profiles with host counters. Selector-attribute memoization and repeated CSS
matching dominate the JS stacks; host counters also expose repeated primitive
DOM reads and shaping-result transfers. Root-level native `toString` samples
alone are not a reliable attribution to a particular conversion.

Changes preserve the canonical state and existing Page ownership:

- A necessary-ancestor filter rejects impossible selectors. Hash collisions
  only admit additional matching work; the pinned matcher decides every match.
- Static selector results use exact attribute contents, ancestor identities,
  stylesheet programs and supported media/viewport inputs. Sibling, structural
  and state-dependent selectors are always re-evaluated. Dynamic candidates are
  retained as programs, not as truth values.
- Ordered matched rules, precise inline state and dialog defaults validate
  reuse of specified declaration arrays. Inheritance and geometry are not
  guessed from a cached inline-only DOM.
- Style-only box data validates the complete ancestor declaration chain,
  environment, root font and relevant presentation attributes. Mutated sizes,
  child membership, text flow, positions and availability get a fresh graph.
  Numeric basis caches are bounded (64 texts, four bases per text, four edge
  records); callers receive separate mutable margin records.
- A lazy private DOM projection reads parent, attributes and precise inline
  state under the canonical arena lock in one transfer. It is immutable derived
  data for the current epoch, not a second mutable DOM. Public read boundaries
  still validate their epoch; internal hit-test batches share one observation.
- Compact parsed text metrics have a 1 MiB accounting budget and a canonical
  realm-owned font-collection revision. Font changes invalidate both native and
  JS projections; failed shaping results are not retained across observations.

Persistent element/program caches use weak keys and are cleared on bootstrap
restoration. No global Page lock, site-specific rule, synthetic hit hint, new
public Web API, graphics backend or frozen workload change is introduced.

## Reproducible live checks

`tools/performance/click_latency.py --site blast --clicks 6 --exercise-form`
performs three verified menu-open / Escape-close cycles in a fresh Context.
Each opening also waits for a visible login input, validates its projected hit,
clicks it, types the fixed non-credential text `mimic-latency-check`, verifies it,
and clears it. No form is submitted. Menu state, form readiness, field click,
and typing/clearing are separate timings. First form readiness includes network
and asynchronous initialization; it must not be described as pure click cost.

The pre-cap candidate (`.build/click2-fast-final/mimic.exe`, SHA-256
`484fb884631298bd46a413c7ba941bd27972d9e70d5a75762172f96799c0b022`)
passed three live sequences, as did the previous executable and pinned Chrome.
Raw data: `.build/click2-blast-form-{previous,final}-{0,1,2}/` and
`.build/click2-blast-form-chrome-0/`. Final-source release measurements are
recorded separately below. Do not pool exploratory `click2-blast-*` runs with
these comparison series; some exploratory runs overlap builds/tests.

## Final release verification

The final executable is `.build/click2-release/mimic.exe`, SHA-256
`b5b473bf381f9c7523568c58c94038b4a414d50108dcda4c1fdc9fa6d3c16e1c`.
The build receipt and frozen harness fingerprints are in
`.build/click2-release/build.json`. Final live data is in
`.build/click2-blast-form-release-{0,1,2}/`, `.build/click2-pairs/`,
`.build/click2-todo-{previous,release,chrome}/` and
`.build/click2-github-probe-release/`.

| Verified interaction, median ms | Previous tested build | Final release | Chrome 152 |
| --- | ---: | ---: | ---: |
| Blast menu open, nine / nine / three opens | 1879.85 | 380.78 | 14.97 |
| First open → visible form, three / three / one trials | 2288.89 | 904.60 | 254.84 |
| Reopen → visible form, six / six / two opens | 2144.84 | 436.67 | 15.62 |
| Reopened form: verified field click | 510.81 | 217.50 | 9.47 |
| Type fixed text and clear | 109.02 | 108.31 | 36.94 |
| TodoMVC checkbox, six toggles each | 39.34 | 40.15 | 9.75 |

Every sequence passed. The final three first-form observations were 904.60,
817.23 and 933.34 ms. First-request timings vary: an earlier candidate received
the login response after about 1.12 seconds, whereas another run took about
0.15 seconds. Neither that network variation nor the one Chrome sequence is a
stable tail-latency estimate. Live peak process-tree RSS medians were about
561 MiB previous and 570 MiB final (ranges 553–564 and 570–582 MiB).

Three alternating local/control-site pairs also passed. Local geometry
mutation/read median-of-trial-medians improved from 110.11 to 31.34 ms; unchanged
reads stayed approximately 2 ms and local clicks approximately 27 ms. Each
local trial excludes one warmup and validates 30 measured rounds (90 per binary).
ChatGPT click → verified modal improved from 944.44 to 833.62 ms over three
trials each. There is no meaningful TodoMVC improvement in this follow-up.

The final instrumented GitHub probe found the same expected SPAN in all six
observations (2267 DOM elements). Its first hit test took 302.24 ms, subsequent
hit tests median 24.14 ms, and subsequent pointer moves median 7.27 ms. This is
not a successful GitHub control-click compatibility claim.

The final internal profile is `.build/click2-profile-release/`, with its sibling
log. Mouse-release handlers fell from the initial input profile's 1463/1464 ms
to 399/280 ms, and the second mouse move from 531 to 113 ms. Both expanded-menu
checks passed. These instrumented wall times support attribution, not ordinary
latency claims. All native and Go profiles plus host counters are retained.

Focused geometry/style/CSS/input/shadow/font/selector checks passed repeatedly.
The final regression fixture's 91 stylesheet comparisons and additional dynamic
state/shadow checks also passed frozen Chrome 152.0.7977.82 directly; receipt:
`.build/click2-style-final-chrome.json`, fixture SHA-256
`095b6d5c83481dd1acd3430279620edc36e13fb8c6c3f55cab7ff717d7822f93`.
New tests cover canonical projection reads/writes and geometry after font load,
removal, addition, descriptor changes and clearing.

The final-source full suite, `go test ./... -count=1 -timeout=15m`, passed
every package (browser 407.863 seconds; CDP 31.929 seconds). The complete log
is `.build/click2-all-tests.log`. The previously observed intermittent child
parser interruption failure did not recur in this run; this is not a claim
that this separate pre-existing issue was fixed.

Fresh clean-baseline and final-source fast gates both passed all six mandatory
semantic workloads, warm runs, N=10/N=25 concurrency waves and memory phases.
Raw receipts: `.build/click2-fast-before/` and `.build/click2-release/`.

| Fast gate metric | Clean `ab5033a` | Final release |
| --- | ---: | ---: |
| DOM completion median ms | 194.55 | 194.06 |
| Static completion median ms | 34.44 | 34.58 |
| React completion median ms | 80.95 | 79.59 |
| N=10 median sessions/s | 75.19 | 73.85 |
| N=25 median sessions/s | 74.22 | 73.92 |
| Static marginal RSS MiB/page | 45.79 | 46.09 |
| React marginal RSS MiB/page | 48.01 | 48.59 |
| Static RSS MiB after teardown/recovery | 124.92 | 122.05 |
| React RSS MiB after teardown/recovery | 145.12 | 144.33 |

These are short regression gates, not evidence of universal throughput gains.
They show approximately unchanged throughput and modest per-page memory cost;
allocator residual memory remains after the 250 ms recovery interval. No full
frozen performance matrix or broad race run is claimed.

## Remaining boundaries

Chrome remains faster on these interactions. This work removes substantial
repeated matching and transfer overhead; it does not implement incremental
layout for every DOM mutation. The limited coordinate model's failed GitHub
control preflight from the first investigation is not claimed fixed. Live HTML,
network completion times and scheduling differ between runs; these small
samples do not establish stable p95 or p99 latency guarantees.
