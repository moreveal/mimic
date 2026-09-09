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

This is not a complete SVG renderer or layout implementation. Text glyph bounds
(text/tspan/textPath), use/symbol instancing, switch selection, font-relative lengths, CSS
transforms contributing to parent bounds, and CSS path data remain unsupported.
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
