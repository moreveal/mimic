# Shared document preloads: Chrome 152 comparison

The original local audit recorded two physical `/ci` requests in all three
runs of `<link rel="preload" as="image">` followed by `new Image()` using the
same URL. The link and image paths independently called the network loader.

Document resource consumers now pass through `Realm.loadResource`. The document
holds a preload registry keyed by resolved URL without fragment, destination,
request mode and credentials mode. A compatible consumer takes the pending or
completed response before reaching the network loader. This sharing is separate
from HTTP caching and works with `Cache-Control: no-store`.

The common path covers images, classic scripts, stylesheets, Fetch and XHR,
including parser, document.write and child-frame script paths. Module fetches
also use it beneath their existing module map; the module map retains module
identity and evaluation semantics. Dynamic modulepreload links now use that
module map rather than an independent generic link request. Element request
construction consistently treats an empty crossorigin attribute as anonymous
CORS and preserves `use-credentials`.

Parser-created images previously skipped normal image loading. They now use the
same update/load path as inserted and detached images, including decoding,
events and document load blockers. Valid image bytes are decoded even when the
HTTP status is 404, as measured in Chrome. Invalid image data causes error events
for both the image preload and its consumer.

Preload ownership belongs to the document. Replacing an image stops only its
subscription; the link's request and event can finish. Page teardown cancels
outstanding work. `document.open()` cancels and clears the old document's preload
registry and suppresses obsolete link events. The registry mutex is per Realm
and is never held during network I/O or execution of JavaScript.

## Frozen comparison

Reference: headful Windows Chrome for Testing **152.0.7977.82**. The shared
JavaScript scenario is `internal/browser/testdata/image_preload.js`. Its SHA-256,
the browser binary hash and exact observed results are retained in
`internal/browser/testdata/resource_preload_chrome152.json`.

The loopback server holds the first response until the consumer is attached,
making the in-flight case explicit. All responses use `Cache-Control: no-store`.
There are 18 cases, repeated over three Chrome document cycles: **54 recorded
observations**. The Mimic regression uses the same JavaScript and compares
physical request counts, load/error events, image dimensions/completion and
Resource Timing initiator entries against the recorded Chrome results.

| Case | Chrome and Mimic physical requests |
| --- | --- |
| Image preload, in flight or completed | 1 |
| Matching anonymous image CORS | 1 |
| Image mode, credentials or destination mismatch | 2 |
| Script and stylesheet, in flight or completed | 1 |
| Fetch with matching CORS, in flight or completed | 1 |
| Fetch with preload mode mismatch | 2 |
| XHR with compatible fetch preload | 1 |
| Image HTTP 404 with valid SVG, in flight or completed | 1, both load |
| Invalid image bytes, in flight or completed | 1, both error |

Reproduce the reference from the repository root (Python requires `websockets`):

```powershell
python tools/image_preload_probe.py --chrome compatibility/.chrome-for-testing/152.0.7977.82/chrome-win64/chrome.exe --output .build/preload-new-capture
```

The output directory must be new. The probe opens its own fresh headful browser
profile on a local HTTP origin and closes its browser/server afterward. It does
not need certificate trust changes or QUIC flags. Compare new evidence before
updating the committed reference; do not adjust expectations to accommodate an
implementation failure.

Focused regressions:

```text
go test -race ./internal/browser -run 'TestDocumentPreloadConsumersChrome152|TestParserPreloadConsumers|TestPreloadCancellationBelongs|TestDynamicLinkLoadEvents|TestImage|TestDetachedImage|TestModulePreloads' -count=1
```

Additional tests check parser-discovered image/script/style consumers and
cancellation on image replacement, Page close and document.open. Resource
destinations without an implemented runtime consumer (such as font/audio/video
preloads) are outside this change. This is not an implementation of a general
browser memory cache, responsive-image selection or all preload link mutation
rules. Navigation and worker requests retain their separate ownership.

The documented dynamic-QPACK gap remains separate. These local request-count
measurements do not implicate it and this change does not modify HTTP/3.
