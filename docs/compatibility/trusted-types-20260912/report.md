# Trusted Types and CSP enforcement - Chrome 152.0.7977.82

Worktree: `E:\GitHub\mimic-trusted-types`, branch
`codex/trusted-types-csp-chrome152`.
Baseline: `da4f93e873d508323832efafe6870d378d79d1c4`.
Implementation commit: `82a6b04c38ed1b94b118cd153dc738e33f24fe4e`.

The reference is a locally measured, headful, fresh-profile Windows Chrome
**152.0.7977.82**, V8 **15.2.124.21**, with no feature overrides. The checked-in
[oracle](../../../internal/browser/testdata/trusted_types_chrome152.json)
contains the capture metadata. No Chromium implementation was copied.

## Result

The 32 scenarios contain **1,288 independently compared observations**:

| Comparison | Before | After |
| --- | ---: | ---: |
| Differences from the frozen oracle | 828 | 8 |
| Differences excluding the explicit SharedWorker execution boundary | 828 | 0 |
| Default-policy invocation mask: innerHTML=1, eval=2, script.src=4 | 0 | 7 |

Chrome's invocation mask is **7**. All eight remaining differences are successful
Chrome SharedWorker constructions that Mimic rejects with `NotSupportedError`
after the required URL conversion. Their default-policy calls and arguments
match Chrome. The other **1,280 observations match**.

[Machine-readable comparison](comparison.json) records differences by scenario,
representative before/after results, input SHA-256 hashes and oracle metadata.
This is a focused semantic comparison, not a claim of complete browser or CSP
compatibility. API counts were not used as a success metric.

## Production integration

- `internal/webapi/trusted_types.js` is the shared Window/Worker model for
  TrustedHTML, TrustedScript, TrustedScriptURL, policy creation, empty values,
  default conversion and sink classification. Private realm binding brands,
  rather than public prototypes or `toString`, establish trust. Factories and
  callbacks belong to their realm. The existing reference bridge preserves
  cross-realm identity and borrowed operations.
- `internal/csp/trusted_types.go` projects the effective policy set. A Realm's
  committed Document owns CSP; a Page-wide policy no longer accidentally
  governs independent frame documents. Response headers, conjunctive policies,
  report-only policies, connected head meta elements and about:blank/srcdoc
  inheritance use the same source of policy state.
- `internal/webapi/trusted_types_sinks.js` integrates binding conversions before
  mutation or execution. Brand checks precede string conversion, matching kinds
  avoid default policy, wrong kinds use the ordinary conversion algorithm, and
  default callbacks receive `(input, requiredType, sinkName)`.
- The engine-neutral `EvalSourceRuntime` boundary checks native eval and all
  Function constructor variants, including saved references and intrinsic
  `.constructor` paths. V8 owns compilation, direct-eval lexical scope and
  function identities; there are no eval/Function JavaScript replacements.
- Script preparation consumes canonical DOM source and its last accepted
  script-source value. Node-based source construction, connected script content
  changes and subtree insertion reach the same check. Clones do not inherit
  source trust. Markup-created scripts keep the parser's inert/started state.
- Event-handler compilation and authorized timer execution use internal
  compilation entry points, avoiding a second, spurious default-policy call.
  Cross-frame property writes transport the original exception object instead
  of replacing it with a generic Error.

The known event-attribute set is derived from the existing versioned declaration
catalog. The independent Chrome oracle verifies all **324** names; this does not
install the corresponding unsupported event APIs. Policy state is per Document
and per realm, without a shared mutable factory or a global runtime lock.

## Covered sinks and semantics

| Area | Covered entry points |
| --- | --- |
| Dynamic code | Direct/indirect eval; Function; saved Function and intrinsic constructors; AsyncFunction, GeneratorFunction and AsyncGeneratorFunction |
| HTML | Element innerHTML/outerHTML/insertAdjacentHTML; template innerHTML; ShadowRoot innerHTML; Element/ShadowRoot setHTMLUnsafe; DOMParser.parseFromString (HTML and XML); Document.parseHTMLUnsafe; Range.createContextualFragment; document.write/writeln; iframe.srcdoc |
| Script URLs | HTMLScriptElement.src; SVG script href/xlink:href and href.baseVal; object.data/codeBase; embed.src; Worker; importScripts; SharedWorker synchronous TT boundary |
| Attributes | setAttribute/setAttributeNS; setAttributeNode/NS; NamedNodeMap writes; attached Attr.value/nodeValue/textContent; known event-handler attributes with namespace handling |
| Script source | HTMLScriptElement.text/textContent/innerText; generic Node setters and Text insertion checked at preparation; mutations of a connected script; nested insertion; clone/source trust; content event handlers |
| Timers/workers | String setTimeout/setInterval, function callbacks, TrustedScript callbacks, delay conversion order; WorkerGlobalScope sink labels; blob-worker CSP inheritance, worker-local policies/defaults and importScripts |
| Identity and errors | Illegal constructors/receivers; missing arguments; forged/proxied/wrong-kind values; stable empty objects; mutable public methods/prototypes; captured policy callbacks; thrown-object identity; TypeError/EvalError distinction and exact TT/CSP diagnostics |
| Ownership/lifecycle | Cross-frame values, borrowed methods/setters, adopted nodes, inert documents, about:blank/srcdoc and network-frame policies, document.open, navigation reset, retired Documents; bootstrap snapshot restoration; concurrent Pages within one BrowserContext |

