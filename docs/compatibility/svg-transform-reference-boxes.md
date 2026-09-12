# SVG transform reference boxes

Independent local HTTP observations against frozen Chrome 152.0.7977.82
(2026-09-12) confirmed that SVG computed transforms must resolve percentages
against the SVG reference box, not the HTML layout rectangle. Two fresh native
processes agreed on all 17 fixture observations.

CSSOM and the SVG coordinate projection now share the reference-box resolver
and planar numeric transform pipeline. The computed matrix excludes
transform-origin, while getCTM includes it. View-box follows the viewport;
fill/content boxes follow local geometry; border/stroke boxes include stroke
bounds for closed rect/circle/ellipse primitives. Changes to dimensions,
viewBox, stroke width, stroke presence and CSS stroke are observed at query
time. Nonplanar computed transforms retain the existing 4D algebra with the
same percentage reference dimensions.

Arbitrary stroked path joins and non-scaling stroke reference boxes remain
explicit geometry boundaries. This change does not implement general stroke
outlines, rendering, or expand the planar getCTM boundary.

`svg_percent_transform_oracle.js` and its frozen JSON cover reference-box
aliases, resizing, viewport mutation, percentage origin, cardinal rotation,
stroke mutation and nonplanar computed translation. Goja and V8 ordinary
realms match exactly; V8 restored realms use the same frozen oracle.

The existing full transform precision fixture now also runs on Goja. CSSOM
strings, geometry lengths, character measurements and non-rotation numeric
observations remain exact. Only unrounded rotation matrix elements permit up
to four ULP for the independent Goja/V8 transcendental implementations. The
original V8 ordinary/restored strict expectations are unchanged.

Private native A/B, fresh Mimic binary outputs, command-line and binary-hash
provenance are retained in `.build/css-transform-review-delegated` in the main
checkout. Native A/B and native/Mimic each have zero differing leaves after
the change. No website, challenge program or capture was used.

Validation:

- `go test ./internal/browser -run '^Test(CSSTransformPrecision|SVGPercentTransform)' -count=1`: PASS, 3.182 s.
- `go test ./internal/browser -run 'Test(CSS|SVG|ExecutionSVG|WebkitCSS|ParserStylesheetUsesSharedLoaderAndCSSOMProjection)' -count=1`: PASS, 122.474 s; covers both engines, restored V8 and foreign owner projections.
