# Native function finalization — 2026-09-11

This semantic package starts at `ed4261b`, after the separately committed
[snapshot ownership correction](snapshot-readonly-lineage-2026-09-11.md).
It supersedes the source-finalization portion of W02 in the
[initial cleanup checkpoint](cleanup-2026-09-11.md), not every function-shape,
receiver, arity or conversion finding grouped under that historical heading.
Only controlled local fixtures and frozen Chrome 152.0.7977.82 were used.

## Findings and correction

| ID | Root cause or hypothesis | Repro | Chrome | Mimic before | Priority | Status | Correction / commit | Limits |
|---|---|---|---|---|---|---|---|---|
| W02-source | Native marking runs before later semantic replacements; static methods omitted | native_function_finalization_oracle.js | native function source | handwritten / vendor JS source | P2 | corrected, validation below | this package | Other callable name/length/constructibility defects remain |
| W02-alias | Marking an iteration alias overwrites a shared intrinsic's source using the Symbol property name | DOMStringList iterator and Array.prototype.values | function values | function Symbol(Symbol.iterator) | P1 | corrected | preserve the callable's existing name | Alias object identity stays unchanged |
| W02-body | Body methods infer the descriptor field name value | Request/Response five body methods | bytes/arrayBuffer/text/json/blob | value | P2 | corrected | name the common Body implementations | Arity/receiver/body-consumption semantics are outside this change |
| W04-Headers | FetchHeaders adds an implementation-only prototype layer | broad discovery | methods own on Headers.prototype | several methods inherited | P2 | confirmed remaining implementation difference | open | Methods exist; missing own descriptors are not absent capabilities |
| W04-stream | Vendor ReadableStream exposes from outside frozen inventory | broad discovery | no own from | own static from | P2 | remaining exposure difference | open | Native-source marking does not implement or authorize the capability |
| P0-RO | Independently customized shared read-only snapshot layouts | separate native regression | n/a | fatal restored string reference | P0 | corrected within reduced repro scope | ed4261b | Historical signatures retain separate attribution limits |
| V-Goja | Retained nested-srcdoc race exceeds its context deadline | prior P0 targeted race | n/a | baseline passed, changed-tree failed | P1 validation | open timing-sensitive result | open | V8 flag does not execute in Goja-only subtest; no deadline relaxation |

The final native-marking pass runs after semantic installers, without rerunning
the structural finalizer that republishes globals and resets prototypes. It
visits installed interface constructors, their static functions and prototype
members. Iterator aliases retain the function's original name, including the
shared Array intrinsic. Ordinary user functions retain their own source. The
five shared Body methods now have explicit names; these implementations are
also used in Workers, but broad Worker native-source parity is not claimed.
No API stubs, site-specific behavior, global runtime lock or frozen expectation
changes were added.

## Before / after and classification

The focused frozen oracle goes from **66 differing normalized leaves to zero**,
with zero Chrome-to-Chrome differences. These leaves overlap the other corpora
and represent the general causes above, not 66 independent bugs. Both ordinary
and restored-snapshot browser regression modes pass.

The original fixed set goes **202/236 to 203/236**: property.window.AbortController
now matches. All other normalized differing leaves are checked; there are no
new differences, regressed matching probes or Chrome drift. The original 28
representatives go **17/28 to 18/28**. The official fuzzer goes **11 to 10
observational groups**, with 33 divergent probes and no unstable/error probes.
Discovery remains 3,533 generated surface probes with the unchanged limit 100.
Groups are not equated to confirmed root causes. The separate sweep improves
113/130 to 114/130: intrinsic function shape now matches because fetch exposes
native source. Its remaining classifications are 11 candidates, 3 environment/
timing reviews and 2 exception mismatches; the unchanged manifest also records
36 skipped entries. These are not claimed as tested or repaired.

Brand matrix goes **1,319 to 1,309 diff records**, removing the ten Body name
observations. State-relations remains **2**, and the general corpus remains
**12/15 matched, 41 records** (network 12, surface 7, observations 22). Complete
record comparison finds no added/changed divergences in those corpora. Their
corresponding Chrome controls have zero differences. Navigation keeps the
known two native-global retargeting leaves and three saved-eval bypass leaves;
other focused navigation groups and all their Chrome controls match.

Broad discovery is retained independently: it initially found 99 differing
leaves, including prototype/exposure gaps. An intermediate source-only build
reduced this to 26 before the common Body names were corrected; the final
broad discovery retains 16 differences. The fixed
focused oracle deliberately checks implemented members; the broader findings
are retained, not removed from discovery or counted as passing functionality.
Separate next-package discovery retains 75 callable-metadata differences
(length/name/constructibility/order and missing own descriptors), with zero
Chrome-control differences; these are not counted as source-finalization fixes.
The initial attempt to use about:blank with a load-waiting harness timed out
before an observation was collected. It is incomplete execution, not semantic
evidence; the succeeding local HTTP fixture runs provide the observations.

## Validation

Focused regression PASS, 0.752 s. Full browser suite PASS (435.027 s test
output; 442.146 s JSON package completion including output drain), with related
webapi/CDP/DOM/network/scheduler/Goja/QuickJS/V8 suites passing; unchanged
packages may use the Go test cache. Targeted browser race PASS, 28.850 s. The
webapi package selected no tests under that race filter and is not counted as
race coverage. Full browser race was not run; the prior Goja timeout remains
open. The complete monitored baseline/changed fast-gate pair PASSED both attempts,
including all mandatory workloads, 10/25 concurrent Pages and teardown memory.
Neither produced a native dump. There were no retries or workload exclusions.

Performance remains an open concern: throughput in this pair is 74.48 to 69.42
sessions/s at 10 Pages and 84.48 to 69.22 at 25; post-recovery private memory is
159.92 to 164.92 MiB static and 168.36 to 175.36 MiB React. DOM execution is
438.72 to 439.59 ms while completion is 469.39 to 484.31 ms. A single monitored
pair does not separate added overhead from observed run variation; profiling is
needed before claiming performance neutrality or attributing the whole delta.
All latency/concurrency/memory receipts, failures and skips remain recorded. No native-stability closure is inferred from this semantic change.

Provenance and machine-readable receipts are retained in
[native-function-finalization-20260911](native-function-finalization-20260911/).
The private originals are under
compatibility/private-captures/native-shape-20260911/; no site captures, cookies,
tokens or native dumps are added to git. Frozen Chrome mode/profile metadata is
also embedded in the focused oracle. The pre-package differential server is
the tested P0 build with runtime source equivalent to ed4261b; the performance
baseline is freshly built from a clean ed4261b worktree.
