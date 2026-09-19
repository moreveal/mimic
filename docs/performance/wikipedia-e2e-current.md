# Wikipedia Playwright E2E performance — current state (2026-09-19)

This is the canonical handoff for performance work on
`tools/runtimecheck/playwright_wikipedia_local.js`. Read this file before
changing runtime, DOM, style, geometry, scheduler, CDP, or Playwright-facing
code for this workload.

## Primary acceptance criterion

The complete unchanged Wikipedia workflow is the primary metric. A faster
microbenchmark, CDP command, locator, geometry read, or synthetic click is
evidence only. Do not expand an optimization unless a fresh matched Wikipedia
E2E run improves materially and correctness remains intact.

Historical production behavior during this investigation was roughly 10 s cold
and 6 s warm, versus roughly 2 s Chrome. Individual runs vary with the local
fixture/controller and process state, so compare fresh before/after binaries
against the same workload and environment.

The latest matched local run around commit `58ca5f6` was:

| build | Wikipedia E2E |
| --- | ---: |
| pre-fix control | 9344 ms |
| `58ca5f6` | 9382 ms |

That is noise/regression, not an E2E speedup. The same change improved one
measured `ECMAScript click + navigation` step from about 1546 ms to 1228 ms,
but it did not move the complete workflow. Do not present that local win as the
solution to the Wikipedia gap.

## Established facts

### CDP / scheduler attribution

Expensive `Runtime.callFunctionOn` commands were correlated through
`Debugger.CallFunction`, `runtime.Call`, owner execution and gov8. For the
expensive calls, `ownerWait` was effectively zero and the wall interval was
inside the executed utility callback. CDP serialization, JSON conversion,
persistent/local conversion and value release were negligible in the measured
calls. Earlier cold runs did contain separate Page/queue waits, but those do not
explain the warm E2E gap.

Deep host tracing found thousands of generic DOM callbacks (for example
`nodeData`), but their measured callback time explained only a minority of the
expensive gov8 wall interval. Do not restart from “Go/V8 crossings must be the
whole root cause” without new E2E evidence.

### Full-document computed-style materialization

A first derived-style observation has a large cold/warm cliff. Full-document
`documentValues` can move that work earlier, but disabling it or prewarming it
does not solve the complete workflow. Per-element foreign reads can be worse for
role scans. A Document-owned derived-state rewrite improved settled scalar
probes but regressed Wikipedia from about 10/6 s to about 16/10 s and was
rejected.

The failed rewrite is historical evidence only. Do not restore broad retained
state or shared execution ownership merely because a settled read gets faster.

### 13k-node actionability fixture

A deterministic fixture with one target button and 13,000 unrelated nodes
proved a real document-size-dependent actionability cost. Representative clean
measurements were approximately:

- `locator.isVisible()`: 776–845 ms;
- `locator.boundingBox()`: 18–23 ms after materialization;
- `locator.click()`: 615–678 ms.

Important destructive experiments:

- returning `null` from `getComputedStyle(target)` made Playwright return
  early and reduced visibility dramatically, but this did **not** prove that
  computed-style calculation itself was the cause;
- synthetic/free `display`, `visibility`, `cursor` and
  `checkVisibility()` did not remove the main scaling cost;
- removing scalar `documentValues` / `query('*')` did not remove the main
  scaling cost;
- the first direct `getBoundingClientRect()` could cost roughly 560–600 ms,
  while subsequent reads were near a few milliseconds;
- for a definite positioned target, both `rect(parent)` and `size(parent)`
  could independently force parent/normal-flow work;
- hit testing scanned the whole document and computed geometry/style/paint
  ordering for candidates;
- protocol scroll-to-visible work was also expensive even when no scroll was
  required.

These findings are real local bottlenecks, but the complete Wikipedia run proved
that eliminating them is insufficient by itself.

## Production actionability fix at 58ca5f6

Commit `58ca5f6 perf: reduce actionability layout work` contains generic
production changes, not target-ID or Playwright-specific shortcuts:

1. definite `absolute`/`fixed` positioned geometry avoids unnecessary
   parent normal-flow materialization where the position is independent;
2. recent measured geometry is used as a conservative paint-order lower bound
   for hit testing; possible occluders above the hint are still fully checked;
3. `DOM.scrollIntoViewIfNeeded` now actually sends `ifNeeded: true`;
4. already-visible elements can take a cheap no-op scroll path when no
   intermediate clipping/scroll context requires more work.

On the 13k fixture, a fresh final run was approximately:

| operation | clean | production fix |
| --- | ---: | ---: |
| visible | 845 ms | 476–531 ms |
| box | 23 ms | 22 ms |
| click | 671 ms | 184–196 ms |

Targeted browser regressions passed, including an occluder above a measured
target and protocol no-op scrolling. Wikipedia also passed, but total E2E did
not improve.

## Experiments that must not be repeated blindly

- Broad Document-owned derived-state architecture: rejected by real E2E
  (approximately 10/6 -> 16/10 s).
- Sharing V8 execution owners across lifecycle boundaries: caused navigation /
  teardown problems and did not establish an E2E win.
- Treating `documentValues` as the standalone root cause.
- Treating `query('*')` alone as the root cause.
- Optimizing only `getAttribute`, `nodeData`, JSON/CDP serialization,
  ownerWait, or gov8 wrapper conversion based on aggregate counts.
- NOPing `checkVisibility`, target computed-style properties, NodeList
  wrappers, parent/children wrappers, or local attribute reads and claiming a
  root cause from a small local delta.
- Persisting the existing geometry caches more aggressively without changing
  the work required to build the initial state; the tested version did not
  improve the target workload.
- Moving the same CPU work into navigation/prewarm and calling it a speedup.

## Rules for the next E2E investigation

1. Build a fresh control binary and a fresh candidate binary from explicit
   commits. Kill unused `mimic-*` processes before each matched run.
2. Keep `tools/runtimecheck/playwright_wikipedia_local.js` unchanged.
3. Use the full Wikipedia E2E as the go/no-go metric from the beginning, not
   only after hours of microbenchmark work.
4. A synthetic/destructive PoC is useful for causal localization. Once it shows
   a large local delta, immediately test whether the corresponding generic
   candidate changes Wikipedia E2E.
5. If Wikipedia does not move materially, record the negative result and leave
   that area. Do not spend hours polishing a local optimization as the E2E
   solution.
6. Separate cold-specific queue/navigation costs from warm runtime costs.
7. Preserve navigation, cross-realm ownership, teardown, memory and observable
   browser semantics.
8. Prefer a table of hypothesis -> destructive experiment -> local delta -> E2E
   delta -> conclusion. “Looks expensive” is not attribution.

## Reproduction

Build Mimic:

```powershell
go build -o .build/mimic-e2e.exe ./cmd/mimic
.\.build\mimic-e2e.exe -listen 127.0.0.1:9222
```

Run the unchanged local workload in another process:

```powershell
$env:PW_MIMIC_ENDPOINT='http://127.0.0.1:9222'
node tools/runtimecheck/playwright_wikipedia_local.js
```

For Chrome, use the existing Chrome branch of the same script and the pinned
Chrome executable. Match headful/headless and viewport conditions before making
fine-grained operation comparisons.

## Historical material

This file supersedes the dated Wikipedia-specific handoffs from 2026-09-15 and
2026-09-18. Git history retains their detailed intermediate evidence. General
performance architecture and benchmark history remain in
`docs/performance/report.md` and the other subsystem-specific reports.
