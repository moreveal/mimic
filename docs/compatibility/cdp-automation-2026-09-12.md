# CDP automation compatibility checkpoint — 2026-09-12

Work was isolated on `codex/cdp-automation`, based on
`9c82cddc6a564de1a38c285c566fc30107f21064`. The original working directory and
its uncommitted changes were not used as implementation input or modified.

## Behavioral result

The real Pyppeteer 2.0.0 and Puppeteer Core 25.10.0 suites each pass 34/34
scenarios against Mimic and the frozen headful Chrome 152.0.7977.82 reference.
All 68 normalized client observations agree. A separate raw protocol suite
matches all 16 interception cases, including request/response stages, explicit
empty patterns, overrides, disabling interception and detaching while paused.

On the common earlier 66-check suite, the unmodified committed baseline passed
13 checks: 12/33 in Pyppeteer and 1/33 in modern Puppeteer, whose new-page setup
blocked most later cases. The candidate passes those same 66 plus the two new
uncaught-page-error scenarios. This is a workload denominator, not a percentage
of all CDP semantics.

Receipts: [final client/interception/event result](cdp-automation-20260912/final.json),
[earlier candidate checkpoint](cdp-automation-20260912/candidate5.json),
[retained Chrome client observations](../../compatibility/cdp_automation_reference.json),
[retained Chrome interception observations](../../compatibility/cdp_automation_interception_reference.json).
The [harness instructions](../../compatibility/cdp_automation.md) reproduce the
checks, pin clients, capture wire traffic and verify oracle provenance.

## Implementation

- The exact Chrome `/json/protocol` capture generates Go wire types, parameter
  validation, the method/event registry, JSON support inventory and readable
  coverage tables. Generation is deterministic and offline after capture.
  There are 665 method definitions, 234 events and 611 types across 58 domains.
- A browser WebSocket supports multiple simultaneous flattened and legacy
  sessions, independent command IDs, target/context lifecycle and nested tab
  auto-attachment used by current Puppeteer. Each Page retains one event loop.
- Runtime handles preserve JavaScript identity, special values, property
  descriptors, object groups and promise lifetimes. Isolated worlds have
  separate globals/wrappers over the same DOM. Console arguments and public
  bindings retain their original values, including across navigation.
- DOM selection, remote-node conversion, document replacement, basic geometry,
  trusted keyboard/mouse input and forms use the underlying browser state.
  Snapshot restoration refreshes world ownership rather than retaining the
  seed realm's input routing. Input fixtures retain 12 measured Chrome cases.
- Navigation/history and child-frame reporting, lifecycle subscription gates,
  quiet-window network idle, response bodies, legacy and Fetch interception,
  and cancellation are usable by both clients. Detaching a debugger releases
  paused interception without cancelling the Page's navigation.
- Headers/offline/cache-read policy belong to a Page; cookies and the HTTP cache
  remain shared within its BrowserContext. Storage cookie commands honor the
  requested context; Network cookie reads filter by URL. User-agent identity,
  languages and client hints share the environment with outgoing headers.
  TLS overrides use separate verified/unverified connection pools, with page
  and browser session scope. Inherited blank-frame origins remain distinct
  from their source URL and Referrer.
- Uncaught timer exceptions deliver native Window error events and CDP
  `pageerror` observations. Cancelling the error event suppresses reporting;
  intervals continue and can still be cancelled using their original ID.

The [runtime ownership notes](../cdp-runtime.md) explain handle teardown,
cross-world parsing, pending promises, shared cache and request-origin ownership.

## Verification

The [validation receipt](cdp-automation-20260912/validation.json) links the
complete package output and distinguishes the unchanged legacy-check failure.

- `go test ./... -count=1 -timeout=12m` passes.
- `go test -race ./internal/cdp ./internal/network -count=1 -timeout=3m` passes.
- Focused browser race tests for Debugger, PageNetworkPolicy and BlankFrameFetch
  pass on both engines; focused input, snapshot, timer/error and document-world
  tests also pass on Goja and V8.
- The generator's five tests and the automation reporter's four tests pass.
  `python tools/generate_cdp.py --check` verifies all CDP projections.
- The pre-existing umbrella `tools/generate_compat.py --check` still fails its
  frozen artifact ledger: `window-secure-member-order.md` is not listed in
  `chrome/152/generated/artifact-hashes.json`. The same failure was reproduced
  in the untouched original checkout. Frozen artifacts and hashes were not
  rewritten to hide it.

## Performance and boundaries

A fresh intermediate build passed the unchanged fast gate, including the
10/25-Page concurrency waves and static/React memory recovery. Median completion
was 218.62/35.73/123.92 ms for DOM/static/React. Median measured 10/25-Page
throughput was 73.19/85.84 Pages/s. Recovery private memory was 146.80/169.79 MiB.
The [receipt](cdp-automation-20260912/performance.json) retains the executable
hash, build state, harness fingerprint, launches, measurements and raw-data hash.
This was one intermediate checkpoint, with other validation active on the host;
it is not a controlled before/after performance comparison. Later timer and
blank-frame fixes were checked by the final semantic and race suites.

The generated [support matrix](../cdp-coverage-generated.md) is authoritative:
9 commands are declared implemented, 99 partial, and 557 unsupported; 26 events
have partial support. Partial means the documented behavior is available, with
the remaining options and event details explicitly scoped. A generated schema
does not turn an unsupported operation into a successful stub.

Remaining areas include screenshots/PDF/rendering, full layout and mobile
viewport behavior, touch/wheel/pen/IME/contenteditable input, native inspector
previews/debugging, service-worker/worker target automation, auth challenges,
streaming bodies and complete network extra-info/paint/frame-start-stop event
fidelity. The raw event receipt preserves observed differences; the passing
client suite does not imply every event is identical to Chrome.
