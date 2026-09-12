# Residual DOM compatibility, 2026-09-12

Target: saved manual-20260912-045102. Diagnostic capture values are never runtime defaults.

## Author selector reentry

The control-geometry implementation called the public `select.querySelectorAll`
while calculating intrinsic width. A focused headful Chrome 152.0.7977.82 probe
observes zero selector calls for `getBoundingClientRect`; Mimic observed two.
The same public dependency existed in trusted keyboard Tab traversal and pointer
hit targeting. All three now use the existing canonical selector operation.
The query algorithm and form geometry model are unchanged.

Independent fresh-profile Chrome controls agreed. Raw evidence and full launch
metadata: `.build/residual-dom-delegated/initial/{chrome-a,chrome-b,passport}.json`.
Focused regression covers rect/list reads, Tab and pointer input, preserves live
option membership, and runs both ordinary and restored Page modes.

Validation: TestInputInternalsDoNotReenterAuthorSelectors,
TestDefaultControlGeometryMatchesChrome152, TestProtocolKeyboardAndTextMatchChrome152.

CSS serialization/inventory and geometry remain under investigation separately.
