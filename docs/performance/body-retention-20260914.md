# Bounded shared response BodyStore slice

This independent slice is measured over the already selected binary Fetch
transfer. The control production executable is byte-for-byte identical to that
binary winner (`74f60fc5f07e40689a12167a94c4fabf374abc96868874c3768ea14845fd0f62`).
No DOM, startup, or handle-lifetime changes are included in either variant.
Measurements must run in the campaign owner's exclusive CPU window.

## Measured decision: reject synchronous spill as the default

Linux C-A-A-C, 24 deliveries per variant/mode at 4 MiB:

| Mode | Fetch ms, control → candidate | Fetch + CDP ms | Go allocated/delivery, MiB | Go live after GC, MiB |
|---|---:|---:|---:|---:|
| Repeated cache | 13.76 → 12.95 | 19.81 → 16.23 | 26.89 → 14.90 | 80.30 → 33.46 |
| Unique no-store | 13.85 → 16.98 | 19.61 → 19.72 | 32.72 → 15.42 | 80.64 → 30.63 |
| Unique cache | 15.81 → 14.81 | 22.88 → 17.82 | 36.93 → 15.70 | 129.42 → 33.00 |

Synchronous overflow adds 22.6% to no-store Fetch delivery and is not promoted.
The memory improvement does not justify that regression. Sharing, explicit owner
release and direct CDP projection proceed as a separate memory-only experiment
which retains the current storage policy. Bounded disk storage remains unaccepted.

The complete process includes startup, every JS byte check, base64 result
verification, CDP projection, Page/Context close, final collection and file cleanup.
External monotonic wall time: 2.338 → 2.239 s median; total user+system CPU:
2.465 → 2.30 s. Peak RSS: 491.1 → 326.8 MiB median. These are two whole-process
samples per variant and are supporting evidence, not a throughput conclusion.
GNU time reported 5.18 s elapsed for the last control while the outer monotonic
measurement was 2.295 s: its wall-clock sample is invalid (probable WSL clock
adjustment) and is not used. CPU counters remain recorded in the receipts.

The candidate retained 4.0 MiB resident bytes per scenario, spilling 44 MiB for
each unique-URL scenario. Every Context-close sample reports zero stored bodies,
files and resident bytes. No-store Page close alone releases every history body;
cached bodies remain until Context close, as required by the separate owner.

The exact measured source is preserved in `sync-spill-source/`; fresh executable
receipts and raw samples are in this directory and `linux-paired/`.

## Implementation boundary

One immutable retained body can have cache and request-history owners. The public
`Response.Body` remains an independent mutable byte slice, including responses
given to legacy `After` interceptors. Cache preserves the pre-interception bytes;
history preserves the final bytes. When an interceptor is present, a bounded
comparison against immutable storage verifies unchanged content before sharing.
It never assumes an inactive CDP interceptor cannot mutate, skips an interceptor,
or aliases its mutable body with cache storage.

Each Context has an 8 MiB retained resident-byte budget for this experiment.
Overflow goes to private temporary files; body retrieval is preserved. Cache
replacement, history eviction, Page close and Context close release their owners.
CDP converts directly from immutable storage to its required string/base64
result, avoiding an independent full raw-body copy. File reads use bounded
intermediate buffers. Mutable Go consumers still receive a full copied body.

The store has one lock for accounting and short owner lookups; it holds no
Context/session lock during disk I/O. Per-body locks exclude release during read.
Context close stops new writes, waits for pending writes/cleanup, releases all
bodies and propagates file cleanup errors. A standalone Loader closes its own
Session when response bodies close; Page Loaders preserve shared Context owners.

Optional retention failures preserve successful resource delivery and produce
trace diagnostics; CDP reports the stored-body error explicitly. Cache read
failure records a diagnostic and falls back to a network load. The legacy
`GetCached(Response, bool)` helper treats a read error as a cache miss.

## Diagnostic and validation

The same opt-in `TestBodyRetentionProbe` is compiled into control and candidate.
For each of repeated cached URL, unique no-store URLs, and unique cached URLs,
it creates a fresh Context/Page and reads 12 responses of 4 MiB. Every response is
validated byte-by-byte in JS; the JS buffer is mutated afterwards to detect aliasing.
Each fetch is followed by the same required CDP body projection, including full
base64-result verification. A no-op legacy After interceptor remains installed.

Fetch completion and CDP body projection are timed separately. Go allocation,
V8 memory, process resident/private memory and store ownership counters are
sampled. Forced Go/V8 collection occurs outside delivery intervals. Measurements
after Page and Context close deliberately keep the Session object reachable:
they test explicit owner release rather than eventual collection of the whole
closed object graph. Protocol projection timing excludes the JSON wire encoding
and transport; full CDP round-trip performance belongs to the campaign harness.

Windows full `internal/network` suite and focused browser/CDP Fetch, Worker,
cache, response-body, interceptor and snapshot regressions passed. Linux focused
network race checks passed. Tests cover file spill, source/caller independence,
mutating After, shared ownership, eviction/replacement, concurrent read/close,
concurrent put/close and unavailable temporary storage.

## Limitations and next decision

This slice bounds resident retained body bytes, not the overall HTTP cache size
or total temporary disk bytes. Existing cache expiration/replacement rules and
the 128-entry history policy remain in effect; distinct cached URLs can retain
disk bodies until cache clear or Context close. File-backed cache hits and
legacy-interceptor comparison incur real I/O included in this slice's timings.
The 8 MiB budget is an experimental default, not a workload-specific shortcut.

Delivery remains buffered until EOF. This slice does not establish streaming
Fetch, early headers, backpressure or bounded memory for `text()/json()/tee`.
The separate physical source proof remains experimental and must not be promoted
as an integrated browser stream.

Promotion requires measured memory benefit without unacceptable delivery or
throughput loss. Production files are `internal/network/body_store.go`,
`internal/network/session.go`, `internal/network/loader.go`,
`internal/cdp/server.go`, `internal/browser/page.go`, `internal/browser/browser.go`.
Focused correctness is in `internal/network/body_store_test.go`.
`internal/browser/body_retention_probe_test.go` is non-frozen diagnostic tooling.
`candidate-tracked.patch` covers only the tracked BodyStore changes; the new
production and test files are copied separately.

Run from the isolated worktree after reserving its CPU window:

```text
python .build/run_body_spike.py --output .build/body-retention/linux-paired --bytes 4194304 --iterations 12
```

`linux-{control,candidate}-build.json` and Windows equivalents contain fresh
build commands, source hashes and executable SHA-256. `linux-paired/receipts.json`
records pre-launch hash verification and exact commands for the C-A-A-C sequence.
Raw samples and the summarized result live next to those receipts.
