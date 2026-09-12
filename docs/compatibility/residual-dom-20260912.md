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

## Canonical transform geometry

Client rectangles now use the existing DOMMatrix column-major 4x4 parser and
algebra through a private bridge. Matrix string parsing itself no longer calls
author-replaceable DOMMatrix methods. This replaces the separate 2D-only client
rectangle parser and covers skew, 3D translation/scaling/rotation and perspective
with positive homogeneous coordinates. Percentages resolve against the element's
box; transform origin remains part of this projection. There is no new renderer,
GPU, global state or Go/V8 crossing.

Frozen controls cover 12 transform families and author method replacement.
Chrome A/B observations are identical. Rectangle differences are at most
0.000012 CSS px from Blink float bounds versus double CPU algebra; regression
uses 0.0001 px tolerance, below a layout unit. Ordinary/restored regression and
existing SVG matrix/geometry tests pass. Raw: `.build/residual-dom-delegated/matrix-controls`.

Perspective camera-plane clipping remains an explicit diagnostic boundary.
Ancestor transform accumulation and general flow/table sizing are separate
outstanding geometry work, not solved by the matrix projection change.

## Generic font resource selection

The frozen host maps `system-ui` to local Ubuntu, confirmed twice by
`CSS.getPlatformFontsForNode` (`isCustomFont:false`). Independent `Ubuntu` and
`system-ui` boxes agree, while explicit Segoe UI differs. This is selected
Environment state, not a Chrome-version constant. The retained receipt is
`frozen-font-profile-20260912.json`.

`Environment.Fonts` now selects generic resource families for isolated Page and
Worker font engines; empty fields preserve existing defaults. For this captured
host, select `Fonts.SystemUI = "Ubuntu"`. Font data still comes from the existing
local OpenType resource engine; absent selected resources produce a diagnostic
boundary instead of substituting captured widths. No Ubuntu metrics are stored
in runtime code. Font line gap metadata is also retained for normal CSS line
height; Canvas ascent/descent values are unchanged.
