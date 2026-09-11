# Runtime and CDP latency, 2026-09-12

Branch: `codex/runtime-cdp-latency`. Baseline: `9c82cddc6a564de1a38c285c566fc30107f21064`.
The primary checkout's existing uncommitted work was not included or modified.
[Portable measurements](runtime-cdp-20260912/measurements.json) retain binary and
frozen-harness hashes, every local timing row, gate statuses, concurrency waves,
recovery memory, and successful and failed live attempts.

## Measured causes and changes

- Windows/Pyppeteer discovery tried the unavailable IPv6 localhost listener
  before IPv4, twice: HTTP discovery and the returned WebSocket URL. On the same
  server, unmodified `localhost` connection took 4056.46 ms; literal IPv4 took
  6.49 ms. The snapshot tool now races localhost address families and uses the
  address that accepted the connection: 58.46 ms. HTTPS and remote endpoints
  retain their original hostname. Both IPv4-only and IPv6-only listeners are
  covered by tests. This is a client connection fix, not a V8 speedup.
- Complex selectors traversed and wrapped every descendant, including text nodes.
  A necessary leaf in the final compound now selects candidate IDs in one host
  call. The existing validated selector library still decides the entire match.
  Union ordering, scope, identity, pseudo-classes, errors, fallback selectors and
  foreign attribute case remain covered. No DOM query result survives a query.
- `getElementsByClassName` rescanned all descendants on every index and length
  access, repeatedly reading owner-document state and creating node records.
  Membership is now selected in canonical DOM and cached against its existing
  arena revision. Parser, host, cross-realm, attribute and tree writes invalidate
  it. The cache stores IDs and preserves live collection and wrapper identity.
- Parent, child-at-index, element-child and selector traversal transport now uses
  canonical IDs rather than repeatedly serializing attributes, text and child
  arrays. Existing wrappers are reused; missing wrappers load their record once.
  In the initial live profile, parent reads alone cost 1.136 s over 133,163 calls;
  an intermediate pump profile spent 1.270 s in 109,307 child-at-index calls.

One event loop per Page, task/microtask ordering, Page isolation and the GPU-free
observation boundary are preserved. There are no website conditions in runtime
code. The opt-in native profiler can now select script names with
`MIMIC_V8_CPU_PROFILE_FILTER`, without adding environment lookups when profiling
is disabled.

## Controlled local CDP measurements

`tools/performance/runtime_latency.py` serves the same 600-section document to
both freshly built V8 binaries. It verifies every result and mutation, records
three Pages with one excluded warmup and five measured operations per Page,
and samples RSS after closing each Page and 250 ms recovery, without forced GC.
The final local pair uses the exact binaries from the final paired fast gates.

| Operation | Baseline median ms | Final median ms |
| --- | ---: | ---: |
| New Page | 2.07 | 1.63 |
| Local navigation / DOMContentLoaded | 104.04 | 91.31 |
| Structural selector | 24.98 | 0.89 |
| Iterate live class collection | 2217.38 | 0.48 |
| Parent/child traversal | 211.37 | 14.46 |
| Mutate and reread live collection | 106.59 | 0.74 |

After the third Page closes, process RSS is 169.66 to 106.41 MiB. These are
operation-specific gains, not a multiplier for all JavaScript or all websites.

## Frozen gate and concurrency

All four unchanged fast-gate invocations completed: initial baseline,
intermediate changes, final changes, and a fresh baseline in reverse order.
All six mandatory workloads, measured 10/25-Page waves and both memory waves
passed. The original benchmark, workload files and expectations were untouched.

Final paired baseline to final results:

| Metric | Baseline | Final |
| --- | ---: | ---: |
| DOM execution ms | 191.24 | 160.25 |
| DOM completion ms | 221.28 | 191.56 |
| Static execution ms | 5.63 | 3.51 |
| Static completion ms | 46.84 | 30.92 |
| React execution ms | 82.00 | 81.86 |
| React completion ms | 121.48 | 150.03 |
| 10-Page throughput, Pages/s | 45.54 | 49.93 |
| 25-Page throughput, Pages/s | 56.05 | 83.20 |
| Static recovery private memory MiB | 156.61 | 142.86 |
| React recovery private memory MiB | 166.85 | 167.25 |
| Static 10-Page wave CPU seconds | 3.94 | 3.64 |
| React 10-Page wave CPU seconds | 5.58 | 5.75 |

The shared host was not quiescent. Across the initial pair, 25-Page throughput
instead changed 86.33 to 51.03 Pages/s; React completion changed 96.59 to 104.61 ms.
Do not claim a stable overall throughput improvement or React latency neutrality.
React completion and broad concurrency variance remain open measurement concerns;
React execution in the final pair was essentially unchanged. No full N=50 matrix
or full-race result is claimed.

## Live scenario and limits

The supplied script is retained at `compatibility/research/blast_other_games.py`,
with a checkout-relative import and the localhost connection fix. The user's
original desktop script was not overwritten.

The initial baseline run took 19.655 s, including 4.048 s connection, 9.228 s first
navigation, 788 ms find-link, 97 ms click and 4.546 s second navigation. After the
collection changes, a successful run took 11.499 s: 29 ms IPv4 connection,
8.280 s first navigation, 242 ms find-link, 13 ms click, 2.061 s second navigation
and 833 ms snapshot. Both captured 43 files, no JavaScript exceptions, one console
error and two failed resource requests. Internet, cache and host load were not
controlled, so total time is not a pure runtime comparison.

The remaining find-link delay in that live run includes an approximately
200–360 ms synchronous style/geometry initialization callback. Native profiling
identified CSSOM construction, source serialization, selector compilation and
style matching; those costs are not removed by this batch. Network requests also
still contribute seconds to navigation. Parser/navigation lock ownership has not
been redesigned here.

Two final-binary live attempts connected through localhost in 86/84 ms but failed
before producing a snapshot. One trace showed a main-document transport deadline
before HTML commit; the client ultimately reported its 60-second navigation
timeout. Both failed attempts are retained, not counted as successful final E2E
runs. Local execution and the frozen gate do not depend on that site's availability.

## Verification and reproduction

- `go test ./...`: passed; browser package 301.977 s.
- Focused browser/CDP race checks: passed, including selectors, live collections,
  parent/child identity and multiple debugger connections sharing a Page clock.
- `go test -race ./internal/dom`: passed.
- Eight snapshot Python tests: passed, including both localhost address families.
- `git diff --check`: passed.
- `tools/check_repository.py`: fails with identical pre-existing findings on the
  baseline and modified checkout. `tools/generate_compat.py --check`: both fail
  with the same retained-artifact manifest mismatch. Neither is reported as green.

```powershell
go build -o .build/mimic.exe ./cmd/mimic
./.build/mimic.exe -listen 127.0.0.1:9336
# In another terminal, from this checkout:
python compatibility/research/blast_other_games.py --endpoint http://localhost:9336 --url https://blast.hk/ --output .build/blast-new
python tools/performance/runtime_latency.py --binary .build/mimic.exe --output .build/runtime-new.json
```

Detailed original receipts/profiles remain in `.build/gate-before`,
`.build/gate-after`, `.build/gate-final`, `.build/gate-baseline-paired`,
`.build/profile-before`, `.build/profile-after-pump`,
`.build/profile-after-collections` and `.build/profile-preamble`.
