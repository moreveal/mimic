# Mimic vs. Chrome: 2026-09-20

Fresh builds on one Windows workstation, using the unchanged frozen workloads. 12/12 correctness gates and 360/360 measured single-page attempts passed. 2 concurrency series stopped or contained a failure. All observations, including failed attempts and excluded warmups, remain in the data. These are controlled fixtures and do not establish general website compatibility.

## Environment

| Item | Value |
|---|---|
| Platform | Windows-11-10.0.26200-SP0 |
| CPU | Intel(R) Core(TM) i7-14700KF |
| RAM | 31.83 GiB |
| Chrome | 152.0.7977.82 (headless=new) |
| Mimic / Chrome V8 | 15.2.124.1-rusty / 15.2.124.21 |
| Started / completed | 2026-09-20T20:24:00.878033+04:00 / 2026-09-20T20:38:31.811856+04:00 |

Executable hashes are checked before every launch. This is an interactive workstation with background applications and antivirus enabled; cache and scheduler variation remain possible.

## Startup and ready memory

Ten fresh processes per runtime, alternating order, after an excluded warmup. Both answer the same Target.getTargets readiness probe. Summed process-tree RSS includes the initial page and can count shared pages more than once; it is not marginal Page memory.

| Runtime | CDP ready p50, ms | p95, ms | Ready RSS, MiB | Ready private bytes, MiB |
|---|---|---|---|---|
| Mimic | 212.62 | 232.79 | 45.04 | 96.03 |
| Chrome | 244.72 | 257.41 | 373.89 | 180.52 |

## Warm execution and completion

Twenty retained samples per workload/runtime. Each iteration creates a new Page and origin in the warm process. Execution includes invoking the workload and detecting its validated result. Completion also includes navigation; Page creation and teardown are excluded. Lower is better.

| Workload | Mimic execution, ms | Chrome execution, ms | Mimic completion, ms | Chrome completion, ms |
|---|---|---|---|---|
| Static DOM | 3.23 | 4.09 | 33.69 | 23.04 |
| JavaScript / crypto | 38.17 | 28.77 | 68.65 | 47.84 |
| DOM mutations | 274.29 | 30.59 | 304.49 | 49.56 |
| Async / networking | 67.34 | 26.59 | 99.98 | 45.41 |
| React | 48.27 | 22.28 | 85.43 | 42.40 |
| WebAssembly | 3.84 | 5.48 | 34.31 | 24.85 |

## Cold end-to-end completion

Ten fresh-process samples per workload/runtime. Includes process startup, Page creation, navigation, execution, teardown and process exit. OS caches are not flushed. Server maintenance and temporary-profile removal are excluded. A failed series is withheld from comparisons.

| Workload | Mimic p50, ms | Chrome p50, ms |
|---|---|---|
| Static DOM | 574.49 | 455.73 |
| JavaScript / crypto | 616.30 | 515.56 |
| DOM mutations | 865.30 | 507.63 |
| Async / networking | 641.49 | 490.31 |
| React | 626.17 | 512.42 |
| WebAssembly | 575.86 | 430.32 |

## CPU and memory during warm work

Process-tree user plus kernel time per session; sampled peaks may miss short-lived allocations.

| Workload | Runtime | CPU, ms/session | Peak RSS, MiB | Peak private bytes, MiB |
|---|---|---|---|---|
| Static DOM | Mimic | 46.88 | 159.51 | 188.00 |
| Static DOM | Chrome | 156.25 | 1190.18 | 599.25 |
| JavaScript / crypto | Mimic | 85.94 | 179.06 | 206.21 |
| JavaScript / crypto | Chrome | 195.31 | 1406.91 | 786.02 |
| DOM mutations | Mimic | 359.38 | 183.18 | 210.19 |
| DOM mutations | Chrome | 218.75 | 1348.84 | 713.21 |
| Async / networking | Mimic | 117.19 | 176.58 | 204.72 |
| Async / networking | Chrome | 226.56 | 1246.93 | 631.34 |
| React | Mimic | 117.19 | 168.84 | 195.81 |
| React | Chrome | 210.94 | 1425.37 | 810.08 |
| WebAssembly | Mimic | 31.25 | 160.82 | 187.62 |
| WebAssembly | Chrome | 171.88 | 1268.99 | 625.02 |

## Concurrent Pages

Fresh process per level, one excluded warmup, then max(5, ceil(20/N)) measured waves. Throughput includes setup and teardown but excludes the separate 250 ms recovery wait. Every attempted level is shown. A stopped series is not a stable successful result.

