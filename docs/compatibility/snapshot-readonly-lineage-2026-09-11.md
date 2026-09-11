# Snapshot read-only layout ownership

This separate P0 package follows semantic commit `71d54f0` and the
[first-chance investigation](snapshot-firstchance-2026-09-11.md). It does not
retroactively relabel every historical native crash as one proven cause.

## Confirmed defect and correction

Two independently built snapshots containing different property-name strings
can crash when both are restored in one process. The reproducer creates two
plain JavaScript object graphs, builds their snapshots, keeps both restored
isolates alive and reads their keys. No browser, HTTP server, site, code-cache
consumer or concurrent Page workload is required.

The pre-fix regression test fails in a fresh child process, exit 2, with
`CHECK(index < size())`. The captured native chain is:

```
StringForwardingTable::GetRawHash
StringTableInsertionKey::StringTableInsertionKey
StringTable::InsertForIsolateDeserialization
SharedHeapDeserializer::DeserializeStringTable
SharedHeapDeserializer::DeserializeIntoIsolate
Isolate::Init / InitWithSnapshot
Snapshot::Initialize
v8::Isolate::Initialize / New
gov8_cps_isolate_new
```

The pinned V8 default isolate group shares ReadOnlyArtifacts between isolates.
Custom snapshot promotion can extend the shared read-only graph. Independently
customized images then contain references to incompatible layouts, while
release V8 omits the debug-only ReadOnlyArtifacts checksum assertion. The
controlled contrast is different snapshots failing, same-source snapshots
passing, and both cases passing with `--no-extensible-ro-snapshot`.

Initialization now sets that flag once, before V8 initializes. The common
read-only heap keeps the stock layout; custom bootstrap objects remain in each
snapshot's private serialized heap. Snapshots, code caching and parallel Pages
remain enabled. No runtime lock, retry, snapshot fallback or workload exclusion
was added. A subprocess regression protects the native-fatal boundary and
checks restored keys and coexistence with an ordinary isolate.

Snapshot header checksum values are logged only as diagnostics. Their equality
is not the test assertion and differing hashes alone are not the evidence of
this defect; restored semantic state and the controlled native failure are.

## Evidence chronology and boundaries

| ID | Observation | Classification | Status / scope |
|---|---|---|---|
| P0-RO | Distinct independently built snapshots fail on simultaneous restore; same-source and sealed-RO controls pass | Confirmed native ownership defect | Corrected; full validation recorded below |
| P0-S1 | Clean 037f8ff gate: SlowEqualsNonThinSameLength reads invalid string header | Native failure, captured | Invalid reference lies in read-only heap; causal link to P0-RO is plausible, not independently proven |
| P0-S2 | Clean 8780592 gate: MakeThin reads invalid internalized-string header | Native failure, captured | Second read-only-heap invalid reference; same limitation on causal attribution |
| P0-S3 | Historical SizeFromMap during CreateBlob | Native failure, historical signature | Not independently reduced or causally linked here |
| P0-S5 | Clean 71d54f0 gate: LookupString reads text bytes as a string map during Object.defineProperty | Native failure, captured | Invalid reference in read-only heap; consistent with layout mismatch, not independent proof of the writer |
| P0-S4 | Historical StringForwardingTable::GetRawHash | Native failure, historical signature | The new reduced repro reaches this function during restore; matching a function name does not prove identical causes for every older occurrence |

The clean 8780592 gate failed on the first measured 10-Page static wave after
a valid warmup; all ten CDP connections broke. Gate exit 1 and Mimic exit 2,
stderr and a 389,882,223-byte first-chance dump were preserved. Its binary
SHA256 is `c0caa775d985b023a63240e30532a33555e797964caf05254df7b6ba4fd73c91`.
The pre-P0 semantic build's subsequent complete passing gate did not close P0.

The original dump's candidate `0x896deb6479` lies inside `storageLength` data;
the second dump's `0x13f33476491` points to a word containing `0x30` immediately
before another string. Both 256 KiB pages have read-only-heap flag `0x400`.
This was checked against the loaded native Contains instructions, not inferred
from virtual-address appearance. Both pages are OS PAGE_READWRITE at capture:
V8 heap classification is distinct from OS protection. The first dump also
contains four non-stack copies of the invalid tagged pointer and concurrent
code-cache deserializers. Exact pointer matches and partial manual unwinds do
not establish which thread originally wrote the bad reference.

The initial smaller control that alternated a snapshot-backed and ordinary
isolate, with and without code caching, passed. It did not reproduce the
failure. The two-distinct-snapshot variant then failed repeatedly. The first
CHECK capture had stderr only because ProcDump ignores breakpoints by default;
the same pre-fix binary was subsequently captured with its documented `-b`
option. This was a diagnostic capture, not a retry counted as a passing gate.

