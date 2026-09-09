# Mimic V8 and Chrome 152: Windows x64 baseline

This is a measurement after semantic fixes. No runtime performance optimizations were performed. The original version's correctness check is saved separately in `pre-fix/raw.json`.

## Environment

| Parameter | Value |
|---|---|
| Start / end date | 2026-09-08T19:07:33.508542+04:00 / 2026-09-08T19:28:28.702770+04:00 |
| Chrome | {'protocolVersion': '1.3', 'product': 'Chrome/152.0.7977.82', 'revision': '@d04cdb24d67b081f6cf80200ffc5233f44b61109', 'userAgent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/152.0.0.0 Safari/537.36', 'jsVersion': '15.2.124.21'} |
| Chromium | 1669021 / d04cdb24d67b081f6cf80200ffc5233f44b61109 |
| Mimic commit | f1a526fc134be4cc009638ee2d1206363132f68c |
| Mimic V8 | 15.2.124.1-rusty |
| OS | Windows-11-10.0.26200-SP0 |
| CPU | {"Name":"Intel(R) Core(TM) i7-14700KF","NumberOfCores":20,"NumberOfLogicalProcessors":28} |
| CPU physical / logical | 20 / 28 |
| RAM GiB | 31.83 |
| Power plan | Power Scheme GUID: 381b4222-f694-41f0-9685-ff5bb260df2e  (Balanced) |
| Power overlay | 0 00000000-0000-0000-0000-000000000000 |
| Antivirus | {"displayName":"Windows Defender","productState":397568} |
| Chrome SHA-256 | ea36dd818a90176f1a70616f0363d9be527229389a6c073a0b1688b9e73f67e9 |
| Mimic SHA-256 | 7f55ba9d2729dcf7dc439d942a8fb488b60a15c6866441485038826feb7bedb3 |

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

In the final series, both systems respond to Target.getTargets after the WebSocket handshake. Each uses 10 fresh processes, alternating system order; warmup is excluded. Date: 2026-09-08T19:28:04.906164+04:00. The main series uses the same shared probe.

| System | n | CDP p50 ms | p95 | Min | Max | SD | Ready RSS MiB |
|---|---|---|---|---|---|---|---|
| mimic | 10 | 215.02 | 223.89 | 213.09 | 224.24 | 4.49 | 103.67 |
| chrome | 10 | 260.60 | 285.07 | 246.47 | 286.69 | 15.03 | 384.44 |

## Cold startup (medians, ms)

| System | Workload | n | Process create | CDP ready | Runtime init | Cold total | Shutdown |
|---|---|---|---|---|---|---|---|
| chrome | static | 10 | 6.21 | 288.77 | 280.78 | 455.76 | 96.23 |
| mimic | static | 10 | 5.45 | 213.87 | 208.40 | 432.03 | 62.12 |
| mimic | cpu | 10 | 5.49 | 223.34 | 218.05 | 475.15 | 60.41 |
| chrome | cpu | 10 | 6.18 | 270.43 | 263.15 | 491.44 | 106.46 |
| chrome | dom | 10 | 6.88 | 292.44 | 282.49 | 503.64 | 114.71 |
| mimic | dom | 10 | 8.58 | 423.41 | 415.74 | 2065.79 | 79.05 |
| mimic | async | 10 | 6.34 | 429.14 | 421.63 | 761.52 | 81.13 |
| chrome | async | 10 | 10.05 | 432.11 | 421.49 | 740.83 | 148.14 |
| chrome | react | 10 | 7.57 | 314.57 | 306.04 | 563.47 | 118.74 |
| mimic | react | 10 | 6.55 | 216.74 | 209.80 | 577.77 | 66.92 |
| mimic | wasm | 10 | 5.79 | 224.15 | 218.37 | 452.04 | 63.95 |
| chrome | wasm | 10 | 7.10 | 317.81 | 308.86 | 515.64 | 100.14 |

## Warm session startup / teardown (medians)

