# Shared retained response bytes without spill

This candidate keeps the existing cache expiration/replacement and 128-entry
request-history retention rules. It changes ownership and copies, without adding
a byte budget or temporary files. The independently measured synchronous-spill
candidate was rejected because no-store Fetch delivery regressed by 22.6%; its
measurements remain in [the rejected-spill archive](data/optimization-20260914/body-retention/).
Its implementation and temporary binaries are discarded after the decision.

## Measured decision: select sharing/release/projection, then run combined gate

The initial three-mode process showed gains for cached responses and an apparent
11.8% no-store Fetch regression after another mode's forced collection/scavenging.
That diagnostic changes the next Page's heap reuse when earlier owners are
released. Its no-store result required isolated validation, not dismissal as noise.

| Initial mode (24 deliveries/variant) | Fetch ms, control → candidate | Fetch + CDP ms | Go live after GC, MiB |
|---|---:|---:|---:|
| Repeated cache | 12.72 → 11.92 | 17.33 → 15.16 | 78.30 → 32.31 |
| Unique no-store, after prior cleanup | 12.72 → 14.22 | 17.26 → 17.34 | 80.74 → 76.74 |
| Unique cache | 14.76 → 14.00 | 20.26 → 16.58 | 129.28 → 76.85 |

The subsequent no-store C-A-A-C used one scenario per process, with the same
production executables, setup and full lifecycle cleanup. It separately measured
natural Go GC and allocation around each operation. No GC was disabled or tuned.

| Isolated no-store, 24 deliveries/variant | Control | Candidate |
|---|---:|---:|
| Fetch median, ms | 13.106 | 12.675 |
| CDP projection median, ms | 4.193 | 2.378 |
| Fetch + CDP median, ms | 17.363 | 16.052 |
| Fetch allocation median, bytes | 14,301,152 | 14,297,796 |
| CDP allocation median, bytes | 19,588,928 | 11,199,972 |
| Natural Fetch GC cycles, total | 9 | 5 |
| Natural CDP GC cycles, total | 6 | 2 |
| Fetch GC pause total, ms | 1.149 | 1.765 |
| CDP GC pause total, ms | 2.048 | 0.291 |
| NextGC before Fetch median, bytes | 110,151,106 | 112,967,010 |
| Complete process wall median, s | 0.9168 | 0.9137 |
| Complete process user + system CPU median, s | 0.840 | 0.825 |

The two missing 4 MiB intermediates explain the CDP allocation reduction; Fetch's
own allocation is unchanged. The earlier Fetch regression did not reproduce in
an isolated process. GC stop-the-world pause totals cannot explain the earlier
1.5 ms-per-delivery difference. Heap reuse and GC phase placement are plausible
contributors, but the short series does not establish a unique causal attribution.

A separate C-A-A-C CPU-profile series used 48 no-store deliveries per process.
Its timings are not mixed with the unprofiled comparison. Across each variant's
two profiles, sampled `runtime.cgocall` CPU was identical at 1.27 s; Go `memmove`
fell 0.37 → 0.31 s while memory clearing rose 0.09 → 0.16 s. There was no new
large store/locking path. These samples include bootstrap, diagnostic JSON,
validation and cleanup; they do not isolate native V8 execution. Windows pprof
could not symbolize the Linux libc samples, which remain explicitly unresolved.

The no-store Page-close samples release all 48 MiB of retained body bytes:
Go heap falls to about 24.0 MiB while control remains about 72.1 MiB until its
Context releases the Page. For cached responses, Page close preserves cache
ownership, and Context close releases all store bodies. Every candidate
Context-close sample has zero retained bodies/bytes. This is the accepted memory
and ownership benefit; it is not a claim of globally bounded retention.

Receipts/raw evidence in [the sharing archive](data/optimization-20260914/body-sharing/):
`linux-paired/` (initial multi-mode), `linux-no-store/`
(isolated normal timing and `gc-allocation-summary.json`), and
`linux-no-store-profile/` (separate CPU profiles and raw counters). The initial
test executable receipts were preserved in `multi-mode-builds/`; only diagnostic
instrumentation changed for the isolated check. The selected production mimic
remained `2c65349400d8bf2833ae1fbfdf57573d1e0464d3b17eaecefd7b0394a69daca3`.

Cache and history reference one immutable stored body when their bytes match.
Public Go responses and mutable After interceptors keep independent byte slices.
Cache is captured before interception; final history is captured afterwards.
An unchanged After result is compared to immutable storage before the two owners
share it. CDP converts directly to string/base64, avoiding the intermediate raw
body clone. Replacement, eviction, Page close and Context close release owners;
the last owner clears the bytes even if the closed object graph remains reachable.

Bulk copies and reads do not hold a Context-wide lock. Context close closes
admission and waits for pending copy registration before releasing storage.
Independent Pages keep their execution model, transport and event-loop ownership.

## Validation and measurement

The diagnostic is identical to the rejected spill slice, including byte checks,
mutable JS output checks, full CDP base64 validation, forced live recovery, Page
close and Context close. Both sides already use binary Fetch transfer. Complete
process receipts also include bootstrap, checks, GC and cleanup, and report CPU.
No diagnostic workload or frozen harness is promoted as production code.

Windows full network and focused browser/CDP Fetch, Worker, cache, body and
interceptor regressions passed. Linux focused network race checks passed.

Fresh build records and executable hashes are in this directory. The build script
restores only sharing-related files for the control, preserving the selected
binary transfer. The standalone control mimic must match SHA-256
`74f60fc5f07e40689a12167a94c4fabf374abc96868874c3768ea14845fd0f62`.

JSON observations have an additional `.gz` archive suffix. Decompression recovers
their original bytes; the archive manifest identifies both hashes. The temporary
diagnostic driver and executables are discarded after the decision.

## Promotion boundary

Tracked production files: `internal/network/session.go`,
`internal/network/loader.go`, `internal/cdp/server.go`,
`internal/browser/page.go`, `internal/browser/browser.go`.
New production file: `internal/network/body_store.go`.
Focused regression tests: `internal/network/body_store_test.go`.
`candidate-tracked.patch` contains only those tracked production changes.

Do not promote `internal/browser/body_retention_probe_test.go`,
`internal/browser/fetch_transfer_probe_test.go`, or
`internal/network/stream_source_spike_test.go` as part of this production slice.

The retained byte count is still not globally bounded. A Page holding many
distinct no-store responses keeps one history copy per response until eviction
or close, as before. Distinct cacheable responses keep their cache owner after
Page close. Bounded async spill requires a separate decision and must include
queue admission, pending immutable buffers, writer completion and cleanup costs.
