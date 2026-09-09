# CSS selector domain, 2026-09-09

The DOM selector entry points now share pinned reusable parsing/matching:
CSS-tree 3.2.1 and DOMSelector 9.1.1 preprocessing → AST translation → css-select
6.0.0 and nth-check 2.1.1. No repository-specific selector or selector algorithm
was added. `querySelector`, `querySelectorAll`, `matches`, `closest` and the CSS
style matcher use this domain layer. CDP querySelector/querySelectorAll call the
same closed-over realm callback, rather than page-overridable public methods.
The new handwritten Go structural parser was removed; a restricted 42-line
legacy helper remains solely for internal lookups and validated native leaves.

Cascadia 1.3.5 was audited first: its 2,144 production Go LOC operate directly on
`*html.Node`, with no tree adapter. An unmodified integration requires a whole-tree
projection; an interface fork would retain significant upstream maintenance and
still lacks modern scope/logical selectors. css-select exposes explicit tree
primitives, so it fits canonical wrapper identity without another DOM. css-what
alone was rejected for user-input parsing after WPT exposed invalid Unicode escape
handling. The small AST translator delegates tokenization and escaping upstream.

## Measured coverage

The selected WPT files are copied unmodified from the Chrome 152 lock's Blink WPT
revision by `compatibility/domain_tests.py`. Source URLs/hashes and exact binary
receipts are included in the JSON evidence. This is a measured subset, not a claim
of complete Selectors Level 3/4 or WPT coverage.

| Evidence | Chrome 152 | Mimic | Interpretation |
|---|---:|---:|---|
| Independent structural/attribute/escape/error probes | 36 reference results | 35 identical | `nth-child(... of S)` is not supported by the matcher |
| WPT selected files | 9 complete | 9 complete | Completion alone is not a pass |
| WPT registered assertions | 2,055 | 81 | Main 1,975-test suite is blocked in document creation before registration |
| WPT passing/failing assertions | 2,054 / 1 | 76 / 5 | The totals have different denominators |
| Escape file | 67 / 68 | 66 / 68 | Two Mimic failures originate in lone-surrogate attribute storage; one Chrome failure is raw NULL selector parsing |
| Scope, exclusive roots, attribute spaces/dashes, removed elements, fragment mutation files | all pass | all pass | Includes canonical identity and mutation freshness |

The large Selectors API suite is blocked by `document.implementation`/HTML document
creation. Two small tests require Window named properties (`root`, `target`). The
remaining two Mimic escape failures occur because an attribute containing a lone
surrogate becomes U+FFFD across the host boundary; selector decoding itself now
uses the mature parser. Chrome rejects a literal NULL identifier where this WPT
expects replacement; Mimic currently follows the WPT expectation, so that result
is an explicit Chrome divergence rather than a claimed oracle match.

Other tracked limits: namespace-aware/XML matching, form state pseudo-classes,
quirks-mode case folding, all Level 4 nth-of forms, and full pseudo-element grammar
need broader domain coverage. DOM API integration rejects jQuery-only extensions
and maps invalid selectors to DOMException SyntaxError. Forgiving logical lists
and ordinary non-selectable pseudo-elements are handled at the AST boundary.

## Architecture and measured cost

Queries return canonical wrappers and static NodeLists; per-query primitive-read
memoization is discarded in `finally`, so mutations between calls are visible.
AST caches are bounded at 128 entries. Compiled predicates have weak scope keys
and a 128-entry per-scope bound; result caching is disabled. Parsed simple
ASCII tag/id/class compounds use the native canonical leaf matcher. Class token
splitting uses CSS ASCII whitespace, including a regression for NBSP.

The generated bundle is 102,982 bytes. It excludes domutils' node model and CSS
property/MDN grammar data. A checked-in build manifest and lock reproduce it.

A hash-verified diagnostic used 201 elements, 20 queries per sample, five samples;
wall time includes the CDP roundtrip. These are small comparative measurements,
not the frozen performance baseline.

| Selector | Mimic before native leaf route | Mimic after | Chrome 152 |
|---|---:|---:|---:|
| `li.a` | 30.244 ms | 0.952 ms | 0.675 ms |
| `li:nth-child(2n+1)` | 49.428 ms | 49.417 ms | 0.749 ms |
| `ul:has(> li.b)` | 26.735 ms | 27.152 ms | 0.410 ms |

The fresh DOM native profile before the leaf route completed correctly
(`.build/selector-library-audit/profile-before-simple`). The root task owns the
full fast-gate latency, throughput and ten-Page retention comparison; this small
selector diagnostic does not substitute for it. The original handwritten matcher
returned false for nth/has, so its timing is not a valid correctness-equivalent
baseline for those selectors.

Evidence: `wpt-chrome.json`, `wpt-mimic.json`, `generic-differential.json`,
`cost-before-native-route.json`, `cost-after.json`, `build-receipt.json`.
Regression tests: `internal/browser/selectors_semantics_test.go`.

## Navigation and element-state integration

`:target` now uses the canonical target element selected at navigation rather
than comparing current element IDs with the current URL. Changing an ID or using
history.replaceState preserves the target; fragment navigation selects a new
canonical target, including legacy named anchors. Percent decoding is supplied
by the URL layer. `:defined` reads the existing upgrade state; `:focus`,
`:focus-within` and `:focus-visible` read the existing focus state and trusted input
modality. These are state adapters passed to the upstream matcher, not new CSS
parsing algorithms. Complex focus heuristics, shadow focus retargeting and history
traversal remain tied to their underlying domains and require broader coverage.

The independent state probe has 7/7 identical Chrome/Mimic results (previously all
seven Mimic states threw SyntaxError). Two unmodified WPT files for focus/focusin
register six tests: Chrome 6/6, Mimic 2/6. The four failures are Event.target being
null inside listeners; the selector returns the correct focused input. This is
recorded as an Events integration gap, not hidden as a selector success.

`internal/browser/selector_state_test.go` covers navigation-target identity,
history URL changes, focus/blur and custom-element upgrade state.
`internal/cdp/dom_query_test.go` verifies CDP scope/nth queries, errors, initial
realm bootstrap and independence from page-overridden querySelectorAll.


## Frozen-workload performance attribution

Fresh unchanged DOM host/native profiles isolated the principal execution
regression: one attribute query caused 3,000 children reads and 3,000 attribute
reads through the host boundary. Mature AST validation now permits native leaf
lookup for case-sensitive attribute presence/equality with safely serializable
values. The case-folding policy is exported from the pinned upstream library;
case-insensitive or complex values stay in the mature matcher.

Warm execution medians in the diagnostic host profile were 72.22 ms for the
isolated original baseline, 129.47 ms before this route, and 86.14 ms afterward.
Host crossings fell from 60,489 to 54,490 (baseline 54,485). Navigation was
39.77 / 48.47 / 47.91 ms respectively, so this repair does not explain or remove
the broader bootstrap cost. These diagnostic profiles are not substitutes for
the full latency/throughput/teardown gate. Exact build/launch hashes, warm samples
and host costs are in `perf-attribution.json`; the frozen harness was unchanged.

A final query-result projection repair keeps canonical IDs directly in the static
NodeList backing store, avoiding redundant eager wrapper and slot arrays before
iteration. Correctness tests preserve static membership, live canonical
attributes and identity after detach. The final isolated host profile measured
79.49 ms warm execution and 47.14 ms navigation, with 54,491 host crossings.
The final profile includes subsequently stabilized integration code and is not
a standalone A/B attribution for this small projection change. It does confirm
that the final execution remains substantially below the pre-repair 129.47 ms;
remaining differences from the 72.22 ms baseline are retained in the report.
