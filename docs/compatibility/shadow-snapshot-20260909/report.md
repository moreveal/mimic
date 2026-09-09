# Shadow DOM lost during static export

Diagnostics on 2026-09-09, iteration 10 on endpoint 19602. The initial diagnosis was recorded before the fix.

The first deterministic difference in date display occurs at the export boundary. A generic autonomous custom element creates a shadow span containing '2 weeks ago' while preserving the light fallback 'Aug26,2026'. Chrome 152 and the Mimic runtime match: shadow text is relative, light text is absolute, and Intl.RelativeTimeFormat('en').format(-2,'week') is identical. Mimic.captureSnapshot preserves only light DOM. Exported HTML contains neither a shadow span nor a declarative shadow template; viewing it consequently shows the fallback.

Live GitHub confirms the same path: relative-time is registered; Aug26 shadow='2 weeks ago', Apr22='5 months ago', Jun27='3 months ago'. Titles are updated with time and timezone. Missing relative dates in a captured snapshot do not imply stopped JS or missing Intl. ShadowRoot.innerHTML also returns an empty string while shadowRoot.textContent is nonempty, a separate manifestation of incomplete serialization semantics.

A generic snapshot serializer must include the ShadowRoot subtree with CSS encapsulation and slot semantics (preferably declarative shadow DOM), accounting for open/closed roots, adopted stylesheets, and nested roots. Simply replacing light text with shadow text would fix only the visible case and break general semantics. Shadow state is stored in the realm, so Go d.InnerHTML cannot serialize it independently without receiving shadow roots.

Evidence: adjacent independent.json, independent-export.html, and site-state.json. Old source-only DOM.getOuterHTML snapshots are also unsuitable for assessing post-hydration mutations, but that is a different bug: Mimic.captureSnapshot correctly reads current light DOM and separately loses shadow trees.

## Shared serialization layer

Added a realm callback with attachment metadata and canonical child IDs, an immutable x/net/html projection, and declarative templates for open/closed and nested roots. Light DOM and slot elements are preserved. The ShadowRoot.innerHTML getter serializes current canonical children; its setter uses the existing mature HTML parser in the shadow host's context. During export, shadow scripts are removed by the normal export filter; resources and CSS pass through existing rewriting. JavaScript is not reexecuted. The projection exists only during export; there is no separate mutable DOM or mutation cache.

TestSnapshotPreservesCanonicalShadowComposition checks open/closed/nested roots, delegatesFocus, a named slot and light fallback, a current escaped mutation, shadow-script removal, and unchanged live state. Integration results are recorded below.

Limitations: adoptedStyleSheets/CSSStyleSheet do not yet have a complete canonical CSSOM in Mimic and are not implemented by this fix. Manual slot assignment cannot be reconstructed by declarative HTML serialization alone; named-slot composition is preserved. This exports current state, rather than implementing the complete getHTML(options) Web API. Projection complexity is linear in light/shadow nodes; no additional measured performance numbers are available yet.

Integration: TestSnapshotPreservesCanonicalShadowComposition PASS; TestShadowProjectionDoesNotMutateCanonicalTree PASS. Fresh hash-verified V8 binary on endpoint 19608: independent custom element Chrome↔Mimic exact live-state match; exported declarative HTML→pinned Chrome 152 preserves open-shadow text and innerHTML, a closed nested root, and named-slot assignedNodes. Evidence: after-independent.json, after-export.html, after-receipt.json. body.innerText is not used as evidence of shadow rendering: Chrome itself returns only light text for this probe.

Production workload: a fresh GitHub→Chrome 152 export gave an exact match for 23/23 relative-time elements by datetime, light text, and shadow text. Visually checked chrome-export.png shows relative dates of 2 weeks/5 months/3 months and populated file rows. The only export warning is an upstream 404 for alert-fill-12.svg. Evidence: after-site.json; screenshot .build/shadow-export-domain/github/chrome-export.png. Production code contains no GitHub/relative-time logic.

Final review identified and independently confirmed a generic fragment-context mismatch: div shadow innerHTML='<tr><td>x</td></tr>' produced TR instead of Chrome's text x. The setter was corrected to use host context through the canonical parser bridge, without invoking a temporary custom-element constructor. TestShadowInnerHTMLUsesHostParserContext and the main export regression PASS after the fix. Before evidence: .build/shadow-review-before.json. Fetch network rejection was also changed to TypeError; custom stream-body errors and abort reasons preserve identity, and 4 targeted tests PASS. Final performance validation follows these changes.
