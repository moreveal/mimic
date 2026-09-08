# Mimic

Mimic models Chrome-visible JavaScript and browser state in Go without running
or embedding Chromium. It has an explicit browser scheduler, resource loader,
separate document/frame/Worker realms, and a subset of CDP.

It is not a renderer, a Chromium wrapper, or a fully compatible Chrome browser.
Generated API names describe known shape; they do not imply implemented semantics.
No external-site result or performance claim is a product guarantee.

## Build and run

The current build is **Windows amd64**, Go **1.26**, with CGO and a compatible
C compiler (tested with `gcc`). Platform support is intentionally unchanged.
Dependencies are pinned in `go.mod`/`go.sum`; the local `third_party/tls-client`
replacement is required. A first build needs those module dependencies available.

```powershell
go build -o .build/mimic.exe ./cmd/mimic
./.build/mimic.exe -listen 127.0.0.1:9222 -chrome 152
# Explicit fallback, using the same binary:
./.build/mimic.exe -listen 127.0.0.1:9222 -chrome 152 -engine quickjs
```

V8 is the default (`github.com/maclof/gov8 v0.1.1`); its bundled native engine
reports **15.2.124.1-rusty**. QuickJS (`quickjs-go v0.7.7`) is retained as an
explicit fallback; goja is retained for development and regression tests.
Neither fallback promises V8-equivalent ECMAScript behavior. gov8 extracts its
packaged DLL into its user cache; no repository-local V8/Chrome download is needed.
No `.env` file or secret is required. Stop the foreground server with Ctrl+C.

CDP listens on loopback by default. It has no authentication and is intended for
trusted local clients. `-navigation-timeout 30s` bounds navigation, CDP evaluation
and event-loop turns. Mimic is not a security sandbox for hostile code.

## CDP

Read `http://127.0.0.1:9222/json/version` or `/json/list`, then connect a WebSocket
client to the returned `webSocketDebuggerUrl`. Send, for example:

```json
{"id":1,"method":"Runtime.evaluate","params":{"expression":"1 + 2"}}
```

`Page.navigate` accepts an HTTP(S) URL. Its reply acknowledges navigation start;
wait for lifecycle events before treating the page as loaded. See the
[CDP matrix](docs/cdp-compatibility.md) for the actual supported contract.
An optional static snapshot exporter is available as
`python compatibility/save_snapshot.py --endpoint http://127.0.0.1:9222 --output snapshots/example`
with Pyppeteer 2.0.0 installed. It may load referenced resources and does not
capture canvas pixels, live form state or embedded frames.

## Architecture and target

`CDP -> browser commands -> canonical state / scheduler / resource loader -> engine`.
`chrome/152` is selected through a version-neutral compatibility registry.
Handwritten Web API semantics consume the generated Blink WebIDL/CDP surface.
Transport/session observations remain authoritative for network protocol,
connection reuse and Resource Timing.

The target is **Chrome 152.0.7977.82**, Chromium
`d04cdb24d67b081f6cf80200ffc5233f44b61109` (r1669021). Exact Blink, V8/CDP and
WPT identifiers are in `chrome/152/target.json`. The target V8 source revision
and the packaged gov8 binary are separately identified; their equivalence has
not been established by this cleanup.
The authoritative behavioral oracle is normal **headful** Windows x64 Chrome with
a fresh controlled profile. Headless observations are explicitly mode-scoped and
non-authoritative until a probe is proven invariant. See
[the oracle policy](docs/oracle-policy.md) and [the headless audit](docs/oracle-headless-audit.md).

Read [architecture and invariants](docs/architecture.md),
[compatibility limits](docs/compatibility.md), and
[generated-data provenance](docs/generated-data.md) before changing state ownership.

## Verification

```powershell
go test ./...
go test -race ./...
python tools/generate_compat.py --check
python tools/check_repository.py
```

Tests use local fixtures; no external E2E investigation is part of this baseline.
The race detector makes generated-surface bootstrap substantially slower; retain
Go's default test timeout. See [the stabilization audit](docs/stabilization.md)
for exact successful and failed runs, including the original hangs.
The nested transport module has its own focused tests documented there.

The [Navigator capability matrix](docs/navigator-capabilities.md) links exact
Chrome captures, before/after differentials and tested semantics. Shape parity
and intentionally unavailable service/hardware backends are reported separately.

`--check` is offline: it verifies retained artifact hashes, pins and deterministic
IDL/CDP projections. Full upstream regeneration is a separate explicit operation.
Historical observations and small before/after captures are retained under
`docs/` and `compatibility/captures/`; optional site-oriented tools live in
`compatibility/research/` and are never loaded by production code.

Known gaps include rendering, complete CSS layout, Canvas/WebGL pixels, media,
full DOM/Web API algorithms, full CDP object handles, cookie/CORS completeness,
Streams backpressure/BYOB and complete fallback microtask semantics. Do not infer
full Chrome compatibility from a passing focused suite.