| System | Workload | n | Create ms | Teardown ms | RSS after teardown MiB |
|---|---|---|---|---|---|
| chrome | static | 20 | 26.84 | 9.44 | 1187.23 |
| mimic | static | 20 | 70.45 | 1.83 | 170.54 |
| mimic | cpu | 20 | 66.46 | 2.25 | 173.77 |
| chrome | cpu | 20 | 30.94 | 10.64 | 1404.60 |
| chrome | dom | 20 | 45.34 | 19.24 | 1426.64 |
| mimic | dom | 20 | 91.56 | 55.19 | 314.25 |
| mimic | async | 20 | 79.98 | 2.47 | 177.12 |
| chrome | async | 20 | 28.83 | 11.13 | 1232.30 |
| chrome | react | 20 | 32.77 | 10.46 | 1434.59 |
| mimic | react | 20 | 70.88 | 4.97 | 192.61 |
| mimic | wasm | 20 | 78.40 | 2.54 | 173.22 |
| chrome | wasm | 20 | 34.03 | 12.20 | 1276.77 |

## Single-session workload latency (ms)

| System | Workload | Mode | n | Nav p50 | Execution p50 | Completion p50 | p95 | Min | Max | SD | CV |
|---|---|---|---|---|---|---|---|---|---|---|---|
| chrome | static | cold | 10 | 21.17 | 2.75 | 23.87 | 66.99 | 21.89 | 99.48 | 23.83 | 0.75 |
| mimic | static | cold | 10 | 79.02 | 1.89 | 80.85 | 83.22 | 76.33 | 83.93 | 2.10 | 0.03 |
| chrome | static | warm | 20 | 16.11 | 3.46 | 19.34 | 23.92 | 17.36 | 24.74 | 2.40 | 0.12 |
| mimic | static | warm | 20 | 71.11 | 1.94 | 73.00 | 77.93 | 68.68 | 81.47 | 3.38 | 0.05 |
| mimic | cpu | cold | 10 | 78.70 | 41.37 | 119.91 | 129.24 | 117.22 | 131.28 | 4.39 | 0.04 |
| chrome | cpu | cold | 10 | 24.92 | 29.16 | 54.94 | 428.87 | 46.88 | 613.28 | 178.07 | 1.43 |
| mimic | cpu | warm | 20 | 73.17 | 41.26 | 114.88 | 122.36 | 107.52 | 125.29 | 3.78 | 0.03 |
| chrome | cpu | warm | 20 | 18.85 | 30.05 | 48.70 | 58.45 | 45.08 | 69.48 | 5.34 | 0.11 |
| chrome | dom | cold | 10 | 24.90 | 12.81 | 37.37 | 69.85 | 31.23 | 88.54 | 16.61 | 0.38 |
| mimic | dom | cold | 10 | 93.90 | 1317.96 | 1411.91 | 1494.77 | 1271.60 | 1507.94 | 68.90 | 0.05 |
| chrome | dom | warm | 20 | 29.61 | 41.15 | 73.01 | 123.81 | 63.29 | 832.83 | 170.11 | 1.54 |
| mimic | dom | warm | 20 | 102.93 | 1703.89 | 1803.17 | 2294.33 | 1335.33 | 2469.65 | 267.01 | 0.15 |
| mimic | async | cold | 10 | 103.59 | 59.46 | 162.18 | 187.80 | 138.59 | 192.66 | 19.15 | 0.12 |
| chrome | async | cold | 10 | 37.09 | 35.05 | 73.36 | 110.69 | 53.70 | 110.80 | 18.55 | 0.23 |
| mimic | async | warm | 20 | 86.27 | 53.55 | 143.49 | 180.08 | 110.52 | 180.35 | 23.59 | 0.17 |
| chrome | async | warm | 20 | 19.72 | 28.66 | 48.98 | 55.82 | 43.08 | 56.39 | 4.14 | 0.08 |
| chrome | react | cold | 10 | 32.14 | 19.55 | 51.97 | 166.36 | 38.77 | 214.19 | 53.48 | 0.73 |
| mimic | react | cold | 10 | 96.14 | 101.59 | 197.88 | 215.48 | 178.97 | 221.80 | 12.95 | 0.07 |
| chrome | react | warm | 20 | 23.68 | 23.77 | 46.02 | 55.60 | 40.77 | 61.29 | 5.48 | 0.12 |
| mimic | react | warm | 20 | 82.87 | 96.41 | 178.77 | 209.35 | 162.54 | 211.41 | 13.46 | 0.07 |
| mimic | wasm | cold | 10 | 80.05 | 9.71 | 89.78 | 94.91 | 88.33 | 96.09 | 2.44 | 0.03 |
| chrome | wasm | cold | 10 | 24.57 | 6.84 | 31.66 | 37.38 | 30.02 | 37.96 | 2.75 | 0.08 |
| mimic | wasm | warm | 20 | 85.98 | 10.11 | 96.58 | 120.03 | 78.46 | 124.01 | 11.97 | 0.12 |
| chrome | wasm | warm | 20 | 19.69 | 5.43 | 24.98 | 29.25 | 21.32 | 29.44 | 2.46 | 0.10 |

