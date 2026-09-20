# Wikipedia product-consumption audit: partial measurement, not an architectural verdict

## Status

**The requested semantic-required / avoidable / unknown attribution of the
9.5 s cold / 6.3 s warm baseline has NOT been established.** This report records
actual diagnostic runs and a limited counterfactual. It must not be cited as
proof of a multiple-second demand-driven opportunity, or as proof that native
acceleration is necessary. No production optimization from this audit remains.

The unchanged `tools/runtimecheck/playwright_wikipedia_local.js` passed cold and
warm with the diagnostic build, and with a cursor-materialization counterfactual.
There was one pair per diagnostic condition, not a performance gate. Detailed
logging substantially perturbed timing and potentially scheduling. These are
counts for the instrumented journeys, not guaranteed counts for baseline.

During cleanup, independent edits appeared in the shared source tree, including
`css_producer.js`, `producerColumn`, `producerDeclarationIndex`, selectors,
scrolling, font metrics, and `surface.go`. They were not made by this audit and
were preserved. The archived diagnostic source patch does not contain these
changes. Further measurements need a fixed source snapshot; the post-cleanup
binary must not be treated as the original control.

## Method and limits

Temporary instrumentation recorded declaration-array construction, computed
property values, box style contexts, intrinsic results, size/flow results and
placement results. Read events record the current producer parent. Main and
isolated realms share canonical node IDs. The join key was:

`Document ID / category / node ID / property / style version / geometry version`.

This is an **audit key**, not a proven complete semantic input signature. In
particular, animation sample time and provisional recursive-plan state are not
fully represented. A repeated key or byte-equal result is not proof that the
second computation could safely have been omitted. Size results can include
recursive/provisional returns. Style contexts contain functions and were not
serialized for equivalence comparison.

Producer intervals subtract nested instrumented producers and exclude result
serialization performed after the measured callback. They still include read
instrumentation, GC, uninstrumented subproducers and clock quantization. Native
Taffy was not separated in this audit, unlike the earlier Part A accounting.
These times cannot be added to Part A buckets or scaled to baseline wall time.

Internal read edges are **implementation edges**, never automatically semantic
edges. The audit did not complete independent root tagging for author reads,
observer sampling/delivery snapshots, input consequences, or navigation/lifecycle.
Those paths may invoke instrumented producers, but coverage of their semantic
roots and dependency closure is unproven. IO/hit-testing product counts therefore
remain unmeasured, not zero. A complete observable-output Chrome oracle was not
run for this instrumentation.

## Recorded production

The repeat columns are occurrences with identical serialized outputs to earlier
production for the same Document/node/property, separated by equal versus changed
epochs. They overlap the other columns; **this is not a partition into required
and avoidable work**. Times are instrumented exclusive producer milliseconds,
not opportunity or baseline milliseconds.

### Cold

| Class | Production events | Unique audit keys | Equal output, same epoch | Equal output, changed epoch | Instrumented exclusive ms |
| --- | ---: | ---: | ---: | ---: | ---: |
| matching/cascade | 50,594 | 50,594 | 0 | 38,104 | 2,502.3 |
| computed property | 105,173 | 105,173 | 0 | 60,664 | 560.5 |
| style context | 50,577 | 50,577 | unknown | unknown | 402.0 |
| intrinsic | 29,619 | 29,619 | 0 | 23,487 | 446.3 |
| size/flow | 31,988 | 31,978 | 10 | 25,014 | 1,772.8 |
| placement | 16,897 | 16,897 | 0 | 11,435 | 172.7 |
| IO/hit-test products | unmeasured | unmeasured | unknown | unknown | unmeasured |

### Warm

| Class | Production events | Unique audit keys | Equal output, same epoch | Equal output, changed epoch | Instrumented exclusive ms |
| --- | ---: | ---: | ---: | ---: | ---: |
| matching/cascade | 19,777 | 19,767 | 10 | 7,399 | 1,631.8 |
| computed property | 51,098 | 51,092 | 6 | 7,210 | 275.9 |
| style context | 19,769 | 19,767 | unknown | unknown | 169.5 |
| intrinsic | 11,156 | 11,156 | 0 | 5,041 | 279.8 |
| size/flow | 12,497 | 12,492 | 5 | 5,585 | 903.9 |
| placement | 8,396 | 8,396 | 0 | 2,999 | 99.2 |
| IO/hit-test products | unmeasured | unmeasured | unknown | unknown | unmeasured |

Except for directly joined computed-value requests, semantic closure and proven
unused membership remain unknown for the table above. In particular, the high
equal-output counts do not answer how much first-build work can be avoided.
The aggregator's `firstIdentity` means first production of a node/category/property
in the recorded run, NOT a complete first-build of a Document.

## Narrow property-consumption result

There were 67,378 isolated computed-property read events in cold and 66,990 in
warm. These joined to 16,932 and 8,430 unique produced keys respectively. Three
distinct requested keys in each run did not join; they remain unresolved.
The corresponding producer intervals sum to 93.7 / 29.1 ms. These sums exclude
their dependencies and do not measure total semantic-required work.

For scalar computed values, the source has a narrower identifiable consumption
boundary: `cssComputedValue` reads the owner computed-values cache and isolated
property projection. `cssBatchedForeignVisibility` reads the separate visibility
record, not the scalar `values` map. The following counts exclude top-level
`documentValues` batch construction reads, but include other reads regardless of
whether they came from author code, Playwright, or another producer.

