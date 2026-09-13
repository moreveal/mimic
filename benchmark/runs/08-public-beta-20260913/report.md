# Mimic V8 and Chrome 152: Windows x64 baseline

This is a measurement of the build identified below, using the frozen benchmark suite.

## Environment

| Parameter | Value |
|---|---|
| Start / end date | 2026-09-13T19:57:38.186817+04:00 / 2026-09-13T20:10:26.711901+04:00 |
| Chrome | {'protocolVersion': '1.3', 'product': 'Chrome/152.0.7977.82', 'revision': '@d04cdb24d67b081f6cf80200ffc5233f44b61109', 'userAgent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/152.0.0.0 Safari/537.36', 'jsVersion': '15.2.124.21'} |
| Chromium | 1669021 / d04cdb24d67b081f6cf80200ffc5233f44b61109 |
| Mimic commit | 2d3b21469d73936eae280098de5534aa43ebc353 |
| Mimic V8 | 15.2.124.1-rusty |
| OS | Windows-11-10.0.26200-SP0 |
| CPU | {"Name":"Intel(R) Core(TM) i7-14700KF","NumberOfCores":20,"NumberOfLogicalProcessors":28} |
| CPU physical / logical | 20 / 28 |
| RAM GiB | 31.83 |
| Power plan | Power Scheme GUID: 381b4222-f694-41f0-9685-ff5bb260df2e  (Balanced) |
| Power overlay | 0 00000000-0000-0000-0000-000000000000 |
| Antivirus | {"displayName":"Windows Defender","productState":397568} |
| Chrome SHA-256 | ea36dd818a90176f1a70616f0363d9be527229389a6c073a0b1688b9e73f67e9 |
| Mimic SHA-256 | d8aa4492ebdf5b0e00a25f970b99515048e32420786c8b6efda190ca52485e39 |

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

In the final series, both systems respond to Target.getTargets after the WebSocket handshake. Each uses 10 fresh processes, alternating system order; warmup is excluded. Date: 2026-09-13T20:10:00.212498+04:00. The main series uses the same shared probe.

| System | n | CDP p50 ms | p95 | Min | Max | SD | Ready RSS MiB |
|---|---|---|---|---|---|---|---|
| mimic | 10 | 219.09 | 231.06 | 213.77 | 234.17 | 7.44 | 28.78 |
| chrome | 10 | 270.36 | 289.97 | 240.72 | 293.99 | 16.91 | 378.52 |

## Cold startup (medians, ms)

| System | Workload | n | Process create | CDP ready | Runtime init | Cold total | Shutdown |
|---|---|---|---|---|---|---|---|
| chrome | static | 10 | 4.67 | 269.35 | 265.01 | 474.22 | 113.34 |
| mimic | static | 10 | 4.54 | 214.25 | 209.53 | 590.94 | 50.45 |
| mimic | cpu | 10 | 3.92 | 213.37 | 209.08 | 607.27 | 49.98 |
| chrome | cpu | 10 | 4.61 | 256.79 | 252.56 | 491.41 | 107.31 |
| chrome | dom | 10 | 6.02 | 266.12 | 260.10 | 449.89 | 101.50 |
| mimic | dom | 10 | 5.71 | 225.28 | 219.17 | 757.22 | 51.02 |
| mimic | async | 10 | 4.65 | 217.55 | 212.06 | 634.94 | 47.76 |
| chrome | async | 10 | 6.10 | 235.07 | 227.67 | 427.96 | 91.54 |
| chrome | react | 10 | 5.88 | 237.23 | 230.82 | 415.48 | 88.16 |
| mimic | react | 10 | 6.33 | 225.28 | 219.62 | 629.07 | 48.51 |
| mimic | wasm | 10 | 4.54 | 214.51 | 209.68 | 574.62 | 49.56 |
| chrome | wasm | 10 | 3.98 | 247.38 | 243.39 | 431.12 | 103.26 |

## Warm session startup / teardown (medians)

