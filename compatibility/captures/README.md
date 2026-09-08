# Retained evidence

`semantic-checkpoints/` retains compact historical `.script-*` and
`.browserscan-path-*` comparisons from the pre-cleanup working tree, with the
leading dot removed. These describe generic module identity/import lifetime,
DOM, crypto and API-shape regressions. They are evidence, not production inputs
or a claim of full conformance. Preserve before/after pairs: they explain why
otherwise unusual implementations exist. Loopback URLs and the public RSA SPKI
probe are synthetic test data, not credentials.

Full website HTML/assets, encrypted payloads, session/network traces, duplicate
benchmark output and local browser downloads were relocated to a private local
archive outside this repository. They are unsuitable Git fixtures. Historical
written conclusions remain in docs; external results were not rerun. See
`docs/cleanup-manifest.json` for the former paths and aggregate file sizes.

Chrome captures must carry `captureMetadata`; mode-less files are invalid.
`navigator-chrome152.json` is the authoritative fresh headful capture.
`navigator-chrome152-headless.json` is retained only for explicitly mode-scoped
regression comparison. Historical BrowserScan evidence is marked headless with an
uncontrolled profile ID and must not supply generic Chrome expectations. See
[`docs/oracle-policy.md`](../../docs/oracle-policy.md) and the
[`headless audit`](../../docs/oracle-headless-audit.md).