## Single-session memory / CPU (medians)

| System | Workload | Mode | Before page MiB | After create MiB | Peak RSS MiB | Peak private MiB | CPU/session ms | CPU/workload ms |
|---|---|---|---|---|---|---|---|---|
| chrome | static | cold | 385.49 | 432.96 | 478.07 | 250.36 | 312.50 | 93.75 |
| mimic | static | cold | 103.43 | 139.06 | 176.42 | 217.73 | 226.56 | 125.00 |
| chrome | static | warm | 1126.46 | 1158.76 | 1187.23 | 593.13 | 156.25 | 46.88 |
| mimic | static | warm | 167.66 | 187.06 | 205.15 | 245.38 | 203.12 | 109.38 |
| mimic | cpu | cold | 103.73 | 139.30 | 176.35 | 215.81 | 250.00 | 164.06 |
| chrome | cpu | cold | 374.85 | 429.99 | 504.97 | 270.08 | 367.19 | 195.31 |
| mimic | cpu | warm | 173.35 | 185.68 | 205.55 | 242.23 | 257.81 | 140.62 |
| chrome | cpu | warm | 1326.05 | 1364.15 | 1404.60 | 781.86 | 242.19 | 132.81 |
| chrome | dom | cold | 375.72 | 433.31 | 483.37 | 256.82 | 351.56 | 164.06 |
| mimic | dom | cold | 103.86 | 139.84 | 264.92 | 305.00 | 2468.75 | 2320.31 |
| chrome | dom | warm | 1341.86 | 1388.25 | 1426.64 | 781.14 | 343.75 | 195.31 |
| mimic | dom | warm | 306.17 | 333.49 | 363.96 | 402.72 | 3320.31 | 3148.44 |
| mimic | async | cold | 104.11 | 139.40 | 175.09 | 216.52 | 312.50 | 187.50 |
| chrome | async | cold | 383.09 | 438.52 | 523.72 | 272.71 | 585.94 | 289.06 |
| mimic | async | warm | 176.34 | 194.32 | 213.74 | 253.45 | 250.00 | 125.00 |
| chrome | async | warm | 1168.56 | 1206.86 | 1232.44 | 623.14 | 218.75 | 101.56 |
| chrome | react | cold | 380.17 | 433.27 | 512.39 | 274.30 | 476.56 | 242.19 |
| mimic | react | cold | 103.90 | 138.66 | 173.52 | 212.84 | 460.94 | 296.88 |
| chrome | react | warm | 1353.70 | 1386.09 | 1434.59 | 808.06 | 257.81 | 140.62 |
| mimic | react | warm | 189.47 | 204.42 | 227.59 | 265.76 | 328.12 | 242.19 |
| mimic | wasm | cold | 104.09 | 139.84 | 172.58 | 212.51 | 203.12 | 117.19 |
| chrome | wasm | cold | 384.90 | 438.99 | 483.80 | 249.74 | 460.94 | 140.62 |
| mimic | wasm | warm | 170.83 | 182.13 | 208.52 | 246.49 | 210.94 | 125.00 |
| chrome | wasm | warm | 1209.78 | 1249.50 | 1276.77 | 637.09 | 218.75 | 78.12 |

## Concurrency / density

Each level uses a separate process, one excluded warmup, and max(5, ceil(20/N)) measured waves. Pages are created concurrently and all begin navigation after a barrier. Completed pages are held until the wave ends to measure simultaneous RSS. Throughput = successful sessions / time from create to final teardown, including the barrier and measurements but excluding HTTP-server setup/cleanup. Latency = create→completion, including barrier wait. This is batch throughput, not an optimized continuous request stream. Holding pages adds to throughput time but not latency. CPU per session = total wave CPU / successes; overlapping per-page CPU intervals are not summed.

