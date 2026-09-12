# SVG scaled text precision, manual-045102

The three producers are now fully localized: `RKUE0` hashes the sum of absolute
group bounding-box components; `JRzmw6` hashes the sum of the first emoji's
first-character rectangle; `oSIr8` hashes the sum of distinct text lengths
divided by 100000. No hashes or captured values enter the implementation.

The saved original operations use four emoji/ZWJ strings and fifteen text
strings, x/y 32, and italic 150px serif for the long strings. The apparent CSS
`scale(1.001)` is a rounded serialization: the native CTM scale is
1.0009980201721191. Re-parsing the serialization therefore was not an exact
reproduction. A bounded offline diagnostic extracts the original elements,
computed font and CTM; a separate clean local probe reconstructs those inputs.

## Structural correction

SVG text now shapes at the effective screen font scale, including ancestor
transforms and SVG viewBox. It follows Chrome's Float32 scale calculation and
font-cache precision (truncate to hundredths), then quantizes the scaled text
advance. Local rectangles multiply by a Float32 reciprocal scale; text length
divides by the scale. SVGRect fields and transformed bounding-box corners retain
their Float32 representation. These arithmetic orders matter and previously
left stable differences even with the correct font resources.

Fresh headful Chrome 152.0.7977.82 A/B has zero differences. The sanitized
19-case oracle compares all text/character/group rectangles and lengths exactly
for four emoji strings at four scales and independent italic text at three
scales. Ordinary and restored-bootstrap regression tests pass; the existing
focused `TestSVG` tests pass unchanged. No whole-suite/race/performance gate
was run.

The private reconstructed producer now matches all three aggregate inputs:
4992.546875, 88.95946502685547, and 0.638944033908844. Every one of the nineteen
text lengths and first-character rectangles matches. The raw producer probe,
controls, metadata and extraction receipt are retained in the main checkout's
`.build/residual-svg-delegated`; the original strings stay private.

## Remaining independent ink-bound observations

This patch does not claim globally exact SVG ink bounds. A broader diagnostic
retains small-serif `A`'s native negative sidebearing, enlarged 300px italic
ink bounds, and three individual long-string bbox-width differences (two near
0.00049px and one near 0.915px). They do not change the original group's bbox.
Replacing these with the existing TrueType hinted bounds was tested and
rejected because it does not reproduce Chrome and incorrectly enlarges color
emoji bounds. No such experimental code is included. Those discrepancies remain
localized to the glyph-ink-bound projection and are not classified as random
or proved impossible to implement. The existing explicit approximate ink-bound
diagnostic remains active.

Pinned source references:
[LayoutSVGInlineText](https://raw.githubusercontent.com/chromium/chromium/152.0.7977.82/third_party/blink/renderer/core/layout/svg/layout_svg_inline_text.cc),
[SVGLayoutSupport](https://raw.githubusercontent.com/chromium/chromium/152.0.7977.82/third_party/blink/renderer/core/layout/svg/svg_layout_support.cc),
[FontDescription](https://raw.githubusercontent.com/chromium/chromium/152.0.7977.82/third_party/blink/renderer/platform/fonts/font_description.cc),
and [FragmentItem::ObjectBoundingBox](https://raw.githubusercontent.com/chromium/chromium/152.0.7977.82/third_party/blink/renderer/core/layout/inline/fragment_item.cc).
