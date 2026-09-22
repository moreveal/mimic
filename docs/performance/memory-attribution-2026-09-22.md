# Page memory attribution, 2026-09-22

## Scope and provenance

This is a diagnostic investigation, not a production optimization. Production
code, frozen benchmark workloads, harnesses, and baselines were not changed.
The source revision was `7bca25044e069966a367d240dff9fc60cf4895d0`.

The direct Browser probe used one temporary opt-in test file, removed after
measurement. It created 25 or 50 independent Pages in one Context, navigated
each to a tiny local HTML page, evaluated a checked DOM value, then sampled
Windows working set/private bytes, Go heap, and every live isolate's V8 heap
statistics. It sampled after Page close, Go GC/scavenging, and Browser close.
The same test binary (SHA-256
`01dd1929d776746f42e1a7238c9935669e93193d0314f9ef27b8315da8d6de11`)
ran the snapshot and no-snapshot 25/50-Page natural-memory comparisons and
the explicitly collected 25-Page comparisons. A second temporary test binary
captured single-Page V8 heap snapshots. The CDP probe used a fresh production
binary (SHA-256
`a6a68f03a92ccdba30d6125a4a317c571e40aecb6ec3b046e458d05524943502`)
and the unchanged frozen static/React fixture through
the existing `benchmark.run` client. Raw local receipts are under
`.build/memory-diagnosis-20260922/`; that directory is intentionally ignored.

Direct Browser results are attribution evidence, not frozen CDP benchmark
numbers. The CDP probes use 2 or 6 waves rather than the complete matrix.
Windows RSS and private bytes vary with allocator history; V8 heap statistics
exclude native snapshot storage, stacks, and process allocator retention.

## Direct Browser: active and retired Page memory

MiB, natural memory unless marked. Each row is a separate process. `V8 physical`
and `V8 used` sum statistics from the live Page isolates only. The first Page
can use ordinary bootstrap while the profile snapshot is being prepared;
46/50 Pages were restored in the snapshot 50-Page run.

| Mode | Pages | Ready RSS | Live RSS | Live private | V8 physical | V8 used | Go heap at live | RSS after Page close + scavenging | RSS after Browser close + scavenging |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Snapshot | 25 | 139.8 | 908.7 | 994.0 | 518.1 | about 399 | 61.1 | 694.2 | 124.6 |
| No snapshot | 25 | 33.1 | 1022.7 | 1099.4 | 867.0 | about 447 | 42.1 | 96.9 | 96.8 |
| Snapshot | 50 | 141.2 | 1581.3 | 1708.3 | 955.6 | 785.3 | 64.1 | 1398.7 | 175.3 |
| No snapshot | 50 | 33.7 | 1956.1 | 2069.1 | 1731.5 | 894.4 | 51.7 | 116.2 | 116.0 |

The snapshot helps **natural active memory** on this simple workload: at 50
Pages, live RSS is 375 MiB lower and V8 physical heap is 776 MiB lower. It
also improves the median create/navigate/evaluate time from 95.82 to 26.74 ms
per Page in this sequential probe. Removing the snapshot is therefore not a
general memory optimization.

The same snapshot run retains approximately 1223 MiB between closing the 50
Pages and closing their Browser, or approximately 24.5 MiB per former Page.
At 25 Pages the corresponding difference is approximately 570 MiB. These are
process observations, not exact isolate allocations: the Browser close also
releases bounded cache data and can trigger allocator trimming. The ownership
mechanism is explicit in code: `bootstrapRuntimePool.release` decrements
`lane.active` but calls `lane.owner.Dispose()` only when the *whole pool* has
already closed. The pool belongs to a Browser-owned snapshot cache. A Page
close therefore leaves its empty isolate ready for reuse until Browser close.

## What V8 GC can and cannot recover

The 25-Page direct probe also invoked `LowMemoryNotification` once per live
isolate. This was an explicit diagnostic intervention, not a production policy.

| Mode | RSS before → after GC | V8 physical before → after GC | Median GC time per Page |
| --- | ---: | ---: | ---: |
| Snapshot | 904.0 → 818.0 MiB | 512.9 → 402.4 MiB | 6.87 ms |
| No snapshot | 1025.6 → 630.4 MiB | 864.5 → 468.0 MiB | 10.44 ms |

After this collection, no-snapshot RSS is lower even though its V8 physical
heap is higher. This reverses the natural-memory ordering. It establishes that
the best snapshot policy depends on V8's collection state and on the native
memory outside its heap. A blanket GC on every Page would add measurable
latency; the result only supports a separately measured idle/pressure policy.

## Snapshot copy and retained JavaScript graph