| Workload | Runtime | Pages | Waves | Success | Sessions/s | Active RSS, MiB | Recovered RSS, MiB | Status |
|---|---|---|---|---|---|---|---|---|
| Static DOM | Chrome | 1 | 20 | 100.0% | 11.60 | 1212.47 | 1154.95 | Completed |
| Static DOM | Mimic | 1 | 20 | 100.0% | 15.95 | 164.73 | 136.39 | Completed |
| Static DOM | Chrome | 5 | 5 | 100.0% | 20.32 | 1385.82 | 1133.92 | Completed |
| Static DOM | Mimic | 5 | 5 | 100.0% | 35.52 | 310.84 | 161.36 | Completed |
| Static DOM | Chrome | 10 | 5 | 100.0% | 19.69 | 1671.69 | 1122.05 | Completed |
| Static DOM | Mimic | 10 | 5 | 100.0% | 48.77 | 489.99 | 205.24 | Completed |
| Static DOM | Chrome | 25 | 5 | 100.0% | 25.33 | 2579.41 | 1151.58 | Completed |
| Static DOM | Mimic | 25 | 5 | 100.0% | 61.46 | 976.46 | 278.59 | Completed |
| Static DOM | Chrome | 50 | 5 | 100.0% | 23.45 | 4093.27 | 1206.18 | Completed |
| Static DOM | Mimic | 50 | 5 | 100.0% | 66.56 | 1735.62 | 364.82 | Completed |
| Static DOM | Chrome | 100 | 5 | 100.0% | 21.53 | 7112.53 | 1251.42 | Completed |
| Static DOM | Mimic | 100 | 0 | 100.0% | 24.46 | 4431.07 | 346.35 | memory pressure (<15% or 2 GiB available) |
| JavaScript / crypto | Chrome | 1 | 20 | 100.0% | 7.98 | 1413.62 | 1336.87 | Completed |
| JavaScript / crypto | Mimic | 1 | 20 | 100.0% | 8.06 | 179.34 | 135.52 | Completed |
| JavaScript / crypto | Chrome | 5 | 5 | 100.0% | 17.31 | 1626.54 | 1295.95 | Completed |
| JavaScript / crypto | Mimic | 5 | 5 | 100.0% | 25.56 | 366.53 | 155.85 | Completed |
| JavaScript / crypto | Chrome | 10 | 5 | 100.0% | 20.53 | 1993.72 | 1290.79 | Completed |
| JavaScript / crypto | Mimic | 10 | 5 | 100.0% | 34.76 | 615.09 | 190.65 | Completed |
| JavaScript / crypto | Chrome | 25 | 5 | 100.0% | 22.24 | 3121.74 | 1315.82 | Completed |
| JavaScript / crypto | Mimic | 25 | 5 | 100.0% | 45.93 | 1308.00 | 276.02 | Completed |
| JavaScript / crypto | Chrome | 50 | 5 | 100.0% | 22.59 | 4987.22 | 1352.02 | Completed |
| JavaScript / crypto | Mimic | 50 | 5 | 100.0% | 49.70 | 2351.58 | 284.30 | Completed |
| JavaScript / crypto | Chrome | 100 | 5 | 100.0% | 21.37 | 8730.54 | 1398.17 | Completed |
| JavaScript / crypto | Mimic | 100 | 5 | 100.0% | 48.38 | 4548.79 | 424.73 | Completed |
| React | Chrome | 1 | 20 | 100.0% | 6.78 | 1390.78 | 1314.08 | Completed |
| React | Mimic | 1 | 20 | 100.0% | 7.64 | 169.68 | 134.54 | Completed |
| React | Chrome | 5 | 5 | 100.0% | 4.83 | 1609.12 | 1284.84 | Completed |
| React | Mimic | 5 | 5 | 100.0% | 26.34 | 337.05 | 165.20 | Completed |
| React | Chrome | 10 | 5 | 100.0% | 10.39 | 1949.55 | 1284.71 | Completed |
| React | Mimic | 10 | 5 | 100.0% | 37.17 | 548.45 | 197.61 | Completed |
| React | Chrome | 25 | 5 | 100.0% | 14.19 | 3050.26 | 1319.25 | Completed |
| React | Mimic | 25 | 5 | 100.0% | 47.43 | 1138.94 | 282.39 | Completed |
| React | Chrome | 50 | 5 | 100.0% | 21.06 | 4773.45 | 1307.29 | Completed |
| React | Mimic | 50 | 5 | 100.0% | 53.09 | 2102.10 | 390.90 | Completed |
| React | Chrome | 100 | 5 | 100.0% | 22.49 | 8528.11 | 1414.06 | Completed |
| React | Mimic | 100 | 0 | 99.0% | 6.63 | 4403.67 | 444.02 | non-zero failure rate |

Recovery uses no forced collection. Allocator pools and shared runtime artifacts can remain resident; this table alone cannot prove leak absence. Private memory, marginal slopes, CPU and latency distributions are included in the numerical data.

## Measurement boundaries

Identical local fixtures, unique origins, HTTP cache disabled, full supported resource loading. The six fixtures cover static DOM, JavaScript/crypto, 3,000 DOM elements, asynchronous networking and Workers, React 18.3.1 and WebAssembly. Completion requires the exact expected result. No paint or network-idle delay is included. External high-resolution clocks, 5 ms result polling and 50 ms memory sampling are unchanged. Slow samples are retained; p95 from 10–20 samples is unstable. These measurements do not establish feature parity or costs on arbitrary sites.

## Data and provenance

[Numerical export](public-results.json) includes all summary metrics, numerical single-page and startup samples, concurrency outcomes and executable/harness hashes. [Full report](report.md) and [raw observations](raw.json) retain detailed evidence. [Optimization decisions](../../../docs/performance/optimization-campaign-20260914.md) distinguish these Windows observations from the primary paired Linux experiments.
