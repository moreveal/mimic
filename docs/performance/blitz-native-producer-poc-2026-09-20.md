# Blitz native producer PoC — 2026-09-20

## Scope

This is a fast viability test for replacing Mimic's JavaScript style/layout
producer with the renderer-free core of Blitz. The experiment uses only
`blitz-dom`, `blitz-html`, and `blitz-traits`, with `blitz-dom` default features
disabled. It does not include paint, renderer, compositor, window, Dioxus,
accessibility, or system-font integration.

Branch: `experiment/blitz-native-poc`. Blitz is pinned to
`e7bf7bca452bc9194ac5c9bcf2ac3c0ebcf02e09`.

The fixture preserves the controlled observation chain's producer shape:
10,000 DOM rows, author CSS, computed-style readback, geometry readback,
viewport invalidation, an inline-style mutation, and hit testing. The viewport
is 1280x800. The target's asserted geometry changes from 800x32 to 640x40.

This does not yet connect Playwright, visibility, role traversal,
IntersectionObserver, scrolling, or Mimic input to Blitz. It therefore is a
producer feasibility result, not a full-E2E performance result.

## Results

Seven fresh executable invocations per threading mode, release build. Values
below are medians in milliseconds.

| Operation | Sequential | Parallel |
| --- | ---: | ---: |
| HTML parse / naive projection | 11.02 | 11.20 |
| First Stylo + Taffy resolve | 9.49 | 13.54 |
| Parse + first resolve | 20.50 | 24.87 |
| 10,000 retained style + rect reads | 0.65 | 0.64 |
| Resolve with no changes | 0.77 | 0.79 |
| Target inline-style mutation + resolve | 7.62 | 11.35 |
| Viewport invalidation + resolve | 3.17 | 3.22 |
| 10,000 hit tests | 1241.82 | 1234.64 |

Parallel traversal loses on this deliberately simple tree because scheduling
cost exceeds the matching work. It remains a separate decision for a
Wikipedia-shaped CSS corpus.

For context, the existing controlled comparison measured Mimic/Chrome's first
synchronous rectangle at 349.3/11.4 ms. Blitz's 20.5 ms includes a deliberately
naive full HTML parse that a production journal-fed native owner should not pay
at an observation boundary. This is strong evidence that Blitz can cover the
dominant style/layout producer budget with the required order-of-magnitude
constant-factor reduction. It is not evidence that Mimic E2E would immediately
fall by the same amount.

## Decision

**GO for an integrated vertical slice, with constraints.** Do not adopt Blitz
as an independent reconstructed DOM. Keep Mimic's Go DOM authoritative and
feed a persistent `blitz-dom` document stable node identities plus mutation
batches. Publish compact style and geometry products from that owner to both
V8 worlds. This avoids repeating the failed document-scale JS projection model.

The first integrated slice should cover the complete controlled chain, not
only `getBoundingClientRect`: visibility, scrolling, IntersectionObserver and
input must consume the same native products. Run the unchanged Wikipedia gate
as soon as the captured page's required layout features are correct.

The primary discovered risk is hit testing. Blitz's current general hit path
is roughly 0.12 ms per call on this 10k tree and appears traversal-bound. This
does not dominate the measured chain at its normal handful of clicks, but it
would be unacceptable for high-frequency pointer workloads. The integration
should retain Mimic's indexed/known-target hit path initially or add a spatial
index over Blitz's authoritative boxes; it must not re-run layout or return
fake target geometry.

Other no-go conditions for production adoption are semantic gaps in tables,
fragmentation, scrolling boxes, pseudo-elements, shadow/slot structure, font
metrics, or incremental invalidation. Those need compatibility fixtures and
the unchanged Wikipedia workflow; the synthetic result cannot decide them.

