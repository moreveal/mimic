# Historical SizeFromMap fault: instruction-level follow-up

This is additional analysis of the original 3faacc6 crash, not a fresh failure
on the current branch. See the [original lifecycle audit](snapshot-lifecycle-audit-2026-09-11.md)
and the separate [serialization write-fault investigation](snapshot-serialization-2026-09-11.md).
Historical reports retain their original scope and status.

The matching native .text identifies RVA 0x11cb34 as
`movzx eax, byte ptr [rdx + 7]`, four bytes into HeapObject::SizeFromMap.
The preserved exception registers give rdx=0xf6fff0fce; adding seven yields
0xf6fff0fd5, exactly the recorded read-fault address. The invalid access is the
initial map field read, before the function computes or returns the object size.
The map argument is already unusable at this boundary. This does not establish
which caller supplied it or whether corruption, reuse or shared heap layout
caused it. The Go stack locates CreateBlob but cannot supply the missing native
caller stack. No original full native dump is available in this evidence set.

The observed read fault must remain distinct from the reproduced
CreateFillerObjectAt write into an OS-readonly page. The latter's controlled
flag intervention does not prove the origin of this historical map argument.
A first-chance recurrence with native stack and memory is needed to identify
the object/map owner and the preceding lifecycle transition.

[Instruction evidence](snapshot-sizefrommap-20260911/instruction-evidence.json)
records raw diagnostic and code hashes and the address calculation. Raw historical
captures remain ignored. No new workload, gate, production change or causal fix
is claimed in this documentation-only follow-up; no target site ran.

The preceding semantic package 64ff17c has a passing full browser suite and
targeted race receipt. Its fresh binary SHA256 is
`fd2d377908c029c7d627fa8009f1d1c7c4ea50c7f96e4785ddfeffd8e7c962c3`.
Those checks do not close this P0. Unchanged heavy checks were not repeated for
instruction analysis and documentation.
