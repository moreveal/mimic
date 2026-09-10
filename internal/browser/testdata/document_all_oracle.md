# HTMLAllCollection reference

Measured on 2026-09-11 with frozen Chrome for Testing 152.0.7977.82,
headless, against a temporary loopback HTTP server. No external page or
challenge code is involved. JSON files retain Browser.getVersion metadata
and the Runtime.evaluate response; normalized observations are under
`result.result.value`. Exceptions are normalized to their class names.

`document_all_oracle.js` exercises operators, identity, descriptors, callable
and named/indexed access, supported names, live collections, and synchronous
same-origin frame access in both directions. Its document is detached and has
a deterministic element tree, so the surrounding harness page cannot affect
the recorded indices or names.

`document_all_navigation_oracle.js` additionally awaits same-origin iframe URL
navigation and checks the retained old Document and its live collection.
Its reference establishes that the old collection keeps its prototype and
special operator behavior while the new Document receives a different
collection and prototype.

The navigation server serves `<!doctype html><body><p id="newDocument"></p></body>`
at `/document-all-navigation`. An initial srcdoc version was also measured in
Chrome, but Mimic currently only reflects srcdoc without starting navigation;
that separate unsupported capability is outside this collection regression.
Both versions produce the same normalized Chrome observations.

The required `TestDocumentAllMatchesFrozenChrome` runs the synchronous corpus
with ordinary bootstrap and snapshot restoration. The separate
`TestDocumentAllRetainedRealmDiagnostic` is opt-in with
`MIMIC_TEST_RETAINED_REALMS=1`: Mimic currently destroys the previous realm on
navigation, making retained Document objects unavailable. This diagnostic runs
the unchanged Chrome navigation expectation and is expected to expose that
existing architecture limitation until realm retention is implemented. It does
not count as a passing navigation compatibility check when skipped.

Notable observations: `item` takes zero required arguments, missing arguments
return null, and extra arguments to the collection call or `item` are ignored.
Numeric-looking names such as `01` are not canonical indices. Duplicate names
return an identity-stable live HTMLCollection. Named own keys are
non-enumerable. A name colliding with an inherited method can appear in
Reflect.ownKeys while its own descriptor is absent and the inherited method
remains visible. Eligibility for matching the name attribute is measured
explicitly rather than assumed equal to id matching.