Stopping limits: any error, <15% or <2 GiB available RAM, >1024 pages input/s for 3 s, or a 180 s timeout. These protect the workstation; the highest passing level is a lower bound on capacity supported here, not proof of an absolute maximum.

Mimic serializes CDP commands with a shared mutex. The measurement includes this behavior; its cost was not profiled. In stopped rows, RSS may have been sampled before all pages completed, and 0 waves means stopping during the excluded warmup. Such rows are diagnostic and excluded from stable-level fits/charts. Between waves, an additional 250 ms recovery, server cleanup, and checkpoint writing are outside batch throughput.

| System | Workload | N | Waves | Success % | RSS MiB | RSS/N MiB | Peak MiB | CPU/session ms | Sessions/s | p50 ms | p95 ms | p99 ms | Stop |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| chrome | static | 1 | 20 | 100.00 | 1216.91 | 1216.91 | 1228.51 | 255.47 | 9.76 | 53.99 | 139.39 | — |  |
| mimic | static | 1 | 20 | 100.00 | 185.60 | 185.60 | 230.73 | 573.44 | 0.67 | 746.34 | 1457.95 | — |  |
| chrome | static | 5 | 5 | 100.00 | 1388.89 | 277.78 | 1404.14 | 384.38 | 4.27 | 734.52 | 1068.49 | — |  |
| mimic | static | 5 | 5 | 100.00 | 520.69 | 104.14 | 618.50 | 398.75 | 7.21 | 408.19 | 1049.21 | — |  |
| chrome | static | 10 | 5 | 100.00 | 1676.50 | 167.65 | 1683.75 | 125.62 | 26.24 | 293.74 | 323.93 | — |  |
| mimic | static | 10 | 5 | 100.00 | 784.49 | 78.45 | 1113.74 | 446.88 | 16.06 | 563.10 | 620.00 | — |  |
| chrome | static | 25 | 5 | 100.00 | 2563.08 | 102.52 | 2564.48 | 136.62 | 22.12 | 978.87 | 997.57 | 1000.70 |  |
| mimic | static | 25 | 5 | 100.00 | 1812.34 | 72.49 | 2550.81 | 466.88 | 11.07 | 2101.12 | 2212.33 | 2233.73 |  |
| chrome | static | 50 | 5 | 100.00 | 4096.81 | 81.94 | 4110.24 | 146.69 | 21.34 | 2090.27 | 2149.39 | 2156.50 |  |
| mimic | static | 50 | 5 | 100.00 | 2810.19 | 56.20 | 4103.99 | 451.06 | 8.59 | 4832.34 | 5612.41 | 5821.24 |  |
| chrome | static | 100 | 5 | 100.00 | 7055.54 | 70.56 | 7074.35 | 158.94 | 19.29 | 4669.69 | 4902.44 | 4924.54 |  |
| mimic | static | 100 | 5 | 100.00 | 5233.28 | 52.33 | 6635.51 | 561.53 | 7.71 | 8624.76 | 11473.89 | 12350.29 |  |
| chrome | cpu | 1 | 20 | 100.00 | 1407.44 | 1407.44 | 1420.14 | 254.69 | 8.69 | 70.00 | 77.47 | — |  |
| mimic | cpu | 1 | 20 | 100.00 | 187.00 | 187.00 | 217.52 | 319.53 | 4.03 | 211.00 | 237.44 | — |  |
| chrome | cpu | 5 | 5 | 100.00 | 1598.98 | 319.80 | 1603.88 | 225.00 | 14.57 | 269.80 | 282.44 | — |  |
| mimic | cpu | 5 | 5 | 100.00 | 460.05 | 92.01 | 621.50 | 425.62 | 11.15 | 402.71 | 445.55 | — |  |
| chrome | cpu | 10 | 5 | 100.00 | 2008.63 | 200.86 | 2013.11 | 200.94 | 17.71 | 442.18 | 478.77 | — |  |
| mimic | cpu | 10 | 5 | 100.00 | 797.39 | 79.74 | 1130.03 | 483.44 | 15.85 | 571.77 | 604.65 | — |  |
| chrome | cpu | 25 | 5 | 100.00 | 3102.97 | 124.12 | 3115.82 | 166.50 | 24.82 | 822.86 | 962.50 | 975.54 |  |
| mimic | cpu | 25 | 5 | 100.00 | 1778.09 | 71.12 | 2530.26 | 581.88 | 11.21 | 2099.31 | 2471.39 | 2483.67 |  |
| chrome | cpu | 50 | 5 | 100.00 | 4946.93 | 98.94 | 4983.07 | 177.25 | 23.20 | 1822.22 | 2120.51 | 2127.51 |  |
| mimic | cpu | 50 | 5 | 100.00 | 3088.88 | 61.78 | 4551.51 | 601.38 | 7.98 | 5965.37 | 6438.45 | 6460.57 |  |
| chrome | cpu | 100 | 5 | 100.00 | 8691.18 | 86.91 | 8692.15 | 184.50 | 22.46 | 3823.68 | 4198.46 | 4224.32 |  |
| mimic | cpu | 100 | 5 | 100.00 | 5976.13 | 59.76 | 8418.51 | 1003.25 | 6.69 | 14434.27 | 14844.61 | 15303.92 |  |
| chrome | react | 1 | 20 | 100.00 | 1447.08 | 1447.08 | 1452.86 | 235.94 | 8.78 | 66.62 | 75.29 | — |  |
| mimic | react | 1 | 20 | 100.00 | 211.40 | 211.40 | 270.98 | 335.94 | 3.78 | 237.16 | 256.41 | — |  |
| chrome | react | 5 | 5 | 100.00 | 1594.96 | 318.99 | 1608.11 | 198.12 | 6.07 | 180.61 | 1174.96 | — |  |
| mimic | react | 5 | 5 | 100.00 | 518.32 | 103.66 | 694.71 | 525.00 | 10.80 | 411.86 | 436.59 | — |  |
| chrome | react | 10 | 5 | 100.00 | 1937.45 | 193.74 | 1945.59 | 200.94 | 8.75 | 1068.37 | 1102.30 | — |  |
| mimic | react | 10 | 5 | 100.00 | 898.64 | 89.86 | 1248.05 | 687.19 | 13.47 | 679.36 | 721.20 | — |  |
| chrome | react | 25 | 5 | 100.00 | 2984.31 | 119.37 | 2987.88 | 159.75 | 22.02 | 842.41 | 1656.99 | 1659.36 |  |
| mimic | react | 25 | 5 | 100.00 | 2129.17 | 85.17 | 2794.68 | 784.75 | 10.92 | 2149.18 | 2318.16 | 2335.62 |  |
| chrome | react | 50 | 5 | 100.00 | 4770.40 | 95.41 | 4801.86 | 187.69 | 19.42 | 2312.20 | 2358.03 | 2363.07 |  |
| mimic | react | 50 | 5 | 100.00 | 3548.31 | 70.97 | 4907.89 | 861.88 | 7.84 | 5933.60 | 6613.71 | 6687.53 |  |
| chrome | react | 100 | 5 | 100.00 | 8315.49 | 83.15 | 8510.30 | 196.03 | 20.66 | 4307.52 | 4490.87 | 4504.58 |  |
| mimic | react | 100 | 5 | 100.00 | 6920.32 | 69.20 | 8003.69 | 1379.09 | 6.73 | 12589.12 | 14025.41 | 14168.68 |  |