| System | Workload | n | Create ms | Teardown ms | RSS after teardown MiB |
|---|---|---|---|---|---|
| chrome | static | 20 | 30.69 | 9.74 | 1175.96 |
| mimic | static | 20 | 2.50 | 5.18 | 121.74 |
| mimic | cpu | 20 | 5.36 | 7.42 | 125.24 |
| chrome | cpu | 20 | 41.12 | 13.88 | 1376.86 |
| chrome | dom | 20 | 30.00 | 12.27 | 1345.12 |
| mimic | dom | 20 | 6.73 | 12.53 | 148.48 |
| mimic | async | 20 | 12.71 | 6.47 | 129.04 |
| chrome | async | 20 | 34.85 | 13.48 | 1227.90 |
| chrome | react | 20 | 33.99 | 13.33 | 1434.89 |
| mimic | react | 20 | 12.37 | 7.29 | 134.98 |
| mimic | wasm | 20 | 2.08 | 5.30 | 122.44 |
| chrome | wasm | 20 | 28.98 | 9.82 | 1272.42 |

## Single-session workload latency (ms)

| System | Workload | Mode | n | Nav p50 | Execution p50 | Completion p50 | p95 | Min | Max | SD | CV |
|---|---|---|---|---|---|---|---|---|---|---|---|
| chrome | static | cold | 10 | 26.94 | 2.98 | 30.49 | 149.01 | 23.70 | 231.78 | 63.58 | 1.21 |
| mimic | static | cold | 10 | 298.87 | 3.37 | 302.29 | 314.44 | 290.92 | 317.61 | 7.74 | 0.03 |
| chrome | static | warm | 20 | 17.77 | 3.67 | 21.32 | 24.70 | 18.00 | 27.57 | 2.02 | 0.09 |
| mimic | static | warm | 20 | 30.43 | 3.16 | 33.68 | 38.98 | 32.23 | 109.48 | 16.96 | 0.45 |
| mimic | cpu | cold | 10 | 291.53 | 34.97 | 326.91 | 333.11 | 323.13 | 333.76 | 3.68 | 0.01 |
| chrome | cpu | cold | 10 | 25.81 | 46.03 | 69.96 | 77.24 | 64.04 | 79.20 | 5.02 | 0.07 |
| mimic | cpu | warm | 20 | 36.12 | 40.84 | 77.37 | 94.28 | 63.43 | 126.39 | 13.29 | 0.17 |
| chrome | cpu | warm | 20 | 23.65 | 27.86 | 52.31 | 57.49 | 43.41 | 61.83 | 5.12 | 0.10 |
| chrome | dom | cold | 10 | 23.22 | 12.59 | 36.23 | 416.77 | 30.29 | 675.05 | 200.90 | 1.89 |
| mimic | dom | cold | 10 | 286.89 | 168.09 | 458.53 | 474.07 | 449.31 | 479.20 | 9.22 | 0.02 |
| chrome | dom | warm | 20 | 20.52 | 32.34 | 53.45 | 59.31 | 27.28 | 67.81 | 7.65 | 0.15 |
| mimic | dom | warm | 20 | 31.58 | 171.21 | 202.30 | 224.78 | 197.73 | 280.44 | 17.98 | 0.09 |
| mimic | async | cold | 10 | 283.59 | 63.01 | 349.02 | 375.38 | 335.04 | 392.67 | 15.79 | 0.04 |
| chrome | async | cold | 10 | 19.63 | 26.71 | 45.08 | 148.44 | 43.89 | 227.23 | 57.27 | 0.89 |
| mimic | async | warm | 20 | 35.12 | 64.79 | 103.96 | 126.09 | 86.46 | 157.79 | 16.34 | 0.15 |
| chrome | async | warm | 20 | 22.30 | 28.96 | 49.53 | 55.25 | 37.89 | 56.05 | 5.28 | 0.11 |
| chrome | react | cold | 10 | 25.07 | 15.23 | 39.73 | 80.67 | 36.73 | 106.15 | 21.05 | 0.45 |
| mimic | react | cold | 10 | 289.94 | 41.03 | 331.14 | 338.24 | 324.77 | 340.13 | 4.24 | 0.01 |
| chrome | react | warm | 20 | 23.21 | 22.48 | 46.47 | 49.64 | 33.20 | 52.09 | 3.74 | 0.08 |
| mimic | react | warm | 20 | 43.15 | 45.32 | 90.42 | 120.94 | 80.30 | 144.83 | 15.01 | 0.16 |
| mimic | wasm | cold | 10 | 287.14 | 4.14 | 291.37 | 299.69 | 279.56 | 302.38 | 7.00 | 0.02 |
| chrome | wasm | cold | 10 | 24.03 | 5.42 | 29.86 | — | 25.98 | 39.45 | 5.08 | 0.16 |
| mimic | wasm | warm | 20 | 30.19 | 3.81 | 33.92 | 38.54 | 32.64 | 100.77 | 14.95 | 0.40 |
| chrome | wasm | warm | 20 | 17.07 | 5.67 | 22.83 | 26.00 | 20.38 | 27.12 | 1.73 | 0.08 |

