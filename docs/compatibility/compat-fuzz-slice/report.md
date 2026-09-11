# Compatibility fuzzer vertical-slice validation

Measured 2026-09-11 on Windows x64 from runtime revision
`3faacc6` (`fix(browser): implement native HTMLDDA document collections`).
Mimic was freshly built in the isolated `codex/compat-fuzz` worktree using
`go build -o .build/mimic.exe ./cmd/mimic`, with the default V8 engine.
The oracle was the repository's exact Chrome 152.0.7977.82 binary, headful, with a
separate initially fresh profile, no feature overrides and a 1280x800 window.
Full oracle metadata is retained in `results/summary.json`.

The existing `compatibility/differential.py`, `oracle.py`, related CDP probes,
Chrome captures and `docs/oracle-policy.md` were inspected first. The new harness
reuses their Python transport and oracle validation/provenance, while isolating
individual execution/reduction targets instead of sharing a mutable page.
No runtime source, reference expectations or existing harnesses were changed.

## Result

The bounded run selects 136 corpus probes and 100 reflection probes from 3,533
potential generated surface probes. It covers all seven requested root objects.

- Chrome against itself: **236 checks, 236 matches, 0 divergences, 0 errors**.
- Chrome against Mimic: **236 checks, 120 matches, 116 divergent probes, 28 unique groups, 0 unstable results, 0 errors**.
- Harness tests: **21 passed**, including live Chrome normalizer, realm, timing
  and timeout-recovery tests.
- All **28 saved JS repro files** were independently evaluated on both engines and reproduced their stored results. The `document.all` CLI replay also reproduced its descriptor divergence.

Examples of measured differences (Chrome / Mimic):

| Probe | Observation | Chrome | Mimic |
| --- | --- | --- | --- |
| `legacy.document-all-descriptor` | prototype depth owning Document's `all` descriptor | 2 | 1 |
| `legacy.live-collection` | repeated `getElementsByTagName` collection identity | true | false |
| `legacy.window-named` | descriptor of a named form on Window's chain | present | absent |
| `legacy.collection-indexed-named` | HTMLCollection own keys | `0`, `compat_named` | empty |
| `realm.inside-iframe` | document tag | `[object HTMLDocument]` | `[object Document]` |
| `realm.window-proxy-navigation` | new document after iframe navigation | true | false |
| `descriptor.location.assign` | writable | false | true |

The fundamental HTMLDDA truthiness/type/null behavior already matches on this
runtime revision; the harness does not mislabel that as a current bug. Its richer
`document.all` reflection probe exposes an actual remaining difference.

The first simultaneous Chrome-vs-Chrome control exposed a stable focus artifact
in `hasFocus()`. Sequential paired observations removed it; the complete control
then matched. A separate live test covers a never-resolving probe followed by a
successful evaluation to prevent timeout-induced transport failures from being
reported as semantic mismatches.

## Scope boundary

Results contain unique **observational signature groups**, not automatically
inferred root causes. Each group keeps the first mismatching leaf of its probe,
its exact Chrome/Mimic values, representative setup-reduced JS and member IDs.
Minimization is 1-minimal deletion of setup statements with a fixed observation;
it does not claim globally minimal JavaScript or remove all unused helper code.

Receiver calls use five safe methods. Realm coverage is a small explicit corpus,
not a full surface-by-realm Cartesian product. Timing resolution distributions,
worker ordering, stateful DOM fuzzing and WebIDL generation remain deferred.
No runtime bugs were automatically fixed. This is the requested stopping point.

See [usage and architecture](../../compat-fuzz.md) for commands and limitations.
