# Compatibility status

Active target: Chrome 152.0.7977.82 Stable, Windows x64, Chromium commit
`d04cdb24d67b081f6cf80200ffc5233f44b61109` (main-branch position r1669021).
The exact Blink trees, V8/CDP revisions, WPT tree and differential-browser
expectation are locked together in `chrome/152/target.json`. This does not
claim complete semantic conformance to Chrome 152.
The differential expectation names a headful fresh controlled profile as the
primary oracle. Headless and historical captures are mode-scoped evidence under
the [oracle policy](oracle-policy.md), not generic version semantics.

Implemented semantics are intentionally narrow: basic documents, selectors,
events, timers, navigation state, same-page history, storage, cookies, fetch,
basic async XHR, scripts, coherent navigator/screen/viewport values, console,
secure random values/UUIDs, base64 conversion, a partial enforced CSP model,
and a Pyppeteer-verified CDP subset (`Target`, `Runtime`, `Page`, `Network`,
`Fetch`, `DOM`, `Storage`, `Log`, `Performance`). Unknown browser-global
accesses are traced. See `cdp-compatibility.md` for verified behavior and gaps.

The known Blink WebIDL and CDP surfaces are generated into the versioned bundle;
many operations intentionally report missing semantics. CSS layout, modules,
WebGL, Canvas pixels, media and shadow DOM remain incomplete. They must be added
as projections of canonical state and shared services, never per-test values.

V8 accepts realm-owned `TrustedScript` values in native `eval`, including direct
eval's lexical scope, indirect global declarations and original exceptions.
The browser supplies a resolver over its private trusted-value slots through
`engine.EvalSourceRuntime`; it does not replace `eval` or coerce arbitrary
objects. The same hook is installed in Window and Worker runtimes. QuickJS and
goja retain their native non-string eval behavior: TrustedScript execution there
remains unsupported. Full Trusted Types CSP enforcement and cross-realm branded
object transfer are not supplied by this hook.

Main-document HTTP redirects commit the final response URL before establishing
the realm origin, history and relative resource URLs. Console groups emit the
ordered `startGroup`, `startGroupCollapsed` and `endGroup` trace/CDP events using
the existing console argument projection. These behaviors have focused local
regressions and a [Chrome 152 headful capture](../compatibility/captures/semantic-checkpoints/trusted-eval-console-redirect-chrome152.json).

Dedicated workers share the Window Fetch implementation and canonical network
loader, with worker-owned promises, network tasks and cancellation. Request and
Response bodies, cloning, headers, Blob, URL, encoding and Abort primitives use
the same JavaScript sources in both realms. Worker termination, `close()` and
Page teardown cancel and join outstanding requests before disposing the runtime.
Redirected worker scripts commit their final URL; blob workers retain an opaque
URL base, so relative requests reject instead of inheriting the creator's base.
See the [worker Fetch capture](../compatibility/captures/semantic-checkpoints/worker-fetch-chrome152.json).

The shared loader follows only 301/302/303/307/308, limits a chain to 20 redirects,
rewrites methods and request bodies as appropriate, and rebuilds destination
cookies and request metadata. Cross-origin redirects remove Authorization.
Fetch exposes the final response URL and `redirected`, supports `redirect:error`,
and returns a filtered `opaqueredirect` for manual redirects. Empty/missing
Location and credential-bearing redirect URLs have separate
[Chrome 152 evidence](../compatibility/captures/semantic-checkpoints/worker-fetch-redirect-edges-chrome152.json).
Response-body cancellation produces its own AbortError; cancellation before the
Fetch promise resolves preserves the supplied signal reason, as measured in the
[body cancellation capture](../compatibility/captures/semantic-checkpoints/worker-fetch-body-abort-chrome152.json).

Fetch remains bounded in both realms: the transport buffers the complete body
before resolving, so headers-first responses and incremental network body reads
are not implemented. CORS response filtering/enforcement and the credentials,
cache and referrer options are incomplete; accepting those options in Request
does not imply transport conformance. Worker Resource Timing and nested workers
also remain incomplete.

