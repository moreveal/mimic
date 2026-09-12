# Canvas font observations against frozen Chrome 152.0.7977.82

Canvas now shares the Page/Worker-owned Go OpenType/HarfBuzz resource model
with DOM and SVG. Advances, kerning within words, letter spacing, alignment,
font ascent/descent and baseline offsets derive from font data. TrueType
vertical hinting uses the existing pure-Go freetype dependency only for Canvas
ink observations; DOM/SVG shaping does not pay for that extra evaluation.
No native font renderer, GPU or platform-specific core code was added.

Independent fresh-profile Chrome A/B controls used the pinned .82 executable,
headful default configuration and a local plain HTML fixture. Full measurements,
version, environment and runner/binary hashes are retained in
`internal/browser/testdata/canvas_font_metrics_chrome152.json`; private raw runs
are in `.build/residual-canvas/{before,shaped,words,expanded,word-spacing,final}`.
The test covers normal/italic/proportional/monospace resources, empty text,
spaces, ligatures, word boundaries, all six baselines, alignment and spacing.

The original approximation had 217 differing leaves in the initial matrix.
Advances and baseline observations now match exactly. Adjacent controls found
that this Chrome build stores wordSpacing and applies it while drawing, but
does not include it in measureText widths. The model preserves that observed
distinction. The extra emHeightAscent/emHeightDescent properties are absent in
Chrome and no longer introduced by the Canvas model.

Horizontal ink edges retain a documented graphics observation boundary:
13 leaves in the expanded matrix differ by exactly one pixel. Chrome's Windows
Skia scaler obtains these edges from DirectWrite GetAlphaTextureBounds; our
portable model bounds font outlines. See the primary
[Skia scaler implementation](https://skia.googlesource.com/skia/+/a2d00a28c563/src/ports/SkScalerContext_win_dw.cpp).
This is not claimed as exact Chrome ink/pixel equivalence. The regression test
checks exact widths, baselines, surface and spacing behavior, separately allowing
at most one pixel for these measured horizontal ink edges. It retains the full
unmodified Chrome observations, rather than replacing them with Mimic results.

Text pixel coverage remains the explicitly approximate local glyph-band model
per `docs/architecture.md`. Bands now use shaped glyph positions and bounds;
maxWidth, baseline, spacing and alignment share the measurement inputs. Local
changes, overlapping readbacks, snapshots, copies, resets and ordering remain
tested. No capture values, field names or expected hashes drive production.

Validation: focused Canvas/HTMLCanvas/SVG text/CSS box/WebGPU rendering tests,
ordinary and restored contexts, plus complete textmetrics and imageresource
package tests passed. The existing relational Canvas test now derives its local
change region from measured advances instead of the former hardcoded 6px cells.
