# Snapshot numeric-graph negative control

This follow-up tests a narrower boundary of the still-open historical
[SizeFromMap fault](snapshot-sizefrommap-2026-09-11.md). It does not supersede
the distinct [serialization write-fault diagnosis](snapshot-serialization-2026-09-11.md).

The diagnostic binary was built from 71d54f043fe55ab4ee3cee188862d4e178bd143f,
before the production read-only snapshot flag change. The worktree also contained
an untracked serialization regression test; it was not part of this standalone
binary. Binary SHA256:
`f284bea693974ad2d5d6dacef5db7438ac2ac551e300f93bcb664767cf3ed1cb`.
Diagnostic source SHA256:
`b16ebca64d842d63f4c5d0ed628854e1febeca578a33cc8610858827b9850867`.

Six fresh processes exercised three graph shapes, once with default flags and
once with `--no-extensible-ro-snapshot`, using the same binary. Shapes were
nested numeric arrays/objects, instances with a class method, and Map/Set tables
with numeric keys. Each process performed eight serial build/restore rounds,
growing from 1,000 to 8,000 entries. Earlier snapshots and runtimes remained live.
Each round checked every retained graph's length/size and selected terminal
value; this was not an exhaustive graph integrity check. All objects were closed
after the last round. The diagnostic had a 60-second context deadline.

ProcDump first-chance collection for C0000005 and 80000003 was attached before
releasing each process's startup barrier. All six processes exited 0 with PASS,
without timeout or native dump. Raw source, identity, process output, debugger
output, and individual results remain ignored under
`compatibility/private-captures/p0-numeric-matrix-20260911/`.

This serial numeric-graph topology did not reproduce a failure even with the
old default flags. It therefore supplies no before/after crash reduction and
does not explain the original invalid map argument. Concurrent builders,
unique-string stress and browser lifecycle workloads are separate topologies.
The original SizeFromMap signature remains open pending a reduced recurrence
with native caller stack and memory. No production code or frozen expectation
changed, and no target site ran.
