# Secure Window member order

The supplemental JSON preserves complete string own-key sequences for Navigator,
Screen, History, Performance prototypes and the AbortSignal interface object.
They come from the completed fixed Chrome replay in
`compatibility/private-captures/headers-final-20260911` (normalized observations:
`.build/fixes-headers-final-observations.json`). The exact probe IDs are
`property.navigator.__proto__`, `property.screen.__proto__`,
`property.history.__proto__`, `property.performance.__proto__`, and
`property.window.AbortSignal`. Their complete observations and Chrome control
were unchanged from the preceding Node/DOMException replay.

These sequences are not alphabetical descriptor data or a new discovery budget.
Only string keys are included; symbol ordering remains with the engine.
This additional capture applies only to the secure Window exposure. It does not
assert the same profile order for insecure, isolated or Worker exposures.

Publication reuses existing descriptors and values after semantic installers.
It does not add properties. A nonconfigurable property anchors the existing
prefix; if that prefix cannot match the captured order, publication leaves the
object alone. Such ordering requires construction in the proper order or an
engine-level implementation and is not silently reported as corrected.

The consuming implementation passes the focused frozen oracle in ordinary and
restored snapshot realms, including a subsequent user delete/redefine. All five
corresponding probes now match in the fixed replay. Full package validation is
recorded in `docs/compatibility/platform-bindings-2026-09-11.md`.
