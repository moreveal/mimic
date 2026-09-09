# Mimic V8 and Chrome 152: Windows x64 baseline

This is a measurement after semantic fixes. No runtime performance optimizations were performed. The original version's correctness check is saved separately in `pre-fix/raw.json`.

## Environment

| Parameter | Value |
|---|---|
| Start / end date | 2026-09-08T23:36:37.859612+04:00 / 2026-09-08T23:48:06.475049+04:00 |
| Chrome | {'protocolVersion': '1.3', 'product': 'Chrome/152.0.7977.82', 'revision': '@d04cdb24d67b081f6cf80200ffc5233f44b61109', 'userAgent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/152.0.0.0 Safari/537.36', 'jsVersion': '15.2.124.21'} |
| Chromium | 1669021 / d04cdb24d67b081f6cf80200ffc5233f44b61109 |
| Mimic commit | 0abfa05799f6a8609bffef111ea411624be84751 |
| Mimic V8 | 15.2.124.1-rusty |
| OS | Windows-11-10.0.26200-SP0 |
| CPU | {"Name":"Intel(R) Core(TM) i7-14700KF","NumberOfCores":20,"NumberOfLogicalProcessors":28} |
| CPU physical / logical | 20 / 28 |
| RAM GiB | 31.83 |
| Power plan | Power Scheme GUID: 381b4222-f694-41f0-9685-ff5bb260df2e  (Balanced) |
| Power overlay | 0 00000000-0000-0000-0000-000000000000 |
| Antivirus | {"displayName":"Windows Defender","productState":397568} |
| Chrome SHA-256 | ea36dd818a90176f1a70616f0363d9be527229389a6c073a0b1688b9e73f67e9 |
| Mimic SHA-256 | ea237da15757c8f1a11e8a8632cc5a9495040cd64d4373e923a746e1833988c6 |

The workstation was not dedicated exclusively to the test: background applications and antivirus were enabled. Process lists, tool versions, arguments, and fixture/binary SHA-256 hashes are in raw.json.

## Methodology and correctness

Both systems create a fresh page and a unique loopback origin. HTTP cache is disabled, responses use no-store, and cookies are not used. Contexts, transport, and the process persist in warm runs; page state is not reused. This provides isolation for this controlled corpus, not a test of tenant/security isolation. Cold runs create a new process and profile. Windows file, DLL, and OS DNS caches are not cleared; “cold” means a fresh process, not a cold disk. Warm HTTP-cache behavior was not measured.

Navigation completes only at the exact URL, with document.readyState === "complete" and __benchRun present. Both systems then explicitly invoke __benchRun; completion requires done and an exact match of the deterministic result. Settle = 0. Paint/networkidle are not awaited. Chrome uses headless=new; results do not automatically apply to headful mode.

External perf_counter/QPC clock; 5 ms polling plus CDP/OS scheduler latency. navigation_ms includes HTML parsing and script loading; execution_ms includes application startup/execution and marker detection. These are not isolated JIT or pure JavaScript timings. The page's js_ms is diagnostic: Mimic's virtual time is often zero.

The process starts suspended, is assigned to a Windows Job Object, and then resumes. process_start_ms measures the process-creation call; CDP readiness runs from the start of creation to the protocol response. runtime_initialization_ms is the remainder between them; internal V8 phases are not separately instrumented. Cold total includes page creation, execution, teardown, and process exit; temporary-profile removal and local-server maintenance are excluded.

CPU is user+kernel for the entire Job Object, including exited descendants. Working set/private bytes sum all current Job Object members every 50 ms and at checkpoints. Short memory peaks may be missed, and shared DLL pages may be counted multiple times. CPU % is relative to one logical core; 100% of the machine = 2800%. CPU peaks are sensitive to Windows counter granularity.

One correctness check and one initial warmup per series are explicitly excluded. Main series: 10 cold and 20 warm runs, without removing slow observations. p95 is published at n≥10 and p99 at n≥100; empirical quantiles use linear interpolation, and tails at n=10–20 are particularly unstable. SD, CV, and min/max for all series are available in summary.csv. Speedups are not aggregated into a single ratio.

| System / workload | Correctness gate |
|---|---|
| chrome/async | VALID |
| chrome/cpu | VALID |
| chrome/dom | VALID |
| chrome/react | VALID |
| chrome/static | VALID |
| chrome/wasm | VALID |
| mimic/async | VALID |
| mimic/cpu | VALID |
| mimic/dom | VALID |
| mimic/react | VALID |
| mimic/static | VALID |
| mimic/wasm | VALID |

### CDP readiness: separate series with an identical probe

In the final series, both systems respond to Target.getTargets after the WebSocket handshake. Each uses 10 fresh processes, alternating system order; warmup is excluded. Date: 2026-09-08T23:47:36.643861+04:00. The main series uses the same shared probe.

