# Compatibility fuzz merge validation

The merge combines main `40a1bfa` with `codex/compat-fuzz` `82780ae`.
Main's live evaluation clocks, request-chain state, CORS/credentials handling,
native HTMLDDA collections and History structured cloning remain in place.
The incoming changes add the bounded differential fuzz harness, canonical live
collections, HTMLDocument identity and reference-aware realm retention across
navigation. The Realm field conflict is resolved by retaining both the History
clone function and the realm ownership fields.

Integration exposed a retained History boundary: an inactive Document's History
must reject access with SecurityError. A Chrome 152 oracle now checks those
operations, including invalid scrollRestoration values being ignored before the
active-document check. This preserves main's native structured clone path.

Scheduler waits now round scaled deadlines upward and honor the requested
interval when the platform timer fires before QPC measures the full interval.
Enqueue and cancellation wakes do not receive that adjustment. The inline-clock
test waits using the canonical monotonic clock instead of assuming Windows
Sleep and QPC have identical resolution.

The animation-frame test previously required exact equality between the frame
timestamp and performance.now() at callback entry. Ten fresh headful Chrome 152
pages disproved that assumption. The test now checks microtask ordering, a frame
timestamp no later than callback entry, and a clock that continues advancing
inside the callback. Raw observations are in
`internal/browser/testdata/animation_frame_clock_chrome152.json`.

The unchanged retained Document.all navigation oracle passes with ordinary and
snapshot bootstrap and is now enabled by default. Historical audit reports are
left unchanged because they describe their recorded baseline revisions.

Validation includes the full browser suite, scheduler/network/DOM/CDP and both
engine packages, targeted scheduler and browser race tests, and the Python fuzz
harness unit tests (21 tests, six optional environment tests skipped).
The retained Document.all oracle was also run separately after enabling it.

A fresh headful Chrome 152 comparison against the merged binary preserves the
previous common-corpus results: 12 of 15 probes match completely; the other
three contain 41 differing leaves. The state-relations corpus retains four
known differing leaves. These are compatibility limits, not a claim of full
Chrome equivalence. Local raw runs are under
`compatibility/private-captures/merge-compat-fuzz-20260911/final`.
Realm bridge-cache and DOM-arena retention limits remain documented in
`compat-fuzz-fixes/navigation-ownership.md`.
