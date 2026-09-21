# Mimic vs. Chrome: 2026-09-21

Fresh builds on one Windows workstation, using the unchanged frozen workloads. 12/12 correctness gates and 360/360 measured single-page attempts passed. 0 concurrency series stopped or contained a failure. All observations, including failed attempts and excluded warmups, remain in the data. These are controlled fixtures and do not establish general website compatibility.

## Environment

| Item | Value |
|---|---|
| Platform | Windows-11-10.0.26200-SP0 |
| CPU | Intel(R) Core(TM) i7-14700KF |
| RAM | 31.83 GiB |
| Chrome | 152.0.7977.82 (headless=new) |
| Mimic / Chrome V8 | 15.2.124.1-rusty / 15.2.124.21 |
| Started / completed | 2026-09-21T07:02:35.012031+04:00 / 2026-09-21T07:17:04.983256+04:00 |

Executable hashes are checked before every launch. This is an interactive workstation with background applications and antivirus enabled; cache and scheduler variation remain possible.

## Startup and ready memory

Ten fresh processes per runtime, alternating order, after an excluded warmup. Both answer the same Target.getTargets readiness probe. Summed process-tree RSS includes the initial page and can count shared pages more than once; it is not marginal Page memory.

| Runtime | CDP ready p50, ms | p95, ms | Ready RSS, MiB | Ready private bytes, MiB |
|---|---|---|---|---|
| Mimic | 734.94 | 1403.30 | 147.71 | 174.53 |
| Chrome | 264.47 | 278.83 | 374.99 | 176.84 |

## Warm execution and completion

Twenty retained samples per workload/runtime. Each iteration creates a new Page and origin in the warm process. Execution includes invoking the workload and detecting its validated result. Completion also includes navigation; Page creation and teardown are excluded. Lower is better.

| Workload | Mimic execution, ms | Chrome execution, ms | Mimic completion, ms | Chrome completion, ms |
|---|---|---|---|---|
| Static DOM | 1.58 | 4.07 | 23.32 | 22.76 |
| JavaScript / crypto | 41.80 | 32.26 | 64.28 | 56.34 |
| DOM mutations | 458.97 | 33.50 | 491.72 | 52.50 |
| Async / networking | 64.26 | 39.08 | 89.86 | 77.53 |
| React | 40.52 | 22.55 | 70.12 | 44.27 |
| WebAssembly | 3.42 | 5.18 | 26.97 | 24.55 |

## Cold end-to-end completion

Ten fresh-process samples per workload/runtime. Includes process startup, Page creation, navigation, execution, teardown and process exit. OS caches are not flushed. Server maintenance and temporary-profile removal are excluded. A failed series is withheld from comparisons.

| Workload | Mimic p50, ms | Chrome p50, ms |
|---|---|---|
| Static DOM | 1061.31 | 508.62 |
| JavaScript / crypto | 1104.81 | 539.76 |
| DOM mutations | 1357.71 | 597.93 |
| Async / networking | 1026.70 | 505.64 |
| React | 1120.66 | 812.51 |
| WebAssembly | 848.62 | 475.25 |

## CPU and memory during warm work

Process-tree user plus kernel time per session; sampled peaks may miss short-lived allocations.

| Workload | Runtime | CPU, ms/session | Peak RSS, MiB | Peak private bytes, MiB |
|---|---|---|---|---|
| Static DOM | Mimic | 31.25 | 254.56 | 285.88 |
| Static DOM | Chrome | 164.06 | 1168.50 | 583.26 |
| JavaScript / crypto | Mimic | 85.94 | 255.12 | 284.00 |
| JavaScript / crypto | Chrome | 250.00 | 1411.78 | 778.09 |
| DOM mutations | Mimic | 632.81 | 229.67 | 257.81 |
| DOM mutations | Chrome | 226.56 | 1388.35 | 734.58 |
| Async / networking | Mimic | 109.38 | 222.96 | 253.41 |
| Async / networking | Chrome | 414.06 | 1262.30 | 634.50 |
| React | Mimic | 93.75 | 229.35 | 258.08 |
| React | Chrome | 226.56 | 1404.01 | 800.95 |
| WebAssembly | Mimic | 15.62 | 224.46 | 252.64 |
| WebAssembly | Chrome | 203.12 | 1275.23 | 636.83 |

## Concurrent Pages

Fresh process per level, one excluded warmup, then max(5, ceil(20/N)) measured waves. Throughput includes setup and teardown but excludes the separate 250 ms recovery wait. Every attempted level is shown. A stopped series is not a stable successful result.