| System | n | CDP p50 ms | p95 | Min | Max | SD | Ready RSS MiB |
|---|---|---|---|---|---|---|---|
| mimic | 10 | 230.44 | 243.69 | 218.84 | 244.59 | 9.69 | 24.37 |
| chrome | 10 | 327.02 | 403.00 | 297.38 | 454.64 | 43.40 | 382.17 |

## Cold startup (medians, ms)

| System | Workload | n | Process create | CDP ready | Runtime init | Cold total | Shutdown |
|---|---|---|---|---|---|---|---|
| chrome | static | 10 | 9.97 | 293.37 | 283.26 | 487.56 | 104.57 |
| mimic | static | 10 | 9.16 | 234.11 | 224.73 | 476.16 | 70.68 |
| mimic | cpu | 10 | 9.51 | 231.60 | 221.55 | 518.44 | 69.08 |
| chrome | cpu | 10 | 9.33 | 298.45 | 284.66 | 545.23 | 113.97 |
| chrome | dom | 10 | 9.39 | 291.87 | 281.57 | 484.66 | 105.97 |
| mimic | dom | 10 | 10.09 | 234.44 | 223.66 | 552.97 | 71.16 |
| mimic | async | 10 | 9.94 | 229.58 | 220.03 | 527.08 | 74.88 |
| chrome | async | 10 | 9.71 | 278.86 | 269.43 | 488.06 | 105.25 |
| chrome | react | 10 | 9.32 | 276.80 | 267.29 | 473.84 | 107.85 |
| mimic | react | 10 | 9.68 | 230.26 | 220.23 | 517.82 | 76.27 |
| mimic | wasm | 10 | 7.19 | 225.56 | 218.98 | 461.82 | 70.35 |
| chrome | wasm | 10 | 7.09 | 318.04 | 311.02 | 567.52 | 119.41 |

## Warm session startup / teardown (medians)

| System | Workload | n | Create ms | Teardown ms | RSS after teardown MiB |
|---|---|---|---|---|---|
| chrome | static | 20 | 39.50 | 14.16 | 1208.15 |
| mimic | static | 20 | 13.80 | 4.28 | 78.02 |
| mimic | cpu | 20 | 4.64 | 5.86 | 79.39 |
| chrome | cpu | 20 | 37.24 | 14.20 | 1376.78 |
| chrome | dom | 20 | 36.97 | 14.38 | 1352.64 |
| mimic | dom | 20 | 12.95 | 5.51 | 86.46 |
| mimic | async | 20 | 13.06 | 4.26 | 80.55 |
| chrome | async | 20 | 37.50 | 14.07 | 1240.78 |
| chrome | react | 20 | 38.89 | 15.70 | 1426.83 |
| mimic | react | 20 | 8.54 | 5.40 | 80.29 |
| mimic | wasm | 20 | 3.35 | 4.62 | 78.29 |
| chrome | wasm | 20 | 39.47 | 15.00 | 1286.12 |

## Single-session workload latency (ms)

