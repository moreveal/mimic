# Frame bridge and bootstrap optimization, 2026-09-10

This production change reduces the existing bridge; it does **not** implement
the experimental shared-isolate ownership model or make the browser Chrome-fast.
Each existing realm still owns its isolate. No page objects, host closures,
security tokens, or mutable browser state are shared between independent Pages.

## Changes

- Run the remote operation and reference description in one owner dispatch.
  Reentrant synchronous callbacks still use the existing cooperative actor
  protocol. A reference to an object in the caller's own realm needs no remote
  dispatch. Owner batches do not run tasks or microtask checkpoints.
- Describe cross-realm values in the owning JavaScript realm using the captured
  WeakMap and reflection intrinsics. Retain a canonical Go handle only when a
  new object is exported. Read live properties, prototypes and proxy shape on
  every operation; do not memoize observable object state.
- Serialize private descriptions with captured JSON/intrinsics and a
  null-prototype envelope. Preserve NaN, infinities and negative zero explicitly.
  Page replacement of JSON.stringify, Object.prototype.toJSON or the array
  iterator must not affect the protocol.
- Cache compiled bootstrap function bodies, including functions reached during
  the first execution. The cache contains immutable V8 code bytes, keyed by
  SHA-256 of exact source plus source name. It is limited to four entries and
  32 MiB; compilation and execution never hold the cache mutex. Every realm
  still executes the bootstrap and creates its own objects and host bindings.
  Only the explicit engine BootstrapRuntime entry point uses this cache;
  document scripts cannot opt in by choosing an internal-looking source name.
- Do not create a cancellation-watcher goroutine for contexts whose Done
  channel is nil. Cancellable execution retains its existing termination path.

The current gov8 v0.1.1 UnboundScript.CreateCodeCache reader retries a buffer
larger than 4 KiB using a handle freed by its first read. A bootstrap-sized
experiment reproduced an access violation. Production uses the separate
CompileFunctionAdvanced/FunctionCodeCache path, which recreates the handle
before retrying. The regression test produces a cache larger than 4 KiB,
destroys its producer isolate, and consumes it in independent concurrent
isolates with different host closures. No upstream module cache was patched.

## Measurement boundaries

The probe is the same 100-iteration prototype/property/function loop (checksum
6050), including fresh iframe initialization and removal. It is not a substitute
for the full protected-site flow, whose control-flow equivalence with Chrome
has not been established. Native Chrome's previously measured approximately
5 ms remains far below the production result.

Private artifacts are under
`compatibility/private-captures/bridge-bootstrap-20260910/`. QPC phase medians
exclude the first round; independent phase medians need not add exactly to the
mixed-loop median. Host counts and persistent-handle deltas are instrumented
observations, not unique-object counts or proof of a teardown leak.

Final QPC measurements (milliseconds, median of five warmed rounds):

| Phase | Previous ownership breakdown | This change |
| --- | ---: | ---: |
| Create/bootstrap/seed iframe | 68.922 | 40.959 |
| 100 remote global gets | 12.960 | 5.034 |
| 300 remote prototype operations | 51.177 | 17.034 |
| 100 global-object/property reads | 34.488 | 10.705 |
| 100 global-function/calls | 53.973 | 18.100 |
| Complete mixed loop | 153.126 | 49.370 |
| Local equivalent | 0.123 | 0.085 |
| Remove iframe | 0.168 | 0.126 |

Setup + mixed + remove is approximately 90.5 ms versus 222.2 ms in the previous
breakdown, about 2.45x locally. These are separate diagnostic runs, not an
interleaved statistical trial. Whole Go benchmark samples (10 iterations each,
including benchmark startup overhead) are 100.9, 103.8 and 126.2 ms/op. The older
approximately 195 ms result and the 222 ms breakdown must not be conflated into
an artificially exact baseline.

The final instrumented 20-loop run reports approximately 1500 host crossings
and 6902 additional persistent handles per loop, versus 2500 and 17101 before.
Child bootstrap compilation is 2.93 ms and execution 37.36 ms in that run,
versus the earlier 20.00 ms compile and 45.75 ms execute observation. There is
no claim that bootstrap execution or bridge overhead has reached zero.

## Validation and resource tradeoffs

- Full `go test ./... -json -count=1 -timeout=600s`: 1042 passing test/subtest
  events, no failures; browser package 319.4 seconds. The final local-reference
  dispatch shortcut was additionally checked with focused frame tests and race
  detection, along with bootstrap cache consumption in concurrent isolates.
- Fresh-build `tools/performance/fast_gate.py`: all six mandatory workloads,
  eight concurrency waves and two memory waves completed. The build receipt
  verifies the executable hash. Frozen workloads and baselines were unchanged.
- Gate completion medians: DOM 505.61 ms, static 61.63 ms, React 122.67 ms;
  previous task-profile gate: 536.56, 72.53, 140.67 ms. These unpaired health
  checkpoints do not establish a general workload speedup. Measured 25-Page
  throughput remained around 59–62 sessions/s.
- Ten-Page memory waves showed private memory after recovery of 149.0/160.9 MiB
  versus 136.3/152.2 MiB in the previous checkpoint. Marginal active private
  memory was 33.23/38.86 MiB per Page versus 31.74/38.22. Serialized code is
  intentionally retained in the bounded process cache; these two waves do not
  isolate that allocation from Go/V8 allocator retention or prove leak freedom.
- The full run initially discovered old `.go` source backups in the private
  diagnostic folder as a mixed-package directory. Those backups were renamed
  to `.go.txt`, and the full run was restarted. No tests were weakened.

## Remaining architectural costs

The bridge still executes Go host callbacks, transfers envelopes, and switches
between realm actors. Removing those costs requires the shared-isolate,
separate-context model described in `frame-bridge-ownership-20260910.md`, with
production handling of WindowProxy navigation, origin changes, retained objects,
per-context hooks and lifecycle. This patch does not claim to have done that.

Code reuse removes most recompilation, not realm initialization. Remaining
bootstrap execution constructs realm-local APIs and binds hosts. Reusing a live
realm would leak state and change observable identity; a future snapshot or
template design must explicitly rebind hosts and prove isolation and teardown.
