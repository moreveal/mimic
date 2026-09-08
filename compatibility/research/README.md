# Historical research tools

These optional scripts observe external compatibility and are never imported by
Mimic's runtime or its Go regression suites. They were relocated during the
2026-09-08 stabilization pass to separate investigation from product code.
Run from the repository root only when external research is explicitly authorized.
Pyppeteer 2.0.0 is the historical client; the raw child-state tool also uses websockets.

`browserscan_benchmark.py`, `prefo_semantic_trace.py`, and the two
`post_fo_child_state*.py` tools are historical site-specific instruments.
`debug_constructed.py` is a constructor-shape diagnostic. No scripts were run in
this cleanup. The obsolete Vue payload patcher depended on a removed downloaded
asset and was archived outside the repository. Its useful conclusions survive
in docs/browserscan-current-result.md and local module regression tests.

Raw outputs may contain live cookies, headers, URLs and payloads. Keep them out
of Git. Retain only reviewed, minimized semantic observations as fixtures.
