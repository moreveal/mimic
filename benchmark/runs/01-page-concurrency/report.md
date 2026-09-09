# Mimic V8 and Chrome 152: Windows x64 baseline

This is a measurement after semantic fixes. No runtime performance optimizations were performed. The original version's correctness check is saved separately in `pre-fix/raw.json`.

## Environment

| Parameter | Value |
|---|---|
| Start / end date | 2026-09-08T18:06:55.459941+04:00 / 2026-09-08T18:11:57.401520+04:00 |
| Chrome | {'protocolVersion': '1.3', 'product': 'Chrome/152.0.7977.82', 'revision': '@d04cdb24d67b081f6cf80200ffc5233f44b61109', 'userAgent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/152.0.0.0 Safari/537.36', 'jsVersion': '15.2.124.21'} |
| Chromium | 1669021 / d04cdb24d67b081f6cf80200ffc5233f44b61109 |
| Mimic commit | b6e24c97e44c8ba722a954f30cb7f68eabef8c51 |
| Mimic V8 | 15.2.124.1-rusty |
| OS | Windows-11-10.0.26200-SP0 |
| CPU | {"Name":"Intel(R) Core(TM) i7-14700KF","NumberOfCores":20,"NumberOfLogicalProcessors":28} |
| CPU physical / logical | 20 / 28 |
| RAM GiB | 31.83 |
| Power plan | Power Scheme GUID: 381b4222-f694-41f0-9685-ff5bb260df2e  (Balanced) |
| Power overlay | 0 00000000-0000-0000-0000-000000000000 |
| Antivirus | {"displayName":"Windows Defender","productState":397568} |
| Chrome SHA-256 | ea36dd818a90176f1a70616f0363d9be527229389a6c073a0b1688b9e73f67e9 |
| Mimic SHA-256 | 7e31bf20d7e0ae996760ff2df847e193b6096ff64605bed1aeefa0b9b0263cba |

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

In the final series, both systems respond to Target.getTargets after the WebSocket handshake. Each uses 10 fresh processes, alternating system order; warmup is excluded. Date: 2026-09-08T18:11:43.906032+04:00. The main series uses the same shared probe.

| System | n | CDP p50 ms | p95 | Min | Max | SD | Ready RSS MiB |
|---|---|---|---|---|---|---|---|
| mimic | 10 | 424.61 | 445.50 | 217.51 | 454.39 | 67.62 | 104.42 |
| chrome | 10 | 326.81 | 412.16 | 297.97 | 446.55 | 42.98 | 378.33 |

## Cold startup (medians, ms)

| System | Workload | n | Process create | CDP ready | Runtime init | Cold total | Shutdown |
|---|---|---|---|---|---|---|---|
| chrome | static | 10 | 6.65 | 298.80 | 292.53 | 519.50 | 114.79 |
| mimic | static | 10 | 5.65 | 214.13 | 208.27 | 440.58 | 62.05 |
| mimic | cpu | 10 | 6.29 | 221.36 | 214.34 | 505.44 | 65.94 |
| chrome | cpu | 10 | 6.95 | 302.06 | 295.06 | 542.75 | 118.52 |
| chrome | dom | 10 | 10.33 | 305.12 | 295.52 | 555.77 | 124.48 |
| mimic | dom | 10 | 9.70 | 230.96 | 221.33 | 1775.37 | 78.11 |
| mimic | async | 10 | 8.86 | 434.43 | 421.52 | 865.51 | 96.52 |
| chrome | async | 10 | 11.36 | 509.46 | 479.13 | 956.09 | 211.38 |
| chrome | react | 10 | 6.56 | 291.86 | 283.97 | 496.09 | 111.99 |
| mimic | react | 10 | 5.52 | 216.52 | 209.35 | 535.03 | 60.70 |
| mimic | wasm | 10 | 5.56 | 219.50 | 214.08 | 449.67 | 62.23 |
| chrome | wasm | 10 | 6.16 | 283.62 | 277.15 | 462.54 | 92.81 |

## Warm session startup / teardown (medians)

