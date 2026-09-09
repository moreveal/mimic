# Fetch Body and Streams: domain validation

2026-09-09, integration binary iteration 8, CDP 19432. Reference: pinned Chrome 152.0.7977.82, CDP 19423. The replacement is independent of GitHub. Source changes after iteration 8: one onabort call instead of two; Go regression tests pass with this fix.

Handwritten Streams were replaced with pinned web-streams-polyfill 4.3.0 (MIT); bundle/source integrity/license are in internal/webapi/vendor/web-streams-polyfill. The Fetch Body layer uses its nullable byte streams, disturbed/locked state, tee cloning, and a shared consumer for text/json/arrayBuffer/bytes/blob. Request body transfer/clone and dependent AbortSignal are implemented; canonical Go transport receives bytes and per-fetch cancellation. Body/Request/Response do not create a second DOM or introduce a separate event loop.

Generic differential with 14 cases: the baseline had 13 differences; iteration 8 matches 13/14. The last difference, duplicate onabort calls, was fixed in source after the build and covered by a Go test. Another 6 probes covered Request transfer/clone, read+release disturbance, unsigned-short status conversion, Response statics, and DataView offsets. An independent URL-serialization difference remains for the redirect trailing slash; no local workaround was added here.

Go `TestFetchCanonicalBytesCloneAndCancellation` and `TestFetchBodyDisturbanceAndRequestTransfer` pass. They check byte-preserving POST/response cloning, repeated consumption, actual server Request.Context cancellation, exact AbortSignal.reason, body-ownership transfer, and a single onabort call. These are canonical transport integration tests, not mocks.

## WPT

Tests and required META helpers were downloaded unchanged from the revision in wpt-manifest.json. The runner creates an ordinary HTML page with upstream testharness.js, scripts, and resource files. Scripts run in Window. Worker/HTTPS variants were not run here. An initial experiment injecting the harness into an already loaded about:blank produced a harness timeout and was excluded from the final evidence. The frozen benchmark harness was unchanged.

Expanded subset, 13 files: Response initialization/static error, disturbed states 1–6/pipe, cancel, bad chunks, and propagation of underlying stream errors. Chrome registered 113 cases: 99 PASS, 14 FAIL. Mimic registered 106: 103 PASS, 3 FAIL; 7 cases were not registered because of a top-level FormData constructor failure in response-init-002. The correct denominator is therefore 113 expected cases, not 106.

Mimic's 3 failures: Response.formData is absent in body-consumer tests. The additional consume-empty/consume-stream/request-body-override subset exposes the same domain limit and registration interruption at FormData; see the separate JSON. Its partial list must not be presented as a complete pass.

Chrome 152 itself failed 14 current upstream assertions: 2 synchronous bodyUsed checks after pipeTo/pipeThrough and 12 exact-identity checks for custom underlying-stream errors. Mimic/the mature library passes these assertions. This is a difference between the frozen browser and current WPT, not grounds to claim Chrome passes everything or rewrite the library for a site.

## Explicit boundaries

- FormData construction/multipart extraction and formData consumption remain unfinished; multipart BodyInit currently throws an explicit NotSupportedError if a FormData object exists.
- The network loader still buffers the entire response. The API body has real stream semantics, but fetch does not yet return at headers before the entire body loads. This is not complete incremental network streaming.
- no-cors filtering, CORS/opaque policy, request-header guards, referrer validation, and redirects require a separate full domain corpus. The presence of Request options does not guarantee the entire policy layer.
- The internal adapter reads the pinned polyfill's `_disturbed` field in one place; this is an intentional version dependency that must be checked on updates. The vendor bundle is unmodified.
- URL/encoding use existing subsystem implementations; the Streams library does not fix them.
- Retention/cancellation with an external reader and cloning need broader memory cases; these semantic tests do not support benchmark/performance conclusions.

Original runners/probes are in .build/fetch-domain; JSON evidence is stored beside this report. Final SHA receipts/fast-gate results refer to the combined integration build.

Final review: an independent Chrome network-failure probe returned TypeError; Mimic before correction returned a raw transport string. The host.fetch wrapper now converts only transport rejection to TypeError, preserving abort reason and custom body-stream error identity. TestFetchNetworkFailureIsTypeErrorAndBodyErrorIdentity and TestFetchCanonicalBytesCloneAndCancellation PASS after correction. Frozen/performance workloads were unchanged.