## Marginal RAM/session

The finite difference between adjacent tested N values is measured and divided by ΔN; this is not a direct measurement of every N→N+1 step. The linear model is a descriptive OLS fit to medians of successful levels only. The intercept is an extrapolation; actual startup overhead includes the initial blank page. Total RSS includes processes and caches retained from earlier waves, and the number of waves depends on N. The fit therefore combines active-page costs with process history. RSS growth relative to the start of the same wave is also shown; it can include background activity and GC.

| System | Workload | N | (Active RSS − before wave RSS)/N MiB |
|---|---|---|---|
| chrome | static | 1 | 58.74 |
| mimic | static | 1 | 13.92 |
| chrome | static | 5 | 51.56 |
| mimic | static | 5 | 17.93 |
| chrome | static | 10 | 54.89 |
| mimic | static | 10 | 18.70 |
| chrome | static | 25 | 56.87 |
| mimic | static | 25 | 14.19 |
| chrome | static | 50 | 57.66 |
| mimic | static | 50 | 17.82 |
| chrome | static | 100 | 58.21 |
| mimic | static | 100 | 16.39 |
| chrome | cpu | 1 | 77.77 |
| mimic | cpu | 1 | 16.81 |
| chrome | cpu | 5 | 67.32 |
| mimic | cpu | 5 | 12.00 |
| chrome | cpu | 10 | 69.91 |
| mimic | cpu | 10 | 15.66 |
| chrome | cpu | 25 | 71.41 |
| mimic | cpu | 25 | 13.42 |
| chrome | cpu | 50 | 72.07 |
| mimic | cpu | 50 | 10.99 |
| chrome | cpu | 100 | 72.93 |
| mimic | cpu | 100 | 10.40 |
| chrome | react | 1 | 80.27 |
| mimic | react | 1 | 17.29 |
| chrome | react | 5 | 67.41 |
| mimic | react | 5 | 19.14 |
| chrome | react | 10 | 67.07 |
| mimic | react | 10 | 18.03 |
| chrome | react | 25 | 67.87 |
| mimic | react | 25 | 18.81 |
| chrome | react | 50 | 68.89 |
| mimic | react | 50 | 16.69 |
| chrome | react | 100 | 69.49 |
| mimic | react | 100 | 19.66 |