| System | Workload | Mode | n | Nav p50 | Execution p50 | Completion p50 | p95 | Min | Max | SD | CV |
|---|---|---|---|---|---|---|---|---|---|---|---|
| chrome | static | cold | 10 | 25.38 | 3.06 | 28.54 | 34.70 | 25.04 | 37.10 | 3.79 | 0.13 |
| mimic | static | cold | 10 | 146.74 | 1.35 | 148.07 | 151.82 | 144.37 | 152.65 | 2.66 | 0.02 |
| chrome | static | warm | 20 | 23.56 | 4.90 | 28.76 | 34.21 | 18.69 | 54.21 | 7.26 | 0.26 |
| mimic | static | warm | 20 | 57.68 | 1.42 | 59.10 | 62.49 | 54.18 | 67.68 | 3.06 | 0.05 |
| mimic | cpu | cold | 10 | 145.58 | 51.63 | 196.37 | 212.90 | 189.73 | 223.42 | 9.15 | 0.05 |
| chrome | cpu | cold | 10 | 27.02 | 33.02 | 64.05 | 71.24 | 53.85 | 71.88 | 6.71 | 0.11 |
| mimic | cpu | warm | 20 | 57.34 | 50.25 | 107.40 | 115.10 | 94.46 | 117.17 | 6.06 | 0.06 |
| chrome | cpu | warm | 20 | 23.32 | 28.99 | 51.75 | 55.85 | 46.77 | 57.61 | 3.01 | 0.06 |
| chrome | dom | cold | 10 | 23.99 | 12.21 | 35.73 | 50.82 | 33.50 | 61.56 | 8.35 | 0.22 |
| mimic | dom | cold | 10 | 147.82 | 74.44 | 221.62 | 239.42 | 218.56 | 248.66 | 8.84 | 0.04 |
| chrome | dom | warm | 20 | 24.10 | 30.54 | 54.73 | 57.51 | 29.68 | 57.90 | 6.15 | 0.12 |
| mimic | dom | warm | 20 | 57.85 | 73.41 | 131.73 | 136.50 | 121.05 | 137.69 | 4.62 | 0.04 |
| mimic | async | cold | 10 | 158.05 | 45.18 | 200.94 | 216.46 | 188.20 | 218.75 | 9.52 | 0.05 |
| chrome | async | cold | 10 | 23.61 | 27.94 | 51.55 | 66.87 | 46.96 | 72.78 | 7.66 | 0.14 |
| mimic | async | warm | 20 | 59.02 | 49.39 | 109.78 | 113.89 | 97.24 | 121.58 | 6.07 | 0.06 |
| chrome | async | warm | 20 | 23.45 | 29.48 | 52.95 | 57.06 | 37.21 | 58.02 | 5.68 | 0.11 |
| chrome | react | cold | 10 | 26.82 | 16.94 | 44.00 | 81.62 | 34.99 | 109.43 | 21.59 | 0.44 |
| mimic | react | cold | 10 | 163.82 | 36.91 | 199.34 | 209.27 | 195.21 | 211.49 | 5.40 | 0.03 |
| chrome | react | warm | 20 | 26.14 | 26.39 | 52.79 | 57.41 | 43.42 | 59.87 | 3.50 | 0.07 |
| mimic | react | warm | 20 | 70.76 | 38.09 | 109.51 | 115.16 | 96.71 | 117.06 | 5.98 | 0.06 |
| mimic | wasm | cold | 10 | 148.61 | 9.19 | 157.90 | 180.93 | 153.53 | 188.01 | 11.31 | 0.07 |
| chrome | wasm | cold | 10 | 26.32 | 6.24 | 34.81 | 67.86 | 29.38 | 90.91 | 18.39 | 0.46 |
| mimic | wasm | warm | 20 | 62.62 | 9.36 | 71.68 | 76.30 | 63.64 | 86.01 | 4.59 | 0.06 |
| chrome | wasm | warm | 20 | 25.08 | 6.62 | 31.77 | 37.40 | 28.36 | 40.77 | 3.09 | 0.10 |

## Single-session memory / CPU (medians)

| System | Workload | Mode | Before page MiB | After create MiB | Peak RSS MiB | Peak private MiB | CPU/session ms | CPU/workload ms |
|---|---|---|---|---|---|---|---|---|
| chrome | static | cold | 378.42 | 425.77 | 477.48 | 251.45 | 390.62 | 171.88 |
| mimic | static | cold | 24.31 | 24.50 | 89.67 | 123.39 | 195.31 | 195.31 |
| chrome | static | warm | 1146.18 | 1196.22 | 1208.15 | 605.57 | 187.50 | 78.12 |
| mimic | static | warm | 77.39 | 77.43 | 98.62 | 131.86 | 93.75 | 78.12 |
| mimic | cpu | cold | 24.35 | 24.52 | 107.12 | 140.92 | 250.00 | 250.00 |
| chrome | cpu | cold | 381.01 | 433.69 | 539.37 | 287.63 | 531.25 | 226.56 |
| mimic | cpu | warm | 78.44 | 78.49 | 115.93 | 148.85 | 140.62 | 140.62 |
| chrome | cpu | warm | 1298.57 | 1345.86 | 1376.78 | 769.87 | 250.00 | 132.81 |
| chrome | dom | cold | 383.78 | 433.43 | 490.63 | 257.37 | 421.88 | 156.25 |
| mimic | dom | cold | 24.35 | 24.52 | 108.04 | 141.43 | 296.88 | 289.06 |
| chrome | dom | warm | 1266.53 | 1314.96 | 1352.64 | 715.03 | 296.88 | 117.19 |
| mimic | dom | warm | 86.24 | 86.25 | 115.55 | 147.90 | 203.12 | 195.31 |
| mimic | async | cold | 24.32 | 24.53 | 92.35 | 125.79 | 210.94 | 210.94 |
| chrome | async | cold | 383.85 | 432.46 | 504.16 | 259.65 | 406.25 | 187.50 |
| mimic | async | warm | 80.27 | 80.28 | 102.70 | 136.77 | 125.00 | 109.38 |
| chrome | async | warm | 1177.50 | 1225.35 | 1240.96 | 623.87 | 218.75 | 125.00 |
| chrome | react | cold | 377.73 | 426.98 | 506.61 | 274.07 | 375.00 | 195.31 |
| mimic | react | cold | 24.37 | 24.50 | 96.60 | 129.23 | 250.00 | 250.00 |
| chrome | react | warm | 1345.86 | 1393.59 | 1426.83 | 810.46 | 281.25 | 156.25 |
| mimic | react | warm | 79.96 | 79.96 | 104.16 | 136.55 | 171.88 | 171.88 |
| mimic | wasm | cold | 24.33 | 24.55 | 90.32 | 123.01 | 203.12 | 203.12 |
| chrome | wasm | cold | 387.76 | 433.00 | 491.12 | 257.19 | 359.38 | 156.25 |
| mimic | wasm | warm | 77.37 | 77.42 | 98.69 | 131.02 | 109.38 | 93.75 |
| chrome | wasm | warm | 1218.35 | 1264.44 | 1286.12 | 640.43 | 218.75 | 78.12 |

