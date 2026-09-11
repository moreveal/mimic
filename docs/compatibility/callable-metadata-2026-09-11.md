# Callable metadata after semantic installation - 2026-09-11

This package starts at signed semantic commit `706fc15`, following
[native source finalization](native-function-finalization-2026-09-11.md).
It addresses another part of the historical W02 finding without claiming
receiver, conversion or capability completeness. The separate native P0
correction remains `ed4261b`; its historical signature limits still apply.

## Cause and implementation

Late semantic installers replace functions after the frozen exposure pass has
set WebIDL metadata. The replacements can regain JavaScript-inferred names,
implementation parameter counts and an unintended [[Construct]] capability.
Existing frozen metadata already identifies the proper names and lengths.

A final callable pass now visits only present global functions and own
prototype bindings in that metadata. It normalizes names and lengths, and uses
concise-method forwarding for prototype operations/accessors that expose a
JavaScript prototype property. The wrapper has no [[Construct]], forwards the
original receiver/arguments and uses captured Reflect.apply. A per-original
WeakMap preserves aliases within the pass. It does not replay publication,
create absent members or rearrange prototype ownership. The pending exposure
reference is cleared after initialization; this does not claim to repair the
separately documented strong cross-realm bridge caches.

| ID | Root cause / hypothesis | Repro | Chrome | Mimic before | Priority | Status | Fix / commit | Limits |
|---|---|---|---|---|---|---|---|---|
| W02-metadata | Late replacement loses callable metadata and exposes JS construction | callable_metadata_oracle.js; brand matrix | frozen arity/names; operations reject newTarget | implementation arity/names; ordinary functions accept construction | P1 | corrected, validation below | this package | Existing generated bindings participate; no new API functionality is implied |
| W02-static-order | Static declarations installed in implementation order | AbortSignal / Response own keys | abort/any/timeout; error/json/redirect | abort/timeout/any; error/redirect/json | P2 | confirmed remaining reflection difference | open | Separate from corrected constructor length |
| W04-Headers | Extra FetchHeaders prototype layer | broad callable metadata discovery | expected methods own | several methods inherited | P2 | confirmed remaining reflection difference | open | Not missing callable implementations |
| W04-stream | ReadableStream.from exposed outside frozen inventory | constructor own keys | absent | present | P2 | remaining exposure difference | open | Vendor behavior is not frozen capability evidence |
| W01-getters | Remaining getter receiver/ownership differences | brand matrix and focused owner probes | branded getters | mixed missing/incorrect behavior | P1 | needs cause-level triage | open | 609 records are not 609 root causes; unsupported candidates are lower priority |
| P0-signatures | Historical native failures beyond reduced distinct-layout repro | separate native reports | n/a | captured failures | P0 | attribution remains open | ed4261b fixes demonstrated layout defect | A passing semantic gate is not a blanket closure |

## Before / after

The focused oracle goes **43 normalized differing leaves to zero**, with zero
Chrome-to-Chrome differences. It checks the existing methods/accessors, names,
lengths, own function keys and constructibility, plus real receiver forwarding,
captured-apply behavior and intrinsic iterator identity. Ordinary and restored
snapshot regression modes pass. Reflection, iterator and foreign-property
mutation focused tests also pass (combined targeted run 4.093 s).

The unchanged 236-probe set goes **203 to 205 matches**, fixing the full
performance.addEventListener and screen.addEventListener observations. Original
representatives go **18/28 to 19/28**. There are no new differing leaves,
regressed matching probes or Chrome drift. AbortSignal constructor length is
corrected but its probe still diverges on static property order; it is not
counted as a fully fixed probe.

Brand matrix goes **1,309 to 609 differing records**, with no added or changed
remaining differences. The 700 removed records comprise 220 operations with
three construction/own-key records each, 32 name records and eight length
records. These include already exposed generated bindings as well as
implemented operations: they do not establish 220 newly implemented APIs or
700 independent defects.

General corpus stays **12/15 matching**, with **41 to 40 records**: FontFaceSet
forEach.length is now correct. Network remains 12 records, surface six,
observations 22. State-relations retains two. All corresponding Chrome controls
have zero differences. These overlapping corpora are not summed. The separate
broad callable discovery goes **75 to 20 differing leaves**, retaining missing
own descriptors, absent members and static order/inventory differences.

## Validation checkpoint

Full browser suite PASS (515.399 s test output, 522.892 s JSON package completion
including output drain); related webapi/CDP/DOM/network/scheduler/Goja/QuickJS/V8
suites PASS, with unchanged packages eligible for Go's test cache. Targeted
browser race PASS, 36.364 s. The webapi race filter selected no tests and is not
counted as coverage. All existing skips remain in the receipt.

Both complete monitored baseline/changed fast gates PASS, with every mandatory
workload, 10/25-Page concurrency and teardown-memory waves executed. Neither
produced a native dump; no retry or workload exclusion was used. Throughput is
73.01 to 64.62 sessions/s at 10 Pages and 69.98 to 62.49 at 25. Static private
memory after recovery is 150.80 to 159.03 MiB; React is 170.87 to 169.02 MiB.
DOM execution/completion is 443.43/476.26 to 450.52/506.93 ms. This remains an
open performance concern, not a neutrality claim or proof that every delta is
caused by one implementation change. The complete suite also took longer than
the previous package, without a controlled suite-timing attribution.

Official fuzzer: 205/236 matches, 31 divergent probes, 9 observational groups,
zero unstable/errors; discovery stays 3,533 with limit 100. Separate sweep stays
114/130, with 11 candidates, 3 environment/timing reviews, 2 exception
mismatches and 36 unchanged skipped entries. Independent profiling of the
previous package's performance concern is queued after that pair, against its
own clean before/after revisions, without changing the frozen gate.
Full browser race is not claimed; the prior Goja retained-srcdoc timeout remains
open. No target site was run and no frozen expectations were weakened.

Machine-readable evidence is retained under
[callable-metadata-20260911](callable-metadata-20260911/); local originals are in
compatibility/private-captures/callable-metadata-20260911/. The frozen oracle
contains exact Chrome headful version/profile metadata. The pre-package server
has runtime source equivalent to 706fc15; the performance baseline is freshly
built from a clean 706fc15 worktree. Native dumps and site/session data are not
committed.
