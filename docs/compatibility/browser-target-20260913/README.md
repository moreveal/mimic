# Browser target discovery and attachment

Puppeteer 25.10.0 connected successfully but `browser.target()` failed with
`Browser target is not found`. Mimic only emitted page/tab discovery events,
despite already identifying a canonical browser target through the browser
WebSocket and implicit `Target.getTargetInfo`.

Frozen Chrome 152.0.7977.82 was measured in both [headful](headful.json) and
[headless](headless.json) modes. With a filter containing `{}`, discovery emits
an attached browser target. Explicit `getTargetInfo` and `attachToTarget` accept
that target ID; the attached session supports browser commands and detachment.
`getTargets`, even with an explicit browser filter, does not enumerate it.
An explicit empty filter matches nothing, unlike an omitted filter.

Mimic now emits its existing canonical browser target when requested by initial
discovery, accepts explicit inspection/attachment, and preserves the browser
session when the initial page closes. The ordinary page/tab paths remain in place.
This enables `browser.target().createCDPSession()` without adapting the client.

The focused regressions cover identity, discovery, filtering, attachment, command
routing, and session survival across initial-page closure. The complete CDP
package passed, and the public quick-start snippets ran with the real Puppeteer
25.10.0 client: profile validation, context creation, viewport update/readback,
disposal, page navigation, and configured locale/theme observations.

This is a scoped compatibility fix, not a complete target-graph implementation.
Mimic retains one server-owned browser target; Chrome's internal browser/UI
targets and complete discovery-filter transition behavior remain outside this
checkpoint. The saved observations include those additional Chrome targets rather
than deleting them to imply equivalence.

The earlier full performance checkpoint in `benchmark/runs/08-public-beta-20260913`
identifies the pre-fix executable. Its numbers are not a before/after measurement
of this change. The follow-up fast gate verifies the new build separately.