## Single-session memory / CPU (medians)

| System | Workload | Mode | Before page MiB | After create MiB | Peak RSS MiB | Peak private MiB | CPU/session ms | CPU/workload ms |
|---|---|---|---|---|---|---|---|---|
| chrome | static | cold | 377.47 | 436.80 | 489.44 | 256.66 | 359.38 | 148.44 |
| mimic | static | cold | 28.80 | 29.19 | 140.50 | 172.20 | 445.31 | 429.69 |
| chrome | static | warm | 1116.66 | 1153.97 | 1175.96 | 588.62 | 164.06 | 70.31 |
| mimic | static | warm | 121.41 | 121.41 | 147.96 | 179.06 | 46.88 | 31.25 |
| mimic | cpu | cold | 28.82 | 29.24 | 142.07 | 173.03 | 468.75 | 468.75 |
| chrome | cpu | cold | 375.62 | 435.81 | 529.43 | 287.30 | 515.62 | 242.19 |
| mimic | cpu | warm | 124.96 | 124.96 | 164.90 | 194.17 | 93.75 | 85.94 |
| chrome | cpu | warm | 1297.35 | 1345.61 | 1376.86 | 779.06 | 226.56 | 125.00 |
| chrome | dom | cold | 378.86 | 435.82 | 501.15 | 264.13 | 359.38 | 156.25 |
| mimic | dom | cold | 28.84 | 29.41 | 164.31 | 194.80 | 617.19 | 585.94 |
| chrome | dom | warm | 1261.21 | 1301.66 | 1345.12 | 715.40 | 250.00 | 132.81 |
| mimic | dom | warm | 148.22 | 148.22 | 191.07 | 221.86 | 281.25 | 265.62 |
| mimic | async | cold | 28.85 | 31.12 | 147.76 | 181.15 | 429.69 | 429.69 |
| chrome | async | cold | 373.52 | 428.00 | 508.47 | 270.76 | 335.94 | 156.25 |
| mimic | async | warm | 128.93 | 128.93 | 163.09 | 195.08 | 109.38 | 109.38 |
| chrome | async | warm | 1162.17 | 1210.62 | 1227.90 | 627.53 | 250.00 | 93.75 |
| chrome | react | cold | 377.74 | 423.86 | 506.79 | 273.35 | 312.50 | 171.88 |
| mimic | react | cold | 28.88 | 29.19 | 143.61 | 173.69 | 453.12 | 429.69 |
| chrome | react | warm | 1353.12 | 1402.27 | 1434.89 | 822.61 | 234.38 | 117.19 |
| mimic | react | warm | 134.59 | 134.59 | 167.37 | 198.62 | 140.62 | 125.00 |
| mimic | wasm | cold | 28.80 | 29.45 | 141.06 | 171.56 | 390.62 | 375.00 |
| chrome | wasm | cold | 369.07 | 427.50 | 487.26 | 257.21 | 304.69 | 109.38 |
| mimic | wasm | warm | 122.10 | 122.10 | 149.51 | 178.54 | 31.25 | 31.25 |
| chrome | wasm | warm | 1205.30 | 1247.84 | 1272.42 | 626.18 | 179.69 | 70.31 |

## Concurrency / density

Each level uses a separate process, one excluded warmup, and max(5, ceil(20/N)) measured waves. Pages are created concurrently and all begin navigation after a barrier. Completed pages are held until the wave ends to measure simultaneous RSS. Throughput = successful sessions / time from create to final teardown, including the barrier and measurements but excluding HTTP-server setup/cleanup. Latency = create→completion, including barrier wait. This is batch throughput, not an optimized continuous request stream. Holding pages adds to throughput time but not latency. CPU per session = total wave CPU / successes; overlapping per-page CPU intervals are not summed.

Stopping limits: any error, <15% or <2 GiB available RAM, >1024 pages input/s for 3 s, or a 180 s timeout. These protect the workstation; the highest passing level is a lower bound on capacity supported here, not proof of an absolute maximum.