## Concurrency / density

Each level uses a separate process, one excluded warmup, and max(5, ceil(20/N)) measured waves. Pages are created concurrently and all begin navigation after a barrier. Completed pages are held until the wave ends to measure simultaneous RSS. Throughput = successful sessions / time from create to final teardown, including the barrier and measurements but excluding HTTP-server setup/cleanup. Latency = create→completion, including barrier wait. This is batch throughput, not an optimized continuous request stream. Holding pages adds to throughput time but not latency. CPU per session = total wave CPU / successes; overlapping per-page CPU intervals are not summed.

Stopping limits: any error, <15% or <2 GiB available RAM, >1024 pages input/s for 3 s, or a 180 s timeout. These protect the workstation; the highest passing level is a lower bound on capacity supported here, not proof of an absolute maximum.

Mimic serializes CDP commands with a shared mutex. The measurement includes this behavior; its cost was not profiled. In stopped rows, RSS may have been sampled before all pages completed, and 0 waves means stopping during the excluded warmup. Such rows are diagnostic and excluded from stable-level fits/charts. Between waves, an additional 250 ms recovery, server cleanup, and checkpoint writing are outside batch throughput.

| System | Workload | N | Waves | Success % | RSS MiB | RSS/N MiB | Peak MiB | CPU/session ms | Sessions/s | p50 ms | p95 ms | p99 ms | Stop |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| chrome | static | 1 | 20 | 100.00 | 1180.28 | 1180.28 | 1191.09 | 250.00 | 8.40 | 59.74 | 79.50 | — |  |
| mimic | static | 1 | 20 | 100.00 | 97.64 | 97.64 | 99.55 | 95.31 | 9.34 | 71.17 | 83.45 | — |  |
| chrome | static | 5 | 5 | 100.00 | 1402.76 | 280.55 | 1409.43 | 183.75 | 16.41 | 226.50 | 248.30 | — |  |
| mimic | static | 5 | 5 | 100.00 | 186.66 | 37.33 | 194.93 | 150.00 | 29.61 | 107.95 | 116.71 | — |  |
| chrome | static | 10 | 5 | 100.00 | 1652.68 | 165.27 | 1668.59 | 137.19 | 21.30 | 375.49 | 399.88 | — |  |
| mimic | static | 10 | 5 | 100.00 | 297.04 | 29.70 | 310.95 | 175.94 | 54.57 | 140.59 | 150.99 | — |  |
| chrome | static | 25 | 5 | 100.00 | 2573.37 | 102.93 | 2579.70 | 138.75 | 21.45 | 1100.45 | 1121.85 | 1122.68 |  |
| mimic | static | 25 | 5 | 100.00 | 612.73 | 24.51 | 660.38 | 199.88 | 57.62 | 320.96 | 344.10 | 350.80 |  |
| chrome | static | 50 | 5 | 100.00 | 4084.75 | 81.69 | 4100.41 | 150.94 | 20.07 | 2072.14 | 2312.10 | 2318.83 |  |
| mimic | static | 50 | 5 | 100.00 | 1181.57 | 23.63 | 1219.91 | 197.56 | 18.42 | 502.88 | 575.95 | 589.00 |  |
| chrome | static | 100 | 0 | 0.00 | 5025.58 | 50.26 | 5411.08 | — | 0.00 | 2368.83 | 2397.64 | 2400.89 | memory pressure (<15% or 2 GiB available) |
| mimic | static | 100 | 0 | 100.00 | 1736.37 | 17.36 | 2468.75 | 212.03 | 50.41 | 1544.90 | 1697.12 | 1776.76 | memory pressure (<15% or 2 GiB available) |
| chrome | cpu | 1 | 20 | 100.00 | 1409.80 | 1409.80 | 1415.55 | 303.12 | 7.67 | 98.27 | 109.71 | — |  |
| mimic | cpu | 1 | 20 | 100.00 | 115.74 | 115.74 | 120.77 | 175.78 | 6.13 | 129.98 | 146.43 | — |  |
| chrome | cpu | 5 | 5 | 100.00 | 1594.10 | 318.82 | 1608.52 | 236.88 | 13.96 | 296.99 | 348.47 | — |  |
| mimic | cpu | 5 | 5 | 100.00 | 268.77 | 53.75 | 281.02 | 206.88 | 21.87 | 170.66 | 183.68 | — |  |
| chrome | cpu | 10 | 5 | 100.00 | 2008.52 | 200.85 | 2011.70 | 218.44 | 16.46 | 467.57 | 557.98 | — |  |
| mimic | cpu | 10 | 5 | 100.00 | 443.45 | 44.34 | 453.31 | 296.25 | 31.93 | 243.96 | 267.40 | — |  |
| chrome | cpu | 25 | 5 | 100.00 | 3117.14 | 124.69 | 3123.39 | 209.75 | 18.15 | 1152.05 | 1312.28 | 1316.45 |  |
| mimic | cpu | 25 | 5 | 100.00 | 998.96 | 39.96 | 1020.30 | 355.25 | 38.88 | 503.87 | 546.28 | 555.60 |  |
| chrome | cpu | 50 | 5 | 100.00 | 4995.09 | 99.90 | 5014.14 | 216.62 | 17.28 | 2472.37 | 2743.63 | 2754.27 |  |
| mimic | cpu | 50 | 5 | 100.00 | 1901.15 | 38.02 | 1933.57 | 377.62 | 45.19 | 806.70 | 910.20 | 924.77 |  |
| chrome | cpu | 100 | 0 | 100.00 | 5856.24 | 58.56 | 7874.45 | 209.22 | 22.64 | 3967.34 | 4209.07 | 4234.06 | memory pressure (<15% or 2 GiB available) |
| mimic | cpu | 100 | 0 | 100.00 | 1940.48 | 19.40 | 3246.90 | 367.03 | 30.77 | 2664.81 | 3057.89 | 3117.66 | memory pressure (<15% or 2 GiB available) |
| chrome | react | 1 | 20 | 100.00 | 1404.31 | 1404.31 | 1413.46 | 285.94 | 8.09 | 88.70 | 101.82 | — |  |
| mimic | react | 1 | 20 | 100.00 | 104.40 | 104.40 | 109.52 | 174.22 | 6.22 | 118.49 | 135.66 | — |  |
| chrome | react | 5 | 5 | 100.00 | 1587.31 | 317.46 | 1596.83 | 281.25 | 4.56 | 1018.15 | 1039.09 | — |  |
| mimic | react | 5 | 5 | 100.00 | 216.09 | 43.22 | 222.02 | 213.75 | 23.33 | 166.14 | 194.44 | — |  |
| chrome | react | 10 | 5 | 100.00 | 1934.16 | 193.42 | 1937.50 | 249.38 | 9.79 | 867.21 | 987.11 | — |  |
| mimic | react | 10 | 5 | 100.00 | 352.66 | 35.27 | 358.29 | 265.00 | 31.62 | 255.54 | 273.80 | — |  |
| chrome | react | 25 | 5 | 100.00 | 3028.67 | 121.15 | 3076.76 | 185.50 | 16.91 | 930.18 | 1921.40 | 1941.97 |  |
| mimic | react | 25 | 5 | 100.00 | 793.38 | 31.74 | 798.49 | 284.00 | 46.78 | 424.79 | 459.72 | 468.47 |  |
| chrome | react | 50 | 5 | 100.00 | 4766.05 | 95.32 | 4771.64 | 220.12 | 15.44 | 2874.44 | 3050.69 | 3057.38 |  |
| mimic | react | 50 | 5 | 100.00 | 1466.54 | 29.33 | 1512.03 | 302.00 | 46.15 | 799.33 | 905.70 | 919.87 |  |
| chrome | react | 100 | 0 | 100.00 | 5431.79 | 54.32 | 7576.46 | 321.72 | 13.07 | 6287.34 | 7470.59 | 7602.23 | sustained paging (>1024 pages/s for 3 seconds) |
| mimic | react | 100 | 0 | 100.00 | 2076.41 | 20.76 | 2581.77 | 334.22 | 36.54 | 2273.61 | 2514.47 | 2535.90 | memory pressure (<15% or 2 GiB available) |

