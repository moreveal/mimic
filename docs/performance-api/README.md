# Performance API compatibility — Chrome 152.0.7977.82

This independent package replaces the separate JavaScript timelines and timing
projections with one Go-owned timeline per document/worker. It targets observable
browser behavior, not Chromium's implementation or identical elapsed times.
The starting revision is `da4f93e873d508323832efafe6870d378d79d1c4`.

The controlled differential currently has **20 complete matches in 23 capture
groups**, compared with no complete matches before the package (11 differing
results and 12 capture errors). The updated runtime has no capture errors. Two groups retain
network-body lifecycle differences; agent-cluster memory measurement remains an
explicit unsupported boundary. A match applies to that fixture's observations,
not every possible behavior of the API. See [the full differential](differential.json).

## Implemented and measured behavior

| Area | Observations covered |
| --- | --- |
| Performance object and hierarchy | 28 captured interface shapes; inherited prototypes, property order/descriptors, constructor legality, brands, receiver checks, generated callable metadata, ordinary/isolated Window and dedicated Worker exposure |
| Entry identity and queries | Canonical wrappers; fresh query arrays; stable start-time sorting, explicit empty type filters, duplicate marks with identical timestamps, entry ownership across retained/reloaded frames; internal serializers ignore public getter overrides |
| User Timing | `mark`, `measure`, `new PerformanceMark`, cloning/cycles/detail identity, private-brand rejection for platform objects (including nested/foreign wrappers), dictionary conversion order, symbols/non-finite values, reserved legacy names, missing mark exceptions, numeric/string/duration combinations, clear operations and retained entries; named lookup selects the greatest start time |
| PerformanceObserver | Mode validation and persistence across disconnect, single-type accumulation, buffering/rebuffering, `takeRecords` arrival order, sorted callback lists, microtask-before-observer delivery, duplicate identity, clear/observer independence, callback receiver/options, dropped-entry reporting |
| Resource Timing | Initiating document/worker ownership; existing loader completion records, transport phases, content/body/transfer sizes, negotiated protocol, cache observations, server timing, redirects and original resource name, accumulated TAO visibility, CORS-visible status/content metadata |
| Resource buffers | Default capacity, resizing, secondary buffer, trusted `resourcetimingbufferfull`, handler/listener order and replacement, growth/clear inside handlers, observer delivery beyond the primary buffer |
| Navigation Timing | One live entry per document, independent milestone stamps, load duration finalization, normal versus buffered notification, final navigation URL, same-document history, iframe/reload origin and type, retained old Performance object, legacy timing/navigation projections |
| Clocks | Existing Page time origin and quantizer reused; clamped `timeOrigin` and `now` share coordinates; active Page turn is observed by borrowed frame operations; dedicated workers retain independent origins/clocks; transport waiting is not a long task |
| Long Tasks | Complete turns including microtasks, 50 ms threshold, buffered records and task attribution; same-origin child tasks are observable by ancestors with iframe metadata; initial wrapper realm follows the observer callback |
| Event Timing | Trusted input counts before listeners; synthetic dispatch excluded; processing intervals, eight-millisecond duration quantization, observer threshold, default event buffer threshold, first-input, simple keyboard/pointer interaction grouping, connected target lookup |
| `performance.memory` | Private MemoryInfo brand, readonly fields, a fresh immutable snapshot per read, cached values, stable profile-derived limit and coherent capacity/usage relations; allocation workload and ordinary/restored realms tested |
| Additional exposed interfaces | VisibilityStateEntry initialization; shape/brands for paint, element, LCP, layout shift, long-animation-frame/script, soft-navigation, interaction-contentful-paint, timing confidence and not-restored-reason interfaces |

“Additional exposed interfaces” describes shape, not entry production. A rendering
pipeline has not been introduced to manufacture paint/layout observations.

## State and ownership

`performanceTimeline` owns membership, sequence identity, buffer limits and observer
queues. The loader's existing immutable trace stream remains the resource source;
consumers advance a sequence cursor rather than rescanning the entire trace.
Redirect history remains on the existing request URL chain. Lifecycle events stamp
their existing realm/document state. JavaScript holds branded projections and
canonical wrapper caches, not another mutable timeline.

Clearing entries releases runtime roots after pending observers materialize their
records. Retained wrappers keep their immutable data and cloned detail. Disconnect
releases the host's callback reference. Small disconnected observer bookkeeping
persists until realm teardown so observation-mode history remains authoritative.
Document/worker teardown releases timeline state. There is no new global runtime
lock or shared mutable Page timeline.

The memory implementation is a **synthetic application-heap projection**. It removes
the adapter's bootstrap baseline from optional allocation samples, adds the
document model, coarsens usage, and derives capacity/limit from the Environment
hardware budget. An engine-neutral `AllocationRuntime` boundary supplies an optional
allocation delta; the current V8 provider is below that boundary. Neither Go heap,
process RSS, raw V8 heap totals nor values copied from a Chrome capture are returned.