| System | Workload | n | Create ms | Teardown ms | RSS after teardown MiB |
|---|---|---|---|---|---|
| chrome | static | 20 | 37.26 | 13.71 | 1195.36 |
| mimic | static | 20 | 85.87 | 2.84 | 170.84 |
| mimic | cpu | 20 | 96.57 | 3.21 | 172.04 |
| chrome | cpu | 20 | 42.00 | 16.63 | 1419.83 |
| chrome | dom | 20 | 40.40 | 17.13 | 1364.52 |
| mimic | dom | 20 | 78.78 | 45.79 | 309.63 |
| mimic | async | 20 | 174.03 | 6.22 | 175.40 |
| chrome | async | 20 | 63.38 | 26.62 | 1251.31 |
| chrome | react | 20 | 28.91 | 10.41 | 1413.74 |
| mimic | react | 20 | 69.43 | 5.11 | 193.82 |
| mimic | wasm | 20 | 79.12 | 2.47 | 174.78 |
| chrome | wasm | 20 | 43.42 | 15.38 | 1246.79 |

## Single-session workload latency (ms)

| System | Workload | Mode | n | Nav p50 | Execution p50 | Completion p50 | p95 | Min | Max | SD | CV |
|---|---|---|---|---|---|---|---|---|---|---|---|
| chrome | static | cold | 10 | 25.63 | 3.21 | 28.47 | 183.28 | 23.57 | 301.29 | 85.98 | 1.51 |
| mimic | static | cold | 10 | 79.63 | 1.93 | 81.54 | 84.69 | 79.74 | 86.01 | 1.82 | 0.02 |
| chrome | static | warm | 20 | 24.09 | 4.71 | 28.50 | 35.92 | 20.91 | 36.93 | 5.14 | 0.18 |
| mimic | static | warm | 20 | 88.05 | 2.97 | 90.26 | 183.27 | 75.58 | 184.81 | 39.67 | 0.35 |
| mimic | cpu | cold | 10 | 84.18 | 44.87 | 129.70 | 143.12 | 122.05 | 147.61 | 8.00 | 0.06 |
| chrome | cpu | cold | 10 | 25.59 | 33.69 | 59.05 | 202.73 | 54.22 | 310.58 | 79.46 | 0.94 |
| mimic | cpu | warm | 20 | 99.92 | 53.92 | 152.89 | 216.19 | 133.22 | 224.62 | 27.46 | 0.17 |
| chrome | cpu | warm | 20 | 28.72 | 37.90 | 65.74 | 80.08 | 53.94 | 101.64 | 10.27 | 0.15 |
| chrome | dom | cold | 10 | 29.32 | 14.28 | 43.42 | — | 37.96 | 84.87 | 15.65 | 0.31 |
| mimic | dom | cold | 10 | 86.03 | 1248.93 | 1335.05 | 1434.41 | 1186.82 | 1481.08 | 91.79 | 0.07 |
| chrome | dom | warm | 20 | 26.84 | 35.84 | 62.09 | 73.55 | 45.89 | 73.88 | 6.82 | 0.11 |
| mimic | dom | warm | 20 | 74.70 | 1271.19 | 1341.98 | 2544.31 | 1130.48 | 2657.22 | 534.58 | 0.33 |
| mimic | async | cold | 10 | 128.73 | 77.35 | 205.45 | 220.91 | 191.04 | 221.00 | 10.25 | 0.05 |
| chrome | async | cold | 10 | 48.48 | 43.50 | 95.66 | 288.07 | 76.59 | 411.86 | 100.74 | 0.77 |
| mimic | async | warm | 20 | 197.10 | 94.09 | 281.86 | 324.80 | 262.76 | 371.20 | 25.27 | 0.09 |
| chrome | async | warm | 20 | 42.20 | 40.57 | 83.05 | 106.16 | 70.39 | 210.69 | 30.06 | 0.33 |
| chrome | react | cold | 10 | 24.69 | 16.09 | 40.82 | — | 38.59 | 160.84 | 39.92 | 0.73 |
| mimic | react | cold | 10 | 84.85 | 93.62 | 178.01 | 206.58 | 170.17 | 206.60 | 14.68 | 0.08 |
| chrome | react | warm | 20 | 19.73 | 23.20 | 43.10 | 49.34 | 37.48 | 52.09 | 3.71 | 0.08 |
| mimic | react | warm | 20 | 77.98 | 89.06 | 168.72 | 183.85 | 155.98 | 218.78 | 12.70 | 0.07 |
| mimic | wasm | cold | 10 | 79.75 | 9.53 | 89.40 | 95.35 | 85.84 | 96.72 | 3.55 | 0.04 |
| chrome | wasm | cold | 10 | 24.81 | 5.35 | 30.32 | 121.74 | 24.14 | 184.10 | 48.93 | 1.06 |
| mimic | wasm | warm | 20 | 80.82 | 10.63 | 91.17 | 115.78 | 83.68 | 116.20 | 11.34 | 0.12 |
| chrome | wasm | warm | 20 | 26.11 | 6.45 | 32.99 | 46.79 | 23.16 | 49.67 | 7.39 | 0.22 |

