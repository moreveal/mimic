# Document getter ownership checkpoint, 2026-09-11

Baseline: 7c25e25 on codex/compat-cleanup-20260911, after the
[Goja observer package](goja-observer-2026-09-11.md). This fixes one confirmed
owner-dispatch cause. It does not close all Document receiver defects.

| ID | Root cause | Repro | Chrome | Mimic before | Priority | Status | Fix | Limitations |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| DOC-OWNER | A getter from another realm treated an active foreign Document as a local inert Document | document_getter_ownership_oracle.js | Owner-backed projections; borrowed getter survives public shadowing | Seven active projections differ; inert domain and captured URL also differ | P1 | Corrected | Captured getters dispatched by private Document owner binding | Setters, methods and invalid receiver cleanup are separate |
| DOC-BRAND | Existing getters accept invalid receivers | broad-discovery-probe.js | Strict except LegacyLenientThis attributes | 637 remaining observation leaves | P1 | Open | None in this package | Includes existing incomplete implementations; not 637 root causes |

The finalizer captures the installed Document getters after semantic installers,
then associates each canonical active or locally wrapped inert Document with a
private owner binding. Borrowed access dispatches to that captured implementation.
It does not read mutable public properties to choose an implementation or create
a second document-state model. Local getters retain their previous receiver
policy. Final callable/native normalization still runs after this installation.

Fresh focused Chrome-vs-Mimic observations improve 9 differing leaves to zero;
Chrome-to-Chrome control is zero. The oracle covers active child documents,
foreign inert documents, own/prototype URL shadowing, local detached defaultView
and retained title/body/defaultView identity after removing an iframe. The
regression runs ordinary bootstrap and snapshot restoration. It does not prove
native global-proxy retargeting, saved-eval gating or complete lifetime reclamation.
The independent broad discovery improves 644 to 637 differing leaves, with zero
control drift. Those seven overlap the focused nine and must not be added.

The fixed original corpus remains 205/236, with 19/28 complete original
representatives, no regressed probes, no new differing leaves and no Chrome
drift. Fresh official fuzzer stays 31 divergent probes in 9 observational groups,
zero errors/unstable results, with the unchanged 3,533 discovery entries and
100-probe budget. These are observational groups, not proven root causes.
General corpus stays 40 difference records, brand 609, relations 2; all three
controls have zero differences. Fresh sweep remains 114/130, 11 candidates,
3 environment/timing reviews, 2 exception mismatches and 36 skipped entries.
Navigation reflection/nested/lifecycle remain zero; retained-native-global and
saved-eval probes retain 2 and 3 leaves respectively, with zero controls.

Frozen Chrome remains 152.0.7977.82 headful, dedicated reused profile
cleanup-bindings-20260911, with observed window/viewport metadata retained.
The capture reports profile freshness as unverified; it is not represented as a
newly created profile. Sweep additionally uses its dedicated headless control.
Only loopback fixtures were executed. No target site or frozen expectations were
changed. [Receipts and reproducible probes](document-getter-ownership-20260911/)
record revision, dirty state, binary/probe hashes and all-leaf comparisons.

Full browser suite PASS: 473.756 s output, 479.699 s package completion.
CDP and WebAPI pass; engine Goja/V8 results were cached. Targeted browser race
PASS 12.698 s; the WebAPI filtered race invocation had no matching tests. Full
browser race is not claimed. Exact skips are retained in the test receipt.

Both complete paired fast gates PASS, including 10/25-Page concurrency and
static/React teardown memory; neither produced a native dump. Before/after DOM
execution/completion: 441.26/472.10 to 444.81/479.53 ms; static 2.86/26.86 to
3.32/28.65; React 53.97/88.92 to 54.83/83.45. Throughput at 10/25 Pages changes
71.12/62.36 to 74.02/72.72 sessions/s. Static/React private memory after recovery
changes 157.96/165.97 to 162.19/170.81 MiB. This single pair includes both
improvements and increases; no performance neutrality or complete lifetime
reclamation is claimed. No retries, snapshot disabling or workload exclusions
were introduced. The earlier throughput concern remains a separate observation.
P0 remains tracked separately in the
[snapshot lineage report](snapshot-readonly-lineage-2026-09-11.md); passing this
semantic oracle cannot establish the cause of unrelated historical native crashes.