| System | Workload | N low→high | ΔRSS MiB | ΔRSS/ΔN MiB |
|---|---|---|---|---|
| chrome | cpu | 1→5 | 191.54 | 47.89 |
| chrome | cpu | 5→10 | 409.65 | 81.93 |
| chrome | cpu | 10→25 | 1094.34 | 72.96 |
| chrome | cpu | 25→50 | 1843.96 | 73.76 |
| chrome | cpu | 50→100 | 3744.25 | 74.89 |
| mimic | cpu | 1→5 | 273.05 | 68.26 |
| mimic | cpu | 5→10 | 337.34 | 67.47 |
| mimic | cpu | 10→25 | 980.70 | 65.38 |
| mimic | cpu | 25→50 | 1310.79 | 52.43 |
| mimic | cpu | 50→100 | 2887.25 | 57.74 |
| chrome | react | 1→5 | 147.88 | 36.97 |
| chrome | react | 5→10 | 342.49 | 68.50 |
| chrome | react | 10→25 | 1046.86 | 69.79 |
| chrome | react | 25→50 | 1786.09 | 71.44 |
| chrome | react | 50→100 | 3545.09 | 70.90 |
| mimic | react | 1→5 | 306.92 | 76.73 |
| mimic | react | 5→10 | 380.32 | 76.06 |
| mimic | react | 10→25 | 1230.54 | 82.04 |
| mimic | react | 25→50 | 1419.14 | 56.77 |
| mimic | react | 50→100 | 3372.01 | 67.44 |
| chrome | static | 1→5 | 171.98 | 42.99 |
| chrome | static | 5→10 | 287.61 | 57.52 |
| chrome | static | 10→25 | 886.59 | 59.11 |
| chrome | static | 25→50 | 1533.73 | 61.35 |
| chrome | static | 50→100 | 2958.73 | 59.17 |
| mimic | static | 1→5 | 335.09 | 83.77 |
| mimic | static | 5→10 | 263.80 | 52.76 |
| mimic | static | 10→25 | 1027.85 | 68.52 |
| mimic | static | 25→50 | 997.85 | 39.91 |
| mimic | static | 50→100 | 2423.09 | 48.46 |

| System | Workload | Fixed fit MiB | Marginal fit MiB | R² | Max stable N |
|---|---|---|---|---|---|
| chrome | cpu | 1268.96 | 74.04 | 1.00 | 100 |
| mimic | cpu | 202.00 | 57.99 | 1.00 | 100 |
| chrome | react | 1273.59 | 70.20 | 1.00 | 100 |
| mimic | react | 233.07 | 67.16 | 1.00 | 100 |
| chrome | static | 1106.40 | 59.47 | 1.00 | 100 |
| mimic | static | 299.26 | 50.01 | 0.99 | 100 |

## Teardown / recovery

