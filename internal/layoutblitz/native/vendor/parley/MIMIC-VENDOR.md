# Pinned Parley source

Source: crates.io `parley` version `0.11.1`, unchanged registry dependency
versions and features. Upstream repository revision recorded by the crate:
`eea3503dd6cf17130cbb07348e0ff2c918300e94` (`parley` directory).
The original MIT and Apache-2.0 licenses are included.
Source: https://github.com/linebender/parley. The registry package's
`.cargo_vcs_info.json` and `Cargo.toml.orig` retain provenance. Local changed
files are `src/layout/data.rs`, `layout.rs`, `line.rs`, and `line_break.rs`;
`bidi.rs` is unchanged (its deprecation warning is upstream).

Local extension: optional CSS parent strut and atomic inline-box baseline
offsets supplied before line breaking. The line breaker incorporates these
inputs into real ascent/descent/height, subsequent line placement and box
positions. No text, glyphs, or externally returned rectangles are fabricated.
Layouts without these optional inputs retain upstream behavior. Builder
clear resets both inputs so pooled layouts cannot leak previous content state.

Blitz fills these inputs from primary-font metrics, used line height, Taffy
baseline output and CSS baseline-shift values. These are internal engine APIs,
not browser-facing APIs or workload-dependent memoization.

The migration builds this path dependency with `std` and `system`, not a
renderer. Dependency versions are pinned by the containing native crate's
`Cargo.lock`, not the archival package-local lock. No modifications to Cargo's
shared registry/cache are required. Native text layout, not a Go/JS surrogate,
consumes these baseline/strut inputs.
