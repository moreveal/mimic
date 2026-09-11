# Consumed Navigation service — 2026-09-12

Navigation is a projection of the existing joint session history. Frame entries
own the stable key, version ID, private serialized Navigation state, and History
storage state. The JavaScript layer owns realm-local wrappers, handlers and
promise reactions; it does not maintain another history database.

Implemented paths: entries/currentEntry/activation, push and replacement,
updateCurrentEntry, navigate, back/forward/traverseTo, reload, cancellation,
interception handlers, committed/finished promises, transition completion,
currententrychange and disposal. History and Location mutations share the same
notifications. Cross-document traversal uses the existing loader, parser and
realm retirement path, with its target index carried by the navigation request.
It does not relabel the active document or emulate a BFCache.

Keys survive replacement; IDs change on replacement and survive reload and
traversal. Disabled initial about:blank documents expose no Navigation entries.
Inactive saved entry wrappers expose the measured disabled values. Each realm
keeps its own wrappers, and snapshot restoration reinstalls private callbacks.

The portable storage graph retains cycles, aliases, undefined, nonfinite numbers,
negative zero, BigInt, Date, RegExp, Map/Set, buffers/views and native error data.
Values are first passed through the existing structured-clone validator. Foreign
realm values are cloned by their actual owner using the existing nested bridge;
this also fixes History's rejection of ordinary cross-realm objects. Unsupported
History storage brands keep the existing explicit clone errors.

Frozen local Chrome 152.0.7977.82 controls establish that cross-document navigate
with a state option does not publish that state in the destination's getState(),
and its old-document result promises remain pending. Same-document and intercepted
state do persist. The implementation follows those observations. Native reload
preserves both the entry key and ID.

Evidence (private, local-only) is in `.build/navigation`: oracle-02 basic,
oracle-04 interception/traversal/cancellation, oracle-09 cross-document, and
oracle-final cross-document plus reload. Final cross-document/reload comparison
has zero control differences and zero Chrome/Mimic differences. No target-site
or external runs were used. Basic and interception oracle fixtures are checked
in as regression tests. Additional tests cover foreign realm state cloning,
cyclic storage, real cross-document traversal and reload identity.

During implementation an initial cross-realm owner-call attempt deadlocked and
hit the 45-second test timeout. Argument decoding was moved before the owner
switch and the existing reentrant crossFrameData dispatcher was reused. Later
focused Navigation, History and snapshot checks passed; no timeout was waived.
The first Goja oracle test compared exported undefined with JSON omission; the
harness now compares JSON serialization on both engines, without changing the
frozen expected observations.