Default-policy coverage includes missing callbacks, no policy, null/undefined
results, thrown objects, return coercion, reentrant calls, callback receiver and
argument order. Report-only CSP invokes default policy but allows an absent or
null result. Native dynamic code rejects default-policy source rewrites. If TT
and unsafe-eval both reject compilation, the CSP EvalError takes precedence
while preserving the observed default-policy invocation.

The CDP bypass cases distinguish policy delivery from already installed
enforcement: enabling bypass after navigation leaves existing TT/eval
restrictions in place; enabling it before navigation suppresses the new policy.
Meta policies delivered during bypass are ignored.

## Validation

Final validation results are recorded in [validation.json](validation.json).
The checks comprise the focused oracle/regression set, the complete
`internal/browser`, `internal/csp`, `internal/webapi` suites, targeted race with
shared-context concurrent Pages and bootstrap restore, and adjacent CDP
snapshot/frame/evaluation tests. Existing tests and frozen expectations were not
weakened. Only the new oracle capture was extended during this batch.

Reproduce the focused capture and comparison with:

```powershell
python tools/compatibility/capture_trusted_types_oracle.py --endpoint http://127.0.0.1:9532
python tools/compatibility/capture_trusted_types_oracle.py --mimic --endpoint http://127.0.0.1:19534 --output .build/trusted-types/after-final.json
python tools/compatibility/compare_trusted_types.py --before .build/trusted-types/before-final.json --after .build/trusted-types/after-final.json --baseline-head da4f93e873d508323832efafe6870d378d79d1c4 --output docs/compatibility/trusted-types-20260912/comparison.json
go test ./internal/browser ./internal/csp ./internal/webapi -count=1 -timeout 12m
go test -race ./internal/browser ./internal/csp -run 'TestTrustedTypes|TestTrustedScriptEval|TestWorkerTrustedScriptEval|TestFrameEvalArguments|TestCSP|TestDynamicScript|TestScript' -count=1 -timeout 3m
```

The capture tool uses fixture port 19532 by default. Its Python dependencies are
the existing compatibility oracle dependencies. Run the .82 reference headful;
inspect its retained metadata instead of assuming that another Chrome is an
acceptable substitute. CDP evaluations set `allowUnsafeEvalBlockedByCSP:false`.

## Explicit boundaries and remaining work

- Production native dynamic-code enforcement is verified on the V8 adapter.
  QuickJS/goja do not currently provide the native EvalSourceRuntime hook.
  They share DOM/policy enforcement but **do not provide equivalent direct
  eval/Function enforcement**. Replacing eval with a wrapper would break lexical
  scope and saved intrinsic references, so that was not used as a substitute.
- SharedWorker execution is explicitly unsupported after its TT binding check.
  Service-worker execution, javascript: navigation and SVG script execution are
  not supplied by this batch. SVG URL bindings are checked; a URL binding is not
  a claim of implementing its underlying browser execution subsystem.
- SecurityPolicyViolationEvent delivery, Reporting API/network CSP reports and
  full console-report parity remain unimplemented. Synchronous errors and
  blocked script-preparation traces are preserved. Existing CSP hash and
  strict-dynamic trust-propagation limitations are unchanged.
- The CSP `trusted-types-eval` source keyword is not implemented as an
  unsafe-eval exemption. An exploratory .82 DevTools evaluation exposed different
  string/TrustedScript behavior with this keyword; it needs a dedicated
  page-script/DevTools boundary oracle before making production allowances.
  The enforced `require-trusted-types-for 'script'`, `trusted-types` policy-name
  directive and ordinary unsafe-eval conjunction are covered here.
- Contextual-fragment and unsafe-HTML entry points use the existing canonical
  parser. Full Range editing/context selection, Sanitizer options and
  declarative shadow parsing are not implemented by this TT layer. This batch
  validates their type conversion/enforcement, not those independent features.
- Bootstrap snapshots restore a clean factory and bind the new Document's CSP.
  They are not live application-heap snapshots: serialization/restoration of
  user policy callbacks is unsupported. Existing MHTML snapshot export does not
  restore executable policy state.

No unrelated broad compatibility cleanup or performance-harness changes were
included. The report makes no throughput or latency improvement claim.