## Marginal RAM/session

The finite difference between adjacent tested N values is measured and divided by ΔN; this is not a direct measurement of every N→N+1 step. The linear model is a descriptive OLS fit to medians of successful levels only. The intercept is an extrapolation; actual startup overhead includes the initial blank page. Total RSS includes processes and caches retained from earlier waves, and the number of waves depends on N. The fit therefore combines active-page costs with process history. RSS growth relative to the start of the same wave is also shown; it can include background activity and GC.

| System | Workload | N | (Active RSS − before wave RSS)/N MiB |
|---|---|---|---|
| chrome | static | 1 | 58.68 |
| mimic | static | 1 | 20.58 |
| chrome | static | 5 | 51.59 |
| mimic | static | 5 | 20.36 |
| chrome | static | 10 | 54.88 |
| mimic | static | 10 | 20.24 |
| chrome | static | 25 | 56.73 |
| mimic | static | 25 | 20.13 |
| chrome | static | 50 | 57.47 |
| mimic | static | 50 | 20.40 |
| chrome | static | 100 | 45.77 |
| mimic | static | 100 | 17.11 |
| chrome | cpu | 1 | 78.35 |
| mimic | cpu | 1 | 37.76 |
| chrome | cpu | 5 | 67.65 |
| mimic | cpu | 5 | 36.26 |
| chrome | cpu | 10 | 70.17 |
| mimic | cpu | 10 | 34.57 |
| chrome | cpu | 25 | 71.49 |
| mimic | cpu | 25 | 35.17 |
| chrome | cpu | 50 | 72.35 |
| mimic | cpu | 50 | 35.16 |
| chrome | cpu | 100 | 54.07 |
| mimic | cpu | 100 | 19.16 |
| chrome | react | 1 | 80.38 |
| mimic | react | 1 | 24.12 |
| chrome | react | 5 | 65.62 |
| mimic | react | 5 | 23.75 |
| chrome | react | 10 | 67.07 |
| mimic | react | 10 | 23.56 |
| chrome | react | 25 | 68.42 |
| mimic | react | 25 | 24.77 |
| chrome | react | 50 | 69.14 |
| mimic | react | 50 | 24.06 |
| chrome | react | 100 | 49.64 |
| mimic | react | 100 | 20.52 |

