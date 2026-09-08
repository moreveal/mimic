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
