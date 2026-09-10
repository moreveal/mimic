# Compat-fuzz root-cause fixes, block 1

Worktree: `E:/GitHub/mimic-compat-fuzz`, branch `codex/compat-fuzz`.
Starting runtime: `3faacc6`; starting harness: `a98b2d2`.
Frozen oracle: Chrome 152.0.7977.82, headful, separate fresh session/profile.
The original checkout, harness, captures and reference expectations were not edited.

## Completed semantic block: collection and HTML document identity

Two shared causes account for six original groups:

1. `htmlCollection` lacked indexed/named reflection and recreated live wrappers for
   each query. Added owner/query-keyed weak caching and live own-key/descriptor
   projections over the existing canonical DOM. Cache keys preserve the original
   argument: equivalent tag case/class whitespace does not imply equal query
   identity in Chrome. Named lookup follows HTML name attributes and element IDs;
   no site/probe names are embedded in implementation.
2. HTML documents were instantiated directly as `Document`. Added the canonical
   empty `HTMLDocument` prototype layer, inheritance and constructor semantics for
   live, inert and parsed HTML documents. XML documents retain XMLDocument identity.
   This fixes tag/constructor observations and all affected descriptor depths
   together, including `document.all`.

| Original group | Root cause | Status |
| --- | --- | --- |
| `261de7a21e7145d7` | HTMLCollection indexed/named reflection | fixed |
| `4055936baf635d06` | live-query wrapper identity | fixed |
| `040b06ea0edd0d8b` | absent HTMLDocument prototype layer, inherited constants | fixed |
| `16623006f5e14633` | HTMLDocument constructor identity | fixed |
| `8279f9ac1a699ee3` | HTMLDocument brand inside iframe | fixed |
| `d4c44ff9b4314406` | HTMLDocument layer above Document.all descriptor | fixed |

All six representative probes now match in full, not merely at their original
first differing leaf. The same change fixes 12 complete probes in the original
236-case set: **120 -> 132 matches**, **28 -> 22 remaining original groups**.
Comparison of complete normalized observations found **0 regressions of previously
matching probes, 0 new differing observation paths, and 0 Chrome oracle drift**.
The baseline probe list was held fixed while validating, so discovery changes
cannot silently remove a previously passing case. All 28 original minimized
representatives were re-evaluated; their temporary fixture origins were consistently
rebound to the same validation origin on both engines.

Details: [regression comparison](block1-regression.json).
The unchanged official harness was also run, regenerating/minimizing the remaining
differences; its summary is retained alongside this report.

## Stop boundary: navigation ownership, not a cache invalidation patch

`6dad37e5c7073bd2` cannot be correctly fixed by clearing `remoteDocumentCache`.
The current bridge caches Document by frame ID (`surface.js`, `remoteDocument`),
resolves every Document operation against `frame.Realm`
(`frame_document_bridge.go`, frameDocumentGet/Set), and closes the old realm on
child-document commit (`frame.go`, commitChildFrameNavigation).
A saved old Document would therefore still retarget the new document or refer to
an already closed realm after a superficial identity fix.

A supplemental Chrome/Mimic measurement confirms this ownership problem:
Chrome keeps an expando on the old Document (`23`) and a property of an old-realm
object (`17`) readable after navigation; Mimic returns undefined for the former
and throws for the latter, while both retain WindowProxy identity. See the
[probe](navigation-retained-references.js) and [results](navigation-retained-references.json).
These are additional observations of the existing navigation root cause, outside
the fixed 236-case regression comparison.

A general fix must distinguish stable browsing-context/WindowProxy identity from
per-document realm identity, retain old realm objects while cross-realm references
exist, route handles by their owning realm rather than the current frame, and
separate inactive-document task cancellation from object-lifetime teardown.
This spans navigation, cross-realm dispatch and Page teardown. Per the requested
stop condition, that architecture change was not started and no misleading partial
WindowProxy fix was installed.

The following **22 groups remain open**. Except where explicitly marked, they
have plausible bounded fixes; they are deferred at the architecture stopping point,
not misrepresented as impossible or fixed.