| System | Workload | N low→high | ΔRSS MiB | ΔRSS/ΔN MiB |
|---|---|---|---|---|
| chrome | cpu | 1→5 | 184.30 | 46.08 |
| chrome | cpu | 5→10 | 414.42 | 82.88 |
| chrome | cpu | 10→25 | 1108.62 | 73.91 |
| chrome | cpu | 25→50 | 1877.95 | 75.12 |
| mimic | cpu | 1→5 | 153.03 | 38.26 |
| mimic | cpu | 5→10 | 174.68 | 34.94 |
| mimic | cpu | 10→25 | 555.51 | 37.03 |
| mimic | cpu | 25→50 | 902.19 | 36.09 |
| chrome | react | 1→5 | 183.00 | 45.75 |
| chrome | react | 5→10 | 346.85 | 69.37 |
| chrome | react | 10→25 | 1094.51 | 72.97 |
| chrome | react | 25→50 | 1737.38 | 69.50 |
| mimic | react | 1→5 | 111.69 | 27.92 |
| mimic | react | 5→10 | 136.58 | 27.32 |
| mimic | react | 10→25 | 440.71 | 29.38 |
| mimic | react | 25→50 | 673.16 | 26.93 |
| chrome | static | 1→5 | 222.48 | 55.62 |
| chrome | static | 5→10 | 249.91 | 49.98 |
| chrome | static | 10→25 | 920.70 | 61.38 |
| chrome | static | 25→50 | 1511.38 | 60.45 |
| mimic | static | 1→5 | 89.02 | 22.26 |
| mimic | static | 5→10 | 110.38 | 22.07 |
| mimic | static | 10→25 | 315.69 | 21.05 |
| mimic | static | 25→50 | 568.84 | 22.75 |

| System | Workload | Fixed fit MiB | Marginal fit MiB | R² | Max stable N |
|---|---|---|---|---|---|
| chrome | cpu | 1275.11 | 74.17 | 1.00 | 50 |
| mimic | cpu | 82.86 | 36.42 | 1.00 | 50 |
| chrome | react | 1275.21 | 69.72 | 1.00 | 50 |
| mimic | react | 78.88 | 27.90 | 1.00 | 50 |
| chrome | static | 1094.34 | 59.58 | 1.00 | 50 |
| mimic | static | 73.86 | 22.05 | 1.00 | 50 |

## Teardown / recovery

