# Mimic vs. Chrome: September 13, 2026

A fresh-build comparison on one Windows workstation using the unchanged local benchmark suite. Mimic and Chrome passed all six correctness gates. Of 360 measured single-page attempts, 359 passed; one cold Chrome/WebAssembly navigation failed with net::ERR_ABORTED. Mimic also had one page-initialization failure in its first measured CPU wave at N=100; that series stopped at 99/100 successes. Static and React completed all measured concurrency levels. These are controlled fixtures, not a test of general website compatibility.

**Active development, early results.** Our goal is direct HTTP lightness with browser compatibility. The memory advantage visible in this run is an encouraging starting point. Slower execution cases remain active optimization targets; these timings describe the current build, not our intended performance ceiling. Future gains must be measured while preserving behavior.

## Environment

| Item | Value |
|---|---|
| Platform | Windows-11-10.0.26200-SP0 |
| CPU | Intel(R) Core(TM) i7-14700KF |
| RAM | 31.83 GiB |
| Chrome | 152.0.7977.82 (headless=new) |
| Mimic engine | V8 15.2.124.1-rusty |
| Chrome V8 | 15.2.124.21 |
| Run started | 2026-09-13T19:57:38.186817+04:00 |
| Run completed | 2026-09-13T20:10:26.711901+04:00 |
| Host conditions | Interactive workstation; Balanced power plan; background apps and antivirus enabled |

Both executables were hash-checked before every launch. The source tree was clean when Mimic was built. A single run on a shared workstation is subject to scheduler, cache, and background-load variation.

## Startup and ready memory

Ten fresh processes per system, with alternating system order and an excluded warmup. Readiness means a WebSocket connection and a successful Target.getTargets response. RSS is the sum of process-tree working sets at that point, including the initial page; it is not per-page memory.

| Runtime | CDP ready p50, ms | p95, ms | Ready RSS p50, MiB | Ready private bytes p50, MiB |
|---|---|---|---|---|
| Mimic | 219.09 | 231.06 | 28.78 | 80.80 |
| Chrome | 270.36 | 289.97 | 378.52 | 177.59 |

## Warm workload completion

Twenty measured iterations per system and fixture, after one excluded warmup. The process stays alive; each iteration creates a fresh page and origin. Completion measures navigation through the validated result; page creation and teardown are excluded. Lower is better.

| Workload | Mimic p50, ms | Chrome p50, ms | Mimic p95, ms | Chrome p95, ms |
|---|---|---|---|---|
| Static DOM | 33.68 | 21.32 | 38.98 | 24.70 |
| JavaScript / crypto | 77.37 | 52.31 | 94.28 | 57.49 |
| DOM mutations | 202.30 | 53.45 | 224.78 | 59.31 |
| Async / networking | 103.96 | 49.53 | 126.09 | 55.25 |
| React | 90.42 | 46.47 | 120.94 | 49.64 |
| WebAssembly | 33.92 | 22.83 | 38.54 | 26.00 |

## Cold end-to-end completion

Ten measured fresh-process iterations per fixture, after one excluded warmup. This includes process startup, page creation, navigation, execution, teardown, and process exit. Temporary-profile removal and local-server maintenance are excluded. Cold means a new process; OS caches are not flushed. The cold WebAssembly comparison is withheld because one Chrome navigation failed; all observations remain in the data.

| Workload | Mimic p50, ms | Chrome p50, ms |
|---|---|---|
| Static DOM | 590.94 | 474.22 |
| JavaScript / crypto | 607.27 | 491.41 |
| DOM mutations | 757.22 | 449.89 |
| Async / networking | 634.94 | 427.96 |
| React | 629.07 | 415.48 |
| WebAssembly | Not compared | Not compared |

## CPU and memory during warm work

Medians across the same 20 iterations. CPU is process-tree user + kernel time over a page session. Peak RSS/private bytes are sampled session peaks. RSS can double-count shared DLL pages; it is not unique physical RAM.

