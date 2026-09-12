# Font fallback ownership and missing glyphs

The integrated Canvas/DOM replay exposed a new control-flow failure: a coverage
search for an absent character retained every rejected candidate in the Page
face cache. Later ordinary serif text then hit the 64-face resource limit and
threw. Rejected candidates now release their cache entries/byte accounting.
Missing characters shape the selected font's actual `.notdef` glyph. Missing
font resources themselves still fail explicitly; no synthetic widths are used.

Frozen Chrome .82 headful A/B controls measured C0/C1 controls, unassigned
characters, replacement characters, normal text after those operations, and
explicit fallback families. The entire matrix now matches exactly in ordinary
and restored contexts. Full raw reference and provenance are retained in
`internal/browser/testdata/font_missing_glyph_chrome152.json`; private runs are
under `.build/residual-canvas/missing-*`.

Independent `CSS.getPlatformFontsForNode` controls after layout identified
Tahoma for Arial's U+FFFD, SimSun for U+007F, and Segoe UI Emoji for emoji.
Receipts are in `.build/residual-svg-delegated/platform-fonts`. The Windows
reference bundle selects an ordered fallback-family policy through
Environment.Fonts.Fallback; the engine accepts other resource policies without
OS-specific execution code. No measured character advances are constants.

TrueType hint programs are also isolated from host callbacks: a malformed or
unsupported program can retain outline observations and emits an explicit
Canvas hinting-approximation trace, rather than unwinding through V8. This does
not suppress missing font/shaping errors or replace browser exceptions broadly.

Validation: frozen missing-glyph and Canvas metrics ordinary/restored tests,
relational Canvas tests, and focused font fallback/isolation/cache tests passed.