| Workload | Runtime | Pages | Waves | Success | Sessions/s | Active RSS, MiB | Recovered RSS, MiB | Status |
|---|---|---|---|---|---|---|---|---|
| Static DOM | Chrome | 1 | 20 | 100.0% | 9.52 | 1211.54 | 1153.09 | Completed |
| Static DOM | Mimic | 1 | 20 | 100.0% | 17.16 | 245.21 | 245.21 | Completed |
| Static DOM | Chrome | 5 | 5 | 100.0% | 18.29 | 1396.87 | 1140.47 | Completed |
| Static DOM | Mimic | 5 | 5 | 100.0% | 35.52 | 338.88 | 290.54 | Completed |
| Static DOM | Chrome | 10 | 5 | 100.0% | 23.06 | 1683.59 | 1129.28 | Completed |
| Static DOM | Mimic | 10 | 5 | 100.0% | 49.45 | 422.69 | 387.74 | Completed |
| Static DOM | Chrome | 25 | 5 | 100.0% | 20.69 | 2598.54 | 1157.55 | Completed |
| Static DOM | Mimic | 25 | 5 | 100.0% | 80.41 | 763.72 | 746.42 | Completed |
| Static DOM | Chrome | 50 | 5 | 100.0% | 21.37 | 4117.22 | 1218.86 | Completed |
| Static DOM | Mimic | 50 | 5 | 100.0% | 87.01 | 1360.69 | 1253.37 | Completed |
| Static DOM | Chrome | 100 | 5 | 100.0% | 23.33 | 7095.85 | 1242.62 | Completed |
| Static DOM | Mimic | 100 | 5 | 100.0% | 89.01 | 2243.30 | 2031.64 | Completed |
| JavaScript / crypto | Chrome | 1 | 20 | 100.0% | 8.69 | 1410.89 | 1333.65 | Completed |
| JavaScript / crypto | Mimic | 1 | 20 | 100.0% | 8.98 | 244.07 | 244.07 | Completed |
| JavaScript / crypto | Chrome | 5 | 5 | 100.0% | 16.18 | 1623.98 | 1287.43 | Completed |
| JavaScript / crypto | Mimic | 5 | 5 | 100.0% | 15.09 | 346.19 | 344.41 | Completed |
| JavaScript / crypto | Chrome | 10 | 5 | 100.0% | 16.77 | 2002.72 | 1301.87 | Completed |
| JavaScript / crypto | Mimic | 10 | 5 | 100.0% | 21.49 | 516.60 | 509.95 | Completed |
| JavaScript / crypto | Chrome | 25 | 5 | 100.0% | 19.76 | 3124.54 | 1324.75 | Completed |
| JavaScript / crypto | Mimic | 25 | 5 | 100.0% | 39.13 | 945.39 | 891.38 | Completed |
| JavaScript / crypto | Chrome | 50 | 5 | 100.0% | 22.01 | 5001.09 | 1355.05 | Completed |
| JavaScript / crypto | Mimic | 50 | 5 | 100.0% | 48.05 | 1487.84 | 1399.56 | Completed |
| JavaScript / crypto | Chrome | 100 | 5 | 100.0% | 22.45 | 8745.16 | 1403.35 | Completed |
| JavaScript / crypto | Mimic | 100 | 5 | 100.0% | 50.75 | 2567.22 | 2533.68 | Completed |
| React | Chrome | 1 | 20 | 100.0% | 8.78 | 1433.39 | 1354.04 | Completed |
| React | Mimic | 1 | 20 | 100.0% | 9.20 | 225.86 | 225.86 | Completed |
| React | Chrome | 5 | 5 | 100.0% | 10.77 | 1600.67 | 1275.59 | Completed |
| React | Mimic | 5 | 5 | 100.0% | 15.49 | 328.32 | 312.02 | Completed |
| React | Chrome | 10 | 5 | 100.0% | 8.41 | 1929.81 | 1257.68 | Completed |
| React | Mimic | 10 | 5 | 100.0% | 20.26 | 478.16 | 458.14 | Completed |
| React | Chrome | 25 | 5 | 100.0% | 15.97 | 3042.06 | 1305.96 | Completed |
| React | Mimic | 25 | 5 | 100.0% | 37.48 | 927.88 | 861.10 | Completed |
| React | Chrome | 50 | 5 | 100.0% | 19.15 | 4782.99 | 1311.38 | Completed |
| React | Mimic | 50 | 5 | 100.0% | 48.33 | 1623.27 | 1524.62 | Completed |
| React | Chrome | 100 | 5 | 100.0% | 14.82 | 8473.57 | 1427.73 | Completed |
| React | Mimic | 100 | 5 | 100.0% | 49.85 | 2736.17 | 2338.18 | Completed |

Recovery uses no forced collection. Allocator pools and shared runtime artifacts can remain resident; this table alone cannot prove leak absence. Private memory, marginal slopes, CPU and latency distributions are included in the numerical data.

## Measurement boundaries

Identical local fixtures, unique origins, HTTP cache disabled, full supported resource loading. The six fixtures cover static DOM, JavaScript/crypto, 3,000 DOM elements, asynchronous networking and Workers, React 18.3.1 and WebAssembly. Completion requires the exact expected result. No paint or network-idle delay is included. External high-resolution clocks, 5 ms result polling and 50 ms memory sampling are unchanged. Slow samples are retained; p95 from 10–20 samples is unstable. These measurements do not establish feature parity or costs on arbitrary sites.

## Data and provenance

[Numerical export](public-results.json) includes all summary metrics, numerical single-page and startup samples, concurrency outcomes and executable/harness hashes. [Full report](report.md) and [raw observations](raw.json) retain detailed evidence. [Optimization decisions](../../../docs/performance/optimization-campaign-20260914.md) distinguish these Windows observations from the primary paired Linux experiments.
