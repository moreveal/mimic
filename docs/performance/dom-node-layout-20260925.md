# Compact Node layout checkpoint (2026-09-25)

`Node` kept its public fields and canonical ID/pointer semantics. Grouping its
three booleans after pointer-sized fields changed the amd64 struct size from
296 to **288 bytes**. In the measured Go allocator this also avoids a larger
size class: the DOM-heavy fixture saves about 32 bytes per retained node. No
new interner, shared Page state or chunk lifetime was introduced.

The control is feature commit `4f4e5ef`; the candidate changes only the field
order. The [diagnostic source](../../tools/performance/dom_density.go) parses
five documents, each with 10,000 `div`/`span` pairs, then collects garbage.
Across three alternating fresh-process pairs, median live Go heap was
73,819,840 → **68,989,840 B**, a **4,830,000 B (6.54%)** reduction. Median
cumulative Go allocation was 99,998,864 → 95,150,400 B (-4.85%). Existing
parser benchmarks retained 927 batch and 4770 stream allocations per parse,
while bytes per parse fell from 114,089 to 107,721 (batch) and about 183,003 to
176,507 (stream). Parser timings overlapped. [Raw paired readings and binary
hashes](data/dom-node-layout-20260925/provenance.json) are preserved.

The profile feature's 100-Page fixture contains only 200 links per Page. Three
alternating fresh-process pairs restored 100/100 Pages in both builds; median
first-live RSS was 3,118,792,704 → 3,116,113,920 B. This 2.6 MiB difference
is below process-level noise relative to ~3 GiB total. The Node layout is a
measured Go-heap improvement on large DOMs, not a claim of multiple-fold total
RAM reduction for 100 V8 isolates.

The unchanged fast gate passed all six semantic workloads, 10/25-Page static
waves and two memory waves in five control and three layout runs. Its warm
median DOM completion was 295.9 ms in the control runs versus 309.0 ms in the
layout runs; React was 90.8 versus 95.1 ms. The parser microbenchmark did not
show this slowdown, so these short-gate differences may include environmental
drift, but they are retained as a possible 4–5% workload cost. No throughput
improvement is claimed. [Per-run gate summaries](data/dom-node-layout-20260925/fast-gate-summary.json)
include executable hashes, wave throughput and memory samples. The full Go
suite, DOM race test and 100-context CDP example were also run.

The larger block-allocation candidate was rejected after a correctness-gate
stall despite a heap win; its [negative result](dom-node-blocks-20260925.md)
and removable patch remain available. Further hot/cold or attribute packing
requires its own compatibility and process-memory evidence.