Mimic serializes CDP commands with a shared mutex. The measurement includes this behavior; its cost was not profiled. In stopped rows, RSS may have been sampled before all pages completed, and 0 waves means stopping during the excluded warmup. Such rows are diagnostic and excluded from stable-level fits/charts. Between waves, an additional 250 ms recovery, server cleanup, and checkpoint writing are outside batch throughput.

| System | Workload | N | Waves | Success % | RSS MiB | RSS/N MiB | Peak MiB | CPU/session ms | Sessions/s | p50 ms | p95 ms | p99 ms | Stop |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| chrome | static | 1 | 20 | 100.00 | 1188.29 | 1188.29 | 1195.21 | 161.72 | 15.29 | 44.50 | 51.02 | — |  |
| mimic | static | 1 | 20 | 100.00 | 150.67 | 150.67 | 187.02 | 82.03 | 14.85 | 44.04 | 64.07 | — |  |
| chrome | static | 5 | 5 | 100.00 | 1348.03 | 269.61 | 1379.60 | 143.75 | 22.52 | 159.15 | 165.02 | — |  |
| mimic | static | 5 | 5 | 100.00 | 338.70 | 67.74 | 371.72 | 90.62 | 36.68 | 61.12 | 165.43 | — |  |
| chrome | static | 10 | 5 | 100.00 | 1683.53 | 168.35 | 1688.32 | 112.81 | 27.36 | 276.42 | 283.80 | — |  |
| mimic | static | 10 | 5 | 100.00 | 560.86 | 56.09 | 596.04 | 120.62 | 58.33 | 86.57 | 239.19 | — |  |
| chrome | static | 25 | 5 | 100.00 | 2542.67 | 101.71 | 2560.18 | 110.62 | 30.21 | 665.56 | 749.13 | 750.96 |  |
| mimic | static | 25 | 5 | 100.00 | 1092.03 | 43.68 | 1250.21 | 131.38 | 70.48 | 168.24 | 491.90 | 499.28 |  |
| chrome | static | 50 | 5 | 100.00 | 4051.56 | 81.03 | 4063.77 | 113.19 | 28.59 | 1450.13 | 1819.05 | 1825.52 |  |
| mimic | static | 50 | 5 | 100.00 | 2210.02 | 44.20 | 2403.24 | 129.12 | 66.95 | 348.26 | 921.98 | 944.52 |  |
| chrome | static | 100 | 5 | 100.00 | 7039.66 | 70.40 | 7043.91 | 120.53 | 27.78 | 3100.21 | 3335.81 | 3348.21 |  |
| mimic | static | 100 | 5 | 100.00 | 3988.31 | 39.88 | 4272.96 | 135.97 | 76.64 | 551.37 | 1785.97 | 2022.13 |  |
| chrome | cpu | 1 | 20 | 100.00 | 1405.11 | 1405.11 | 1418.78 | 218.75 | 8.71 | 71.89 | 84.32 | — |  |
| mimic | cpu | 1 | 20 | 100.00 | 165.96 | 165.96 | 200.59 | 131.25 | 8.97 | 74.79 | 97.60 | — |  |
| chrome | cpu | 5 | 5 | 100.00 | 1627.67 | 325.53 | 1637.71 | 173.12 | 21.07 | 192.58 | 195.80 | — |  |
| mimic | cpu | 5 | 5 | 100.00 | 380.04 | 76.01 | 397.09 | 150.00 | 29.61 | 97.98 | 215.08 | — |  |
| chrome | cpu | 10 | 5 | 100.00 | 1995.12 | 199.51 | 2004.06 | 205.62 | 19.32 | 423.29 | 488.14 | — |  |
| mimic | cpu | 10 | 5 | 100.00 | 680.95 | 68.09 | 710.40 | 230.94 | 36.36 | 154.59 | 354.52 | — |  |
| chrome | cpu | 25 | 5 | 100.00 | 3069.84 | 122.79 | 3083.02 | 178.25 | 22.77 | 909.89 | 1122.31 | 1133.86 |  |
| mimic | cpu | 25 | 5 | 100.00 | 1513.77 | 60.55 | 1535.49 | 254.62 | 45.49 | 307.48 | 651.22 | 665.87 |  |
| chrome | cpu | 50 | 5 | 100.00 | 4943.03 | 98.86 | 4963.36 | 182.44 | 22.08 | 1895.41 | 2344.28 | 2358.95 |  |
| mimic | cpu | 50 | 5 | 100.00 | 2734.01 | 54.68 | 3001.87 | 281.25 | 48.20 | 513.14 | 1470.13 | 1498.60 |  |
| chrome | cpu | 100 | 5 | 100.00 | 8656.86 | 86.57 | 8671.02 | 198.22 | 19.60 | 4474.83 | 4606.46 | 4631.87 |  |
| mimic | cpu | 100 | 1 | 99.00 | 4032.66 | 40.33 | 4033.02 | 666.35 | 24.16 | 3061.20 | 3558.43 | 3603.99 | non-zero failure rate |
| chrome | react | 1 | 20 | 100.00 | 1437.69 | 1437.69 | 1446.02 | 256.25 | 8.57 | 74.56 | 86.72 | — |  |
| mimic | react | 1 | 20 | 100.00 | 169.15 | 169.15 | 203.62 | 154.69 | 7.50 | 95.88 | 141.95 | — |  |
| chrome | react | 5 | 5 | 100.00 | 1564.70 | 312.94 | 1576.46 | 199.38 | 10.66 | 227.86 | 1131.83 | — |  |
| mimic | react | 5 | 5 | 100.00 | 382.02 | 76.40 | 392.49 | 205.62 | 24.97 | 116.46 | 269.13 | — |  |
| chrome | react | 10 | 5 | 100.00 | 1910.35 | 191.03 | 1917.84 | 201.88 | 8.33 | 1114.93 | 1149.14 | — |  |
| mimic | react | 10 | 5 | 100.00 | 650.29 | 65.03 | 669.29 | 237.19 | 37.35 | 152.47 | 387.26 | — |  |
| chrome | react | 25 | 5 | 100.00 | 2986.12 | 119.44 | 3058.18 | 184.00 | 15.43 | 918.66 | 1886.61 | 1915.15 |  |
| mimic | react | 25 | 5 | 100.00 | 1433.85 | 57.35 | 1556.33 | 284.62 | 47.53 | 308.97 | 718.20 | 732.29 |  |
| chrome | react | 50 | 5 | 100.00 | 4758.96 | 95.18 | 4847.53 | 197.06 | 18.68 | 2399.78 | 2529.84 | 2538.80 |  |
| mimic | react | 50 | 5 | 100.00 | 2692.14 | 53.84 | 2980.11 | 297.38 | 50.52 | 518.83 | 1400.38 | 1425.07 |  |
| chrome | react | 100 | 5 | 100.00 | 8503.54 | 85.04 | 8537.23 | 203.69 | 20.07 | 4301.83 | 5071.69 | 5082.95 |  |
| mimic | react | 100 | 5 | 100.00 | 5175.78 | 51.76 | 5458.82 | 335.62 | 44.47 | 1095.62 | 3000.32 | 3095.34 |  |