Document streaming uses a persistent HTML5 tree builder over canonical DOM node
handles. `document.open()` retains the Document, Window and application globals,
clears listeners in the old connected tree, and starts a new input stream.
`write()` and `writeln()` accept split markup and execute completed inline
scripts synchronously. Reentrant writes insert before the unparsed suffix;
ordinary external scripts pause parsing while their network requests run, and
later writes queue behind that suffix. `close()` completes the existing stream
and its document lifecycle. Network-document parsing uses the same insertion
state, so writes from parser scripts do not replace the document. See the
[identity and parser capture](../compatibility/captures/semantic-checkpoints/document-stream-chrome152.json)
and [dispatch, entry-document and external-script capture](../compatibility/captures/semantic-checkpoints/document-stream-dispatch-chrome152.json).

The adapted tree builder and its upstream license are in `internal/htmlstream`;
there is no secondary DOM tree to synchronize after script mutations. Stream
teardown aborts its tokenizer and outstanding stream-owned script requests.
Full async/defer/module scheduling for explicitly written documents remains
incomplete, as do the existing navigation CSP meta-policy discovery rules.
The three-argument `document.open` Window overload and inert-document streams
are explicit unsupported boundaries.

Awaited evaluations wait for work across all active frame queues, including
child fetch completions, with cancellation and no polling waiter. The existing
per-realm queues remain; this change does not replace them with a new shared
scheduler. V8 supports nested script evaluation and synchronous cross-actor
callbacks while keeping each isolate on its owning thread. Microtasks stay
deferred until the outer JavaScript stack unwinds.

Same-origin cross-realm object proxies forward property keys and descriptors,
including canonical accessor identity and symbol keys. Nonconfigurable
descriptors are mirrored only as required by ECMAScript proxy invariants; the
source realm remains authoritative. See the
[reflection capture](../compatibility/captures/semantic-checkpoints/cross-realm-reflection-chrome152.json).
V8 supports nonconfigurable accessor descriptors. The pinned Goja dependency
rejects valid Proxy accessor descriptors even in a local minimal reproduction;
a capability probe makes that specific reflection case an explicit
NotSupportedError on affected engines. It does not change the reported
configurability or substitute a copied getter.
`defineProperty`, `deleteProperty`, `setPrototypeOf` and `preventExtensions`
explicitly throw NotSupportedError rather than modifying an unobservable proxy
shell. Specialized Window/Document facades retain their existing narrower
reflection support.

Cross-realm construction invokes captured `Reflect.construct` in the source
realm. It preserves the source prototype, `new.target`, returned object identity
and thrown values, including original Error identity. Proxy targets preserve
array branding and function constructability without executing the candidate
constructor. Primitive arguments and same-origin object/function references are
supported. A caller-local or other realm's `newTarget` remains an explicit
unsupported boundary. See the
[construction capture](../compatibility/captures/semantic-checkpoints/cross-realm-construct-chrome152.json).

Synchronous calls, property assignment and construction pass same-origin
objects by reference. Imported proxies retain the original object's provenance;
returning one to its source realm resolves the original value, including cyclic
graphs, callbacks and constructor results. Getters execute in the owning realm
only when the operation actually reads them. Reference import validates frame,
realm and origin rather than exporting object contents. The
[constructor argument capture](../compatibility/captures/semantic-checkpoints/cross-realm-constructor-arguments-chrome152.json)
covers `Set` membership and callback receiver identity. Handles to a replaced
realm remain explicitly unavailable after navigation; retaining detached old
realms as Chrome does is a separate lifecycle limitation. Asynchronous
`postMessage` retains its existing structured-clone boundary.

The 2026-09-09 live checkpoint in `.build/voxel-worker-reference-indexed/`
ran for 60 seconds on a fresh binary. The earlier Worker fetch, cross-realm
construction/reference errors and DOM-task timeout did not recur; the challenge
continued to further network requests. The top document nevertheless remained
HTTP 403 and did not navigate to the shop. This is not evidence of challenge
completion. The remaining unsupported API observations are diagnostic leads,
not established causes; identifying the next blocker requires a corresponding
Chrome comparison rather than adding every API encountered during enumeration.