Native symbols use a relinked map from the pinned object files. Loaded and
relinked `.text` SHA256 match:
`a36943141cfa80d45bda17dc9ce6c3b72a793ef05f7cc5ff664c650990206114`.
The engine is V8 15.2.124.1-rusty, distinct from Chrome's 15.2.124.21.

## Validation checkpoint

Before regression: FAIL in native code. After: three fresh subprocesses PASS,
checking both retained snapshots and an ordinary isolate. Full browser suite
PASS, 415.099 s; related engine/browser infrastructure packages PASS. Fixed
corpus remains 202/236, original representatives 17/28, with no new differing
leaves, regressed matches or Chrome drift. General, brand, state-relations and
navigation observations retain the preceding semantic baseline, with clean
corresponding Chrome controls.

The official fuzzer retains 202/236 matches and 11 observational groups;
discovery remains 3,533 surface probes with the same 100-probe limit. The
separate sweep retains 113/130 matches (12 candidates, 3 environment/timing
reviews, 2 exception mismatches). These groups are not proven root causes.

Fresh fixed Chrome-to-Chrome control: 236/236 and all 28 representatives
match. General corpus: 12/15, 41 diff records; brand: 1,319 records;
state-relations: 2 records. These overlapping corpora are not summed or counted
as root causes. Their separate Chrome controls have zero differing records.

Five existing lifecycle topologies each ran three times on baseline and changed
builds: concurrent lifecycles, barrier/concurrent builders, barrier/sequential
builders, concurrent Pages/sequential builders, and ordinary Pages. All ten
processes passed without timeout. Test-only serialized-builder controls are not
production synchronization. These stress processes captured stderr/native
stacks, without first-chance debugging.

Full V8 engine race suite PASS (2.713 s). The broader targeted race command
FAILED: TestIframeSrcdocRetainsExportedDescendant/goja exceeded its 20 s context.
An isolated control on clean 71d54f0 passed (17.213 s), while the changed-tree
isolated run failed (22.641 s). The V8 initialization flag does not execute in
that Goja-only subtest. This timing-sensitive validation failure remains open;
no deadline was increased and no retry converted it into a pass. Full browser
race was not run. Ordinary full-suite stress skips are listed in the receipt;
the lifecycle tests were explicitly enabled in the paired runs above.

The same complete fast gate and first-chance collector ran once on clean
71d54f0 and three predeclared times on the changed build. Baseline FAILED at the
first measured 10-Page static wave after valid warmup (gate exit 1, Mimic exit
2). All three changed gates PASSED mandatory correctness, warm workloads,
10/25-Page concurrency and static/React teardown-memory waves. No changed run
produced a native dump; no workload was removed or retried. Passing finite
samples do not prove absence of every historical native failure.

The new 408,324,557-byte baseline dump reaches LookupString+0x5c through
PropertyKey, OrdinaryDefineOwnProperty, DefineOwnProperty, DefineProperty and
Builtin_ObjectDefineProperty. Candidate 0x30fa2404fc9 points into string data;
the supposed map word is 0x636e657571657246. Its page has V8 read-only flag 0x400
and OS PAGE_READWRITE. The first-chance stack is preserved separately from the
later fatal stderr stack; they are not assumed to identify the same instruction.

Tested baseline SHA256:
`14fcc636d3942b65728a2608ba8dd243adca730e226939167d8b404b20bb3feb`.
All three changed gates and differential server used SHA256:
`a01e6602eb732cc1b7c8a8e53f69dbbac34975ca937dff6b4065236e7b53d867`.
The build receipt records 71d54f0 plus the owned engine flag and regression-test
changes. Chrome remained frozen 152.0.7977.82, headful profile
cleanup-bindings-20260911; the sweep also uses its dedicated headless profile.

[Compact evidence, hashes, failures and complete gate receipts](snapshot-readonly-lineage-20260911/)
retain the exact execution status. [Performance checkpoint](../performance/report.md)
records latency, throughput and retained memory together. Baseline crash leaves
its concurrency/memory comparison incomplete; no performance-neutrality claim
is made. P0-RO is corrected within the demonstrated scope. P0-S1 through P0-S5
remain separately tracked, particularly the unreduced CreateBlob signature.

Ignored local evidence roots:
`compatibility/private-captures/p0-distinct-layout-20260911/`,
`p0-ro-lineage-20260911/`, `p0-ro-lineage-paired-20260911/`,
`p0-ro-lineage-check-dump-20260911/`, and
`location-paired-before-20260911/`. Raw native dumps are not committed.