## Single-session memory / CPU (medians)

| System | Workload | Mode | Before page MiB | After create MiB | Peak RSS MiB | Peak private MiB | CPU/session ms | CPU/workload ms |
|---|---|---|---|---|---|---|---|---|
| chrome | static | cold | 385.60 | 433.38 | 482.96 | 251.58 | 398.44 | 156.25 |
| mimic | static | cold | 104.09 | 138.91 | 169.71 | 209.12 | 210.94 | 125.00 |
| chrome | static | warm | 1136.95 | 1181.66 | 1195.36 | 604.31 | 210.94 | 62.50 |
| mimic | static | warm | 170.17 | 180.69 | 207.94 | 247.62 | 218.75 | 132.81 |
| mimic | cpu | cold | 104.36 | 139.14 | 173.89 | 211.98 | 281.25 | 171.88 |
| chrome | cpu | cold | 383.11 | 434.17 | 516.70 | 277.00 | 500.00 | 257.81 |
| mimic | cpu | warm | 171.54 | 192.01 | 209.73 | 247.27 | 312.50 | 210.94 |
| chrome | cpu | warm | 1341.59 | 1388.11 | 1419.83 | 777.79 | 328.12 | 187.50 |
| chrome | dom | cold | 379.41 | 432.90 | 503.90 | 262.84 | 414.06 | 171.88 |
| mimic | dom | cold | 104.18 | 138.62 | 265.13 | 305.66 | 2281.25 | 2054.69 |
| chrome | dom | warm | 1280.60 | 1327.88 | 1364.52 | 724.49 | 343.75 | 140.62 |
| mimic | dom | warm | 301.04 | 339.72 | 345.53 | 386.02 | 2695.31 | 2539.06 |
| mimic | async | cold | 104.43 | 139.95 | 178.23 | 218.60 | 414.06 | 242.19 |
| chrome | async | cold | 380.35 | 436.25 | 519.80 | 276.86 | 898.44 | 382.81 |
| mimic | async | warm | 174.19 | 186.54 | 211.35 | 248.96 | 539.06 | 320.31 |
| chrome | async | warm | 1188.18 | 1232.31 | 1251.44 | 635.80 | 453.12 | 218.75 |
| chrome | react | cold | 381.81 | 434.25 | 500.82 | 267.67 | 351.56 | 203.12 |
| mimic | react | cold | 104.09 | 139.63 | 173.35 | 214.17 | 367.19 | 234.38 |
| chrome | react | warm | 1332.69 | 1373.25 | 1413.74 | 802.03 | 218.75 | 132.81 |
| mimic | react | warm | 189.12 | 211.07 | 236.45 | 274.20 | 343.75 | 234.38 |
| mimic | wasm | cold | 103.90 | 140.79 | 174.61 | 214.48 | 210.94 | 117.19 |
| chrome | wasm | cold | 386.18 | 428.52 | 492.75 | 255.49 | 320.31 | 132.81 |
| mimic | wasm | warm | 173.25 | 180.90 | 204.84 | 241.72 | 234.38 | 125.00 |
| chrome | wasm | warm | 1178.64 | 1221.07 | 1246.79 | 616.93 | 234.38 | 78.12 |

## Concurrency / density

Each level uses a separate process, one excluded warmup, and max(5, ceil(20/N)) measured waves. Pages are created concurrently and all begin navigation after a barrier. Completed pages are held until the wave ends to measure simultaneous RSS. Throughput = successful sessions / time from create to final teardown, including the barrier and measurements but excluding HTTP-server setup/cleanup. Latency = create→completion, including barrier wait. This is batch throughput, not an optimized continuous request stream. Holding pages adds to throughput time but not latency. CPU per session = total wave CPU / successes; overlapping per-page CPU intervals are not summed.

Stopping limits: any error, <15% or <2 GiB available RAM, >1024 pages input/s for 3 s, or a 180 s timeout. These protect the workstation; the highest passing level is a lower bound on capacity supported here, not proof of an absolute maximum.