Follow-up inspection of caught console exceptions found a reproducible
`undefined.call` failure that an error-kind-only trace filter missed. Replaying
the same recorded response bodies in native Chrome 152 and Mimic isolates a
function-versus-string argument divergence before a `charCodeAt` invocation.
Native pause-on-all-exceptions confirms it does not throw the matching error;
this is not simply different console reporting. The browser operation responsible
for the upstream value divergence is still under investigation. Function-source,
console-coercion and frame-eval corrections are independently measured fixes,
not evidence that this challenge now completes.

A successful native capture of `iroshop.tech/mimic-e2e` is retained locally under
`compatibility/private-captures/iroshop-2026-09-09-chrome152`: initial challenge
403, then `cf_clearance` and an application response without the challenge header.
The application itself returned Next.js 404. All 98 parsed script sources and
31 retrieved response bodies were saved without retrieval errors; global browser
Tracing was intentionally omitted because the existing browser had unrelated
tabs. The separate `voxel-replay-2026-09-09` private fixture preserves the failing
program and offline replay helpers. These token-bearing artifacts are gitignored;
the reusable capture tool and its limitations are documented in
[`tools/compatibility`](../tools/compatibility/README.md).

Classic V8 scripts retain their supplied resource names. Callback diagnostics
include engine-owned source coordinates without reading an application's
`stack` getter or invoking `Error.prepareStackTrace`.

Navigation timing is document-scoped (URL, loader, time origin and finalized
duration), including same-document history changes. Each frame finalizes its
navigation observers after its own load handlers finish. These behaviors have
a [Chrome 152 headful capture](../compatibility/captures/semantic-checkpoints/frame-navigation-timing-chrome152.json).
Redirect counters and detailed legacy `performance.timing` fields remain
incomplete.

Same-document session history is joint across the frame tree: child
push/replace operations update that document's URL and state, while length and
traversal refer to the Page's joint entries. Relative resource URLs use the
calling document's updated URL. Cross-origin URL rewrites throw `SecurityError`
before mutation. Child Location navigation stays in that frame; replace and
reload preserve the current entry. These behaviors have
[Chrome 152 headful evidence](../compatibility/captures/semantic-checkpoints/frame-history-chrome152.json)
and Goja/V8 regressions.

History support is still bounded: cross-document restoration/BFCache is not
implemented and emits `history.crossDocumentTraversal` instead of relabeling a
live document. State retains realm-owned values and does not yet perform
structured serialization. Traversal events currently use the generic Event
implementation rather than complete PopStateEvent/HashChangeEvent semantics;
ordering when several documents change in one traversal remains incomplete.
Initial-load entry replacement, detached-frame history pruning, empty trailing
fragment serialization, and the existing remote WindowProxy Location facade
remain incomplete.

`Function.prototype.toString` preserves the engine's original source for
unmarked functions; only explicitly marked platform functions receive the
Chrome native-function text. It does not normalize user source containing
`[native code]` or read a callable's `name` to synthesize source. The
[Chrome 152 callable-source capture](../compatibility/captures/semantic-checkpoints/function-source-chrome152.json)
also covers bound functions, callable/revoked proxies and throwing name getters.
V8 passes that complete fixture. Goja and QuickJS still have an independent
intrinsic limitation: their own function stringification can read a bound
function's `name` getter. The overlay does not conceal that backend discrepancy.

Console formatting converts only arguments consumed by `%s`, `%d`, `%i` and
`%f`, preserving conversion exceptions. `%o`, `%O` and `%c` consume an argument
without converting it; `%%` and unknown specifiers consume none. Plain object
logging does not call application getters or conversion hooks. Trace output
retains its string-argument schema and uses `[object]` / `[function]` diagnostic
placeholders; interactive object inspection and CDP object handles are not
implemented by this formatter. These semantics are covered by the
[Chrome console matrix](../compatibility/captures/semantic-checkpoints/console-format-chrome152.json)
and its [argument-consumption cases](../compatibility/captures/semantic-checkpoints/console-format-edges-chrome152.json).

Same-origin frame `eval` preserves non-string argument identity, including
functions, boxed strings and objects with throwing conversion hooks. Locally
branded TrustedScript values use their internal source and evaluate in the
target frame without invoking public `toString`. The bridge retains its origin
validation even when there is no source to execute. See the
[Chrome 152 eval-argument capture](../compatibility/captures/semantic-checkpoints/frame-eval-arguments-chrome152.json).
