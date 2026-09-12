# CSS transform numeric state

Independent local-fixture checks against headful Chrome 152.0.7977.82 found
that serializing a declaration and then reparsing it for geometry discarded
significant numeric digits. CSSOM intentionally exposes six significant digits;
the authoritative declaration must retain its parsed value separately from
that public text.

The existing declaration record now carries `parsedValue` for transforms.
Inline mutation, clone, cascade, stylesheet mutation and adopted-sheet sources
preserve that record/precision. Public CSSOM remains rounded. Explicit author
reassignment of the serialized string intentionally parses that new string.
Computed transform matrices and HTML/SVG geometry consume the retained value.
No second synchronized style map or graphics backend was introduced.

The frozen controls also establish that SVG scale/translation parameters use
Float32, while explicit matrix components and trigonometric results retain
Double precision. Simple transform parsing drops fractional digits after the
seventh; a mixed list requiring the general parser retains them. Rotation
range reduction and degree conversion order follow the geometry operation.
Translation of a text character rectangle preserves its Float32 width/height
instead of rounding both far edges and subtracting. Multiword unquoted font
family names serialize with quotes, including in computed style.

The parser/rotation observations are consistent with Chromium's
[CSS fast parser](https://github.com/chromium/chromium/blob/d04cdb24d67b081f6cf80200ffc5233f44b61109/third_party/blink/renderer/core/css/parser/css_parser_fast_paths.cc)
and [degree trigonometry](https://github.com/chromium/chromium/blob/d04cdb24d67b081f6cf80200ffc5233f44b61109/ui/gfx/geometry/sin_cos_degrees.h).
Frozen execution, rather than source inference, supplies test expectations.

## Validation

`css_transform_precision_oracle.js` covers scale above/below one, matrix,
skew, positive/negative/radian/exponent rotations, mixed transform lists,
translation, calc, unrelated mutation, clone, identical style-attribute writes,
serialized reparse, cssText, removal, stylesheet mutation/cascade, adopted
sheets, HTML geometry, and font-family serialization.

Two separate fresh headful Chrome processes/profiles, no feature overrides:
**0 A/B differences; 0 Chrome/Mimic differences**. Exact versions, window,
viewport, origin/security state, binary/probe digests and environment profile
are retained with the frozen expectations. Raw launch receipts, outputs and
runner are preserved outside the disposable worktree in main's
`.build/residual-svg-final-delegated/precision`.

Focused browser tests pass in ordinary and restored-bootstrap modes:
`TestCSSTransformPrecisionMatchesFrozenChrome`,
`TestSVGPrecisionMatchesFrozenChrome`,
`TestCSSMatrixProjectionMatchesFrozenChrome`,
`TestCSSRuleSerializationMatchesFrozenChrome`, plus constructed stylesheet,
canonical inline-style and style-rule-index checks. Existing explicit SVG
nonplanar/reference-box boundaries and approximate text ink bounds remain;
this change does not claim a complete CSS layout or rendering implementation.
