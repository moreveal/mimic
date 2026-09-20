# Wikipedia causal PoCs — not production fixes

Base: `eac6b5c30028800eceeed9c557e59487771c9d6e`. The frozen workload is
unchanged (SHA-256 `5B340D5C51D1CE7F291C1FB9F635DF7E5A7D29CD12255EE2E09E59918325E444`).
These experiments intentionally violate browser behavior. Passing the workload
does **not** establish compatibility. See the canonical performance report.

## Final checkpoint: diagnosis stopped

The user requested planning instead of further experiments. The production plan
is [wikipedia-production-plan.md](../../../docs/performance/wikipedia-production-plan.md).
No experimental production changes remain applied.

The final five-pair medians are **2251/2147 ms cold/warm**, with paired control
2717/2664 ms. See `results/best-plus-query-all-paired-results.json`. Do not merge
deltas from different paired series or interpret this as a correct browser run.

`wikipedia-joint-upper-bound.patch` preserves the final complete destructive
combination, including its new Go helper. It applies independently to the base,
not on top of the other patches. It includes unconditional split-derived freeze
and unsafe hints, plus flags for author suppression, fabricated geometry/scroll
bypass, stale attributes/topology and query-result memoization. In this patch,
setting `MIMIC_POC_FREEZE_DERIVED=0` does not disable its unconditional freeze.
Input target routing bypasses correct occlusion checks. Query memoization is
Playwright-specific, ignores invalidation and must never enter production.

`wikipedia-best-read-hints.patch` is an earlier standalone combination, also with
unconditional freeze/unsafe hints. `wikipedia-read-plan.patch` isolates the
scalar/inherited/CSS-source experiments; it has not been compatibility-validated
and is not a ready-to-ship implementation. Receipts for intervening combinations
are in `results/`. The querySelector-only memo receipt has no actual cache hits;
only the subsequent querySelectorAll-capable patch tested effective replay.

## Earlier author-ablation checkpoint

`wikipedia-author-ablation.patch` is an unapplied, removable experimental patch:

- `MIMIC_POC_NO_AUTHOR_SCRIPT=1`: skips classic author-script execution while
  preserving currentScript cleanup and the checkpoint; removes subsequent
  script effects, tasks and observer registration. Modules are not suppressed.
- `MIMIC_POC_NO_AUTHOR_STYLE=1`: skips author stylesheet matching/cascade;
  inline/default declarations and stylesheet fetches remain. Fonts, visibility,
  geometry and downstream demand change; this is not a pure matching CPU bound.
- `MIMIC_POC_FREEZE_DERIVED=1`: reuses selected derived maps from the first
  retained owner observation after loading, while allowing fresh DOM read views.
  Deliberately wrong invalidation; only fields present at capture are retained.
  This is not a hard bound on every possible incremental layout architecture.

No production source change remains applied. Apply the patch only in a separate
experimental checkout. Build a clean control before applying, then build the PoC
to a different binary. `git apply --check` must succeed first.

Example paired run (PowerShell, from repository root):

```powershell
$env:POC_CONTROL_BINARY='E:\GitHub\mimic\.build\mimic-wiki-control.exe'
$env:POC_BINARY='E:\GitHub\mimic\.build\mimic-poc-author-freeze.exe'
$env:POC_TRIALS='3'
$env:MIMIC_POC_NO_AUTHOR_SCRIPT='1'
$env:MIMIC_POC_NO_AUTHOR_STYLE='1'
$env:MIMIC_POC_FREEZE_DERIVED='1'
node tools/performance/pocs/wikipedia-pairs.cjs all-three
```

For an incremental freeze comparison using the **same PoC binary** on both
sides, set `POC_CONTROL_BINARY` equal to `POC_BINARY` and
`POC_CONTROL_ENV='{"MIMIC_POC_FREEZE_DERIVED":"0"}'`. Both sides then retain
the author-script/style ablations. For a clean comparison with a flag-capable
control, explicitly disable all three flags in `POC_CONTROL_ENV`.

Each sample starts its own server on port 9438, runs the unchanged workload
cold and then warm, and closes that server. Build order alternates. Keep this
port free; do not run other performance experiments concurrently. Logs and
receipts are written under `.build`; receipts include binary hashes, stage
times and experiment flags. The `results/` files preserve this session's raw
measurements. Do not combine absolute results from different sessions into a
single paired percentage. Three pairs are screening evidence, not a shipping
gate. The single-pair guarded-Taffy result is especially provisional.

Local rejected experiments are also saved under `.build`:
`wikipedia-bulk-taffy-combined.patch`,
`wikipedia-taffy-first-experiment.patch`, and
`wikipedia-taffy-first-guarded.patch`. The unguarded Taffy-first combination
failed with recursive width resolution; guarded Taffy-first did not show a
material complete-workload gain. Bulk line shaping was essentially flat warm.