| Workload | Runtime | CPU, ms/session | Peak RSS, MiB | Peak private bytes, MiB |
|---|---|---|---|---|
| Static DOM | Mimic | 46.88 | 147.96 | 179.06 |
| Static DOM | Chrome | 164.06 | 1175.96 | 588.62 |
| JavaScript / crypto | Mimic | 93.75 | 164.90 | 194.17 |
| JavaScript / crypto | Chrome | 226.56 | 1376.86 | 779.06 |
| DOM mutations | Mimic | 281.25 | 191.07 | 221.86 |
| DOM mutations | Chrome | 250.00 | 1345.12 | 715.40 |
| Async / networking | Mimic | 109.38 | 163.09 | 195.08 |
| Async / networking | Chrome | 250.00 | 1227.90 | 627.53 |
| React | Mimic | 140.62 | 167.37 | 198.62 |
| React | Chrome | 234.38 | 1434.89 | 822.61 |
| WebAssembly | Mimic | 31.25 | 149.51 | 178.54 |
| WebAssembly | Chrome | 179.69 | 1272.42 | 626.18 |

## Concurrent pages

Each level uses a fresh process, one excluded warmup wave, and max(5, ceil(20/N)) measured waves. Pages are created concurrently and held until all work completes. Throughput is successful sessions divided by total measured wave time, including page setup and teardown, but excluding the separate recovery wait. It is not directly comparable to the completion-only latency table above.

Rows show every attempted level. A stopped series is not a stable result. The harness stops on failures, wave timeout, low available system memory, or sustained system-wide paging; higher levels are then not attempted.

| Workload | Runtime | Pages | Waves | Success | Sessions/s | Active RSS, MiB | RSS after recovery, MiB | Status |
|---|---|---|---|---|---|---|---|---|
| Static DOM | Chrome | 1 | 20 | 100.0% | 15.29 | 1188.29 | 1131.31 | Completed |
| Static DOM | Mimic | 1 | 20 | 100.0% | 14.85 | 150.67 | 124.53 | Completed |
| Static DOM | Chrome | 5 | 5 | 100.0% | 22.52 | 1348.03 | 1068.62 | Completed |
| Static DOM | Mimic | 5 | 5 | 100.0% | 36.68 | 338.70 | 206.63 | Completed |
| Static DOM | Chrome | 10 | 5 | 100.0% | 27.36 | 1683.53 | 1136.02 | Completed |
| Static DOM | Mimic | 10 | 5 | 100.0% | 58.33 | 560.86 | 301.63 | Completed |
| Static DOM | Chrome | 25 | 5 | 100.0% | 30.21 | 2542.67 | 1136.20 | Completed |
| Static DOM | Mimic | 25 | 5 | 100.0% | 70.48 | 1092.03 | 491.23 | Completed |
| Static DOM | Chrome | 50 | 5 | 100.0% | 28.59 | 4051.56 | 1193.96 | Completed |
| Static DOM | Mimic | 50 | 5 | 100.0% | 66.95 | 2210.02 | 910.56 | Completed |
| Static DOM | Chrome | 100 | 5 | 100.0% | 27.78 | 7039.66 | 1248.70 | Completed |
| Static DOM | Mimic | 100 | 5 | 100.0% | 76.64 | 3988.31 | 1519.87 | Completed |
| JavaScript / crypto | Chrome | 1 | 20 | 100.0% | 8.71 | 1405.11 | 1328.94 | Completed |
| JavaScript / crypto | Mimic | 1 | 20 | 100.0% | 8.97 | 165.96 | 126.49 | Completed |
| JavaScript / crypto | Chrome | 5 | 5 | 100.0% | 21.07 | 1627.67 | 1295.22 | Completed |
| JavaScript / crypto | Mimic | 5 | 5 | 100.0% | 29.61 | 380.04 | 197.66 | Completed |
| JavaScript / crypto | Chrome | 10 | 5 | 100.0% | 19.32 | 1995.12 | 1297.75 | Completed |
| JavaScript / crypto | Mimic | 10 | 5 | 100.0% | 36.36 | 680.95 | 328.06 | Completed |
| JavaScript / crypto | Chrome | 25 | 5 | 100.0% | 22.77 | 3069.84 | 1283.30 | Completed |
| JavaScript / crypto | Mimic | 25 | 5 | 100.0% | 45.49 | 1513.77 | 539.42 | Completed |
| JavaScript / crypto | Chrome | 50 | 5 | 100.0% | 22.08 | 4943.03 | 1330.05 | Completed |
| JavaScript / crypto | Mimic | 50 | 5 | 100.0% | 48.20 | 2734.01 | 804.45 | Completed |
| JavaScript / crypto | Chrome | 100 | 5 | 100.0% | 19.60 | 8656.86 | 1375.74 | Completed |
| JavaScript / crypto | Mimic | 100 | 1 | 99.0% | 24.16 | 4032.66 | 273.40 | non-zero failure rate |
| React | Chrome | 1 | 20 | 100.0% | 8.57 | 1437.69 | 1358.97 | Completed |
| React | Mimic | 1 | 20 | 100.0% | 7.50 | 169.15 | 135.65 | Completed |
| React | Chrome | 5 | 5 | 100.0% | 10.66 | 1564.70 | 1244.96 | Completed |
| React | Mimic | 5 | 5 | 100.0% | 24.97 | 382.02 | 219.10 | Completed |
| React | Chrome | 10 | 5 | 100.0% | 8.33 | 1910.35 | 1249.64 | Completed |
| React | Mimic | 10 | 5 | 100.0% | 37.35 | 650.29 | 327.84 | Completed |
| React | Chrome | 25 | 5 | 100.0% | 15.43 | 2986.12 | 1278.65 | Completed |
| React | Mimic | 25 | 5 | 100.0% | 47.53 | 1433.85 | 642.25 | Completed |
| React | Chrome | 50 | 5 | 100.0% | 18.68 | 4758.96 | 1313.19 | Completed |
| React | Mimic | 50 | 5 | 100.0% | 50.52 | 2692.14 | 1103.45 | Completed |
| React | Chrome | 100 | 5 | 100.0% | 20.07 | 8503.54 | 1396.86 | Completed |
| React | Mimic | 100 | 5 | 100.0% | 44.47 | 5175.78 | 2009.55 | Completed |