| System | Workload | N | Ready RSS MiB | RSS after waves MiB | Whole-series CPU s | CPU % |
|---|---|---|---|---|---|---|
| chrome | static | 1 | 380.93 | 1124.48 | 5.00 | 210.07 |
| mimic | static | 1 | 24.19 | 77.45 | 1.91 | 89.01 |
| chrome | static | 5 | 395.39 | 1144.91 | 4.59 | 301.52 |
| mimic | static | 5 | 24.26 | 85.49 | 3.75 | 444.18 |
| chrome | static | 10 | 379.16 | 1116.79 | 6.86 | 292.22 |
| mimic | static | 10 | 24.34 | 94.63 | 8.80 | 960.09 |
| chrome | static | 25 | 375.59 | 1161.12 | 17.34 | 297.67 |
| mimic | static | 25 | 24.44 | 109.54 | 24.98 | 1151.75 |
| chrome | static | 50 | 386.73 | 1216.35 | 37.73 | 302.94 |
| mimic | static | 50 | 24.31 | 135.42 | 49.39 | 363.93 |
| chrome | static | 100 | 363.75 | 1042.04 | 10.25 | 356.34 |
| mimic | static | 100 | 24.95 | 180.66 | 21.20 | 1068.83 |
| chrome | cpu | 1 | 381.52 | 1332.51 | 6.06 | 232.63 |
| mimic | cpu | 1 | 24.82 | 79.34 | 3.52 | 107.82 |
| chrome | cpu | 5 | 368.96 | 1265.62 | 5.92 | 330.78 |
| mimic | cpu | 5 | 24.29 | 88.90 | 5.17 | 452.48 |
| chrome | cpu | 10 | 380.64 | 1309.29 | 10.92 | 359.60 |
| mimic | cpu | 10 | 24.64 | 97.74 | 14.81 | 945.88 |
| chrome | cpu | 25 | 387.54 | 1329.86 | 26.22 | 380.72 |
| mimic | cpu | 25 | 24.27 | 120.93 | 44.41 | 1381.12 |
| chrome | cpu | 50 | 387.15 | 1371.12 | 54.16 | 374.23 |
| mimic | cpu | 50 | 24.21 | 144.15 | 94.41 | 1706.53 |
| chrome | cpu | 100 | 368.77 | 1372.54 | 20.92 | 473.74 |
| mimic | cpu | 100 | 24.44 | 187.15 | 36.70 | 1129.48 |
| chrome | react | 1 | 377.21 | 1326.34 | 5.72 | 231.21 |
| mimic | react | 1 | 24.93 | 80.89 | 3.48 | 108.32 |
| chrome | react | 5 | 377.21 | 1267.98 | 7.03 | 128.12 |
| mimic | react | 5 | 24.32 | 100.51 | 5.34 | 498.78 |
| chrome | react | 10 | 391.77 | 1264.48 | 12.47 | 244.18 |
| mimic | react | 10 | 24.35 | 118.10 | 13.25 | 837.83 |
| chrome | react | 25 | 391.39 | 1318.66 | 23.19 | 313.75 |
| mimic | react | 25 | 24.80 | 177.01 | 35.50 | 1328.44 |
| chrome | react | 50 | 392.16 | 1309.33 | 55.03 | 339.79 |
| mimic | react | 50 | 24.32 | 262.45 | 75.50 | 1393.59 |
| chrome | react | 100 | 383.76 | 1421.87 | 32.17 | 420.56 |
| mimic | react | 100 | 24.29 | 349.13 | 33.42 | 1221.13 |

## Local server (measured independently)

HTTP handler time runs from the handler receiving a request to completion of response writing; it excludes TCP/server scheduler queues and network roundtrip. Before the main workload, the server is created outside the measured interval. Requests and bytes for each resource are recorded in raw.json.

| System | Workload | Mode | Requests | Server p50 ms | Server p95 ms | Server max ms |
|---|---|---|---|---|---|---|
| chrome | static | cold | 20 | 0.34 | 0.45 | 0.57 |
| mimic | static | cold | 20 | 0.22 | 0.39 | 0.83 |
| chrome | static | warm | 40 | 0.31 | 0.46 | 0.48 |
| mimic | static | warm | 40 | 0.29 | 0.53 | 0.59 |
| mimic | cpu | cold | 20 | 0.34 | 0.82 | 1.71 |
| chrome | cpu | cold | 20 | 0.34 | 0.58 | 0.64 |
| mimic | cpu | warm | 40 | 0.25 | 0.83 | 2.65 |
| chrome | cpu | warm | 40 | 0.29 | 0.49 | 1.14 |
| chrome | dom | cold | 20 | 0.35 | 0.49 | 0.62 |
| mimic | dom | cold | 20 | 0.33 | 1.00 | 1.11 |
| chrome | dom | warm | 40 | 0.31 | 0.46 | 0.56 |
| mimic | dom | warm | 40 | 0.27 | 0.39 | 0.88 |
| mimic | async | cold | 50 | 0.15 | 0.58 | 0.76 |
| chrome | async | cold | 50 | 0.25 | 0.51 | 0.55 |
| mimic | async | warm | 100 | 0.14 | 0.65 | 1.93 |
| chrome | async | warm | 100 | 0.16 | 0.40 | 1.08 |
| chrome | react | cold | 50 | 0.55 | 1.07 | 1.26 |
| mimic | react | cold | 50 | 0.33 | 0.49 | 0.74 |
| chrome | react | warm | 100 | 0.36 | 0.96 | 1.11 |
| mimic | react | warm | 100 | 0.32 | 0.56 | 0.87 |
| mimic | wasm | cold | 20 | 0.20 | 0.34 | 0.38 |
| chrome | wasm | cold | 20 | 0.27 | 0.55 | 0.69 |
| mimic | wasm | warm | 40 | 0.22 | 0.39 | 0.49 |
| chrome | wasm | warm | 40 | 0.29 | 0.52 | 1.45 |