| Original groups | Common cause / required semantic work |
| --- | --- |
| `6dad37e5c7073bd2` | **Architecture boundary:** old-Document/realm ownership across navigation, described above. |
| `c035c36945679951` | frameElement builds an independent wrapper instead of routing a canonical owner-realm node reference. |
| `fb77ad41b9b7b4e0` | host cross-origin rejection is emitted as generic Go error; use typed security failures projected as caller-realm DOMException, without parsing arbitrary user errors. |
| `931e39fff24f9387` | missing live WindowProperties intermediate prototype, named lookup/descriptors and shadowing. Preserve global identity, do not wrap globalThis in another Proxy. |
| `21a81b154f26e799`, `c5a808c0f491bb2b`, `a6c93c3a079303c5` | native binding finalization precedes later replacements; handwritten/internal parameters leak into public callable source/length. Finalize installed platform bindings after semantic modules; use actual WebIDL signatures. |
| `7ce0490158dc72b8` | generated constructors link instance-prototype inheritance but omit interface-object inheritance. |
| `4c01ec91c8369240`, `4d8f2d7aa415e51f`, `76be3fd8e5525b76`, `90dd2730882e2b50` | selector/Performance bindings omit receiver, required-argument and DOMString gates or apply them in wrong order. Validate receiver, then arity, then conversion before the operation. |
| `babc848f0fbe86db` | hasFocus is a generated unsupported stub; requires a focused/active document observation, not unconditional true. |
| `56e9263979f49328`, `98f341941429c165`, `d7352700b9d1b6e4`, `f5bf6be29751c1f4` | incomplete Location LegacyUnforgeable installation, readonly own methods and ancestorOrigins derived from frame ancestry. |
| `2e3fa5c55a4a766c`, `91b505855c3b52b5`, `a99b7b19d5053e5d`, `a9a67828d18d5d5b` | prototype insertion order follows JS classes/installation modules instead of Blink binding order. Preserve descriptors while installing from measured interface-order metadata. Current generated alphabetical member data is insufficient. |
| `2f13fa24fe6af546` | Window inventory/order mixes binding order with missing/extra capabilities. Ordering is bounded; publishing placeholder APIs merely to equalize keys is prohibited. Required unsupported capabilities need a separate scoped implementation decision. |

Additional collection boundary measured while fixing reflection: Blink can expose
duplicate numeric own keys when a named ID equals a supported index. ECMAScript
Proxy invariants forbid that result, so exact parity needs a native legacy-object
interceptor. It is documented beside the implementation; it was not hidden by
weakening a probe or a reference expectation, and is outside the original 28 groups.

## Validation and diagnostics

- Focused collection, HTMLDocument, inert/XML, document.all and bootstrap-snapshot
  tests passed; focused identity race tests passed.
- Full `go test ./internal/browser ./internal/webapi -count=1`: **PASS** (browser 369.877s, webapi 0.237s).
- Full `go test -race ./internal/browser ./internal/webapi -count=1`: **NOT PASSING**. `TestPartitionCookiesFrameAndWorkerTransport/goja` exceeded its evaluation deadline (13.45s), then the browser suite hit the default 10-minute overall limit while starting frame-reflection tests. The log contains no `WARNING: DATA RACE`; this does not establish a clean full race run. Webapi passed (2.211s). See [race log](race-tests.txt).
- Isolated rerun including cookie/frame reflection and affected tests: the same cookie deadline failed; no other selected test failed.
- Baseline check: exported untouched `3faacc6` with `git archive`, then ran only `TestPartitionCookiesFrameAndWorkerTransport` with `-race` from that separate source tree. **The same goja deadline failure reproduced** (13.966s total). See [baseline cookie log](baseline-cookie-race.txt). This failure predates these changes.
- Affected collection/document/frame-reflection race tests alone: **PASS** (24.354s). See [affected race log](affected-race-tests.txt).
- Original 236-case observation comparison: completed, results above.
- Original 28 representative probes: completed, 6 full matches / 22 still different.
- Official compat-fuzz corpus: run with the same 100-surface budget, default V8
  and default snapshot policy: **236 checks, 132 matches, 104 divergent probes, 22 unique groups, 0 unstable, 0 errors**. No runtime flags were changed to make it pass.

The unchanged baseline binary crashed once inside native V8
`SnapshotCreator.CreateBlob`; a fresh retry completed all 236 baseline observations.
This is recorded as a pre-existing intermittent runtime failure, not a semantic
finding or a regression introduced by this patch. The diagnostic excerpt is in
[baseline-snapshot-crash.txt](baseline-snapshot-crash.txt).
