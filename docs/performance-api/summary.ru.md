# Performance API: Chrome 152.0.7977.82 ↔ Mimic

Independent branch `codex/performance-api-chrome152`, based on revision
`da4f93e873d508323832efafe6870d378d79d1c4`.

A shared Go-owned timeline is implemented for every document and worker.
JavaScript objects project that state; resources use the existing network trace,
navigation uses the document lifecycle, and time uses the Page scheduler and
Environment clocks. No new global runtime lock was introduced, and the core
remains platform-neutral.

The package covers:

- `performance`, the `PerformanceEntry` hierarchy, brands, descriptors and
  receiver checks, canonical identity, `getEntries*`, sorting, `mark`/`measure`,
  detail cloning, and clear operations;
- observer modes, queues, buffering, `takeRecords`, callback/microtask ordering,
  resource-buffer overflow, and dropped entries;
- resource, navigation and legacy timing, redirects, TAO/CORS visibility,
  server timing, `timeOrigin`/`now`, iframe and worker ownership, reload, and
  snapshot/restore;
- long tasks with microtasks and same-origin child attribution, trusted event
  timing, first input, EventCounts, and basic interaction grouping;
- `performance.memory`: coherent immutable snapshots of the synthetic model,
  a profile-derived limit, and allocation deltas through the engine abstraction.
  Raw Go/V8 heap totals and constants copied from Chrome captures are not exposed.

**Differential result: 0 → 20 fully matching groups out of 23.** Before the
changes there were 11 mismatches and 12 capture errors; afterward there were
three mismatches and no capture errors. Twenty-eight interface shapes were
checked. This covers specific controlled observations and is not a claim of
complete implementation of every exposed API.

Three differences remain: Mimic buffers the document body in advance, completes
opaque Fetch differently, and does not yet implement agent-cluster memory
breakdown. `measureUserAgentSpecificMemory` explicitly rejects with
`NotSupportedError`. Rendering-related types expose interface shape without
producing paint, layout, LCP, or LoAF records. Complex input interactions,
cross-origin long-task attribution, BFCache/prerender, and exact Chrome heap/GC
policy remain incomplete.

An earlier full ordinary Go suite passed, but the **final repeat failed**: during
snapshot navigation, `responseEnd` remained zero at DOMContentLoaded, load, and
after loading. The defect remains open; expectations and skips were not changed
to hide it. The focused race suite, including four independent Pages, passes.
The **full race run did not finish**: the browser package reached its ten-minute
timeout in a Goja child-navigation test, with no data race reported before the
timeout. Six new semantic skips explicitly correspond to the three boundaries in
ordinary and snapshot modes. The shared manifest/audit checks also fail on the
clean baseline with the same 278 findings. All skips and unsuccessful attempts
are retained.

Four frozen fast gates each completed 184 VALID executions. Final React
completion was **6.8%** slower than the repeated baseline, while throughput at
10/25 Pages was **9.2%/24.4%** lower. Environmental variation does not explain
the entire regression, so performance neutrality was not established. Final
private memory after teardown and 250 ms recovery was **151.29/162.00 MiB** for
static/React.

Details and reproduction steps: [full report](README.md),
[before/after differential](differential.json), [tests/race/skips](validation.json),
and [all performance runs](performance.json).
