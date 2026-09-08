# Stabilization audit — 2026-09-08

Scope: cleanup, reproducibility and lifecycle corrections only. No Git repository
was initialized, no remote repository contacted, no external website investigated.
All Go verification used the installed dependency cache with GOPROXY=off and
GOSUMDB=off. Host: Windows amd64, Go 1.26.1, CGO_ENABLED=1, gcc, Python 3.14.2.

## Architecture

Preserved the Chrome-semantics runtime without Chromium, primary V8, explicit
QuickJS fallback, goja development adapter, version-neutral Chrome registry,
canonical Environment/Browser state, task sources and explicit V8 checkpoints,
separate frame/Worker realms and the shared asynchronous resource loader.

Ownership findings and caller serialization requirements are recorded in
architecture.md. No independent cookie/session authority or fabricated transport
observations were introduced. JavaScript graphics values remain projections of
the environment. DOM wrappers, Performance projections and WindowProxy retention
were preserved. The unused Realm.baseURL copy only propagated to other copies
and was removed; committed origin remains a separate security fact.

## Files and evidence

66 artifact targets (3,088 files, 1,695,942,195 bytes) were reversibly moved into a
private local archive outside this repository. Recursive deletion was blocked by
automatic safety review, so no destructive workaround was used. The archive is
not part of the deliverable and must not be published. Former relative paths,
file counts and byte sizes are in cleanup-manifest.json.

This includes .tmp, old .build binaries, .tmp-* root captures, downloaded Chrome,
source cache, Python bytecode/dependencies, raw website assets and duplicate
hidden benchmark/network output. Fresh verification binaries/logs are confined
to ignored .build/. Twenty-six small module/DOM/crypto/API before/after captures
remain under compatibility/captures/semantic-checkpoints/ with documented purpose.
Five optional instruments moved to compatibility/research/. The obsolete Vue
payload patcher was archived with the downloaded asset it depended on.

README, architecture, roadmap and historical evidence references were updated.
The engine-spike command and its low-level V8 helpers remain because the engine
differential harness actually uses them. The upstream dependency source/license
remains intact; third_party/README.md records all local deltas.

## Corrected defects

- Streams constructors wrote through readonly generated WebIDL descriptors.
  TransformStream and writer state now uses private slots. The async-Promise
  executor in the regression hid rejections and hung; the test now propagates them.
- A dynamic-script test assumed network completion within 1 ms. It now waits for
  load/error. Resource Timing tests select observations by URL, not parallel-load
  ordering. Timing-only tests exclude generated realm bootstrap from their budget.
- CDP tests directly called a Page while asynchronous navigation was running.
  They now use the serialized protocol path. CDP integration uses the production
  V8 default; broad browser semantics still exercise goja and explicit QuickJS/V8.
- All CDP sessions share a command barrier. Disconnect/shutdown cancels session
  operations and joins handlers/pumps before closing pages/transports. Context
  cancellation reaches realm network/module work and Worker agents before joins.
- Context-aware checkpoints cover evaluation, init/dynamic scripts, frame delivery
  and worker turns. V8 cancellation watchers are joined on the owner thread,
  preventing late termination and failed-dispatch goroutine leaks.
- V8 disposal no longer closes a command channel under concurrent senders or
  returns its internal stop sentinel as a public error. Actor identity is recorded
  after native thread locking. Persistent handles are released on the owner thread.
- Worker teardown joins started agents and suppresses queued late callbacks;
  self.close exits the worker loop. fetch/XHR external work is counted/joined.
- Removing/replacing an iframe cancels its pending document transport; detached
  WindowProxy identity remains retained. A local cancellation regression passes.
- Removed the unused pprof listener/import and corrected stale V8-spike comments.
  CLI Ctrl+C initiates server shutdown.

## Test history and final baseline

Initial focused command passed:

```powershell
go test ./internal/scheduler ./internal/state ./internal/network ./internal/engine/... -count=1 -timeout=90s
```

The initial `go test ./... -count=1 -timeout=90s` failed: Resource Timing order
assumption and Streams hang. `go test ./internal/browser -run
'^TestReadableWritableAndTransformStreams$' -v -count=1 -timeout=10s` also timed out.
An intermediate full run exposed the dynamic-script 1 ms assumption. These were
fixed, not skipped or hidden with larger global timeouts.

Initial `go test -race ./... -count=1 -timeout=90s` timed out while the browser
package was still doing expensive goja generated-surface bootstrap and exposed
short CDP timing assumptions. A default-timeout race run subsequently exposed the
CDP test's unsynchronized direct Page access and the resource-test setup budget.
Those defects were corrected. No tests are quarantined/skipped by this cleanup.