## Marginal RAM/session

The finite difference between adjacent tested N values is measured and divided by ΔN; this is not a direct measurement of every N→N+1 step. The linear model is a descriptive OLS fit to medians of successful levels only. The intercept is an extrapolation; actual startup overhead includes the initial blank page. Total RSS includes processes and caches retained from earlier waves, and the number of waves depends on N. The fit therefore combines active-page costs with process history. RSS growth relative to the start of the same wave is also shown; it can include background activity and GC.

| System | Workload | N | (Active RSS − before wave RSS)/N MiB |
|---|---|---|---|
| chrome | static | 1 | 58.46 |
| mimic | static | 1 | 26.10 |
| chrome | static | 5 | 57.62 |
| mimic | static | 5 | 30.29 |
| chrome | static | 10 | 54.97 |
| mimic | static | 10 | 30.75 |
| chrome | static | 25 | 56.50 |
| mimic | static | 25 | 30.36 |
| chrome | static | 50 | 57.30 |
| mimic | static | 50 | 29.45 |
| chrome | static | 100 | 57.97 |
| mimic | static | 100 | 28.95 |
| chrome | cpu | 1 | 77.82 |
| mimic | cpu | 1 | 36.87 |
| chrome | cpu | 5 | 68.42 |
| mimic | cpu | 5 | 40.43 |
| chrome | cpu | 10 | 70.59 |
| mimic | cpu | 10 | 41.79 |
| chrome | cpu | 25 | 71.81 |
| mimic | cpu | 25 | 39.42 |
| chrome | cpu | 50 | 72.36 |
| mimic | cpu | 50 | 40.07 |
| chrome | cpu | 100 | 72.98 |
| mimic | cpu | 100 | 37.55 |
| chrome | react | 1 | 80.23 |
| mimic | react | 1 | 33.39 |
| chrome | react | 5 | 64.30 |
| mimic | react | 5 | 37.50 |
| chrome | react | 10 | 66.70 |
| mimic | react | 10 | 32.84 |
| chrome | react | 25 | 68.40 |
| mimic | react | 25 | 36.50 |
| chrome | react | 50 | 68.97 |
| mimic | react | 50 | 35.29 |
| chrome | react | 100 | 71.14 |
| mimic | react | 100 | 34.49 |

