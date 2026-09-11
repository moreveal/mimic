# Concurrent numeric snapshot evidence

This separate P0 checkpoint preserves the completed three-before/three-after
experiment in [evidence.json](snapshot-numeric-allocator-20260911/evidence.json).
Each child retains four concurrent builders' numeric/class graphs for eight
rounds. Probe hashes, executable hashes, revisions, topology, debugger commands,
dump hashes and resolved native frames are recorded there. Raw dumps and stderr
remain in ignored `compatibility/private-captures/p0-numeric-test-matrix-20260911`.

All three baseline children fail in the first round. Two stacks stop in
`ReadOnlySpace::EnsureSpaceForAllocation`, with allocator fields containing the
freed-memory pattern `feeefeeefeeefeee`; the third stops in
`ReadOnlySpace::AllocateRaw` with `baadf00dbaadf00d`. All unwind through read-only
promotion and `SnapshotCreatorImpl::CreateBlob`. The three corrected children
complete all 32 build/restore operations and exit zero. The compared native text
section hash is identical. The existing `--no-extensible-ro-snapshot` startup
fix and numeric regression remain enabled; no global runtime lock or retry was
introduced.

The before collector exits with code one after collecting its first dump. That
is **not** an observed native child exit code; the original child status is not
available from this launch mode. The exact allocator free site remains untraced.
These results support the read-only allocator ownership diagnosis for this
reproduction, but do not prove that every previously observed snapshot failure
has that cause. They do not close the wider snapshot P0 by themselves.

The later semantic package's complete V8 race tests pass. Its first performance
pair stops at the host-memory guard before the concurrent workloads execute,
with live child processes and no native dumps. That is an incomplete workload,
not evidence of a new native crash. See the current
[package report](platform-bindings-2026-09-11.md) for the final workload status.
