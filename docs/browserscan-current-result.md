# BrowserScan as an external compatibility test — September 8, 2026

Page under test: https://www.browserscan.net/bot-detection

BrowserScan was used read-only through a normal Pyppeteer CDP connection. The runtime contains no URL checks, BrowserScan name checks, result substitution, or detector-specific branches. The reference for general semantics is pinned Chrome `152.0.7977.82`.

## Compatibility outcome

The page loads with HTTP 200, reaches `document.readyState=complete`, executes client modules, and displays the result. The final run has no JS exceptions or console errors. The trace separately lists missing surfaces that the page inspects as fingerprint data (`Navigator.*`, `setAttributeNS`, `requestIdleCallback`, and others); these did not stop the test and remain visible in the report.

The selected set of 20 differential tests matches Chrome: `20/20`. This is neither an exhaustive check of every API called nor proof of complete application execution. RSA-OAEP was captured separately before the fix (`NotSupportedError` in Mimic) and afterward (matching tested `CryptoKey` metadata and a 128-byte `ArrayBuffer`; this test did not verify decryption of the result).

## Site result

A separate observation in a regular Chrome session shows `Test Results: Normal` and populated individual results. This is a separate session, not a repeat measurement of the pinned CDP run below. The report collector previously did not recognize `Normal`; it now reads the result next to the `Test Results:` heading and does not mistake individual check statuses for the overall result. No new benchmark has been run with this updated collector yet.

The independent local regression `TestV8FetchJSONCompletesRenderedState` passes for `[]`, an array with data, and invalid JSON: the Fetch/Promise/DOM chain updates the UI in each case.

- Chrome 152: `Robot` in the automated control run; regular Chrome showed `Normal`.
- Mimic: `Robot`. The DOM contains `Test Results:Robot`; the final benchmark recorded HTTP 200, `readyState=complete`, and empty page-exception and console-error lists.

The `[]` response proved to be an acknowledgment of an error report, not the bot-detection result. The decrypted diagnostic payload contained `type: "vue_error"` and `Cannot read properties of null (reading 'ce')`. The cause was repeated evaluation of the same ES module through V8's static and dynamic paths: the application received two instances of Vue state. A shared module cache eliminated the error. The next interruption occurred because a deferred `import()` used the already canceled context of the original scheduler task; detection chunks failed with `context canceled`. Module loading now has the lifetime of the document realm. The missing `Document.createTextNode` was also implemented.

## General semantics fixed

- `script` types, static and dynamic ES modules;
- a shared identity/evaluation cache for static and dynamic V8 modules;
- deferred `import()` after the original browser task completes;
- UTF-8 `TextEncoder`, `Headers`, `Request`, `Response`, and basic `fetch` integration;
- `DOMMatrix`, `Text`, `Comment`, and `DocumentFragment` nodes and their Chrome-compatible properties;
- `Document.createTextNode`;
- `SVGSVGElement.createSVGRect`, `IntersectionObserver`, `History`, `HTMLLinkElement`, and `HTMLMetaElement`;
- load events for dynamic `link` and `modulepreload`;
- `Performance.timing`, `Performance.mark`, and `BroadcastChannel`;
- `SubtleCrypto.digest`, public SPKI RSA-OAEP import, and RSA-OAEP encryption for SHA-1/256/384/512;
- correct absence of intentionally absent `Document.namespaces` and lowercase `script.crossorigin` properties.

Each difference was first reproduced in a separate Chrome 152 ↔ Mimic test. Regressions for the corrected contracts were added to the browser package.

## Evidence retained after cleanup

Compact `.script-*` and `.browserscan-path-*` comparisons were moved to
`compatibility/captures/semantic-checkpoints/` without the leading dot in their names.
They preserve before/after results for modules, RSA-OAEP, DOM, and general semantics.
Raw BrowserScan results, network headers, payloads, and the saved HTML page
were moved to a local archive outside the repository because they may contain session data.
The conclusions above are historical observations and were not reverified during cleanup.