| System | Workload | N | Ready RSS MiB | RSS after waves MiB | Whole-series CPU s | CPU % |
|---|---|---|---|---|---|---|
| chrome | static | 1 | 383.48 | 1159.74 | 5.11 | 249.35 |
| mimic | static | 1 | 102.22 | 176.56 | 11.47 | 38.28 |
| chrome | static | 5 | 332.75 | 1137.44 | 9.61 | 164.01 |
| mimic | static | 5 | 103.14 | 374.97 | 9.97 | 287.37 |
| chrome | static | 10 | 389.84 | 1131.01 | 6.28 | 329.58 |
| mimic | static | 10 | 103.59 | 661.30 | 22.34 | 717.87 |
| chrome | static | 25 | 364.82 | 1145.07 | 17.08 | 302.28 |
| mimic | static | 25 | 103.95 | 1492.52 | 58.36 | 516.64 |
| chrome | static | 50 | 389.62 | 1222.39 | 36.67 | 313.06 |
| mimic | static | 50 | 103.95 | 1970.16 | 112.77 | 387.62 |
| chrome | static | 100 | 380.28 | 1244.48 | 79.47 | 306.53 |
| mimic | static | 100 | 103.07 | 3724.89 | 280.77 | 432.78 |
| chrome | cpu | 1 | 376.11 | 1331.39 | 5.09 | 221.20 |
| mimic | cpu | 1 | 103.09 | 170.14 | 6.39 | 128.75 |
| chrome | cpu | 5 | 379.06 | 1266.21 | 5.62 | 327.77 |
| mimic | cpu | 5 | 103.76 | 412.24 | 10.64 | 474.71 |
| chrome | cpu | 10 | 382.85 | 1307.62 | 10.05 | 355.80 |
| mimic | cpu | 10 | 104.61 | 685.97 | 24.17 | 766.34 |
| chrome | cpu | 25 | 380.23 | 1320.33 | 20.81 | 413.25 |
| mimic | cpu | 25 | 104.55 | 1497.66 | 72.73 | 652.01 |
| chrome | cpu | 50 | 385.54 | 1350.01 | 44.31 | 411.26 |
| mimic | cpu | 50 | 103.67 | 2588.43 | 150.34 | 480.02 |
| chrome | cpu | 100 | 386.50 | 1396.63 | 92.25 | 414.41 |
| mimic | cpu | 100 | 103.32 | 5001.08 | 501.62 | 671.67 |
| chrome | react | 1 | 387.10 | 1367.54 | 4.72 | 207.07 |
| mimic | react | 1 | 103.29 | 190.79 | 6.72 | 126.99 |
| chrome | react | 5 | 391.81 | 1270.96 | 4.95 | 120.19 |
| mimic | react | 5 | 104.94 | 439.19 | 13.12 | 567.13 |
| chrome | react | 10 | 394.12 | 1268.07 | 10.05 | 175.84 |
| mimic | react | 10 | 103.02 | 744.64 | 34.36 | 925.33 |
| chrome | react | 25 | 379.76 | 1290.96 | 19.97 | 351.80 |
| mimic | react | 25 | 104.00 | 1800.48 | 98.09 | 856.75 |
| chrome | react | 50 | 391.14 | 1334.94 | 46.92 | 364.56 |
| mimic | react | 50 | 103.80 | 2831.55 | 215.47 | 675.47 |
| chrome | react | 100 | 393.52 | 1390.19 | 98.02 | 404.95 |
| mimic | react | 100 | 103.56 | 5207.88 | 689.55 | 928.08 |

## Local server (measured independently)

HTTP handler time runs from the handler receiving a request to completion of response writing; it excludes TCP/server scheduler queues and network roundtrip. Before the main workload, the server is created outside the measured interval. Requests and bytes for each resource are recorded in raw.json.