## Validity of measured series

| System | Workload | Mode | Successes / attempts | Comparison use |
|---|---|---|---|---|
| chrome | static | cold | 10/10 | VALID |
| mimic | static | cold | 10/10 | VALID |
| chrome | static | warm | 20/20 | VALID |
| mimic | static | warm | 20/20 | VALID |
| mimic | cpu | cold | 10/10 | VALID |
| chrome | cpu | cold | 10/10 | VALID |
| mimic | cpu | warm | 20/20 | VALID |
| chrome | cpu | warm | 20/20 | VALID |
| chrome | dom | cold | 10/10 | VALID |
| mimic | dom | cold | 10/10 | VALID |
| chrome | dom | warm | 20/20 | VALID |
| mimic | dom | warm | 20/20 | VALID |
| mimic | async | cold | 10/10 | VALID |
| chrome | async | cold | 10/10 | VALID |
| mimic | async | warm | 20/20 | VALID |
| chrome | async | warm | 20/20 | VALID |
| chrome | react | cold | 10/10 | VALID |
| mimic | react | cold | 10/10 | VALID |
| chrome | react | warm | 20/20 | VALID |
| mimic | react | warm | 20/20 | VALID |
| mimic | wasm | cold | 10/10 | VALID |
| chrome | wasm | cold | 10/10 | VALID |
| mimic | wasm | warm | 20/20 | VALID |
| chrome | wasm | warm | 20/20 | VALID |

![Total working set (MiB)](total-rss.png)

![Marginal working set (MiB/session)](marginal-rss.png)

![Successful sessions / second](throughput.png)

![Session latency (ms)](latency.png)

![CPU (% of one logical core)](cpu.png)

## Engineering conclusions

1. By median warm navigation→completion, Mimic is faster on: none of the measured workloads. CDP readiness (shared probe, separate cold runs): mimic 230.44 ms; chrome 327.02 ms

2. Chrome is faster by the same metric on: async (109.78 / 52.95 ms Mimic/Chrome); cpu (107.40 / 51.75 ms Mimic/Chrome); dom (131.73 / 54.73 ms Mimic/Chrome); react (109.51 / 52.79 ms Mimic/Chrome); static (59.10 / 28.76 ms Mimic/Chrome); wasm (71.68 / 31.77 ms Mimic/Chrome).

3. Fixed process overhead (CDP ready, including the initial page): mimic 24.35 MiB; chrome 379.91 MiB. Startup latency and the OLS intercept are reported separately above.

4. Estimated marginal RAM/session: chrome/cpu 74.17 MiB; mimic/cpu 36.42 MiB; chrome/react 69.72 MiB; mimic/react 27.90 MiB; chrome/static 59.58 MiB; mimic/static 22.05 MiB.

5. Maximum stable N by workload: mimic/cpu 50; chrome/cpu 50; mimic/react 50; chrome/react 50; mimic/static 50; chrome/static 50.

6. Throughput at the highest common stable level:

cpu, N=50: Mimic 45.19 and Chrome 17.28 successful sessions/s; RSS 1901.15 and 4995.09 MiB; CPU/session 377.62 and 216.62 ms.

react, N=50: Mimic 46.15 and Chrome 15.44 successful sessions/s; RSS 1466.54 and 4766.05 MiB; CPU/session 302.00 and 220.12 ms.

static, N=50: Mimic 18.42 and Chrome 20.07 successful sessions/s; RSS 1181.57 and 4084.75 MiB; CPU/session 197.56 and 150.94 ms.

7. Invalid comparisons after fixes: none at the correctness gate; individual iteration errors remain in raw.json. Before fixes: DOM, async, React, WebAssembly (see pre-fix).

8. Limitations: one busy workstation; headless Chrome; different V8 builds; HTTP cache disabled; limited corpus sizes and semantic checks; a synthetic React fixture, not a Next.js/production application. Polling and sampling add system load. RSS sums working sets, not unique physical RAM; no financial model of session cost is provided. Paging is a system-wide counter, not attribution of hard faults to a specific process. Mimic virtual-clock values are not used for comparisons.

9. Supported scope: “On Windows x64, local controlled workloads passed result validation in Mimic V8 and Chrome 152.0.7977.82; a reproducible harness, raw observations, and separate latency, CPU, and memory metrics are published. cpu, N=50: Mimic 45.19 and Chrome 17.28 successful sessions/s; RSS 1901.15 and 4995.09 MiB; CPU/session 377.62 and 216.62 ms.”

10. The data do NOT support claims that “Mimic is X times faster than Chrome in general,” full browser compatibility, tenant-isolation security, an advantage in pure V8/JIT execution, gains on arbitrary sites, or monetary savings without an operating model.
