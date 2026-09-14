# Mimic v0.1.1

Changes since v0.1.0-beta.2:

- Canvas readbacks now serialize the canonical observation state as PNG, so
  repeated and overlapping reads remain consistent.
- Animated Canvas state is bounded to prevent unbounded memory growth.
- CSS-connected `FontFace` objects are now exposed through `document.fonts`.
- Cross-world shadow DOM mutations no longer fail when nodes cross realm
  boundaries.
- The development preview reconnects after the Mimic server restarts.
