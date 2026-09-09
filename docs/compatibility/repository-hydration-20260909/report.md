# Repository hydration: first generic module blocker

Workload: <https://github.com/moreveal/AuthMeReloaded>, observed 2026-09-09.
Production source before the change: `5c0be11524f802ce3c18ec4f95a53637055c4369`.
The first identified blocking semantic divergence is missing `import.meta.url`.
It is independently reproduced and fixed. **The repository page still does not
fully hydrate**: execution now reaches additional unimplemented browser APIs.
No repository name, hostname, asset name or response-specific behavior was added
to production code. The frozen performance harness and baseline were untouched.

## Evidence and causal chain

The user's exporter waits for `load` and defaults to zero additional settling.
That can capture an intermediate Chrome state, but does not explain Mimic's
persistent state: its DOM remains unchanged at load +2 and +10 seconds.

The authoritative repeat used Chrome **152.0.7977.82**, V8 **15.2.124.21**,
headful Windows x64, a fresh dedicated profile, a 1280×800 outer window and no
feature overrides. Actual viewport 1272×653, visible and focused. Binary SHA-256:
`ea36dd818a90176f1a70616f0363d9be527229389a6c073a0b1688b9e73f67e9`.
See `evidence.json` for provenance, response statuses and module chronology.

1. Both browsers receive the document and script resources. In the verified
   Mimic repeat, all 202 observed responses are HTTP 200; no fetch/XHR request
   for repository metadata is issued. This is not a failed metadata response.
2. Chrome issues eight Fetch requests, all HTTP 200: `latest-commit`,
   `recently-touched-branches`, `branch-and-tag-count`, `refs?type=branch`,
   `branch-infobar/master`, `overview-files/master`, `_sidebar`, and
   `tree-commit-info`. No XHR occurs in this observed path.
3. The shared `wp-runtime-802d2cd51dadf61b.js` module derives its asset base from
   `import.meta.url`. Missing URL causes
   `Error: Automatic publicPath is not supported in this browser` before
   dependent application modules can initiate hydration. This is ordinary
   bundler behavior, not a GitHub API requirement.
4. Original Mimic trace misleadingly ends those module tasks without errors.
   V8's module evaluation returns a **rejected Promise** for this synchronous
   throw. A temporary read-only inspection of that result exposed the same
   rejection in all nine dependent module entries, beginning with `environment`.
   The temporary diagnostic was replaced by normal module exception reporting.
5. The independent fixture, written without site code, confirms the semantic:
   `import.meta.url` is absent before the fix; the fixture cannot construct its
   data URL and retains `pending`. After the fix both browsers expose matching
   entry/dependency/inline/dynamic URLs and change the DOM to `populated`.

DOM evidence counts below are **literal substring occurrences**, not a claim
about the number of visible rows. Exported document bytes are hashed as well.

| Capture | `Skeleton` at load | +2 seconds | +10 seconds |
|---|---:|---:|---:|
| Fresh pinned Chrome | 29 | 0 | 0 |
| Mimic after first fix | 115 | 115 | 115 |

Before-fix Mimic also has 115 occurrences at all three observations. Its
`load` → +10 second DOM hash is unchanged. Chrome's intermediate load state
varies with networking; both captured Chrome navigations reach zero skeleton
occurrences by the +2 second observation. No timeout threshold is used as proof
of the cause: the missing module URL and rejected evaluation establish it.

## Independent reproduction and implementation

`compatibility/import_meta_differential.py` serves a local document, ES module
graph and JSON response. `independent-before.json` and `independent-after.json`
preserve the observations. It checks URL queries/fragments, a dependency's own
base, stable and distinct metadata objects, null prototype, URL property
attributes, an inline module and a later dynamic import. The module uses its
own URL to fetch JSON and replace a DOM text node.

The V8 host import-meta initializer sets `url` from the module resource-name
registry. V8 continues to own the stable object and normal property semantics.
No global lock, page identity change, scheduler intervention or string rewriting
is involved. Browser module evaluation additionally inspects already-settled
rejections without waiting or running a microtask checkpoint. Pending top-level
await is left pending; the reporting path is covered by a dedicated test.

