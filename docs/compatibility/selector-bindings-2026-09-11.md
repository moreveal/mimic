# Selector binding cleanup, 2026-09-11

Follow-up to [interface inheritance](cleanup-2026-09-11.md), on clean
`24d706b6388605f015b87f58a0ed6e89116120db`, in `codex/compat-cleanup-20260911`.
No external workload or target site was launched. This package fixes a confirmed
common binding defect and does not finish the broader cleanup.

The late selector module installed ordinary functions without receiver/arity/
DOMString gates. Document and Element operations now validate canonical private
node brands before checking the required argument and converting it. Genuine
foreign Documents/Elements work through their existing canonical node metadata;
prototype forgeries do not. Conversion failures keep their original exception
identity and occur outside CSS parser error translation. Concise methods provide
nonconstructible callable shape; native names/source marking is retained.

Covered operations: Document querySelector/querySelectorAll/getElementById;
Element querySelector/querySelectorAll/matches/webkitMatchesSelector/closest.
These call the existing shared selector implementation, including CDP's direct
query callback, without adding another parser or DOM state model.

DocumentFragment bindings are deliberately unchanged. Synthetic foreign fragments
lack the private-brand bridge needed to validate borrowed methods. A local-only
WeakMap check would reject genuine foreign fragments. HTMLAllCollection borrowed
methods also remain open: they require a captured owner-operation bridge that
cannot be diverted by overriding the public item/namedItem property. Neither
boundary is concealed by prototype-based brand guessing.

## Fresh evidence

Private receipts: `compatibility/private-captures/cleanup-bindings-20260911/`.
`identity.json` records the clean source and baseline binary SHA256;
`final-identity.json` records the tested final binary and source dirtiness.
Chrome is frozen 152.0.7977.82, Windows x64, headful, a new controlled profile,
1280x800, no feature overrides. Sweep headless uses a separate new profile.

| Set | Before | After |
|---|---:|---:|
| Unchanged 236 probes, complete match | 138 | 178 |
| Original 28 representatives, complete match | 10 | 12 |
| Official observational groups | 18 | 17 |
| Previously matching probes regressed | вЂ” | 0 |
| New differing paths / reference drift | вЂ” | 0 / 0 |
| General corpus | 12/15 | 12/15 |
| General corpus differing records | 41 | 41 |
| Brand-matrix differing records | 1351 | 1319 |
| State-relations differing records | 4 | 4 |
| Sweep | 113/130 | 113/130 |

All normalized observations are compared, not only the first grouping leaf.
The 40 additional matching probes arise from the shared selector binding cause;
they are not 40 independent fixes. Original representatives `4c01ec91c8369240`
and `90dd2730882e2b50` now match in full. Other original brand groups still contain
Performance/hasFocus observations; those are not claimed fixed. Corpora overlap
and their totals must not be added.

The new selector_binding Chrome oracle measures brand forgeries, null receiver,
missing argument, Symbol conversion, conversion side effects, caller TypeError,
borrowed calls, constructibility and original exception identity. It runs on
ordinary and explicitly restored snapshot realms. A failing before-test was
retained. The DocumentFragment extension considered initially was removed before
final validation to avoid introducing a foreign-fragment regression; its earlier
full-suite invocation was stopped as superseded, not counted as a pass.

The first fixed Chromeв†’Chrome control matched 234/236: two hasFocus observations
changed falseв†’true while another capture was running. No selector observation
differed. This is recorded as environment-sensitive focus evidence, not a runtime
fix or a normalization exception. The serial control receipt is retained separately.
The serial fixed control passed 236/236 and all 28 representatives. General
corpus, brand-matrix and state-relations headful controls all match.

## Validation

- Full final browser suite PASS (333.875 s), webapi/DOM/CDP PASS:
  [log](selector-bindings-20260911/full-tests.txt).
- Affected selector browser tests under race PASS (23.696 s):
  [log](selector-bindings-20260911/targeted-race.txt). Full browser race not run.
- Full-leaf fixed comparison and original representative replay:
  [results](selector-bindings-20260911/results.json).
- Official fuzzer: 236 checks, 178 matches, 58 divergent probes, 17 groups,
  zero unstable/errors; unchanged 3533 discovered probes and 100-probe limit.
- Frozen generated-data and retained-oracle provenance checks PASS.
- No performance change or snapshot repair is claimed by this semantic package.

## Continuing work

The next required investigation is P0 snapshot native crash, with first-chance
dump collection. A successful gate is insufficient to close it. After that,
confirmed remaining semantic candidates include captured owner operations for
borrowed methods, Performance receiver/argument gates, Location descriptors,
WindowProperties named access and late callable finalization. Existing native
global retargeting, delegated eval and reachability boundaries remain explicit.
