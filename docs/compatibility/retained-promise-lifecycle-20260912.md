# Retained Promise lifecycle in Chrome 152

Removing an iframe preserves language Promise reactions on its retained realm.
Callbacks created in the parent or child and parent Promise assimilation all
run. Window.queueMicrotask callbacks and newly requested WebCrypto digests stop
with the inactive document. A digest requested before removal still completes.
Navigation differs: the old execution context's jobs remain disabled.

Mimic previously applied navigation's checkpoint shutdown to frame removal too.
The fix preserves the retained language queue on removal, keeps timers/resources
closed, gates queueMicrotask delivery on the canonical realm state, and leaves
digests requested after deactivation pending after argument conversion.
Console uses the same renamed execution-context state projection.

The synthetic fixture and frozen capture are in
`internal/browser/testdata/retained_promise_*`. Native A/B controls agree; tests
exercise ordinary and restored bootstrap, plus the existing frozen navigation,
realm ownership and cross-frame checkpoint regressions. Private raw matrices
are retained under `.build/residual-continuations-delegated/promise-*` in the
original checkout. No target-specific value or callback is implemented.

This repairs a confirmed independent lifecycle discrepancy. The saved target's
`ZwhIC5` still retained its sentinel in the replay after this change; this is
**not** evidence that its missing result is fixed. Its exact cause remains under
investigation. Separate cross-realm WebCrypto BufferSource and algorithm error
conversion gaps were exposed by an additional invalid-input control and remain
open; the lifecycle fixture uses valid same-realm inputs.
