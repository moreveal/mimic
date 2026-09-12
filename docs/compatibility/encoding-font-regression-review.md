# Encoding and font regression review

The two TextEncoder/Blob failures came from source corruption in ff5ccc9:
UTF-8 literals were rewritten as mojibake while changing the favicon test.
The original inputs and expected text are restored with explicit Unicode
escapes. Runtime encoding behavior and expected byte arrays are unchanged.

The combining-mark font failure was semantic. Nominal glyph coverage rejected
Segoe UI Emoji for a decomposed accented letter because that font lacks the
standalone combining mark, despite having the canonical composed glyph that
HarfBuzz can produce. The environment fallback order then selected Tahoma,
changing the SVG advance from the frozen 8.375 to 8.421875.

Coverage now also considers NFC composition after nominal coverage fails.
Original text and cluster offsets still go to HarfBuzz; no text normalization
is exposed to authors. The existing portable Unicode normalization dependency
is reused, without native font or renderer dependencies.

Fresh independent frozen Chrome 152.0.7977.82 A/B controls agree exactly on 32
measurements spanning three canonical pairs, a following character and four
font-family lists. Mimic matches all 32. The new oracle also runs in ordinary
and restored V8; the original font fallback regression covers Goja and V8.
The native reference expectations were not weakened.

Raw native controls, Mimic output and binary hash provenance remain in the
main checkout at `.build/encoding-font-review-delegated`.

Validation: `go test ./internal/textmetrics -count=1` passed (2.798 s).
The focused browser Font/CanvasFont/SVGText/CSSTransformPrecision and the two
encoding tests passed (13.651 s), including ordinary/restored V8 coverage.
