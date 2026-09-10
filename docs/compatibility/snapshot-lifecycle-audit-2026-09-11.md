# Snapshot lifecycle native crash investigation, 2026-09-11

Status: **open; no production fix claimed**. Browser snapshot/owner code and
the native DLL have not been changed as part of this investigation. Serial
lifecycle coverage and an opt-in concurrent reproducer were added.

## Evidence

The bulk compatibility sweep on `3faacc6` completed 15 independent fresh-page
probes before its shared Mimic process crashed. The 119 following disconnected
probe attempts are consequences of that one process failure, not 119 defects.
The fault occurred while a background bootstrap snapshot was being serialized;
the next probe name does not establish the failing API.

The original native exception was `0xc0000005` in
`SnapshotCreator.CreateBlob` / `buildBootstrapSnapshot`. Linking the **existing**
shim object and pinned V8/Temporal libraries with `/MAP` produced a DLL with an
identical `.text` section. The original address resolves to
`v8::internal::HeapObject::SizeFromMap + 4` (RVA `0x11cb34`). This identifies an
invalid heap-map read during serialization; it does not identify where the
invalid reference originated.

A bounded concurrent test using four independent browser Contexts, each
creating/navigating/closing 32 Pages, subsequently reproduced a native fatal
error: `Check failed: index < size()`. Its stack resolves to
`StringForwardingTable::GetRawHash` → string internalization → `JSON.parse` →
ordinary `EvalBootstrap`. This is a second failure signature, **not proof that
the original serializer crash and this failure have the same cause**.

## Controlled runs

| Configuration | Result |
|---|---|
| Minimal engine snapshot create/restore/close, 100 repetitions | Pass |
| Browser Page lifecycle, 32 Pages × 20 serial repetitions | Pass |
| Original first 20 CDP probes × 4 in one process | 80 pass |
| Four concurrent Contexts × 32 Pages, snapshots enabled, `-count=3` | Native fatal within about 2 seconds; run aborted |
| Same concurrent topology, factory explicitly disables snapshots | Pass |
| Concurrent snapshot topology, `-race -count=1` | Pass; no Go race reported |
| Concurrent Pages, test-only sequential snapshot builders, `-count=3` | Pass |
| Concurrent Pages and builders, subsequent `-count=3` confirmation | Pass |
| Barrier starts four builders together, concurrent builders, `-count=3` | Pass |
| Same start barrier, test-only sequential builders, `-count=3` | Pass |

These results establish an intermittent native failure under the concurrent
snapshot configuration. The ordinary control and race run are bounded
observations, not a proof of absence. No global runtime lock, snapshot disabling,
or serializer retry has been introduced to mask the failure.

The diagnostic gate wraps only `BuildBootstrapSnapshot` in the **test factory**;
ordinary Page execution, runtime creation/restore and disposal remain concurrent.
The start barrier ensures four build requests are admitted together. Because both
gated and ungated followups passed, these experiments do **not** establish that
overlapping builders are the cause, and do not justify a production lock.

## Ownership checks

The current creator runs on its own locked OS thread. Its handle scope and
context wrapper close before `CreateBlob`; `SetDefaultContext` retains the
context in V8 ([upstream implementation](https://chromium.googlesource.com/v8/v8.git/+/refs/heads/lkgr/src/snapshot/snapshot.cc)). Cancellation watchers are joined before serialization. The
native creator consumes its isolate after serialization, and consumer startup
blob copies are retained for each consumer isolate's lifetime. Reviewing these
boundaries did not yet establish the source of the invalid reference.

The pinned shim's `v8-gn.h` disables pointer compression; no multiple-cage flag
change was tried. Snapshot shim state consists of per-creator/per-blob wrappers
and the initialized process platform/ArrayBuffer allocator. No mutable shared
snapshot lookup table was found in that wrapper. This does not exclude an
upstream V8 shared-state issue or corruption caused elsewhere in the embedder.

## Reproduce

Normal regression:

```powershell
go test ./internal/browser -run '^TestBootstrapSnapshotPageLifecycle$' -count=1
```

The concurrent reproducer is deliberately opt-in because it can terminate its
test process in native code. It is not a passing CI regression for a fixed bug:

```powershell
$env:MIMIC_SNAPSHOT_LIFECYCLE_STRESS = '1'
try {
    go test ./internal/browser -run '^TestBootstrapSnapshotConcurrentPageLifecycles$' -count=3 -timeout=90s
    go test ./internal/browser -run '^TestOrdinaryConcurrentPageLifecycles$' -count=1 -timeout=60s
    go test ./internal/browser -run '^TestBootstrapSnapshotBarrierConcurrentBuilders$' -count=3 -timeout=90s
    go test ./internal/browser -run '^TestBootstrapSnapshotBarrierSequentialBuilders$' -count=3 -timeout=90s
} finally {
    Remove-Item Env:MIMIC_SNAPSHOT_LIFECYCLE_STRESS
}
```

Local evidence is retained under
`compatibility/private-captures/audit-20260911-3faacc6/` (original
`mimic-sweep.err`, full sweep manifests and raw CDP results), and
`.build/snapshot-*.log` / `.build/snapshot-symbols/gov8.map` for the bounded
followups. The audited binary SHA256 is
`a80e2f47120b3b82c79c363f904707d5fe3df1146e4304cbee598eb63c86eedc`;
its native shim SHA256 is
`919522b4d4ed80671586a1b7a0efc144a533e91e08319715e76ed27962cee0ff`.

The next investigation should capture a native dump at the first invalid
heap/string-table access and separate snapshot construction, restore, and
ordinary code-cache consumption. A cause-specific fix requires that distinction;
the current evidence does not justify serializing all Pages or changing browser
semantics.
