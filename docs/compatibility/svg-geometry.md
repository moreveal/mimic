# SVG geometry observations

SVG nodes use the selected Chrome profile's specific interfaces, including
SVGGElement and the SVGGraphicsElement/SVGGeometryElement inheritance chain.
Creation respects SVG tag-name case and qualified local names. HTML parsing,
createElementNS, and cross-realm wrappers observe the same canonical DOM nodes.

`SVGGraphicsElement.getBBox()` computes an independent SVGRect from canonical
geometry. It does not allocate a raster surface, render an image, or require a
GPU. It includes neither the receiver's own transform nor ancestor transforms.
Containers union their children's bounds after each child's transformation.
Nested SVG viewports apply their viewBox, position and preserveAspectRatio when
contributing to a parent's box; a viewport's own getBBox stays in its user space.

## Implemented observations

- Rectangles, circles, ellipses, lines, polygons and polylines; image and
  foreignObject use their declared rectangular geometry.
- SVG path M/L/H/V/C/S/Q/T/A/Z commands, relative forms, repeated segments,
  compact arc flags, and partial valid path prefixes. Quadratic/cubic extrema
  and elliptical arc extrema are calculated analytically, not by rasterization
  or fixed point sampling.
- Group unions, nested groups, SVG transform lists, and nested viewports.
- Unitless/px, percentages, and absolute in/cm/mm/q/pt/pc lengths; percentages
  resolve against the relevant SVG viewport. Supported CSS geometric
  declarations take precedence over presentation attributes.
- Zero/negative dimensions, empty and move-only paths, malformed point lists,
  defs exclusion from container contributions, and visibility/display cases
  retained in the Chrome fixture.
- Detached trees return zero bounds. A hidden SVG viewport or hidden HTML
  ancestor suppresses measurement. Hidden SVG groups can still be queried;
  children marked display:none do not contribute to parent bounds.
- Synchronous attribute changes are reflected without a second geometry state
  model. Each call returns a fresh SVGRect; modifying it cannot change the DOM
  or a subsequent result.
- Genuine nodes from another realm are accepted; unrelated receivers and
  prototype-only imitations are rejected. Ordinary getAttribute, children,
  localName or isConnected overrides do not replace internal geometry data.
- Chrome 152 ignores the supplied getBBox options object, including accessors;
  the implementation preserves that measured behavior.

## Explicit boundaries

This is not a complete SVG renderer or layout implementation. Text paths, use/symbol instancing, switch selection, font-relative shape lengths, nonplanar CSS transforms, and CSS path data remain unsupported.
Encountering these supported-interface but unimplemented geometry operations
throws NotSupportedError and records a semantic-missing diagnostic. They do not
silently contribute a fabricated empty rectangle.

General CSS layout, animation, external resources, filters and a complete SVG
length grammar are outside this implementation. New cases need a native oracle
before extending the observation model. In particular, this change does not
establish a successful Voxel/Cloudflare result.

Work per query is bounded: at most 16,384 path segments, 65,536 values in a
numeric list and 256 nested geometry levels. Exceeding a bound reports an
explicit NotSupportedError. State is realm/Page-owned; there is no global
mutable geometry registry or graphics backend.

## Validation

`internal/browser/testdata/svg_bbox_oracle.js` contains 55 observation groups,
captured from isolated Chrome 152.0.7977.82 in
`compatibility/captures/semantic-checkpoints/svg-bbox-chrome152.json`.
Coordinates in this fixture are rounded to five decimal places before exact
comparison; interfaces, receiver errors, identities and other results are exact.
This is a test comparison precision, not a claim that every possible SVG path
matches native floating-point calculations bit for bit.

The same fixture runs on V8 and Goja through TestDocumentCompatibilityOracle.
TestSVGBoundingBoxUnsupportedGeometry ensures unimplemented children do not
silently become successful measurements. Existing SVG namespace/snapshot tests
remain unchanged.

## Text observations and shared font sizes

Text/tspan observations use Page-owned local OpenType resources and CPU shaping,
without painting or a native graphics backend. Glyph ink edges are approximate;
Windows raster hinting is not reproduced. Missing fonts and unsupported shaping
remain explicit boundaries. Font resources are released with the Page.

`css_font_metrics.js` now computes font size for both CSSStyleDeclaration and
SVG text. It resolves inherited values, em/rem/percent, absolute CSS units,
font-size keywords and a two-term additive calc expression. Root rem uses the
initial 16px size; descendants use the current computed root size. Query-time
resolution avoids stale caches after style changes. Unsupported expressions and
font-dependent units still have an explicit diagnostic rather than guessed metrics.
This does not add general layout or complete CSS math/variable resolution.

`css-font-size-chrome152.json` preserves 24 native observations, including SVG/CSS
agreement, presentation-attribute precedence, root mutations and zero-size text.
These run on both engines. They establish the covered size semantics, not exact
native glyph ink or successful challenge passage.

## Font fallback

Text shaping now selects a face for each Unicode grapheme cluster, considering
CSS family order and a bounded Windows-profile fallback search. Adjacent
clusters using one face are shaped as a run, preserving kerning and ligatures.
Joiners and variation selectors are retained for shaping rather than requiring
standalone visible glyphs. Text/emoji presentation selectors participate in face
selection using font color-glyph metadata; this does not render color glyphs.
Fallback caches are Page-owned; a per-call cluster cache avoids repeated lookup.

The SVG line box keeps the primary font's ascender/descender, as measured in
Chrome 152, while fallback glyph advances come from the selected font. The
`font-fallback-chrome152.json` oracle contains 44 observations across four
family lists: Latin, mixed emoji, repeated emoji, combining accents, ZWJ,
skin-tone modifiers and text/emoji variation selectors. Advances and vertical
line metrics compare exactly. Horizontal ink edges use the existing outline
approximation and are checked within one CSS pixel of the untouched capture.

This is not a complete clone of Windows font fallback mapping. Unsupported RTL
shaping, unavailable fonts, synthetic italic faces and missing full-cluster
coverage remain explicit failures. Coverage search is limited by existing
64-face/64 MiB per-Page resource bounds. No GPU or native renderer is required.
The local regression does not establish a successful Cloudflare result.

## CSS transforms

CSS transform lists now contribute to ancestor SVG bounds while an element's
own getBBox stays in its local coordinates. CSS takes precedence over the
transform presentation attribute (including CSS none). Matrices are composed
in list order and around transform-origin, using view-box or fill-box bounds.
Percentages, absolute lengths, em/rem, and angle units use measured semantics.
Nested transforms and query-time style changes share the canonical DOM state.

Supported operations are 2D matrix, translate/scale/rotate/skew and axis variants,
plus planar matrix3d, translate3d with zero Z, translateZ(0), scale3d with unit Z,
and rotateZ. Origin also applies to SVG attribute transforms. Nonplanar 3D,
perspective, stroke/border reference boxes and unsupported expressions retain
explicit failures with transform value, origin, reference box, node and reason.
This does not add rendering or a general CSS layout engine.

The 25-group svg-css-transform Chrome 152 oracle covers child vs parent boxes,
composition, origins, percentages, fill/view reference boxes, attribute and
stylesheet precedence, nested groups and mutations. It uses the existing SVG
fixture rounding of coordinates to five decimal places, with exact comparison.

## Systematic SVG audit

See [SVG audit, 2026-09-10](svg-audit-2026-09-10.md) for the broader interface,
value/list, reflection, coordinate, text, path, use and DOMMatrix coverage,
reproduction commands and explicit remaining boundaries. Surface presence alone
is not treated as implemented semantics; remaining generated SVG operations now
fail with an interface/member diagnostic.
