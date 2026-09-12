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

## CSS rule serialization

Shared specified-value transform serialization now emits comma separators and
uses the existing simple CSS length calculation reducer; `var`/`env` token streams
remain unmodified. The same serializer feeds inline declarations and stylesheet
rules. Keyframes retain their distinct newline grammar, including empty blocks,
and canonicalize `from`/`to` selectors. Media feature ranges serialize their AST
comparison operators with required spacing; grouping rules keep their own format.

Independent frozen controls cover 5 rule families and 11 transform values,
including matrices, scale/translate/rotate3d/skew, multiple transforms, calc and
unresolved substitution. Zero Chrome A/B and Chrome/Mimic differences. The retained
synthetic oracle runs ordinary/restored; raw captures are in
`.build/residual-dom-delegated/css-controls`. This does not claim a complete CSS
math expression parser or all keyframe editing interfaces.
