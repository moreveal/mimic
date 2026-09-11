# Concurrent snapshot serialization P0 checkpoint

This investigation follows semantic commit a60bdb3 and supplements, without
rewriting, the [historical lifecycle audit](snapshot-lifecycle-audit-2026-09-11.md)
and [read-only layout correction](snapshot-readonly-lineage-2026-09-11.md).
A second independent minimal failure is now reproducible during CreateBlob.
No new production workaround is needed: the existing ed4261b flag prevents this
specific failure too. Other historical signatures are not declared equivalent.

| ID | Cause or hypothesis | Repro | Before | After | Priority | Status | Correction | Limits |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| P0-RO-SERIAL | Concurrent late shared read-only-space finalization writes an OS-protected page | TestConcurrentBootstrapSnapshotSerialization; four builders, eight rounds, no restoration | Native write AV in round zero | Full rounds pass | P0 | Confirmed and covered | Existing ed4261b stock-RO-heap configuration | Exact historical SizeFromMap crash is still not reduced |
| P0-S1 | Original serializer invalid map read | Historical sweep dump | SizeFromMap + 4 during CreateBlob | No exact reduced reproduction | P0 | Open attribution | No additional fix claimed | Same API boundary is not proof of same cause |

The minimal fixture builds four distinct plain object graphs concurrently, then
closes their snapshot blobs. It never creates a consumer runtime, browser Page,
network request, site capture or code-cache consumer. Eight rounds exercise 32
builds. The serial control executes the same graphs and count without overlap.
No runtime lock, retry or snapshot-disabling production path was added.

Clean 71d54f0: serial PASS; concurrent FAIL (exit 2) in the first round. Current
engine: serial and concurrent PASS. A second diagnostic executable built from
71d54f0 accepts only the stock-RO-heap flag as an intervention. Three predeclared
pairs on that same executable reproduce three default-mode native write faults
and three complete `--no-extensible-ro-snapshot` passes. Every attempt has its
own first-chance collector and retained stderr; no failed attempt was discarded.
The two variants use the same graph/topology, and a fresh process for each run.

The first-chance PC resolves using the matching native DLL text section to:

```
Heap::CreateFillerObjectAt + 0xae
ReadOnlySpace::RepairFreeSpacesBeforeSerialization + 0xa9
ReadOnlyHeap::OnCreateHeapObjectsComplete + 0x2d
SnapshotCreatorImpl::CreateBlob + 0x449
gov8_snapshot_create_blob + 0xe1
```

The faulting instruction is `mov qword ptr [rsi], rax`. The target address belongs
to a V8 read-only page (flag 0x400), and the dump records OS PAGE_READONLY (0x2).
Thus this is a write into an already protected page, unlike the earlier retained
string-pointer faults on PAGE_READWRITE. Source inspection places filler repair
before read-only heap finalization in CreateBlob. The non-extensible mode seals
the stock read-only heap during initial setup and avoids that late extension
path. The dump does not identify the exact peer instruction that protected this
page; it does establish the invalid write, its serialization boundary and the
controlled flag intervention. It does not by itself prove the historical
SizeFromMap read fault has this same causal history.

The native DLL is V8 15.2.124.1-rusty, not Chrome's 15.2.124.21. Its loaded and
relinked .text SHA256 both equal
`a36943141cfa80d45bda17dc9ce6c3b72a793ef05f7cc5ff664c650990206114`.
[Evidence](snapshot-serialization-20260911/) includes build identities, raw hashes,
all ten diagnostic outcomes, source fixtures, native frames, memory protection,
regression output and source hashes. Raw dumps remain ignored locally.

The new regression runs in a fresh child so a native failure preserves stderr
without terminating unrelated tests. Baseline regression FAILS; current three
fresh regression children PASS. Full V8 engine suite PASS (0.939 s); full engine
race PASS (4.216 s). An initial baseline command matched no test because fixture
copying used the wrong working directory; it is retained privately as an empty
filter error and excluded from validation. The corrected baseline is the failure
reported here. One diagnostic build initially used the wrong module directory;
it was rebuilt from the verified baseline before any controlled run.

Production runtime code is unchanged from a60bdb3. Its just-completed full browser
suite passed (473.756 s), targeted browser race passed, and both complete paired
fast gates passed 10/25-Page concurrency and teardown memory with no native dumps.
Its fixed differential/control and affected corpus receipts remain applicable;
this test-only package does not claim new Chrome semantic matches. No unchanged
full browser or performance gate was rerun for a test/documentation-only change.
The regression and repeated flag controls add evidence beyond those passing gates.
P0 remains open for historical signatures not demonstrated by either minimal
reproducer, and Goja retained-srcdoc race timeouts remain separately open.
