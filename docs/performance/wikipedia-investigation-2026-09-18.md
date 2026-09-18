# Wikipedia Playwright Performance Investigation — 2026-09-18

## Scope

This note records the evidence from the September 18 investigation of
`tools/runtimecheck/playwright_wikipedia_local.js`. The production baseline was
approximately 10 s cold and 6 s warm on the development machine, while pinned
Chrome was approximately 2–3 s cold for the same workflow.

The experimental Document-owned derived-state rewrite was preserved separately
on branch `experiment/document-derived-state-20260918` at commit `49abc2c`. It
is intentionally not part of `main` because it regressed the target workload.

## Established measurements

### Runtime.callFunctionOn is not dominated by the CDP bridge

Instrumentation of `Debugger.CallFunction` showed that `syncShadowSnapshots`,
function-declaration evaluation and finish/serialization were normally tiny.
The expensive calls spent almost all of their wall time inside the Playwright
injected JavaScript executed by `state.invoke("call", ...)`.

Many calls were 0–5 ms, while selector/actionability calls were commonly
150–700 ms. Therefore a fixed CDP/debugger crossing cost does not explain the
multi-second workflow gap.

### Canonical `getAttribute` was not the 170 ms locator cost

Hard-coding `getAttribute("href")` to bypass the canonical Go attribute lookup
did not materially change Playwright `locator.getAttribute("href")`. Most of
that operation was locator resolution and injected JavaScript before the final
attribute getter.

### Structural host crossings are real but secondary

A structural projection experiment removed roughly 70k repeated structural
crossings (`parentNode`, `ownerDocument`, sibling/child traversal, etc.) from a
role lookup and produced measurable local improvements. Host callback timing,
however, showed that the aggregate Go callback time of these crossings was not
the dominant cold cost. Structural locality is still useful for the warm role
floor, but it cannot explain the large first-read cliff by itself.

### A first derived-style read has a large materialization cliff

On a deterministic, fully settled JavaScript article fixture, repeated reads
showed a strong cold/warm split. Representative observations included:

- first `#firstHeading` visibility read: about 1.6–1.7 s;
- identical second visibility read: about 6–15 ms;
- first role lookup: hundreds of milliseconds;
- later role lookups: about 170–190 ms;
- first `documentValues(["display", "visibility"])`: roughly 450–500 ms in
  earlier matched probes;
- the same derived state after materialization: roughly 31 ms.

A four-way prewarm experiment established that this first-read cost is
transferable: full-document traversal alone did not remove it, while traversal
combined with computed-style reads moved much of the cost into the prewarm.
This proves a lazy derived-state population effect, but does **not** by itself
prove that all derived state should be eagerly or globally retained.

### The cold visibility cliff contains both style and geometry work

CPU and phase instrumentation on a settled page showed that Playwright's own
`isElementVisible` path calls `getComputedStyle()` and
`getBoundingClientRect()`. An earlier Mimic-specific visibility bypass did not
bypass those Playwright reads, so the previous conclusion that visibility,
style and layout had been ruled out was too strong.

A representative first visibility operation spent about 1.58 s inclusive in
foreign computed-style/geometry observation. One full-document style batch over
about 6.5k elements took about 0.94 s, including approximately:

- selector `query("*")`: ~75 ms in that sample;
- computed values: ~806 ms;
- `matchingStyles`: ~365 ms;
- stylesheet/context work: ~179 ms;
- cascade: ~36 ms;
- visibility observation: ~45 ms.

The other major portion was geometry/layout/text work; `shapeTextMetrics` alone
was observed around 255 ms in one profile, with Taffy layout around 49 ms.

This means selector matching is significant but is not the complete root cause
of the first-read cliff.

### `documentValues` is an amplification workaround, not a standalone root cause

The isolated-world computed-style path chose between per-element foreign reads
and a full-document `documentValues` batch. Disabling the batch improved a
single scalar heading read but made a role scan dramatically worse because the
role algorithm then performed thousands of isolated-world to owner-world style
reads.

Representative matched result:

| Operation | document batch | per-element foreign reads |
| --- | ---: | ---: |
| heading #1 | ~1.58 s | ~1.44 s |
| heading #2 | ~6 ms | ~20 ms |
| role #1 | ~269 ms | ~1.68 s |
| role #2 | ~185 ms | ~234 ms |