| System | Workload | Mode | Requests | Server p50 ms | Server p95 ms | Server max ms |
|---|---|---|---|---|---|---|
| chrome | static | cold | 20 | 0.19 | 0.42 | 0.44 |
| mimic | static | cold | 20 | 0.19 | 0.25 | 0.27 |
| chrome | static | warm | 40 | 0.20 | 0.43 | 0.48 |
| mimic | static | warm | 40 | 0.15 | 0.33 | 0.52 |
| mimic | cpu | cold | 20 | 0.17 | 0.38 | 0.76 |
| chrome | cpu | cold | 20 | 0.22 | 0.43 | 0.55 |
| mimic | cpu | warm | 40 | 0.14 | 0.25 | 0.28 |
| chrome | cpu | warm | 40 | 0.22 | 0.45 | 0.54 |
| chrome | dom | cold | 20 | 0.33 | 0.47 | 0.49 |
| mimic | dom | cold | 20 | 0.28 | 0.44 | 0.47 |
| chrome | dom | warm | 40 | 0.26 | 0.53 | 1.79 |
| mimic | dom | warm | 40 | 0.19 | 0.39 | 0.43 |
| mimic | async | cold | 50 | 0.11 | 0.28 | 0.37 |
| chrome | async | cold | 50 | 0.19 | 0.55 | 0.62 |
| mimic | async | warm | 100 | 0.13 | 0.36 | 0.47 |
| chrome | async | warm | 100 | 0.12 | 0.40 | 1.58 |
| chrome | react | cold | 50 | 0.45 | 1.08 | 1.33 |
| mimic | react | cold | 50 | 0.24 | 0.47 | 0.62 |
| chrome | react | warm | 100 | 0.25 | 0.75 | 1.29 |
| mimic | react | warm | 100 | 0.22 | 0.42 | 0.67 |
| mimic | wasm | cold | 20 | 0.15 | 0.28 | 0.32 |
| chrome | wasm | cold | 20 | 0.25 | 0.49 | 0.53 |
| mimic | wasm | warm | 40 | 0.19 | 0.41 | 0.51 |
| chrome | wasm | warm | 40 | 0.22 | 0.47 | 0.51 |

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

1. By median warm navigation→completion, Mimic is faster on: none of the measured workloads. CDP readiness (shared probe, separate cold runs): mimic 215.02 ms; chrome 260.60 ms

2. Chrome is faster by the same metric on: async (143.49 / 48.98 ms Mimic/Chrome); cpu (114.88 / 48.70 ms Mimic/Chrome); dom (1803.17 / 73.01 ms Mimic/Chrome); react (178.77 / 46.02 ms Mimic/Chrome); static (73.00 / 19.34 ms Mimic/Chrome); wasm (96.58 / 24.98 ms Mimic/Chrome).

3. Fixed process overhead (CDP ready, including the initial page): mimic 104.04 MiB; chrome 380.72 MiB. Startup latency and the OLS intercept are reported separately above.

4. Estimated marginal RAM/session: chrome/cpu 74.04 MiB; mimic/cpu 57.99 MiB; chrome/react 70.20 MiB; mimic/react 67.16 MiB; chrome/static 59.47 MiB; mimic/static 50.01 MiB.

5. Maximum stable N by workload: mimic/cpu 100; chrome/cpu 100; mimic/react 100; chrome/react 100; mimic/static 100; chrome/static 100.

6. Throughput at the highest common stable level:

cpu, N=100: Mimic 6.69 and Chrome 22.46 successful sessions/s; RSS 5976.13 and 8691.18 MiB; CPU/session 1003.25 and 184.50 ms.

react, N=100: Mimic 6.73 and Chrome 20.66 successful sessions/s; RSS 6920.32 and 8315.49 MiB; CPU/session 1379.09 and 196.03 ms.

static, N=100: Mimic 7.71 and Chrome 19.29 successful sessions/s; RSS 5233.28 and 7055.54 MiB; CPU/session 561.53 and 158.94 ms.

7. Invalid comparisons after fixes: none at the correctness gate; individual iteration errors remain in raw.json. Before fixes: DOM, async, React, WebAssembly (see pre-fix).

8. Limitations: one busy workstation; headless Chrome; different V8 builds; HTTP cache disabled; limited corpus sizes and semantic checks; a synthetic React fixture, not a Next.js/production application. Polling and sampling add system load. RSS sums working sets, not unique physical RAM; no financial model of session cost is provided. Paging is a system-wide counter, not attribution of hard faults to a specific process. Mimic virtual-clock values are not used for comparisons.

9. Supported scope: “On Windows x64, local controlled workloads passed result validation in Mimic V8 and Chrome 152.0.7977.82; a reproducible harness, raw observations, and separate latency, CPU, and memory metrics are published. cpu, N=100: Mimic 6.69 and Chrome 22.46 successful sessions/s; RSS 5976.13 and 8691.18 MiB; CPU/session 1003.25 and 184.50 ms.”

10. The data do NOT support claims that “Mimic is X times faster than Chrome in general,” full browser compatibility, tenant-isolation security, an advantage in pure V8/JIT execution, gains on arbitrary sites, or monetary savings without an operating model.