| System | Workload | N low→high | ΔRSS MiB | ΔRSS/ΔN MiB |
|---|---|---|---|---|
| chrome | cpu | 1→5 | 222.56 | 55.64 |
| chrome | cpu | 5→10 | 367.46 | 73.49 |
| chrome | cpu | 10→25 | 1074.72 | 71.65 |
| chrome | cpu | 25→50 | 1873.19 | 74.93 |
| chrome | cpu | 50→100 | 3713.83 | 74.28 |
| mimic | cpu | 1→5 | 214.08 | 53.52 |
| mimic | cpu | 5→10 | 300.90 | 60.18 |
| mimic | cpu | 10→25 | 832.82 | 55.52 |
| mimic | cpu | 25→50 | 1220.25 | 48.81 |
| chrome | react | 1→5 | 127.00 | 31.75 |
| chrome | react | 5→10 | 345.65 | 69.13 |
| chrome | react | 10→25 | 1075.78 | 71.72 |
| chrome | react | 25→50 | 1772.84 | 70.91 |
| chrome | react | 50→100 | 3744.58 | 74.89 |
| mimic | react | 1→5 | 212.87 | 53.22 |
| mimic | react | 5→10 | 268.28 | 53.66 |
| mimic | react | 10→25 | 783.56 | 52.24 |
| mimic | react | 25→50 | 1258.29 | 50.33 |
| mimic | react | 50→100 | 2483.64 | 49.67 |
| chrome | static | 1→5 | 159.74 | 39.93 |
| chrome | static | 5→10 | 335.50 | 67.10 |
| chrome | static | 10→25 | 859.14 | 57.28 |
| chrome | static | 25→50 | 1508.89 | 60.36 |
| chrome | static | 50→100 | 2988.10 | 59.76 |
| mimic | static | 1→5 | 188.03 | 47.01 |
| mimic | static | 5→10 | 222.15 | 44.43 |
| mimic | static | 10→25 | 531.18 | 35.41 |
| mimic | static | 25→50 | 1117.98 | 44.72 |
| mimic | static | 50→100 | 1778.30 | 35.57 |

| System | Workload | Fixed fit MiB | Marginal fit MiB | R² | Max stable N |
|---|---|---|---|---|---|
| chrome | cpu | 1270.69 | 73.68 | 1.00 | 100 |
| mimic | cpu | 138.84 | 52.53 | 1.00 | 50 |
| chrome | react | 1228.75 | 72.19 | 1.00 | 100 |
| mimic | react | 142.36 | 50.52 | 1.00 | 100 |
| chrome | static | 1082.13 | 59.48 | 1.00 | 100 |
| mimic | static | 153.42 | 38.85 | 1.00 | 100 |

## Teardown / recovery