The pinned gov8 v0.1.1 native shim's `HoldBlob` in
`internal/shim/features/snapshots_handles.inc` allocates `new char[len]` and
copies the entire startup blob into isolate-lifetime storage before setting V8
`CreateParams.snapshot_blob`. The two observed profile artifacts were
approximately **8.38 and 9.05 MiB**. Fifty restored isolates can therefore
retain roughly **419–453 MiB** in native blob copies alone. This is a concrete
non-V8-heap cost, independent of any RSS attribution guess. The current Go
`ShareImmutableBytes` avoids another Go copy but cannot eliminate this shim
copy. A shared native immutable blob with explicit lifetime ownership requires
a gov8/V8 concurrency proof; merely handing V8 a Go pointer is unsafe.

A single restored Page's V8 heap snapshot, captured after heap-snapshot GC,
contains 15.15 MiB shallow live objects; an ordinary-bootstrap Page contains
17.76 MiB. In the restored Page the largest types are property backing arrays
(5.69 MiB), strings (4.55 MiB), objects (1.32 MiB), closures (1.24 MiB), and
code (1.13 MiB). One retained bootstrap source string accounts for about
3.82 MiB. The ordinary Page has 3.69 MiB of code, explaining much of the
snapshot's V8 heap advantage. These are shallow heap sizes, not RSS and not a
proof that source can be dropped: function source and stack observations must
stay compatible.

The existing initialization-stage probe gives the latency side of the trade:
after cache warm-up, restored runtime construction takes roughly 7–12 ms and
binding installation 2–4 ms; ordinary construction takes roughly 1–2 ms but
installation takes roughly 70–102 ms. This is a focused Go Browser probe,
not CDP E2E latency.

## CDP workload cross-check

The targeted CDP probe ran the frozen static fixture in a persistent process,
holding all Pages alive at the same boundary before measuring active RSS. All
150/150 sessions completed correctly in each six-wave 25-Page series. One
snapshot series retained the following RSS two seconds after each wave's
`Target.closeTarget`: **475, 205, 888, 1204, 1353, 1390 MiB**. The no-snapshot
series recorded **163, 180, 212, 164, 177, 348 MiB**. The snapshot run's
active RSS and completion latency also varied markedly between waves. Cache
profile population and native allocator history were not held constant in
these six-wave runs; these values establish the long-lived-process problem,
not a clean per-Page snapshot delta.

Separate CDP 10/25/50-Page and React 25/50-Page probes all passed their
unchanged workload checks. Their first waves had much higher and less stable
RSS than later waves when snapshot preparation overlapped the work. In the
50-Page static two-wave comparison, the second wave's incremental RSS above
its own pre-wave baseline was 1705 MiB with snapshot and 1693 MiB without;
the absolute RSS differed because the snapshot process entered that wave with
239 MiB more retained memory. The React 50-Page second-wave increments were
1887 versus 1802 MiB. These short probes do **not** establish a consistent
warm active-memory advantage for either mode under CDP; they demonstrate why
pre-wave memory and cache state must be part of every comparison.

The unchanged full benchmark run, archived locally in
`.build/unpublished-benchmarks/13-release-20260922`, measured 2567 MiB at 50
static Pages and 4248 MiB at 100. Its median recovery RSS was 2253 and
4199 MiB respectively. The React 100-Page series stopped after teardown
timeouts. These remain the reportable current-production symptoms; the
targeted probes explain mechanisms without replacing that benchmark.

## Decision from this diagnosis

1. **First: bound idle isolate retention.** Dispose zero-active lanes above a
   small measured reserve, on their existing owner thread. This should reduce
   memory between waves and after spikes. It cannot lower peak memory while all
   Pages are live. Test 25/50/100 Page create/close cycles, Page independence,
   native teardown, callbacks, and recovered RSS before promoting it.
2. **Second: remove the isolate-lifetime native blob copy or shrink the blob.**
   The 8.38–9.05 MiB copied per isolate is a measured active-memory target.
   First prove a refcounted immutable native backing satisfies V8's lifetime
   and concurrency contract; otherwise profile which snapshot seed components
   can be reduced without changing reflection, source, or realm behavior.
3. **Third: investigate the WebAPI graph.** Property arrays and the retained
   source dominate the live V8 heap. Native lazy publication is worth a narrow
   compatibility proof; the earlier JavaScript Proxy trial saved only
   0.5–2.5 MiB/Page and regressed DOM latency. Dropping source or visible
   descriptors without Chrome evidence is not acceptable.
4. **Then: evaluate pressure-aware GC and admission control.** GC can reclaim
   memory at a measured 6.9–10.4 ms/Page in this probe. Admission control caps
   peak process memory but reduces concurrent throughput; it is an overload
   policy rather than a reduction in per-Page cost.

Keep one isolate per active Page while these changes are tested. The historical
multi-Page isolate pool reduced RSS but already caused a native concurrent
teardown crash. A process-per-Page design adds process overhead and does not
remove the duplicated JS graph or snapshot blob. Run focused semantic and
multi-wave memory gates first; reserve the full frozen matrix for the final
candidate.