| Command | Result |
|---|---|
| `go test ./...` | PASS; browser 29.475 s, CDP 0.973 s; all root-module packages |
| `go test -race ./...` | PASS; browser 189.576 s, CDP 2.913 s, no reported Go data races |
| `go build -o .build/mimic.exe ./cmd/mimic` | PASS; Windows amd64, V8 default, QuickJS selectable |
| `python tools/smoke_local.py` | PASS for default V8 and explicit QuickJS |
| `python tools/generate_compat.py --check` | PASS offline; deterministic projections and retained hashes |
| `python tools/check_repository.py` | PASS; publishable source/evidence tree |

Generated exposure JSON was subsequently normalized from CRLF to LF without
changing its parsed contents, and the artifact hashes refreshed; the ordinary
suite/build and offline checks were repeated after normalization. The initial
failing commands above describe the pre-fix state, not a green baseline.
No final required suite is reported as passing after a timeout.

Additional passing checks:

- `go test -race ./internal/cdp -count=1 -timeout=60s`.
- V8 cancellation/disposal/persistent-handle lifecycle regressions.
- `go test -race ./internal/browser -run 'TestRemovingFrameCancelsPendingDocumentTransport|TestV8IframeRemovalUnblocksParentLoadBeforeDisconnect|TestSameOriginWindowProxyForwardsRealmGlobalsAfterDetach' -count=1 -timeout=30s`.
- From third_party/tls-client: `go test . -run '^TestWinningHTTP3TransportIsTheCachedTransport$' -count=1` and its `-race` variant.
- `go build -o .build/mimic.exe ./cmd/mimic` and `python tools/smoke_local.py`:
  V8 default and explicit `-engine quickjs` both pass local CDP/Promise smoke.
- `python tools/generate_compat.py --check`: pinned metadata/hashes and byte-exact
  deterministic JS/Go projections from retained inputs; consecutive runs agree.
- `python tools/check_repository.py`: UTF-8/JSON/Python syntax, credential/path,
  local artifact and first-party runtime vendor-pattern checks pass.

The race detector instruments Go accesses, not all native V8/QuickJS internals.
The root suite excludes the nested third-party module. Its external upstream
integration tests, WPT, Chrome recapture and external E2E were not run.

## Reproducibility and security

See generated-data.md for exact pins and limits of offline verification. Raw
upstream IDL/CDP downloading was not repeated. Exposure captures are historical
inputs, with ephemeral context metadata; they are not byte-deterministic browser
recaptures. The gov8 binary and target V8 source provenance are distinct records.

The entire original tree was inventoried and text scanned without printing
credentials. Raw captures contained session/credential-like strings and local
paths; they were moved outside the repository. The publishable tree scan found
no credential patterns or machine-specific user paths. Cookie/header field names,
synthetic test values and public RSA SPKI keys are intentional. This is a heuristic
scan plus artifact review, not a guarantee against every possible secret encoding.
Private owner-controlled acceptance URLs were removed from public notes.

First-party production runtime has no conditions based on domains, Cloudflare,
Turnstile, BrowserScan, challenge patterns or vendor tokens. The retained upstream
TLS package includes an unselected CloudflareCustom profile in its general catalog;
Mimic selects Chrome_152_PSK explicitly. No vendor-driven runtime path was added.

.gitignore excludes local builds, traces/logs/profiles/PIDs, downloads/caches,
Python environments, editor state and secret/environment files. Intentional
captures and generated data remain visible. .gitattributes fixes textual artifact
line endings. No environment file or secret is needed, so .env.example is unnecessary.

## Remaining limitations

No full Chrome compatibility claim. Complete rendering, layout, media, cookie/CORS
algorithms, CDP object lifecycle and Streams backpressure/BYOB are still missing.
QuickJS's bounded marker checkpoint and goja's automatic draining differ from V8.
Persistent values remain rooted until realm closure; long-lived realm/trace growth
needs a separate ownership design. Direct embedded Page/DOM callers must honor the
single command-owner contract. Tests cover concrete lifecycle races, not a formal
proof of all possible callback interleavings. CDP is a trusted local control port,
not an authenticated remote service or hostile-code security sandbox.

Build/run commands and CDP usage are in README.md. Version-control readiness is
based on the final test table, artifact provenance and publishable-tree scan; the
private archive must stay outside any future Git root.

## Version-control readiness

Ready for Git initialization with the documented semantic/verification boundaries.
The publishable tree contains source, pinned generated data, reviewed evidence
and documentation; local verification outputs are ignored. Git was not initialized
and no remote was contacted. The private archive remains outside the repository.
