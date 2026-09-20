# Taffy atomic postorder PoC (2026-09-20)

Isolated worktree PoC; no production JavaScript was changed. The candidate
replaced the eager `rect(root)` input build, for an exact static root seed, with
observation-local `size(node)` calls for the leaf `MeasureHeight` and
`inlineContentHeight` consumers. Scratch results were invisible until the
Taffy build completed.

## Correctness

- Focused `HeightObservation`, definite-height/incomplete-box, scroll
  invalidation, and Taffy tests passed with the candidate and verifier enabled.
- A full Wikipedia run passed with exact verification enabled.
- The root seed (`x/y`) and every directly consumed legacy height had zero
  mismatches.

## No-publication result

The first version discarded all local legacy results after Taffy input
construction.

| mode | control (ms) | candidate (ms) | median delta |
| --- | --- | --- | --- |
| cold | 9811, 9887, 10166 | 10616, 11677, 11128 | +1241 ms (+12.55%) |
| warm | 6212, 6153, 6236 | 6729, 6798, 6984 | +586 ms (+9.43%) |

A representative warm stage comparison showed only +21 ms in JavaScript
heading visibility, followed by +224 ms in `scrollIntoViewIfNeeded` and +314 ms
in click/navigation. The omitted canonical geometry was rebuilt later.

## Atomic commit-after-complete result

The refined version published only complete directly consumed size results
after the full Taffy response was available. Recursive scratch entries were
not published.

| mode | control (ms) | candidate (ms) | median delta |
| --- | --- | --- | --- |
| cold | 10198, 9497, 9841 | 10316, 10043, 9708 | +202 ms (+2.05%) |
| warm | 7430, 6262, 6187 | 6448, 6410, 6313 | +148 ms (+2.36%) |

The warm control outlier does not change the decision: using the two lower
control observations would make the candidate regression larger. A stage run
still showed approximately +25 ms at first visibility, +66 ms at scroll, and
+16 ms at click/navigation.

## Decision

Reject the PoC. The measured `rect(root)` prepass is not removable overhead: it
materializes a reusable canonical descendant geometry graph. Selectively
recomputing the same exact height inputs locally either moves that work to
later stages or duplicates enough dependency work to regress both cold and
warm E2E.