| Property | Cold batch keys | Cold keys without a non-batch read | Warm batch keys | Warm keys without a non-batch read | Associated exclusive ms, cold / warm |
| --- | ---: | ---: | ---: | ---: | ---: |
| cursor | 26,126 | 26,046 | 12,637 | 12,595 | 227.6 / 143.9 |
| display | 26,126 | 15,956 | 12,637 | 6,788 | 72.5 / 20.4 |
| visibility | 26,126 | 18,634 | 12,637 | 7,372 | 39.9 / 20.6 |
| content | 13,382 | 9,346 | 6,582 | 4,580 | 68.2 / 28.5 |
| total | 91,760 | 69,982 | 44,493 | 31,335 | 408.2 / 213.4 |

These are **unconsumed scalar-result candidates within the recorded paths**, not
a proof that their entire producer intervals are avoidable. Constructing an
unconsumed scalar can prime a declaration/inheritance/style cache later needed
elsewhere. Nor is the table a full property-level cascade audit: it does not
record demand for each declaration winner or selector candidate. The larger
matching/cascade bucket cannot be assigned to these unused scalars.

## Counterfactual: do not materialize other nodes' cursor

In a temporary diagnostic branch, `documentValues` omitted `cursor` for all
nodes except the requested element. Missing values remained missing; a later
request used the existing resolution path. Display, visibility, the batch's
visibility records, and all geometry algorithms were left intact. No fake
computed value or hard-coded Wikipedia answer was returned.

| Recorded count | Cold control | Cold diagnostic | Warm control | Warm diagnostic |
| --- | ---: | ---: | ---: | ---: |
| batch cursor keys | 26,126 | 7 | 12,637 | 5 |
| all computed-value events | 105,173 | 92,434 | 51,098 | 38,466 |
| matching/cascade events | 50,594 | 61,999 | 19,777 | 19,777 |
| style-context events | 50,577 | 61,983 | 19,769 | 19,769 |
| size/flow events | 31,988 | 40,250 | 12,497 | 12,497 |
| intrinsic events | 29,619 | 36,437 | 11,156 | 11,156 |
| placement events | 16,897 | 19,893 | 8,396 | 8,396 |

Both unchanged cold/warm journeys passed. In warm, 12,632 fewer computed values
did **not** reduce the recorded counts of expensive geometry or cascade work.
Cold produced more work; with one heavily instrumented pair we cannot separate
schedule perturbation from algorithmic displacement. Do not claim a controlled
cold regression mechanism or an E2E speedup from this run.

Instrumented E2E was 24,926 / 17,188 ms for control and 25,452 / 17,919 ms for
the diagnostic. Warm computed-value exclusive time was 275.9 -> 253.0 ms,
despite 143.9 ms previously being associated with the unread cursor keys.
This illustrates why the unread-result time cannot simply be counted as saved
work. With this instrumentation and sample size it is not a reliable causal
timing estimate either.

PASS establishes the frozen workload assertions only. It does not establish
equivalence of every author observation or observer snapshot to Chrome. The
counterfactual is therefore supporting evidence for scalar overmaterialization,
not a complete semantics certificate. It was removed from runtime source.

## Architectural question: what is and is not established

* Property-level overmaterialization exists at the recorded scalar boundary.
  Removing one nearly unused column does not remove the major shared producers.
* Size/flow/placement semantic closure has not been measured independently of
  implementation recursion. No quantified document-wide geometry opportunity is
  established by these data.
* Predicate/proof execution has not been counterfactually verified against
  numeric geometry and sampling-time snapshots. No opportunity is assigned to it.
* X semantic-required seconds, Y avoidable seconds and Z unknown seconds for the
  original 9.5 / 6.3 s runs remain unresolved. There are **zero seconds of newly
  demonstrated baseline E2E savings**, which is not a claim that Y equals zero.
* Consequently, the audit cannot endorse a path to <1 s through work elimination,
  nor conclude that most costly work is mandatory and a bulk/native rewrite is
  necessary. Either conclusion would overstate the evidence.

The cursor diagnostic is already a minimal intervention for the measured scalar
opportunity; its count result does not justify expanding it into production.
There is not yet a measured semantic budget supporting a different production
vertical slice. Before choosing one, the missing root coverage, semantic edges,
and unperturbed attribution must actually be measured, not inferred from this
producer graph. This remains unfinished audit work, not an implementation plan.

## Artifacts and reproducibility

Local artifacts under `.build/`:

* `consumption-audit-1-{receipt,summary}.json`, cold/warm logs, and
  `consumption-audit-1.jsonl` (1,044,263,315 bytes).
* `consumption-audit-cursor-1-{receipt,summary}.json`, cold/warm logs, and
  `consumption-audit-cursor-1.jsonl` (1,175,294,077 bytes).
* Control binary SHA-256:
  `173f8229753e635b0784ff5863c827eaa02fa71a538ea6fa4743e06d308f227e`.
* Cursor diagnostic binary SHA-256:
  `d2ea99914fc213f59b657a734bc3954a41d819ca0b6b512d89bcb9967e850107`.

`tools/performance/pocs/product-consumption-audit-source.patch` archives the
four instrumented source files as a diff against
`27e456100168b7b4b824c6a376092b6b92d71f0c`, including prior Part A instrumentation.
It is **not** a patch to apply on top of the current dirty tree. It includes the
cursor switch; unset `MIMIC_AUDIT_CURSOR_TARGET_ONLY` for control and set it to
`1` for the diagnostic. Build with `go build -o <binary> ./cmd/mimic` in an
isolated snapshot. The runner accepts `<prefix> <binary>` and refuses an existing
audit log. The summarizer accepts `<prefix>` and rewrites only its derived summary.

Post-cleanup build and unchanged cold/warm workload passed (10,238 / 6,253 ms),
but that tree contains the independent concurrent edits. These are smoke-test
results only, not an audit performance comparison. Raw logs were retained;
their approximately 2.22 GB disk cost is diagnostic output, not runtime memory.
