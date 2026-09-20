# Intrinsic reuse eligibility, 2026-09-20

Diagnostic only: no cached result is returned and no production source was edited.
The isolated worktree is `.build/poc-intrinsic-eligibility`; base is
`eac6b5c30028800eceeed9c557e59487771c9d6e` plus the incoming runtime source snapshot.

## Conservative global style epoch

Binary SHA256: `97D452C4BF434790B0A27765A870CF2A81FBBEF4A836F000AF5657A771D9C3D3`.
Both diagnostic workload runs passed all assertions.

| Observed main-realm computation | Cold | Warm |
| --- | ---: | ---: |
| Width/size existing-cache misses | 44,160 | 18,877 |
| First computation of key | 11,321 | 11,219 |
| Repeated computations | 32,839 | 7,658 |
| Eligible retained hits | 0 | 0 |
| Union potential avoided body time | 0 ms | 0 ms |
| Summed exclusive measured body time | 2,488 ms | 1,179.7 ms |
| Global style epoch mismatch | 32,839 | 7,658 |
| Subtree generation mismatch | 170 | 98 |
| Own/ancestor retained style identity mismatch | 10,893 | 3,942 |
| Environment mismatch | 1,769 | 1,769 |

Miss reasons overlap. Every observed repeat was rejected by the conservative
global style epoch; unchanged subtree generation alone is insufficient proof
that descendant computed styles remain unchanged after external mutations.

The diagnostic computes canonical subtree generations by snapshot comparison,
not a proposed production O(1) update mechanism. Key setup time is excluded
from measured computation bodies; nested body times are de-duplicated, and
potential hits are unioned beneath the first eligible ancestor. Existing
observation cache hits are excluded. Font version, environment, containing
width, exact retained style identity and dynamic-observation exclusion remain
part of the key.

These are not E2E timing-gate results: diagnostic traffic and snapshot work
perturb task scheduling. Stage evaluations observe the main realm only and
cannot retrieve old-realm computations occurring between the final old-page
sample and navigation teardown. Therefore zero measured hits is not a hard
whole-E2E upper bound. It closes only this conservative key on the covered
observations; dependency-aware style proof is the next diagnostic variant.

Receipts: `results/intrinsic-eligibility-cold.json` and
`results/intrinsic-eligibility-warm.json`. Full tracked runtime build delta:
`intrinsic-eligibility-tracked-build.patch`; additional diagnostic helper:
`intrinsic_diagnostic.go.txt`; incoming journal helper:
`intrinsic-build-mutation_journal.go.txt`.

## Journal-aware proof variant

Binary SHA256: `06C58570F7F8DE91B15FBAB9A241769D57B6C25C9B05C9B45F5D0E6FAB31A242`.
Cold priming and warm diagnostic both passed. The candidate replaces the global
style mismatch only when a contiguous canonical mutation journal proves all
changes to be selector-independent data/aria attributes. The shared selector
parser supplies attribute dependencies; stylesheet source references, escaped
sources, inline attr()/escapes, shadow roots and unsupported/gap records all
force a conservative miss. Exact subtree/style/font/environment/width inputs
remain required. This is a narrow proof, not a complete invalidation engine.

| Covered observation | Cold | Warm |
| --- | ---: | ---: |
| Width/size existing-cache misses | 42,167 | 12,944 |
| First computations | 11,321 | 11,175 |
| Repeats | 30,846 | 1,769 |
| Eligible hits / avoided ms | 0 / 0 | 0 / 0 |
| Exclusive measured body ms | 2,483.2 | 788.8 |
| Journal gap rejection | 23,287 | 1,769 |
| Class mutation rejection | 7,559 | 0 |

In this warm run the JavaScript and ECMAScript samples contained only first
computations, not repeat misses. Main_Page repeated 1,769 computations after a
journal gap and environment change. The difference in computation counts from
the earlier diagnostic confirms that instrumentation changes scheduling; do not
compare the diagnostic elapsed times as optimization results. The current
bounded journal does not establish safe intrinsic reuse for covered repeats.
No retained intrinsic cache was implemented.

Receipts: `results/intrinsic-journal-eligibility-{cold,warm}.json`; build delta:
`intrinsic-journal-eligibility-tracked-build.patch`; diagnostic helper:
`intrinsic-journal-diagnostic.go.txt` (plus the same incoming journal helper).
Both isolated binaries/worktree are retained for reproducibility; no shared
production source contains this instrumentation and no process remains running.

## Structural/text journal, conservative region prototype

Binary SHA256: `7102E8FD0A763E5B010139508A71ACD1FFE4BE64DEBB82ADE27B98A9EDA34DBC`.
Cold early-stop after `ecmascript_scroll` passed. Canonical mutation records now
cover create/insert/remove/fragment/textContent/characterData/innerHTML paths
with target, parent and old parent. Uncovered arena revisions still miss.

The observed JavaScript document had 35,931 width/size misses, 7,559 first
computations and 28,372 repeats. Repeats were rejected by 20,813 journal gaps
and 7,559 class mutations. Main_Page added 3,538 gap-rejected repeats. Eligible
hits and avoided time remained zero. The stylesheet classifier found 3,280
descendant/local rules and 1,316 wider/unsupported rules. At this prototype
stage any wider rule widened the whole document; this was too conservative to
prove that every actual mutation affects the document. It is evidence of a
proof limitation, not an intrinsic-cache impossibility result.

Receipt: `results/intrinsic-structural-eligibility-cold.json`; runtime delta:
`intrinsic-structural-eligibility-tracked-build.patch`; changed journal helper:
`intrinsic-structural-mutation_journal.go.txt`.

## Complete revision-source accounting

Binary SHA256: `4F1A9FB243A3A47F057D34A93F3905BAEE91C9B7FF314FE0646108E501ABE47C`.
Cold early-stop passed. Diagnostic arena Unlock records the source caller for
every otherwise unrecorded revision; the diagnostic journal bound is 16,384.
Selector scopes distinguish local, parent-subtree, ancestor-chain and unknown
root rather than widening every sibling/structural selector to the root.

Anonymous gaps disappeared. JavaScript: 7,559 first computations and 21,419
repeats. Eligible hits/avoided time remained 0/0. The repeated epochs contain:

| Revision kind | Repeated computations exposed to the kind |
| --- | ---: |
| class attribute | 21,419 |
| structural insertion/removal | 13,860 |
| CreateDocumentFragment | 13,860 |
| ParseInertDocument | 6,951 |
| InvalidateObservations | 6,909 |

These counts overlap and are NOT mutation counts. Samples show detached DIV,
LABEL and UL construction and attribute setup. Main_Page additionally includes
CreateHTMLDocument, AdoptNode, CopyInlineStyle/CopyNodeState and script flags
alongside jQuery-style FIELDSET feature probes. Thus arena/storage revisions
include substantial work outside the active connected observation tree.

The classifier still has 472 unknown-root rules (including pseudo-elements,
:target/:dir); its zero-hit outcome is not proof that actual mutations affect
the whole document. Investigation moved to the more concrete boundary:
connected observation revision versus detached/inert arena mutations. No
additional classifier run or intrinsic production cache was made.

Receipt: `results/intrinsic-all-revisions-cold.json`. Tracked build delta:
`intrinsic-all-revisions-tracked-build.patch`; additional helpers:
`intrinsic-all-revisions-diagnostic.go.txt` and
`intrinsic-all-revisions-mutation_journal.go.txt`. The isolated worktree remains
at the executed source state; speculative post-run classifier edits were removed.