Recovery memory is sampled 250 ms after page teardown, without forced garbage collection. Retained memory can include shared runtime artifacts and allocator caches; this measurement does not establish leak absence or a return to the initial footprint. Private memory, CPU per successful session, and latency distributions are in the data.

## What the fixtures exercise

| Fixture | Checked behavior |
|---|---|
| Static DOM | Baseline text and root count |
| JavaScript / crypto | Deterministic arithmetic and SHA-256; standard JavaScript operations |
| DOM mutations | 3,000 elements; final classes, node counts, and text |
| Async / networking | Promises, timers, messaging, a Worker, fetch, and XHR |
| React | React 18.3.1; 200 cards, fetch, effects, and three state transitions |
| WebAssembly | Instantiation and 100,000 integer-add calls |

## Measurement boundaries

- All content is served locally, with identical fixtures and expected results for both systems. No external websites are timed.
- HTTP cache is disabled and each page uses a unique origin. Warm runs retain the process, transport, and browser context. This is not a tenant-isolation test.
- An external high-resolution clock measures elapsed time. Result polling is every 5 ms; memory sampling is every 50 ms. Short peaks may be missed.
- No paint or network-idle wait is included. These are browser workflow measurements, not isolated JavaScript engine timings.
- Slow samples are retained. p95 at 10вЂ“20 samples is unstable; no single-page p99 or aggregate speedup is claimed.
- Chrome has capabilities Mimic does not implement. These results do not imply feature parity, savings on arbitrary sites, or production cost estimates.

## Data and provenance

These timings identify the measured build by executable hash. A subsequent browser-target compatibility fix, enabling Puppeteer browser.target().createCDPSession(), was validated separately; the timings are not a performance measurement of that follow-up change.

[Download the numerical results](public-results.json): all single-page summary metrics, numerical completion/startup samples (including excluded warmups), concurrency results, and executable/harness hashes. The runtime, benchmark harness, full diagnostic captures, and research remain private. This public summary is therefore not independently reproducible from this repository alone. The exact full-run data hash is included to identify the retained private record.

[Request private beta access](https://github.com/moreveal/mimic-runtime/blob/main/BETA.md) В· [Back to Mimic](https://github.com/moreveal/mimic-runtime)
