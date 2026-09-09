# Template content domain (2026-09-09)

The implementation uses canonical fragment nodes for HTML template contents.
The template-to-content link is separate from ordinary parent links. No duplicate
JavaScript child array, alternate renderer tree, or fabricated Document was added.

## Coverage and oracle

| Test group | Mimic before | Mimic after | Pinned Chrome 152 |
|---|---:|---:|---:|
| Unmodified pinned template WPT, five files | 0/232 | 1/232 | 232/232 |
| Independent current-document template fixtures | 2/11 | 12/12 | 12/12 |
| Canonical Go template tests | not present | 3/3 | not applicable |

The additional twelfth fixture covers inert custom-element construction and
activation. It was first reproduced independently against the intermediate
implementation, then fixed and compared with Chrome.

**The WPT suite is not claimed as covered.** All five harnesses complete, but
227 after failures stop at the unsupported `document.implementation.createHTMLDocument`
helper. Three more encounter missing/null nodes from the fixture setup. One
explicitly observes that template content still reports the active ownerDocument,
where Chrome uses a distinct inert template document. The single passing test
checks template hierarchy. The current-document fixtures isolate actual template
algorithms while retaining these original WPT failures in the coverage matrix.

Oracle: headful Chrome 152.0.7977.82, Chromium commit
`d04cdb24d67b081f6cf80200ffc5233f44b61109`, WPT tree
`5251311032215fe14bbd74cd0867e14cb4b58e3f`. Executable SHA-256, listener PID/path,
source hashes, and metadata accompany the captures in
`.build/hydration-complete/templates-wpt-final.json` and
`templates-final-{hash,process}.json`. Chrome WPT evidence is `templates-oracle.json`.

## Implemented behavior

- The existing `golang.org/x/net/html` tokenizer/tree builder still parses HTML.
  Its parsed template children are adopted into canonical template fragments
  immediately, for initial documents and fragment parsing.
- Document queries, textContent, ordinary child traversal, parser script discovery,
  title and meta processing do not traverse template content.
- `content` has stable identity. `innerHTML` parses in template context and
  replaces content through the shared mutation transaction, preserving observer
  target and aggregated records.
- Nested content and manually appended ordinary template children remain distinct.
  Canonical HTML serialization chooses template content, while ordinary DOM text
  traversal chooses ordinary children.
- Deep cloning/importing templates copies each content tree once. Script
  already-started flags survive cloning. Initial template parser scripts remain
  inert until activation; fragment-parser scripts stay inert when cloned.
- Template-host-inclusive insertion cycles are rejected before mutation.
- Custom elements parsed/cloned within template content remain unupgraded until
  activation in the active document. Explicit normal active-document construction
  keeps its ordinary behavior.
- Selected compatibility catalogs without HTMLTemplateElement or CSS still
  initialize successfully.

## Limits

Alternate/inert Document identity, XML/XHTML template fixtures, cross-realm owner
adoption, declarative shadow DOM, and complete parser lifecycle conformance remain
separate domains. Current ownerDocument behavior is an explicit known limitation.
The independent fixtures do not substitute for the helper-blocked WPT suite.

`TestTemplateChrome152`, canonical template tests, selected-catalog tests, and
`TestMutationReactionChrome152` pass on the final source. Performance validation
and final production workload captures are recorded by the enclosing change; the
fresh prior profile was `forms-profile-before`. Frozen benchmarks were not edited.
