# Window reflection and cross-frame iterator results

This change fixes the local semantic differences identified after the
`manual-20260911-044046` capture. It does not infer a protected site's decision
from local compatibility or performance tests.

## Reflection

Parent-realm `Object.getOwnPropertyNames(childWindow)` now reads the child's
current global object, rather than the small bridge wrapper. Descriptors,
symbols, definition/deletion, and prototype observations use the same current
realm. Descriptor inspection does not invoke getters. References returned by
descriptors still retain their immutable object owner.

V8 uses a detectable, noncallable native interceptor as the WindowProxy target.
Unlike an ordinary JS Proxy target, it can replace nonconfigurable descriptors
when navigation replaces the Document. Its prototype remains immutable and
preventExtensions returns false, as measured in Chrome. The existing HTMLDDA
interceptor implementation is reused without changing its undetectable/callable
behavior or rebuilding the native DLL.

Window globals are published in frozen Chrome 152 order before unforgeable
descriptors are finalized. Secure, insecure and isolated profiles were measured
independently (1232, 981 and 1233 initial string keys). Snapshot restoration
preserves the same order, including late V8 intrinsics. Runtime reflection is
not sorted; user-defined properties and delete/reinsert retain insertion order.
The original alphabetized descriptor captures remain unchanged. Order capture
provenance is in `../../chrome/152/generated/window-secure-order.md`.

The frozen Window reflection oracle tests both ordinary and restored V8 realms:
key equality, descriptors, symbols, identity, getter counts, mutation, navigation,
nonconfigurable properties, forbidden cross-origin descriptors, cross-origin
own-key shape and null prototype. This is not a complete implementation of all
allowed cross-origin methods and their descriptors. The Goja fallback still has
the ordinary Proxy limitation for replacing nonconfigurable descriptors across
navigation; the native V8 path covers that case.

## Optimization boundary

The bridge identifies only the exact captured native Array iterator `next`.
That function creates a fresh result object with no preexisting user aliases.
Its primitive own `done`/`value` fields can travel with the canonical remote
reference and be read locally while the result remains unexposed. Access checks
still run on each read. Mutation or reference escape permanently invalidates
those fields, including transitive exports through another object or callback.
An existing canonical wrapper is never rearmed.

Custom/reused iterator results, overridden next functions, object-valued results
and symbols use the ordinary bridge. No future array element is read early.
Identity, prototypes and descriptors remain remote. Intrinsics used by the cache
are captured; metadata has null prototypes to avoid invoking user toJSON hooks.
Reflection and encoding of keys/descriptors run within one owner turn rather
than introducing an owner-thread round trip for each internal field.

## Controlled measurements

Five fresh targets per process, four serial process groups in A/B/B/A order,
after other tests had stopped. A is the corrected runtime with the iterator
optimization; B differs only by a Go build overlay disabling that specialization.
Each probe consumes the complete 1234-key array (two test globals included),
reads each property twice and records descriptors, types, identity and getter
counts. Those recorded observations are identical across all 20 samples.

| Phase, median ms | Corrected runtime, specialization disabled | Enabled |
|---|---:|---:|
| Complete measured operation | 520.35 | 288.65 |
| Name enumeration | 0.70 | 0.70 |
| Remote array materialization | 340.20 | 140.75 |
| Property reads | 177.70 | 146.20 |

Independent phase medians need not sum to the median total. Total ranges are
444.2–569.8 ms and 272.4–329.3 ms. This is a 44.5% median reduction in this local
operation, not a promised reduction for the original 2.594-second task.
Earlier runs overlapping the full browser tests are exploratory, not the
reportable A/B result. Frozen Chrome's corresponding earlier local measurements
were about 8–9 ms, so substantial cross-frame overhead remains.

Raw controls, source and binary hashes:
`compatibility/private-captures/window-sweep-controls-20260911/` and
`.build/window_sweep_controls.py`. The overlay is
`.build/window-cache-disabled.json`. No frozen harness was edited.

## Validation and remaining instability

The full browser suite passed (372.132 s). Subsequent owner-turn consolidation,
all-profile order data and cache-hardening checks passed focused browser,
snapshot-equivalence and race tests. V8/Goja engines, CDP, scheduler, webapi,
Chrome profiles and the compatibility package passed. A broad package glob also
picked up unrelated scratch Go programs in private captures; these are not
repository test packages. The standalone bootstrap restore fixture was extended
to check rebinding the cached realm ID.

The first fast gate lost the shared process at the 25-Page static concurrency
wave. Its log was removed by the frozen runner's cleanup, so its precise native
failure is unknown. A clean b94c63c baseline gate and a complete corrected-runtime
repeat both passed all mandatory workloads, 10/25-Page waves and memory checks.
The repeat saved process logs without changing workload or assertions.
This does not prove the initial failure is fixed or that it is the same as the
previously documented snapshot lifecycle failure. Keep that instability open;
do not characterize the gate as an unconditional clean first pass.

Receipts: `.build/window-reflection-fast-gate{,-baseline,-repeat}/raw.json`.
The companion performance report records general-workload and memory results.
