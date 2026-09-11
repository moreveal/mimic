# Concurrent numeric snapshots: promotion and repair write faults

This extends the [serial numeric negative control](snapshot-numeric-boundary-2026-09-11.md).
It supplies fresh native evidence after the semantic package; it does not close
the [historical SizeFromMap read fault](snapshot-sizefrommap-2026-09-11.md).

The same standalone binary, built from 71d54f043fe55ab4ee3cee188862d4e178bd143f,
runs three graph shapes with default flags and with
`--no-extensible-ro-snapshot`. Four builders share a startup barrier. Each performs
eight build/restore rounds, growing numeric graphs from 1,000 to 8,051 entries.
All previously restored runtimes and snapshots remain live until every builder
finishes. Each round checks its retained graphs' size and selected terminal value;
this is not an exhaustive graph integrity check. Shapes are arrays/objects,
class instances with a method, and Map/Set tables. No unique-key string workload
or browser/site capture is needed.

Binary SHA256 is `cc98e0a415e4f5cb8a23b139abd4197bb0e1eac97662b72824fd6bd823991617`.
Source SHA256 is `2ce743f5020cac1d9633e9c23a8fe1a72ed25b519dec8b6e50977116c78c09fb`.
The loaded and symbol-matched DLL .text hashes agree:
`a36943141cfa80d45bda17dc9ce6c3b72a793ef05f7cc5ff664c650990206114`.

All three default-flag processes crash in their first creation round, before
any RESTORE log, and exit 2. All three flag-enabled processes complete every
round and exit 0. None times out. ProcDump was attached before releasing the
startup barrier; each failing process has a first-chance full native dump.

| Shape | Native instruction | Call chain | Destination |
| --- | --- | --- | --- |
| Arrays and tables | RVA 0x5e4be, CreateFillerObjectAt+0xae: mov qword ptr [rsi], rax | RepairFreeSpacesBeforeSerialization → OnCreateHeapObjectsComplete → SnapshotCreatorImpl::CreateBlob | Committed PAGE_READONLY region, V8 read-only-heap flag 0x400 |
| Classes | RVA 0x5d53d1, ReadOnlyPromotionImpl::UpdatePointersVisitor::ProcessSlot+0x141: mov qword ptr [r8], r9 | VisitPointers → ReadOnlyPromotion::Promote → SnapshotCreatorImpl::CreateBlob | Committed PAGE_READONLY region, V8 read-only-heap flag 0x400 |

The first signature repeats the [previous serialization diagnosis](snapshot-serialization-2026-09-11.md).
The second localizes a distinct promotion write into an already sealed read-only
page. Both are prevented in this controlled matrix by the existing production
flag change ed4261b. The smaller serial/default controls passed; this comparison
also changes retained graph count and is not a one-variable concurrency experiment.
The native writes support the shared custom read-only snapshot
state boundary, without attributing every historical read fault to these writes.

`TestConcurrentNumericSnapshotsPreserveLiveGraphs` retains a class-based regression
in a fresh subprocess. It fails on the pre-fix baseline (child exit 2, additional
read AV with no native dump in that test invocation), passes three fresh production
children (0.734 seconds overall), and passes with the full V8 race suite (5.256
seconds). The undumped baseline test's read signature remains unclassified; it is
not relabeled as ProcessSlot or SizeFromMap. No global lock, retry or snapshot
disable was added. This commit adds evidence and regression coverage, not another
production fix.

[Evidence and reproducible source](snapshot-numeric-concurrent-20260911/) preserve
build identity, exact instructions/registers, page protection, native unwind and
dump hashes. Raw stderr, exit codes, debugger logs and dumps remain ignored under
compatibility/private-captures/p0-numeric-concurrent-20260911. The original
SizeFromMap signature still needs a reduced recurrence with its caller and memory.