Mimic serializes CDP commands with a shared mutex. The measurement includes this behavior; its cost was not profiled. In stopped rows, RSS may have been sampled before all pages completed, and 0 waves means stopping during the excluded warmup. Such rows are diagnostic and excluded from stable-level fits/charts. Between waves, an additional 250 ms recovery, server cleanup, and checkpoint writing are outside batch throughput.

| System | Workload | N | Waves | Success % | RSS MiB | RSS/N MiB | Peak MiB | CPU/session ms | Sessions/s | p50 ms | p95 ms | p99 ms | Stop |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| chrome | static | 1 | 0 | 0.00 | 379.36 | 379.36 | 454.16 | — | 0.00 | 47.70 | — | — | memory pressure (<15% or 2 GiB available) |
| mimic | static | 1 | 1 | 100.00 | 204.61 | 204.61 | 204.61 | 234.38 | 6.45 | 152.69 | — | — | memory pressure (<15% or 2 GiB available) |
| chrome | cpu | 1 | 0 | 100.00 | 449.85 | 449.85 | 532.32 | 671.88 | 6.45 | 138.28 | — | — | memory pressure (<15% or 2 GiB available) |
| mimic | cpu | 1 | 0 | 100.00 | 160.64 | 160.64 | 180.16 | 265.62 | 4.03 | 245.17 | — | — | memory pressure (<15% or 2 GiB available) |
| chrome | react | 1 | 0 | 0.00 | 376.88 | 376.88 | 436.02 | — | 0.00 | 31.85 | — | — | memory pressure (<15% or 2 GiB available) |
| mimic | react | 1 | 0 | 0.00 | 132.71 | 132.71 | 133.18 | — | 0.00 | 79.08 | — | — | memory pressure (<15% or 2 GiB available) |

## Marginal RAM/session

The finite difference between adjacent tested N values is measured and divided by ΔN; this is not a direct measurement of every N→N+1 step. The linear model is a descriptive OLS fit to medians of successful levels only. The intercept is an extrapolation; actual startup overhead includes the initial blank page. Total RSS includes processes and caches retained from earlier waves, and the number of waves depends on N. The fit therefore combines active-page costs with process history. RSS growth relative to the start of the same wave is also shown; it can include background activity and GC.

| System | Workload | N | (Active RSS − before wave RSS)/N MiB |
|---|---|---|---|
| chrome | static | 1 | -2.23 |
| mimic | static | 1 | 46.79 |
| chrome | cpu | 1 | 76.68 |
| mimic | cpu | 1 | 56.40 |
| chrome | react | 1 | 1.87 |
| mimic | react | 1 | 29.21 |

| System | Workload | N low→high | ΔRSS MiB | ΔRSS/ΔN MiB |
|---|---|---|---|---|

| System | Workload | Fixed fit MiB | Marginal fit MiB | R² | Max stable N |
|---|---|---|---|---|---|

## Teardown / recovery

| System | Workload | N | Ready RSS MiB | RSS after waves MiB | Whole-series CPU s | CPU % |
|---|---|---|---|---|---|---|
| chrome | static | 1 | 376.29 | 517.31 | 0.47 | 597.45 |
| mimic | static | 1 | 103.66 | 153.88 | 0.23 | 151.13 |
| chrome | cpu | 1 | 369.82 | 548.04 | 0.67 | 433.62 |
| mimic | cpu | 1 | 104.24 | 168.34 | 0.27 | 106.94 |
| chrome | react | 1 | 369.84 | 502.15 | 0.16 | 346.76 |
| mimic | react | 1 | 103.50 | 133.69 | 0.11 | 130.65 |

## Local server (measured independently)

HTTP handler time runs from the handler receiving a request to completion of response writing; it excludes TCP/server scheduler queues and network roundtrip. Before the main workload, the server is created outside the measured interval. Requests and bytes for each resource are recorded in raw.json.

