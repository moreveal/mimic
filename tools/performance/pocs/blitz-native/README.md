# Blitz native producer PoC

This experiment measures the renderer-free `blitz-dom` style/layout core on the
same 10,000-row author-CSS shape used by Mimic's controlled observation chain.
It intentionally does not link Blitz paint, window, renderer, Dioxus,
accessibility, or system-font features.

```powershell
cargo run --release --manifest-path tools/performance/pocs/blitz-native/Cargo.toml
$env:BLITZ_SEQUENTIAL = '1'
cargo run --release --manifest-path tools/performance/pocs/blitz-native/Cargo.toml
```

The HTML parser is included only to establish an explicit upper bound for a
naive second-DOM projection. A Mimic integration should mutate a persistent
native document from the canonical mutation journal instead of serializing and
reparsing HTML at observation boundaries.

