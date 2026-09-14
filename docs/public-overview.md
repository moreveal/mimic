# Public product repository

The public product overview is https://github.com/moreveal/mimic-overview.
The implementation repository remains private. Beta contact: Discord `moreveal`.

The public repository has its own clean Git history. Its release set consists of
README, FAQ, beta and quick-start documentation, a dated benchmark summary,
numerical benchmark results, the original README hero image, SVG charts, and
MIT-licensed runnable client examples. Public Beta executable archives are
published only as releases in the overview repository, with PolyForm Shield
1.0.0 and third-party notices. Implementation sources, internal research,
captures, traces, cookies, local paths, and private Git history are not exported.
See [the release process](../tools/release/README.md) for native builds, packaged
example verification, checksums, and publication gates.

## September 14, 2026 evidence

The full frozen run is in
[`09-optimized-20260914`](../benchmark/runs/09-optimized-20260914/public-summary.md).
`build.json` records the clean source revision and executable hashes;
`launches.jsonl` records checks before each process launch. `raw.json` is the
retained detailed measurement record. `public-results.json` is the exact numeric
export also published in the public repository. The public export excludes local
paths, process lists, commands, network traces, and private source references.

The README charts highlight common-probe startup calibration and static
concurrency scaling. Warm execution and other tradeoffs are described in Known
limitations and retained in the full report. Ready RSS and active RSS at 50 static pages are
different measurements; neither is advertised as per-page memory. The stopped Windows
100-Page levels remain visible, and the separate completed Linux 100-Page comparison
is linked from the optimization report with its higher Mimic marginal memory. All six warm
workloads, cold end-to-end results, CPU/memory metrics, and attempted concurrency
levels are included in the public benchmark document.

The full gate automatically generates two presentation charts from a completed
run, including a provenance receipt. Regenerate them from the raw run or numerical export
using `tools/performance/readme_charts.py` with the results path and output asset
directory. `--preview` writes PNG previews for visual inspection. The public and
private repositories use identical chart files and the original hero image.

The product messaging describes an actively developed runtime aiming for direct
HTTP lightness and browser compatibility. Slower results are optimization targets,
not proof of future gains. Original baselines and frozen harnesses are retained.

Lead the public README with renderer-free browser execution and scoped React,
WebAssembly, Worker, DOM, networking, CDP and concurrent-page checkpoints.
Keep Cloudflare, BrowserScan and Amazon under real-world compatibility after the
benchmarks. Profiles describe reproducible browser environments rather than lead
with fingerprinting. The public history is organized into product, usage and
benchmark commits; future updates should append ordinary commits rather than
regularly rewriting published history.

## Public live-site claims

These are dated development observations, separate from the frozen benchmark:

- Cloudflare laboratory success and BrowserScan Normal verdict:
  [September 12 live comparison](performance/live-browser-comparison-20260912.md).
- Repeated laboratory challenge-to-content transition:
  [additional live observations](performance/stall-investigation-20260912.md#additional-live-compatibility-observations).
- Amazon storefront content and offline snapshot validation:
  [September 11 snapshot observations](performance/snapshot-hydration-20260911.md#live-site-observations).

Do not expand these into an overall anti-bot pass rate, a claim about why a service
admitted a session, or full Amazon login/checkout support. No live-site captures
are included in the public release.

## Public usage examples

The public quick start follows the [environment profile contract](environment-profiles.md):
CLI profile validation, V8 locale/timezone behavior, Puppeteer connection,
native proxies, context creation, and dynamic viewport updates. Full profile
replacement requires a fresh context. Fingerprint-related observations are
configurable only within the supported contract; this is not arbitrary identity
replacement or a change of the real public IP.
