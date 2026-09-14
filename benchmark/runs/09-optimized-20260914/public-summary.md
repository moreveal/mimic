# Mimic vs. Chrome: 2026-09-14

Fresh builds on one Windows workstation, using the unchanged frozen workloads. 12/12 correctness gates and 360/360 measured single-page attempts passed. 3 concurrency series stopped or contained a failure. All observations, including failed attempts and excluded warmups, remain in the data. These are controlled fixtures and do not establish general website compatibility.

## Environment

| Item | Value |
|---|---|
| Platform | Windows-11-10.0.26200-SP0 |
| CPU | Intel(R) Core(TM) i7-14700KF |
| RAM | 31.83 GiB |
| Chrome | 152.0.7977.82 (headless=new) |
| Mimic / Chrome V8 | 15.2.124.1-rusty / 15.2.124.21 |
| Started / completed | 2026-09-14T03:42:47.674701+04:00 / 2026-09-14T03:56:50.108854+04:00 |

Executable hashes are checked before every launch. This is an interactive workstation with background applications and antivirus enabled; cache and scheduler variation remain possible.

## Startup and ready memory

Ten fresh processes per runtime, alternating order, after an excluded warmup. Both answer the same Target.getTargets readiness probe. Summed process-tree RSS includes the initial page and can count shared pages more than once; it is not marginal Page memory.

| Runtime | CDP ready p50, ms | p95, ms | Ready RSS, MiB | Ready private bytes, MiB |
|---|---|---|---|---|
| Mimic | 229.57 | 276.30 | 28.84 | 80.90 |
| Chrome | 236.68 | 269.42 | 376.26 | 175.55 |

## Warm execution and completion

Twenty retained samples per workload/runtime. Each iteration creates a new Page and origin in the warm process. Execution includes invoking the workload and detecting its validated result. Completion also includes navigation; Page creation and teardown are excluded. Lower is better.

| Workload | Mimic execution, ms | Chrome execution, ms | Mimic completion, ms | Chrome completion, ms |
|---|---|---|---|---|
| Static DOM | 3.61 | 4.79 | 38.63 | 27.29 |
| JavaScript / crypto | 39.63 | 27.80 | 72.48 | 49.36 |
| DOM mutations | 96.58 | 29.63 | 131.10 | 52.26 |
| Async / networking | 67.90 | 28.92 | 101.73 | 54.24 |
| React | 42.14 | 23.43 | 84.21 | 47.59 |
| WebAssembly | 4.09 | 4.91 | 38.33 | 27.35 |

## Cold end-to-end completion

Ten fresh-process samples per workload/runtime. Includes process startup, Page creation, navigation, execution, teardown and process exit. OS caches are not flushed. Server maintenance and temporary-profile removal are excluded. A failed series is withheld from comparisons.

| Workload | Mimic p50, ms | Chrome p50, ms |
|---|---|---|
| Static DOM | 603.77 | 418.52 |
| JavaScript / crypto | 618.03 | 434.52 |
| DOM mutations | 665.23 | 420.75 |
| Async / networking | 650.65 | 457.93 |
| React | 641.90 | 484.00 |
| WebAssembly | 591.00 | 450.47 |

## CPU and memory during warm work

Process-tree user plus kernel time per session; sampled peaks may miss short-lived allocations.

| Workload | Runtime | CPU, ms/session | Peak RSS, MiB | Peak private bytes, MiB |
|---|---|---|---|---|
| Static DOM | Mimic | 46.88 | 132.14 | 161.73 |
| Static DOM | Chrome | 187.50 | 1192.59 | 595.12 |
| JavaScript / crypto | Mimic | 93.75 | 158.16 | 187.26 |
| JavaScript / crypto | Chrome | 250.00 | 1408.86 | 789.30 |
| DOM mutations | Mimic | 234.38 | 161.49 | 190.82 |
| DOM mutations | Chrome | 226.56 | 1381.75 | 735.54 |
| Async / networking | Mimic | 156.25 | 156.70 | 189.73 |
| Async / networking | Chrome | 210.94 | 1223.23 | 626.18 |
| React | Mimic | 148.44 | 149.13 | 178.77 |
| React | Chrome | 234.38 | 1401.66 | 797.12 |
| WebAssembly | Mimic | 46.88 | 142.01 | 172.05 |
| WebAssembly | Chrome | 187.50 | 1276.15 | 632.98 |

## Concurrent Pages

Fresh process per level, one excluded warmup, then max(5, ceil(20/N)) measured waves. Throughput includes setup and teardown but excludes the separate 250 ms recovery wait. Every attempted level is shown. A stopped series is not a stable successful result.

