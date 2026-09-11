# Snapshot stability control during compatibility cleanup

P0 remains **open**. This is a new bounded control, not a reinterpretation of
[the earlier native failures](snapshot-lifecycle-audit-2026-09-11.md) or the
unclassified 25-Page fast-gate disconnect in `037f8ff` development.
No production snapshot or locking changes were made.

After the semantic browser suite and CDP comparisons finished, the existing
opt-in reproducer ran sequentially against baseline and corrected test binaries:

| Topology, per build | Baseline runtime 037f8ff | Corrected runtime |
|---|---|---|
| Four Contexts × 32 Pages, snapshots, count=3 | exit 0 | exit 0 |
| Four Contexts × 32 Pages, ordinary initialization, count=1 | exit 0 | exit 0 |
| Four builders start at a barrier, snapshots, count=3 | exit 0 | exit 0 |

The ordinary initialization case is the existing test-only diagnostic control;
snapshots were not disabled in production or in the snapshot cases. There was
no retry, global runtime lock or workload exclusion. All six native test
processes completed; no new failure signature or native stack was produced.
These finite passes do not establish a cause, a fix, or a failure-rate bound.

[Machine receipts](cleanup-20260911/snapshot-results.json) record command,
test executable SHA256, duration and exit code for every process. Complete
stdout/stderr logs remain under
`compatibility/private-captures/cleanup-20260911/{before,final}-Test*.log`.
The baseline test executable was compiled before the later XHR adjustment, with
a Go overlay restoring both then-modified runtime inputs (`generated/surface.js`
and `fetch_primitives.js`) from `git show 037f8ff:path`. Its added inheritance test
was not selected. Thus the selected baseline runtime is unchanged; the corrected
test executable includes the final XHR adjustment. Each executable was run from
the same browser package working directory with the same native dependency and
`MIMIC_SNAPSHOT_LIFECYCLE_STRESS=1`.

The subsequent unchanged fast gate **failed on clean 037f8ff**, in the first
measured 10-Page static concurrency wave (after a valid warm-up wave). The process
exited with code 2. Retained stderr starts with
`Exception 0xc0000005 0x0 0x7d 0x7ffc7feaeed8`, followed by
`signal arrived during external code execution` and a truncated cgocall/syscall
stack. PC is an ASLR address, not a resolved function. It must not be labeled a
serializer or StringForwardingTable crash from this evidence.

The failed process log and exit receipt are retained as
`gate-before/process-logs/37918.{log,json}` in the private capture directory;
the build has clean status and exact SHA256 in `gate-before/build.json`.
This was not retried or converted to a pass. A separate final-build gate runs
the same unchanged workloads; its result cannot establish that this intermittent
baseline failure is fixed. See the performance report for that result.

No native dump was captured. `procdump`, `cdb`, and `windbg` were not available
on PATH, and the standard Windows Kits x64 cdb location was absent. The short
stderr stack is insufficient to localize the native PC. A future diagnostic run
needs a first-chance native debugger/dump collector; installing one or changing
global crash-dump policy was not part of this bounded semantic package.

Next diagnostic work remains first-failure dump capture and separation of
snapshot construction, restoration, and ordinary bootstrap/code-cache execution.
`HeapObject::SizeFromMap` during serialization and
`StringForwardingTable::GetRawHash` during ordinary bootstrap remain distinct
historical signatures; these controls provide no evidence that they share a cause.