The existing code therefore chooses between two bad cost models: broad
full-document materialization or many fine-grained foreign reads.

## Failed architectural rewrite

The investigation then over-generalized the evidence and attempted to replace
realm-local observation with a Document-owned read/derived-state system. The
experiment included:

- a canonical DOM mutation journal;
- shared Document read projections;
- shared V8 private capabilities between realms;
- retained selector/cascade/computed/geometry values;
- dependency tracking, later replaced by compact generation stamps;
- removal of the automatic scalar `documentValues` path.

This was conceptually cleaner in several isolated cases, and a settled
`#firstHeading` cold visibility probe fell to tens of milliseconds. However,
the actual Wikipedia E2E regressed from approximately 10/6 s cold/warm to about
16/10 s. An early shared-V8-owner version also caused the second cross-document
navigation to stall until Playwright's 30 s timeout. Restricting shared owners
fixed the navigation failure but did not recover the performance regression.

The rewrite therefore failed the primary acceptance criterion and was removed
from `main`.

## Why the rewrite regressed

The experiment added costs to the mutation-heavy and navigation-heavy part of
the workload in order to improve a settled-read microbenchmark. Wikipedia is
not predominantly a settled-read workload during the measured E2E interval.
Parsing, DOM insertions, resource completion, stylesheet changes, script work,
state changes and navigation continuously invalidate derived observations.

The first dependency implementation was especially pathological: cached values
replayed very large sets of leaf dependencies into enclosing derived values,
creating millions of bookkeeping operations during one role lookup. Compact
generation stamps removed that particular amplification but did not make the
overall rewrite faster than the original architecture.

Sharing V8 execution owners also crossed a lifecycle boundary that should not
have been changed without navigation-specific evidence. A Playwright utility
world can survive while a Document/main realm is being replaced. Coupling both
to one execution owner introduced teardown/commit interactions and caused the
navigation timeout regression.

The central methodological error was treating the transferable ~500 ms cold
materialization as proof that a broad Document-owned retained renderer state was
required. The measurements proved that redundant lazy construction exists;
they did not prove that paying mutation/invalidation maintenance across the
entire lifecycle is cheaper.

## Current conclusions

1. Keep the production baseline architecture until an alternative beats the
   real E2E workload and correctness gates, not only settled probes.
2. The ~170–190 ms warm role floor and the first style/layout cliff are distinct
   problems. Do not force one architecture to solve both without evidence.
3. Structural JS/host locality can reduce the warm accessibility traversal, but
   it should be evaluated independently from style/layout lifetime changes.
4. The first style read does too much work. The next investigation should
   identify why a scalar Playwright observation causes expensive work across
   thousands of elements and which portions can be made local or reusable
   without imposing equivalent work on parsing/navigation.
5. Style selector matching is a substantial component of cold computation but
   not the whole cost. Optimizing selector dependency forms alone is unlikely to
   close the Chrome gap.
6. Text shaping and geometry have their own cold materialization costs and need
   separate lifetime/cache analysis.
7. Do not move the same CPU work into navigation or an explicit prewarm and call
   it an improvement. Measure total E2E and CPU.
8. Do not change V8 realm/execution-owner lifecycle as a performance shortcut
   unless a matched experiment demonstrates that it is required and navigation
   teardown is covered explicitly.

## Recommended next investigation

Return to the ~10/6 s production baseline and use the frozen Wikipedia workflow
as the primary signal. Keep navigation/network variance out of attribution by
also using deterministic settled fixtures, but require every proposed
optimization to improve the complete E2E before expanding it.

For the first expensive style/layout observation, measure exclusive CPU by
layer and by number of affected nodes:

1. stylesheet source/index preparation;
2. candidate-rule filtering;
3. selector matching;
4. cascade/declaration construction;
5. computed-property resolution and inheritance;
6. visibility computation;
7. text shaping;
8. layout/geometry;
9. serialization/realm transfer.

For each layer, test the smallest change capable of disproving its causal role.
Prefer bounded caches/indexes whose maintenance cost is paid only when their
input actually changes. In particular, investigate whether the style engine can
avoid constructing complete declaration/cascade state for properties and nodes
that the caller never observes.

The success criterion remains the real workflow: materially below the 10 s cold
and 6 s warm baseline, stable cross-document navigation, and unchanged browser
semantics. A faster microbenchmark is evidence, not completion.
