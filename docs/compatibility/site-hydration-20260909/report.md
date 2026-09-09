# Weather and additional site hydration

Reference: headful frozen Chrome 152.0.7977.82, measured on 2026-09-09.
These changes repair browser semantics; no production hostname, site selector,
application module ID, or benchmark exception was introduced.

## Failure chain

Weather's Next.js bootstrap expected `document.currentScript` during Promise
reactions at the classic-script cleanup checkpoint. Restoring it before that
checkpoint prevented the client from starting. The next stages exposed separate
compatibility defects:

- Domain cookies with a leading dot were invisible to the matching host. The
  location cookie was missing from the application's initial data request.
- Descendant resource `load` events reached Window. Readiness callbacks registered
  more listeners while the document was still interactive: the observed count
  exceeded 120,000. Chrome stops that event path at Document.
- Inline classic scripts incorrectly emitted resource load events.
- A sanitizer repeatedly removed `attributes[0]`, but `removeAttributeNode` was a
  no-op. More than 12 million calls were observed in one diagnostic run. Canonical
  Attr wrappers and a live NamedNodeMap now reflect actual removal.

Supporting workload fixes cover live class/tag collections, inert HTML documents
and ownership/adoption, streamed UTF-8 decoding, constructed/adopted stylesheets,
TreeWalker traversal, attribute source order, and event dispatch/click semantics.
MDN's Lit bindings required source attribute order as well as the stylesheet API.
Snapshots serialize adopted styles without mutating the live tree.

## Verification

Focused offline regressions accompany each semantic repair. Small independent
fixtures were evaluated in frozen Chrome, including currentScript cleanup,
cookies, collections, inert documents, Attr removal, event paths, constructed
stylesheets, TreeWalker and TextDecoderStream chunk/error behavior.

The live-site captures are diagnostic evidence, not frozen benchmarks. Dynamic
weather observations, advertisements, and time-dependent text are not exact-output
fixtures. Raw captures and screenshots are kept under
`.build/site-compat-20260909/`; public, compact differential results are alongside
this report. API keys, cookies, full third-party responses and personalized
advertising are deliberately excluded from committed evidence.

## Scope

These results concern hydration and exported article/forecast rendering. They do
not claim complete browser rendering, media playback, visual editing, or universal
Web API support. Wikipedia's article and navigation run; its visual editor still
advertises the existing editing/selection capability boundary. Constructed CSSOM
supports the measured stylesheet operations; the runtime remains a lightweight
execution environment, not a Chromium renderer. TextDecoderStream inherits the
existing UTF-8 decoder's explicit encoding boundary.

## Site outcomes

The ordinary `tools/mimic_snapshot.py` path (wait for load, then a two-second
settle) completed for all four sites on the integrated build:

| Site | Before | Verified result |
|---|---|---|
| weather.com | Placeholder forecast; client bootstrap/data request failure | Zero forecast placeholders; current, hourly and daily weather data populated; exported forecast visible |
| react.dev/learn | Client exception; then export timeout from blank child frames | Quick Start article restored; ordinary export completes |
| MDN JavaScript | Lit stylesheet/attribute binding failures | Custom elements hydrate; shadow trees and adopted CSS survive export |
| Wikipedia JavaScript | jQuery/MediaWiki startup failures | Article and navigation initialize; menu click works; exported icons and selected appearance controls preserved |

Initial blank iframe completion now uses the canonical frame lifecycle, including
synchronous owner load where measured in Chrome. Unsuccessful HTTP script responses
raise resource errors instead of executing their bodies; Fetch keeps its original
status/body semantics. Snapshot repairs also retain SVG data-URL quoting and choose
portable image/font filename extensions from response media types.

Final validation: `go test ./...`; race tests for network, DOM and scheduler;
`tools/generate_compat.py --check`; focused snapshot tests after the final MIME fix;
currentScript differential against frozen Chrome. The currentScript comparison
checks identities at the relevant checkpoints, not equality of unrelated timer
interleavings. No frozen harness or reference expectation changed.

Snapshots intentionally omit embedded frames and executable content. Weather's
third-party advertising resources can return HTTP 403; those warnings are retained,
not hidden or bypassed. Forecast data requests succeeded. Dynamic advertisements,
carousel scroll position and renderer-dependent animation/layout are not claimed
as pixel-identical. See `site-results.json` for the export receipt and counts.
