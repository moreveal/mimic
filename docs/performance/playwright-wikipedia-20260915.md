# Wikipedia Playwright runtime check — 2026-09-15

## Unchanged scenario

[Executable code](../../tools/runtimecheck/playwright_wikipedia.js), using
Playwright core 1.63.0 and installed Chrome 152: navigate to Wikipedia Main Page;
fill the search field with JavaScript and submit with Enter; wait for the article;
read its heading, first programming-language paragraph and body text; find the
exact ECMAScript link by accessible role/name; scroll, click and wait for the
second article; go back, verify the JavaScript heading and close the page.

The script ran in Chrome before implementation and again at the final checkpoint,
then unchanged through Mimic CDP. Only the existing launch/connect environment
switch differs. No locator, assertion, timeout, wait or action was adapted for
Mimic. The committed file is byte-identical to the original script, SHA-256:
`b962b0d45ad4a3cf093e2983b01b097851c3de56a0edd82d2f05a7c586062464`.

Install `playwright-core@1.63.0` in an external test directory and set `NODE_PATH`
to its `node_modules`, or run the script from that directory. Set
`PW_CHROME_EXECUTABLE` to Chrome's executable; for Mimic, unset it and set
`PW_MIMIC_ENDPOINT` to the server URL. Run `node playwright_wikipedia.js`.
`DEBUG=pw:protocol` adds detailed request/response timing. The scenario uses no
internal Mimic API.

## Recorded final execution

Fresh executable SHA-256:
`32088321825a663f0541c16656c5f10b2183ac1f0bf66b1017866a7ae461936f`.
The release fast gate independently built and checked this same hash. Source
was `0448cde` plus this change, not an old binary. These live-site workstation
observations are not controlled benchmark medians: network and background work
can affect them.

| Main step | Chrome elapsed / step ms | Mimic elapsed / step ms | Result |
| --- | ---: | ---: | --- |
| Main Page opened and read | 1019 / 1019 | 2013 / 2013 | HTTP 200, correct title |
| Search filled and read back | 1033 / 14 | 2655 / 642 | JavaScript |
| Search submitted, article ready | 1992 / 959 | 8136 / 5481 | JavaScript heading and URL |
| Article DOM read | 2008 / 16 | 8645 / 509 | Paragraph assertion passed |
| Role locator, scroll, link read | 2110 / 102 | 13670 / 5025 | ECMAScript link |
| Click and second article ready | 2469 / 359 | 19547 / 5877 | ECMAScript heading and URL |
| Back navigation | 2705 / 236 | 21250 / 1703 | JavaScript restored |
| Page closed / PASS | 2711 / 6 | 21270 / 20 | Both PASS |

Mimic's trace contains 122 completed CDP commands, none taking 10 seconds.
The slowest was `Input.dispatchMouseEvent`, 2466 ms; the next was the same method,
1716 ms. The slowest `Runtime.callFunctionOn` was 1622 ms. A Playwright step
includes multiple commands and navigation waits, so it is not a single CDP latency.

PASS means the script's assertions passed, **not complete Chrome equivalence**.
Main Page heading text was empty in Mimic at the attached-only read, versus
`Main Page` in Chrome; body text lengths were 63203 and 58453. An old-page request
reported `net::ERR_ABORTED` during Mimic's search navigation (central-login check).
An earlier successful run also recorded language/autocomplete request aborts.
The observations were not suppressed and the script was not
weakened. Exact text/DOM equivalence needs separate behavioral investigation.

Local detailed receipts: `.tmp/playwright-blackbox/chrome-final.log`,
`mimic-release.log`, `mimic-release-protocol.log`, and
`.tmp/performance-cmap-release/{build,raw}.json`. The protocol trace is not
committed because real browser traces can contain session data.

## Measured cause and general fixes

The original approximately 10-second input was not a transport timeout. Profiling
attributed about 9.16 seconds to 1228 text measurements, while native layout took
about 106 ms. Missing glyphs repeatedly decoded complete unsuitable fallback
fonts. Catalog scanning alone was not the main cause.

The catalog now retains immutable nominal glyph coverage and rejects impossible
candidates before decoding outlines and shaping tables. Full-face checks still
decide normalization, variation selectors and color glyphs. Candidate order is
preserved. The prior unverified extra default font ordering was removed; fallback
results use the exact cluster, not a Unicode block. A neighboring character's
coverage does not prove that it has the same preferred font. Regression tests
compare filtered shaping against the full-decoding path.

Style reads use one canonical epoch containing viewport and media preferences;
retained selector matches use its environment suffix. Foreign scalar style reads
do not build unused caller-side observations. `innerText` traverses in the owner
realm under one observation, avoiding per-descendant style crossings. Mutation
checks and canonical ownership remain in place.

This does **not** establish elimination of every runtime rebuild. Cold layout,
mutation invalidation, whole-document hit testing and repeated accessible-name
reads remain measurable. Warm role-locator profiling before the final bridge
change took about 1.8 seconds without new layout/shaping and made roughly 57000
epoch and 38000 media host calls. The changes reduce redundant bridge/cache work,
but the remaining end-to-end gap to Chrome is not closed.

## Validation

The local `go test ./... -count=1 -timeout=20m` run passed (browser 523.214 s,
CDP 33.347 s). It began before the final environment-suffix and inert-text
adjustments; those changes passed their targeted tests afterward. CI validates
the complete committed tree. Focused style, geometry, mutation, isolated-world
and observation tests passed.
New tests cover media-preference changes, isolated innerText mutations and
coverage-filter equivalence, including composed text and emoji presentation.
The complete fast gate passed all six correctness fixtures, static concurrency
at N=10 and N=25, and both teardown/memory scenarios. Warm completion medians:
DOM 125.82 ms, static 36.00 ms, React 79.28 ms. The frozen harness was unchanged.

An earlier gate stopped at its memory-pressure guard and is retained as a failed
attempt. Stale test servers were stopped before the successful repeat; the user's
separate port-9222 runtime was left running. This iterative gate does not replace
the full benchmark-matrix baseline.