The chosen 64 KiB accounting quantum, 50 ms refresh, capacity policy and randomized
timing-confidence policy are model policies, not claims of byte-for-byte Chrome
equivalence. The exact pinned Chromium [memory implementation](https://chromium.googlesource.com/chromium/src/+/152.0.7977.82/third_party/blink/renderer/core/timing/memory_info.cc)
also has precision/cache modes; its [timing-confidence implementation](https://chromium.googlesource.com/chromium/src/+/152.0.7977.82/third_party/blink/renderer/core/timing/performance_timing_confidence.cc)
returns a fresh projection of navigation state. The captured tests compare
relations and lifetimes, not absolute bytes, noise distribution or GC scheduling.

## Remaining boundaries

1. **Opaque Fetch completion:** the controlled Chrome opaque response has no
   resource entry at the observation point. Mimic consumes the underlying body
   eagerly and records the completed transport. The exact differing result is
   retained; it is not removed from the oracle or hidden behind a URL special case.
2. **Streaming document responseEnd:** Chrome can run the initial inline script
   before body completion. Mimic's loader buffers the body before parsing, so
   responseEnd is already available. Later lifecycle stages and observer identities
   agree in the retained CDP capture. The final full-suite repeat additionally
   observed zero responseEnd at DOMContentLoaded, load and after-load in the
   snapshot variant. This is an unresolved timing/lifecycle defect; it is not
   classified as harmless clock noise or covered by the initial-row skip.
   Streaming support requires coordinated Fetch/document work.
3. **`measureUserAgentSpecificMemory`:** exposed in an isolated document. Shape
   matches, but Mimic now rejects asynchronously with `NotSupportedError` instead
   of the generated stub's undefined return. The frozen oracle returns a memory
   breakdown. A consistent agent-cluster accounting model, including related
   documents/workers and cross-origin attribution, is not implemented here.
4. **Heap accounting:** per-realm usage is not Chrome renderer/agent-cluster heap
   aggregation. Alternate engines without an allocation provider retain the
   document-only model. Exact GC, memory pressure, external-memory extremes and
   Chrome's precision/cache policy remain environment-dependent or incomplete.
5. **Rendering-dependent entries:** no production of paint/FCP, LCP, element,
   layout-shift, long-animation-frame/script, soft-navigation or
   interaction-contentful-paint records. `supportedEntryTypes` reflects exposure;
   it is not evidence of a rendering backend.
6. **Input/long-task completeness:** composition, simultaneous keys/pointers,
   cancellation/scroll interactions, actual presentation feedback, cross-origin
   long-task privacy attribution, ancestor/sibling task observation and detached
   callback realms need further controlled work. EventCounts iterators currently
   snapshot values. These are not claimed as fully Chrome compatible.
7. **Other lifecycle/network details:** BFCache/prerender/activation and unload
   attribution, service-worker/interim-response timing, parser render-blocking
   classification, some failure/opaque/manual-redirect paths, dynamic visibility
   transitions and cross-origin navigation chains remain incomplete. Absolute
   durations depend on Environment clock scales and transport conditions.
8. **CDP Performance domain:** this package implements the browser Performance API;
   the existing minimal `Performance.getMetrics` domain is not expanded or counted
   as part of this compatibility result.
9. **Platform-object serialization:** Blob/File detail storage still depends on
   the existing shared serializer's platform support. It is not claimed as
   complete alongside the ordinary structured-data and rejection tests.

## Reproduction and evidence

The capture runner validates the exact browser version, uses a fresh page and
controlled local HTTP origin, and records profile/security/viewport provenance
and fixture hashes. No Chrome feature flags or frozen benchmark fixtures changed.
`--isolated` adds COOP/COEP; `--gc` uses CDP collection only in the controlled memory
oracle page to complete the asynchronous measurement.

```powershell
python tools/generate_performance_interfaces.py --check
python tools/compatibility/performance_oracle.py performance_surface performance_user_timing performance_observer
python tools/compatibility/performance_oracle.py performance_surface --worker
python tools/compatibility/performance_oracle.py performance_surface --isolated
python tools/compatibility/performance_oracle.py performance_specific_memory --isolated --gc
go test ./internal/browser -run '^TestPerformance' -count=1 -v
```

The runner expects the owned exact Chrome instance at port 9357 by default; use
`--endpoint` explicitly for another instance. `--mimic --output ...` saves a
comparison capture without overwriting the frozen Chrome reference.

## Validation

* Earlier `go test ./... -count=1`: **PASS**, 19 tested packages, 1935 passed tests/subtests;
  browser package 334.474 s. The first broad run failed the Worker Fetch unit
  fixture because its mock host lacked the new Performance binding. The mock was
  updated without removing Fetch assertions; the complete rerun passed.
* Final full-suite repeat at `6a7edee`: **FAIL**. The snapshot variant of
  `TestPerformanceNetworkAndLifecycleMatchFrozenChrome` observed zero responseEnd
  at DOMContentLoaded, load and after-load (rows 1–3). All other observed fields in
  these rows agree. The earlier successful run does not override this result;
  the final package does not have a clean full-suite status. No expectation or
  skip was changed to make the repeat pass.
* Additional isolated-surface tests: **PASS** in ordinary and restored realms.
  Final private-brand cloning regression and related User Timing/Worker/History
  checks also passed under race after the broad run (8.869 s).
* `go test -race ./... -count=1`: **INCOMPLETE / FAIL**, default 10-minute timeout
  in the browser package during `TestChildLocationNavigatesOnlyChildAndResolvesFromHistoryURL/goja`.
  The other 18 tested packages passed. No data-race report or failed test assertion
  preceded the timeout. Teardown also printed an uncaught circular-JSON diagnostic.
* Focused race across Performance and related resource/navigation/clock/lifetime
  tests: **PASS**, 137 passed tests/subtests, 78.889 s for browser. A separate
  four-Page concurrent timeline race regression also passed (1.44 s test time).
* The six new semantic skips are three known boundaries, each in ordinary and
  snapshot mode: opaque Fetch completion, initial streaming responseEnd, and
  agent-cluster memory attribution. Existing optional profile/native-backend/
  snapshot-stress skips remain. The [validation receipt](validation.json) lists
  every skipped test, package status and retained log hash; no skips count as passes.
* Performance schema/capture validation: **PASS**, 28 shapes and 23 exact `.82`
  captures with verified fixture hashes. Existing oracle provenance check: **PASS**.
* Existing `generate_compat.py --check`: **FAIL**, artifact manifest coverage.
  `check_repository.py`: **FAIL**, 278 findings. A separate clean checkout of
  `da4f93e` reproduces both; the audit finding sets are identical (zero added/removed).
* Frozen fast-gate initially could not import `websockets.sync` from the oracle
  Python environment. Measurements use the existing pinned benchmark environment
  (`websockets 13.1`, `psutil 7.2.2`); no dependency lock or harness was changed.

Raw logs and original gate outputs remain under `.build`. Exact exception wording,
stack formatting and unmeasured exotic receiver/new-target cases are not claimed
as fully equivalent to Chrome.

## Performance checkpoint

All four unchanged fast gates completed: **184 VALID executions per gate**, eight
verified binary launches, 10/25-Page concurrency waves and both recovery-memory
waves. Each gate built its own executable and verified its hash before launch.
The [performance receipt](performance.json) retains source revisions, binary and
harness hashes, individual throughput waves and memory phases. The final code
checkpoint is `6a7edee8454070601371b6bf371aa879fd3381c0`; subsequent changes are reports.

| Warm median / throughput | Initial base | Intermediate package | Final package | Base repeated after final |
| --- | ---: | ---: | ---: | ---: |
| DOM completion, ms | 176.67 | 174.62 | 180.34 | 180.17 |
| Static completion, ms | 33.10 | 32.88 | 34.07 | 34.98 |
| React execution, ms | 50.07 | 51.64 | 55.36 | 60.34 |
| React completion, ms | 87.09 | 87.15 | 104.50 | 97.83 |
| 10-Page throughput, Pages/s | 77.21 | 75.41 | 68.20 | 75.15 |
| 25-Page throughput, Pages/s | 89.60 | 91.73 | 68.19 | 90.16 |

The initial pair was approximately neutral for completion latency. The final
package's React completion is **20.0% slower than the initial base and 6.8% slower
than the repeated base**. Final 10/25-Page throughput is **9.2%/24.4% below the
repeated base**. Repeating the base documents substantial host variation, but
does not explain away the throughput decline. These are unresolved regression
signals; performance neutrality is **not established**. No workload shortcuts,
frozen harness changes or speculative optimization were used to hide them.

For 10 active Pages, final private memory is 494.61 MiB (static) / 536.34 MiB
(React), against initial-base 491.19 / 533.84 and repeated-base 488.66 / 538.45 MiB.
After teardown and 250 ms recovery, final private memory is 151.29 / 162.00 MiB,
against initial-base 154.39 / 163.29 and repeated-base 155.07 / 165.18 MiB. Final
active/recovery RSS is 440.65/122.46 MiB (static) and 484.16/133.18 MiB (React).
This short gate does not establish long-run retention or allocation-pressure
neutrality. Dedicated allocation profiling was not run; it remains appropriate
for diagnosing the concurrency regression. These process measurements are
validation data and are never returned as `performance.memory` values.
