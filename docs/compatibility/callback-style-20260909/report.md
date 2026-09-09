# Callback exceptions, computed style, and live navigation

## Correctness fixes

The V8 adapter previously returned `(nil, nil)` when a direct JavaScript
function call threw. V8 also printed an unsolicited uncaught-exception message.
Calling `Function.prototype.bind` with an object receiver reproduced the reported
"Bind must be called on a function" message without any network workload.
Both ordinary and reentrant calls now catch and return the exception. Reentrant
host callbacks rethrow the original JavaScript value, preserving its identity
and TypeError classification. Regression coverage includes bind, toString,
ordinary objects, null, undefined, numeric exceptions and subsequent runtime use.

Computed style previously evaluated both fallback dimensions eagerly, including
when reading unrelated properties or dimensions already supplied by CSS. The
fallback values are now lazy and use internal geometry rather than overridable
JavaScript offset getters. Live declarations still reflect subsequent mutations.
A Chrome 152.0.7977.82 probe confirmed that author-defined throwing offsetWidth
and offsetHeight getters do not intercept computed opacity or declared dimensions.
The existing limited CSS/layout model has not been expanded.

## Live weather.com observations and boundaries

An isolated Mimic run still exceeded a 35-second `load` wait after these fixes.
Frozen Chrome 152 also exceeded the same site's 30-second `load` wait; its
readyState remained interactive although the weather content was visible.
Concurrent test runs therefore do not explain this timeout on their own.
This comparison does not establish equivalence of all remaining load blockers.

A snapshot using DOMContentLoaded plus a three-second delay was written in
14.93 seconds (navigation 8.23 seconds, capture 3.66 seconds). Visual inspection
found skeleton placeholders instead of the forecast. Saving a file is therefore
not evidence of application readiness, and this timing is not a successful
weather-content validation. A live teardown also stalled and required stopping
the diagnostic process. Full weather hydration/load completion and teardown
remain unresolved; the fixes above must not be reported as resolving them.

The external snapshot client now accepts an explicit `--wait-until` choice of
`load` or `domcontentloaded`, retaining `load` as its default. A fixed settle delay
is insufficient for this workload; use a verified application-ready condition.
No website-specific behavior, resource blocking, timeout suppression, or
artificial warm-up was added to Mimic.

## Validation

Focused regression tests, the full Go suite, race checks, and generated-artifact
validation passed. Fast-gate measurements and build hashes are retained alongside
this report. Live timings are unpaired observations; background interactive
runtimes and network variance prevent attributing small timing differences.