| System | Workload | N | Ready RSS MiB | RSS after waves MiB | Whole-series CPU s | CPU % |
|---|---|---|---|---|---|---|
| chrome | static | 1 | 372.07 | 1131.31 | 3.23 | 247.21 |
| mimic | static | 1 | 28.77 | 124.53 | 1.64 | 121.81 |
| chrome | static | 5 | 367.24 | 1068.62 | 3.59 | 323.71 |
| mimic | static | 5 | 28.78 | 206.63 | 2.27 | 332.41 |
| chrome | static | 10 | 372.94 | 1136.02 | 5.64 | 308.61 |
| mimic | static | 10 | 28.78 | 301.63 | 6.03 | 703.55 |
| chrome | static | 25 | 369.76 | 1136.20 | 13.83 | 334.22 |
| mimic | static | 25 | 29.34 | 491.23 | 16.42 | 925.89 |
| chrome | static | 50 | 372.14 | 1193.96 | 28.30 | 323.61 |
| mimic | static | 50 | 28.73 | 910.56 | 32.28 | 864.46 |
| chrome | static | 100 | 388.77 | 1248.70 | 60.27 | 334.88 |
| mimic | static | 100 | 28.84 | 1519.87 | 67.98 | 1042.06 |
| chrome | cpu | 1 | 378.47 | 1328.94 | 4.38 | 190.45 |
| mimic | cpu | 1 | 28.82 | 126.49 | 2.62 | 117.68 |
| chrome | cpu | 5 | 374.59 | 1295.22 | 4.33 | 364.77 |
| mimic | cpu | 5 | 28.80 | 197.66 | 3.75 | 444.19 |
| chrome | cpu | 10 | 378.26 | 1297.75 | 10.28 | 397.24 |
| mimic | cpu | 10 | 28.88 | 328.06 | 11.55 | 839.69 |
| chrome | cpu | 25 | 369.53 | 1283.30 | 22.28 | 405.90 |
| mimic | cpu | 25 | 29.32 | 539.42 | 31.83 | 1158.34 |
| chrome | cpu | 50 | 376.11 | 1330.05 | 45.61 | 402.89 |
| mimic | cpu | 50 | 28.89 | 804.45 | 70.31 | 1355.61 |
| chrome | cpu | 100 | 367.40 | 1375.74 | 99.11 | 388.41 |
| mimic | cpu | 100 | 28.83 | 273.40 | 65.97 | 1609.66 |
| chrome | react | 1 | 384.25 | 1358.97 | 5.12 | 219.49 |
| mimic | react | 1 | 28.92 | 135.65 | 3.09 | 116.05 |
| chrome | react | 5 | 378.00 | 1244.96 | 4.98 | 212.44 |
| mimic | react | 5 | 29.41 | 219.10 | 5.14 | 513.53 |
| chrome | react | 10 | 383.26 | 1249.64 | 10.09 | 168.14 |
| mimic | react | 10 | 28.86 | 327.84 | 11.86 | 885.88 |
| chrome | react | 25 | 371.80 | 1278.65 | 23.00 | 283.96 |
| mimic | react | 25 | 28.95 | 642.25 | 35.58 | 1352.76 |
| chrome | react | 50 | 369.09 | 1313.19 | 49.27 | 368.10 |
| mimic | react | 50 | 28.71 | 1103.45 | 74.34 | 1502.26 |
| chrome | react | 100 | 373.65 | 1396.86 | 101.84 | 408.84 |
| mimic | react | 100 | 28.86 | 2009.55 | 167.81 | 1492.67 |

## Local server (measured independently)

HTTP handler time runs from the handler receiving a request to completion of response writing; it excludes TCP/server scheduler queues and network roundtrip. Before the main workload, the server is created outside the measured interval. Requests and bytes for each resource are recorded in raw.json.

| System | Workload | Mode | Requests | Server p50 ms | Server p95 ms | Server max ms |
|---|---|---|---|---|---|---|
| chrome | static | cold | 20 | 0.29 | 0.51 | 0.61 |
| mimic | static | cold | 20 | 0.16 | 0.43 | 0.46 |
| chrome | static | warm | 40 | 0.19 | 0.36 | 0.38 |
| mimic | static | warm | 40 | 0.15 | 0.34 | 0.50 |
| mimic | cpu | cold | 20 | 0.13 | 0.42 | 0.45 |
| chrome | cpu | cold | 20 | 0.22 | 0.41 | 0.49 |
| mimic | cpu | warm | 40 | 0.22 | 0.55 | 1.24 |
| chrome | cpu | warm | 40 | 0.25 | 0.48 | 0.52 |
| chrome | dom | cold | 20 | 0.29 | 0.46 | 0.54 |
| mimic | dom | cold | 20 | 0.38 | 0.92 | 0.94 |
| chrome | dom | warm | 40 | 0.20 | 0.38 | 0.43 |
| mimic | dom | warm | 40 | 0.14 | 0.33 | 0.39 |
| mimic | async | cold | 50 | 0.14 | 0.52 | 1.04 |
| chrome | async | cold | 50 | 0.16 | 0.40 | 0.49 |
| mimic | async | warm | 100 | 0.15 | 0.86 | 1.73 |
| chrome | async | warm | 100 | 0.16 | 0.40 | 1.21 |
| chrome | react | cold | 50 | 0.45 | 1.02 | 1.10 |
| mimic | react | cold | 50 | 0.32 | 1.08 | 1.28 |
| chrome | react | warm | 100 | 0.33 | 0.79 | 0.86 |
| mimic | react | warm | 100 | 0.31 | 1.05 | 1.76 |
| mimic | wasm | cold | 20 | 0.27 | 0.81 | 1.14 |
| chrome | wasm | cold | 18 | 0.23 | 0.35 | 0.40 |
| mimic | wasm | warm | 40 | 0.14 | 0.25 | 0.25 |
| chrome | wasm | warm | 40 | 0.20 | 0.35 | 0.47 |

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
| chrome | wasm | cold | 9/10 | INVALID — error or semantic mismatch; no speed claim |
| mimic | wasm | warm | 20/20 | VALID |
| chrome | wasm | warm | 20/20 | VALID |