| Workload | Runtime | Pages | Waves | Success | Sessions/s | Active RSS, MiB | Recovered RSS, MiB | Status |
|---|---|---|---|---|---|---|---|---|
| Static DOM | Chrome | 1 | 20 | 100.0% | 8.70 | 1195.82 | 1138.68 | Completed |
| Static DOM | Mimic | 1 | 20 | 100.0% | 11.60 | 137.99 | 111.55 | Completed |
| Static DOM | Chrome | 5 | 5 | 100.0% | 23.03 | 1354.80 | 1100.93 | Completed |
| Static DOM | Mimic | 5 | 5 | 100.0% | 35.40 | 266.34 | 128.91 | Completed |
| Static DOM | Chrome | 10 | 5 | 100.0% | 24.05 | 1672.49 | 1129.30 | Completed |
| Static DOM | Mimic | 10 | 5 | 100.0% | 58.08 | 427.59 | 166.00 | Completed |
| Static DOM | Chrome | 25 | 5 | 100.0% | 27.19 | 2555.39 | 1135.20 | Completed |
| Static DOM | Mimic | 25 | 5 | 100.0% | 64.73 | 863.17 | 226.32 | Completed |
| Static DOM | Chrome | 50 | 5 | 100.0% | 21.00 | 4079.95 | 1204.00 | Completed |
| Static DOM | Mimic | 50 | 5 | 100.0% | 69.64 | 1568.62 | 327.83 | Completed |
| Static DOM | Chrome | 100 | 5 | 100.0% | 21.42 | 7032.88 | 1230.30 | Completed |
| Static DOM | Mimic | 100 | 0 | 100.0% | 28.33 | 4261.57 | 220.37 | memory pressure (<15% or 2 GiB available) |
| JavaScript / crypto | Chrome | 1 | 20 | 100.0% | 8.36 | 1410.04 | 1333.15 | Completed |
| JavaScript / crypto | Mimic | 1 | 20 | 100.0% | 8.21 | 155.96 | 116.28 | Completed |
| JavaScript / crypto | Chrome | 5 | 5 | 100.0% | 17.15 | 1597.24 | 1264.19 | Completed |
| JavaScript / crypto | Mimic | 5 | 5 | 100.0% | 26.85 | 337.32 | 138.62 | Completed |
| JavaScript / crypto | Chrome | 10 | 5 | 100.0% | 20.35 | 1977.37 | 1279.54 | Completed |
| JavaScript / crypto | Mimic | 10 | 5 | 100.0% | 36.18 | 552.32 | 159.82 | Completed |
| JavaScript / crypto | Chrome | 25 | 5 | 100.0% | 21.40 | 3103.66 | 1321.52 | Completed |
| JavaScript / crypto | Mimic | 25 | 5 | 100.0% | 45.55 | 1184.45 | 194.19 | Completed |
| JavaScript / crypto | Chrome | 50 | 5 | 100.0% | 25.91 | 4950.75 | 1340.87 | Completed |
| JavaScript / crypto | Mimic | 50 | 5 | 100.0% | 50.55 | 2233.92 | 256.05 | Completed |
| JavaScript / crypto | Chrome | 100 | 5 | 100.0% | 18.66 | 8651.08 | 1377.55 | Completed |
| JavaScript / crypto | Mimic | 100 | 0 | 100.0% | 19.29 | 4784.39 | 332.48 | memory pressure (<15% or 2 GiB available) |
| React | Chrome | 1 | 20 | 100.0% | 7.82 | 1420.46 | 1341.79 | Completed |
| React | Mimic | 1 | 20 | 100.0% | 7.61 | 147.53 | 115.53 | Completed |
| React | Chrome | 5 | 5 | 100.0% | 6.47 | 1586.57 | 1267.60 | Completed |
| React | Mimic | 5 | 5 | 100.0% | 26.23 | 305.87 | 145.00 | Completed |
| React | Chrome | 10 | 5 | 100.0% | 7.98 | 1913.55 | 1251.44 | Completed |
| React | Mimic | 10 | 5 | 100.0% | 37.20 | 495.99 | 176.36 | Completed |
| React | Chrome | 25 | 5 | 100.0% | 16.91 | 2984.65 | 1278.16 | Completed |
| React | Mimic | 25 | 5 | 100.0% | 46.98 | 1041.46 | 251.05 | Completed |
| React | Chrome | 50 | 5 | 100.0% | 16.40 | 4786.41 | 1328.32 | Completed |
| React | Mimic | 50 | 5 | 100.0% | 50.29 | 1924.60 | 351.08 | Completed |
| React | Chrome | 100 | 5 | 100.0% | 18.12 | 8277.64 | 1356.93 | Completed |
| React | Mimic | 100 | 0 | 100.0% | 19.66 | 4914.61 | 541.29 | memory pressure (<15% or 2 GiB available) |

Recovery uses no forced collection. Allocator pools and shared runtime artifacts can remain resident; this table alone cannot prove leak absence. Private memory, marginal slopes, CPU and latency distributions are included in the numerical data.

## Measurement boundaries

Identical local fixtures, unique origins, HTTP cache disabled, full supported resource loading. The six fixtures cover static DOM, JavaScript/crypto, 3,000 DOM elements, asynchronous networking and Workers, React 18.3.1 and WebAssembly. Completion requires the exact expected result. No paint or network-idle delay is included. External high-resolution clocks, 5 ms result polling and 50 ms memory sampling are unchanged. Slow samples are retained; p95 from 10–20 samples is unstable. These measurements do not establish feature parity or costs on arbitrary sites.

## Data and provenance

[Numerical export](public-results.json) includes all summary metrics, numerical single-page and startup samples, concurrency outcomes and executable/harness hashes. [Full report](report.md) and [raw observations](raw.json) retain detailed evidence. [Optimization decisions](../../../docs/performance/optimization-campaign-20260914.md) distinguish these Windows observations from the primary paired Linux experiments.
