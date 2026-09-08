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