![Total working set (MiB)](total-rss.png)

![Marginal working set (MiB/session)](marginal-rss.png)

![Successful sessions / second](throughput.png)

![Session latency (ms)](latency.png)

![CPU (% of one logical core)](cpu.png)

## Engineering conclusions

1. By median warm navigation→completion, Mimic is faster on: none of the measured workloads. CDP readiness (shared probe, separate cold runs): mimic 219.09 ms; chrome 270.36 ms

2. Chrome is faster by the same metric on: async (103.96 / 49.53 ms Mimic/Chrome); cpu (77.37 / 52.31 ms Mimic/Chrome); dom (202.30 / 53.45 ms Mimic/Chrome); react (90.42 / 46.47 ms Mimic/Chrome); static (33.68 / 21.32 ms Mimic/Chrome); wasm (33.92 / 22.83 ms Mimic/Chrome).

3. Fixed process overhead (CDP ready, including the initial page): mimic 28.86 MiB; chrome 378.09 MiB. Startup latency and the OLS intercept are reported separately above.

4. Estimated marginal RAM/session: chrome/cpu 73.68 MiB; mimic/cpu 52.53 MiB; chrome/react 72.19 MiB; mimic/react 50.52 MiB; chrome/static 59.48 MiB; mimic/static 38.85 MiB.

5. Maximum stable N by workload: mimic/cpu 50; chrome/cpu 100; mimic/react 100; chrome/react 100; mimic/static 100; chrome/static 100.

6. Throughput at the highest common stable level:

cpu, N=50: Mimic 48.20 and Chrome 22.08 successful sessions/s; RSS 2734.01 and 4943.03 MiB; CPU/session 281.25 and 182.44 ms.

react, N=100: Mimic 44.47 and Chrome 20.07 successful sessions/s; RSS 5175.78 and 8503.54 MiB; CPU/session 335.62 and 203.69 ms.

static, N=100: Mimic 76.64 and Chrome 27.78 successful sessions/s; RSS 3988.31 and 7039.66 MiB; CPU/session 135.97 and 120.53 ms.

7. Invalid comparisons in this run: none at the correctness gate; individual iteration errors remain in raw.json.

8. Limitations: one busy workstation; headless Chrome; different V8 builds; HTTP cache disabled; limited corpus sizes and semantic checks; a synthetic React fixture, not a Next.js/production application. Polling and sampling add system load. RSS sums working sets, not unique physical RAM; no financial model of session cost is provided. Paging is a system-wide counter, not attribution of hard faults to a specific process. Mimic virtual-clock values are not used for comparisons.

9. Supported scope: “On Windows x64, local controlled workloads passed result validation in Mimic V8 and Chrome 152.0.7977.82; a reproducible harness, raw observations, and separate latency, CPU, and memory metrics are published. cpu, N=50: Mimic 48.20 and Chrome 22.08 successful sessions/s; RSS 2734.01 and 4943.03 MiB; CPU/session 281.25 and 182.44 ms.”

10. The data do NOT support claims that “Mimic is X times faster than Chrome in general,” full browser compatibility, tenant-isolation security, an advantage in pure V8/JIT execution, gains on arbitrary sites, or monetary savings without an operating model.