| System | Workload | Mode | Requests | Server p50 ms | Server p95 ms | Server max ms |
|---|---|---|---|---|---|---|
| chrome | static | cold | 20 | 0.20 | 0.47 | 0.66 |
| mimic | static | cold | 20 | 0.14 | 0.25 | 0.29 |
| chrome | static | warm | 40 | 0.23 | 0.38 | 0.42 |
| mimic | static | warm | 40 | 0.22 | 0.43 | 1.38 |
| mimic | cpu | cold | 20 | 0.21 | 0.38 | 0.38 |
| chrome | cpu | cold | 20 | 0.34 | 0.57 | 0.61 |
| mimic | cpu | warm | 40 | 0.20 | 0.38 | 0.39 |
| chrome | cpu | warm | 40 | 0.26 | 0.50 | 0.57 |
| chrome | dom | cold | 18 | 0.32 | 0.92 | 1.68 |
| mimic | dom | cold | 20 | 0.22 | 0.40 | 0.46 |
| chrome | dom | warm | 40 | 0.31 | 0.42 | 1.37 |
| mimic | dom | warm | 40 | 0.19 | 0.34 | 0.35 |
| mimic | async | cold | 50 | 0.13 | 0.32 | 1.08 |
| chrome | async | cold | 50 | 0.23 | 0.60 | 1.09 |
| mimic | async | warm | 100 | 0.20 | 0.48 | 0.60 |
| chrome | async | warm | 100 | 0.26 | 1.21 | 1.97 |
| chrome | react | cold | 45 | 0.33 | 0.88 | 1.00 |
| mimic | react | cold | 50 | 0.21 | 0.52 | 0.61 |
| chrome | react | warm | 100 | 0.28 | 0.98 | 2.11 |
| mimic | react | warm | 100 | 0.21 | 0.45 | 0.57 |
| mimic | wasm | cold | 20 | 0.18 | 0.40 | 0.48 |
| chrome | wasm | cold | 20 | 0.34 | 0.51 | 0.86 |
| mimic | wasm | warm | 40 | 0.20 | 0.40 | 0.45 |
| chrome | wasm | warm | 40 | 0.29 | 0.53 | 0.74 |

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
| chrome | dom | cold | 9/10 | INVALID — error or semantic mismatch; no speed claim |
| mimic | dom | cold | 10/10 | VALID |
| chrome | dom | warm | 20/20 | VALID |
| mimic | dom | warm | 20/20 | VALID |
| mimic | async | cold | 10/10 | VALID |
| chrome | async | cold | 10/10 | VALID |
| mimic | async | warm | 20/20 | VALID |
| chrome | async | warm | 20/20 | VALID |
| chrome | react | cold | 9/10 | INVALID — error or semantic mismatch; no speed claim |
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

1. By median warm navigation→completion, Mimic is faster on: none of the measured workloads. CDP readiness (shared probe, separate cold runs): mimic 424.61 ms; chrome 326.81 ms

2. Chrome is faster by the same metric on: async (281.86 / 83.05 ms Mimic/Chrome); cpu (152.89 / 65.74 ms Mimic/Chrome); dom (1341.98 / 62.09 ms Mimic/Chrome); react (168.72 / 43.10 ms Mimic/Chrome); static (90.26 / 28.50 ms Mimic/Chrome); wasm (91.17 / 32.99 ms Mimic/Chrome).

3. Fixed process overhead (CDP ready, including the initial page): mimic 104.12 MiB; chrome 379.51 MiB. Startup latency and the OLS intercept are reported separately above.

4. Estimated marginal RAM/session: insufficient successful levels for a model.

5. Maximum stable N by workload: mimic/cpu 0; chrome/cpu 0; mimic/react 0; chrome/react 0; mimic/static 0; chrome/static 0.

6. Throughput at the highest common stable level:

7. Invalid comparisons after fixes: none at the correctness gate; individual iteration errors remain in raw.json. Before fixes: DOM, async, React, WebAssembly (see pre-fix).

8. Limitations: one busy workstation; headless Chrome; different V8 builds; HTTP cache disabled; limited corpus sizes and semantic checks; a synthetic React fixture, not a Next.js/production application. Polling and sampling add system load. RSS sums working sets, not unique physical RAM; no financial model of session cost is provided. Paging is a system-wide counter, not attribution of hard faults to a specific process. Mimic virtual-clock values are not used for comparisons.

9. Supported scope: “On Windows x64, local controlled workloads passed result validation in Mimic V8 and Chrome 152.0.7977.82; a reproducible harness, raw observations, and separate latency, CPU, and memory metrics are published. ”

10. The data do NOT support claims that “Mimic is X times faster than Chrome in general,” full browser compatibility, tenant-isolation security, an advantage in pure V8/JIT execution, gains on arbitrary sites, or monetary savings without an operating model.
