# Canonical geometry integration regression audit

Reference: frozen headful Chrome 152.0.7977.82, Windows x64, two fresh profiles. The independent `css_geometry_integration_oracle.js` and matching JSON retain launch hashes, browser versions, origins, viewport and visibility metadata. Raw receipts are `.build/residual-dom-delegated/geometry-integration-final-controls` in the main checkout. Chrome A/B and Mimic have zero differing observation leaves.

## Production corrections

* Offset coordinates now use the offset parent padding edge, including a positioned ancestor through a static intermediate node. Static BODY denotes the initial containing block. This restores the unchanged execution-cleanup frozen reference, including negative offsets and translated/scaled rectangles.
* Opposing top/bottom insets resolve auto height for non-replaced absolute boxes using the containing padding box. Borders, padding, margins and border-box sizing remain in the canonical graph. Querying a generated-ratio child before its parent completes the parent flow first. This fixes the unchanged snapshot DOM regression.
* Client dimensions subtract borders from the canonical untransformed box. The inset oracle exposed 84px border height versus the correct 80px client height; borrowed realm projections receive the same numeric fields.

No renderer, OS geometry backend or additional per-element host crossing is introduced. Existing unsupported full formatting/scrollbar/transform-containing-block cases are not expanded here.

## Stale test assumptions

The nonfrozen in-flow iframe test expected its 1px iframe to make a 1px parent. Frozen Chrome makes an 18px default line box. The nonfrozen cascade test expected authored `translateX(2px)` from computed style; frozen Chrome returns `matrix(1, 0, 0, 1, 2, 0)`. Both exact values are retained in the new oracle and the assertions remain exact.

The unchanged-sampling diagnostic referred to removed `host:rect`. It now counts `host:computedStyleAvailable`, which is visited by the canonical box sampler. The original invariants remain: initial work must happen, ten unchanged opportunities must do no further work, viewport change must invalidate it.

The original `intersection_observer_regressions.js` is preserved byte for byte. Its ratio target has no top/bottom inset: frozen Chrome places it at y=300..600, and the overflow clip is y=0..300. Their edge intersection has height zero, not 300. The independent successor `intersection_observer_regressions_v2.js` preserves every other assertion and tests both zero intersection height and edge intersection truth. Test wiring now uses that successor.

This last intersection result is inferred from independently frozen rectangles, not claimed as a successful native callback capture. Attempts to run the original async fixture in fresh targets, new windows and the initial browser window recorded hidden document state and no native observer callbacks in this desktop session. Those unsuccessful diagnostic runs are retained under `geometry-integration-io*-controls`; they are not reference expectations. The new successor passes Mimic's full original lifecycle sequence. A future interactive native callback run can independently confirm the inferred edge result.

## Validation

The focused integration tests, unchanged execution-cleanup frozen ordinary/restored reference, CSS catalog/box/foreign-owner oracles, and geometry restored-realm matrix passed together (90.464s). The new oracle is exercised in both runtime engines and a restored child realm. No full-suite, race or performance gate is run for this bounded correction.
