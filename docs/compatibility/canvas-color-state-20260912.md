# Canvas color state and coverage

Frozen reference: headful Chrome **152.0.7977.82**, executable SHA256
`ea36dd818a90176f1a70616f0363d9be527229389a6c073a0b1688b9e73f67e9`.
The public oracle fixtures contain the full observations, control metadata and
binary/probe receipts. Native A/B differences are zero.

The canonical Canvas buffer now retains its color space and format. Float16
contexts preserve extended and negative components in a premultiplied backing
buffer. ImageData reads/writes, context attributes, overlapping reads, bitmap
copies, close, clearing and resize all project that buffer. Engine-specific half
storage is allocated lazily after bootstrap restoration. Eager allocation in the
portable bootstrap incorrectly reported missing Float16Array in restored V8;
the ordinary/restored regression catches this integration failure.

The exact state oracle also measures Chrome's P3 image-sampling projection:
drawImage of a P3 float surface passes through an RGBA8 source snapshot, including
direct canvas and ImageBitmap sources and colorSpaceConversion:none. Its original
surface remains float16. Equivalent sRGB copies preserve extended components.
This format projection is general, not a rule for any particular pixel value.
State/metadata/error/ownership observations match exactly in both boot modes.

Paint conversion uses colorimetric D50-adapted matrices for sRGB and Display P3,
extended sRGB transfer functions and a common premultiplied buffer. Matrix source:
[Skia named gamuts](https://api.skia.org/namespaceSkNamedGamut.html).
The model does not reproduce every Skia shader/SIMD rounding decision. Expanded
six-color, four-context measurements retain **20 numeric leaf differences**:
integer quantization and half precision after paint/color conversion. They are
documented renderer precision residuals, not exact pixel equivalence. The test
allows one premultiplied-byte uncertainty at the measured alpha >= .5 (two
unpremultiplied bytes); alpha, formats, styles and attributes stay exact. Direct
HDR state/copy tests have no numeric tolerance. Frozen expectations are retained
unchanged; no expected pixels or hashes are embedded in production.

Path observations integrate convex polygon coverage over pixel squares, with a
bounded shared subpixel grid for compound winding, stroke and clip intersections.
Coverage participates in composition, including fractional clearing. Native
rasterizer edge coverage remains approximate under the architecture's graphics
contract. The independent path oracle verifies fractional edges, repeatability,
overlap, copies, local changes and opacity. All relational observations match.

Focused validation: Canvas/HTMLCanvas/WebGPUCanvas/SVG subset and the new ordinary
and restored state/coverage/color tests pass. Full batch gates are recorded by the
integration report, not inferred from these focused results.