Relevant specification: [HTML HostGetImportMetaProperties](https://html.spec.whatwg.org/multipage/webappapis.html#hostgetimportmetaproperties).
This scoped change supplies `url`; it does not claim complete `import.meta`
support, including `resolve`, import maps, redirects or inline-module cache
identity across multiple inline entries.

The local task log preserves `classic` → `module` → `microtask` in both engines.
In this capture Chrome processes DOMContentLoaded before the fetch completion,
while Mimic processes fetch first. Tasks from these different sources do not
establish the module-URL failure; both after-fix fixtures populate the DOM.
No event-loop ordering fix is inferred or applied from that observation alone.

## Remaining boundary

After the URL fix, `environment` completes without the publicPath rejection.
`github-elements` now fails reading `define` from an undefined value; the
observed missing registry is `customElements`. Other entries reach
`TypeError: Illegal constructor`; trace includes `MutationObserver`,
`AbortController`, `ClipboardItem`, and `TextDecoder` missing semantics.
These are follow-up candidates, **not independently proven fixes in this change**.
Metadata requests still do not start and skeletons remain. Waiting longer in
the exporter will not supply those semantics.

General asynchronous unhandled-rejection reporting remains incomplete. The new
reporting handles an already-rejected module evaluation Promise; it does not
claim to observe every later top-level-await rejection. Some response bodies
are unavailable through Mimic's bounded CDP body cache and are explicitly
recorded as unavailable, never treated as empty successful responses.

## Re-running the workload

Start separate dedicated Mimic and pinned headful Chrome instances with fresh
profiles. Build Mimic and record its SHA-256 before starting it; record the
pinned Chrome hash too. Use the actual executable paths and recorded hashes:

```powershell
python compatibility/site_hydration_probe.py https://github.com/moreveal/AuthMeReloaded --endpoint http://127.0.0.1:19323 --output .build/site-chrome-new --binary PATH_TO_PINNED_CHROME --sha256 RECORDED_CHROME_HASH --chrome
python compatibility/site_hydration_probe.py https://github.com/moreveal/AuthMeReloaded --endpoint http://127.0.0.1:19322 --output .build/site-mimic-new --binary PATH_TO_JUST_BUILT_MIMIC --sha256 RECORDED_BUILD_HASH
python compatibility/import_meta_differential.py --chrome http://127.0.0.1:19323 --mimic http://127.0.0.1:19322 --output .build/import-meta-new.json
```

The generic site probe accepts any URL; the site is not part of the frozen
performance workloads. It records CDP errors, requests, statuses, available
bodies, lifecycle events, DOM snapshots and Mimic's scheduler/module trace.
Its hash check validates the supplied executable; the caller must connect it
to the dedicated process launched with that executable, not an unrelated server.

Full local captures remain under `.build/repository-compat`; their hashes are
in `local-artifact-hashes.json`. Committed evidence omits cookies and bulk site
assets. Earlier exploratory captures used fresh pages in the same controlled
Chrome profile; only `chrome-fresh-verified` is the fresh-profile repeat.

## Validation and measured costs

- `go test ./...`: pass.
- Focused module tests, including pending Promise checkpoint behavior: pass.
- Focused module/engine race checks: pass.
- `tools/generate_compat.py --check`: pass, pinned generated files unchanged.
- `tools/check_repository.py`: fails on pre-existing machine paths and local
  artifacts, including installed pinned Chrome and historical performance files.
  These unrelated files were not changed or removed.
- Fresh DOM/static/React native profiles were collected before production edits
  with `profile_gate.py`; all three correctness checks pass. Build and launch
  hash receipts remain in `profile-before`.
- Before/final `fast_gate.py`: all six correctness workloads pass; warm ×5,
  static N=10/25 ×3 and memory after ten live Pages complete. Both raw runs and
  all launch hash checks are preserved in `gate-before.json` / `gate-after.json`.

See [measured deltas](performance-table.md). DOM completion changes +1.82%,
static completion +1.65%, static N10 throughput −4.62% and N25 −1.47%.
Static retained RSS above ready baseline rises 69.367 → 72.480 MiB; React is
89.164 → 89.172 MiB. These are noisy single-gate observations, not evidence
of a performance improvement from a module-only change on classic-script
workloads. The intermediate `gate-after` overlapped site investigation and is
not used for the final comparison. No full performance milestone is claimed;
the full matrix was not rerun for this single scoped compatibility correction.
