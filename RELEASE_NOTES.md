# Mimic v0.1.8

Changes since v0.1.7:

## Browser compatibility

- Added Page-local geolocation provider overrides through `Emulation.setGeolocationOverride` / `clearGeolocationOverride` and the deprecated Page aliases. Implemented position and coordinate objects, optional altitude/heading/speed, `toJSON`, cache and zero-timeout behavior, live watches, cancellation, navigation persistence and Page isolation. Added geolocation permission grants/settings/reset through CDP and enforced secure origins and Permissions Policy. No operating-system GPS provider is connected. See the [geolocation guide](https://github.com/moreveal/mimic/blob/v0.1.8/docs/compatibility/geolocation.md).
- Implemented legacy MouseEvent/UIEvent creation and initialization, including aliases, parameter conversion, cancellation reset and dispatch protection.
- Corrected frozen Chrome 152 platform member ordering, textarea-only `textLength`, and `chrome.csi()` document/lifecycle timing.
- Fixed internal lazy Web API loading under strict CSP and Trusted Types without granting author scripts additional eval privileges. WebGL, WebGPU and audio observations can initialize under those policies.
- Improved Canvas full-turn arcs, shadows and image blend operations, and preserved CSS fallback for `:has()` and style batches. Improved computed-style enumeration, shadow layout admission and related browser observations.
- Combined all Accept-CH response field lines. Fulfilled responses now update canonical cookies and Client Hints and expose actual fulfillment latency without fabricated transport timings.
- Validated restored frame bridge bindings before installation so malformed bootstrap snapshots fail cleanly and retry ordinary initialization.

## Profiles, isolation and performance

- Added coherent portable BrowserContext profiles, shared validation for generated/imported profiles, and broader GPU/font recipes. Added context-isolation coverage and examples for 100 isolated cookie/profile tasks. See [environment profiles](https://github.com/moreveal/mimic/blob/v0.1.8/docs/environment-profiles.md).
- Bounded bootstrap memory and reduced retained state; packed DOM node storage without changing storage semantics.
- Removed a measured cross-realm DOM parent getter round trip by invoking the captured getter in its owner realm. Recorded Flickr latency measurements and remaining limitations.

## Compatibility checkpoints

- Flickr Turnstile produced nonempty tokens in a diagnostic run and two clean live runs after the lazy-loading/CSP fix. The milestone tag is [`flickr-turnstile-pass-20260926`](https://github.com/moreveal/mimic/tree/flickr-turnstile-pass-20260926). See the [preserved comparison report](https://github.com/moreveal/mimic/blob/v0.1.8/docs/compatibility/flickr-turnstile-prepack-2026-09-26.md).
- Google returned search HTML with nine result headings after the legacy event fix. Ten later, distinct searches succeeded sequentially in one Mimic Page/context while changing geolocation overrides; all ten JS coordinate checks matched. The milestone tags are [`google-serp-pass-20260926`](https://github.com/moreveal/mimic/tree/google-serp-pass-20260926) and [`google-serp-geolocation-10-pass-20260926`](https://github.com/moreveal/mimic/tree/google-serp-geolocation-10-pass-20260926). These are observed checkpoints, not guarantees of future server acceptance or evidence that Google uses the coordinates for ranking.
- Preserved Chrome captures and used offline replay to localize execution differences. Documented controlled Chrome launch conditions and excluded instrumentation and extension-surface confounders. Private cookies, tokens, challenge bodies and credentials are not distributed.

## Developer workflow

- Require focused local tests, with broad regression coverage in GitHub CI. Preserve successful Chrome 152 captures and reuse them rather than repeatedly launching the same live case.
- Keep the reviewed CDP semantic-support registry synchronized with generated runtime/documentation projections and focused test evidence. Corrected the fulfillment timing fixture to use the loader's elapsed-time clock.

## Build provenance

The Windows and Linux executables are reused unchanged from the successful [CI run 36223794041](https://github.com/moreveal/mimic/actions/runs/36223794041), source revision `6012b628ae6f3d643a0a28840ceb7f3b829d277e`. The release tag adds release notes only; the manifest records binary source, packaging revision, checks and checksums separately. The binary's development-version banner identifies its actual source revision.

Mimic remains a renderer-free public beta for Windows and Linux amd64. See the [compatibility boundaries](https://github.com/moreveal/mimic/blob/v0.1.8/docs/compatibility.md). Full source changes: [v0.1.7...v0.1.8](https://github.com/moreveal/mimic/compare/v0.1.7...v0.1.8).
