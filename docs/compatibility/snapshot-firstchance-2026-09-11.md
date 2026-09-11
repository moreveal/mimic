# Native first-chance capture, 2026-09-11

P0 remains open. This diagnostic follow-up supersedes only the absence of a dump
in [the preceding control report](snapshot-cleanup-control-2026-09-11.md).
It does not establish a repair or a common cause for the historical serializer
and StringForwardingTable signatures.

## Reproduction

The unchanged full fast gate was monitored with Microsoft ProcDump 12.01,
`-ma -e 1 -f C0000005 -n 1`, attached to each owned Mimic process before workloads.
The executable's Authenticode signature was valid (Microsoft); SHA256 is
`d1fc99ae304bd1d2bf28abeb62531da959e2431916194981b88c958fd713a8e6`.
No global dump policy, runtime lock, snapshot switch, workload exclusion or
failure retry was introduced. Debugger timing prevents performance conclusions.

Five complete monitored gates on clean `d50d3ec` passed without an exception.
The first subsequent gate on clean `037f8ff` failed in the first measured
10-Page static wave, after its successful warm-up. The gate exited 1 and the
Mimic process exited 2. The baseline binary SHA256 was
`dbda73d51636a58a77fe23fdfe44a9da7b9bc50c28f197897878e873d033f556`.
These are finite observations, not a failure-rate comparison or proof that the
semantic changes fixed native memory corruption. Both used the same workload
sequence, native library and collector; execution was sequential.

The first collector attempt failed before workload execution because its
readiness parser did not handle mixed ASCII/UTF-16 output. It is not a gate pass.
After correcting only the private collector, a separate deliberately crashing
Python child verified first-chance collection. That synthetic dump is not Mimic
evidence. All runs and their identities are in [receipts](snapshot-firstchance-20260911/receipts.json).

## Captured evidence

The real 401,148,508-byte full dump, stderr, exit receipt, build manifest and
unchanged gate raw output remain ignored under
`compatibility/private-captures/snapshot-firstchance-baseline-20260911-run1/`.
The native exception is `C0000005`, read address `0x6874676e70`, at
`gov8_shim.dll + 0x1360d5`. The loaded DLL SHA256 is
`919522b4d4ed80671586a1b7a0efc144a533e91e08319715e76ed27962cee0ff`.
A symbol relink using the existing pinned objects has an identical `.text`
SHA256 (`a36943141cfa80d45bda17dc9ce6c3b72a793ef05f7cc5ff664c650990206114`).
The RVA resolves to `String::SlowEqualsNonThinSameLength + 0x135`.

Manual x64 unwind using the dump's PE unwind records reaches:

```
String::SlowEqualsNonThinSameLength (guard overload)
String::SlowEqualsNonThinSameLength
StringTable::TryLookupKey<InternalizedStringKey>
StringTable::LookupKey<InternalizedStringKey>
StringTable::LookupString
Runtime_InternalizeString
Builtins_CEntry_Return1_ArgvOnStack_NoBuiltinExit
Builtins_MapPrototypeSet
Builtins_InterpreterEntryTrampoline (two frames)
Builtins_JSEntryTrampoline / Builtins_JSEntry
Invoke / Execution::Call / Function::Call
gov8_function_call_ctx
```

Go stderr independently identifies `Function.Call → evalScopedCode →
EvalBootstrap → runContext → Runtime.loop`. This capture is ordinary bootstrap
execution, not an observed `CreateBlob` failure. The pinned native engine is
15.2.124.1-rusty; it is not Chrome's exact V8 patch revision.

At the fault, the candidate string pointer is `0x896deb6479`. Its untagged
address contains `ength\0\0\0`, inside the payload of `storageLength`, followed
by valid-looking objects containing `frameResolve` and `createWindowObject`.
The other operand contains `WebkitBorderImage`, length 17. The failing instruction
loads a map from the candidate then reads its instance type. The candidate lies
in a 256 KiB private memory region, not a captured thread stack.

The caller's machine code checks hash and length before this call, yet the
captured candidate no longer has a string header. Concurrent reuse, stale roots
or another overwrite are hypotheses; the dump does not identify the writer.
The ASCII payload is not evidence of a string conversion defect. No speculative
runtime fix was applied.

## Remaining investigation

Capture allocation/GC ownership for the implicated heap page and string-table
entry, including other isolate threads and snapshot creator lifetimes. Reduce
the fixed concurrent topology while retaining the failure, then compare an
actual ownership correction against the same baseline load. Shared code-cache
bytes, native handle lifetimes and artifact/header ABI boundaries still need
discrimination. Historical `SizeFromMap` and `StringForwardingTable::GetRawHash`
failures remain separate signatures until causal evidence links them.
