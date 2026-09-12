# Exact cause of OjmeV1[82]

The earlier disconnected-document correction was real but did not change this
payload field. Fresh offline replay still produced true in both packet cycles.

Targeted observation at VM IP 159914 (second cycle 46910) shows a div with class
YYXc3 appended directly to BODY, and `.YYXc3 { display: flex; }` appended in HEAD.
The element is connected, its owner is the live caller document, defaultView
exists, readyState is complete and document.hidden is false. Chrome's pierced
CDP DOM snapshot additionally shows **a closed shadow root on BODY**. The probe
div is unassigned light DOM, outside the flat tree. Its computed display is empty
in Chrome and was flex in Mimic. Neither iframe visibility nor document activity
explains that difference.

Computed declarations now separate active-document eligibility from flat-tree
participation. Disconnected/inactive nodes have empty declarations. Connected
nodes excluded by shadow distribution retain the indexed property inventory but
return empty property values, as measured in Chrome. Named/default slot lookup,
first-slot selection and suppressed fallback content derive from the existing
DOM and shadow maps; no second mutable distribution state was introduced.
Foreign-node eligibility runs through a private callback in the owner realm so
closed roots remain private and getComputedStyle borrowing still works.

The SLOT wrapper now uses HTMLSlotElement. Element.slot and HTMLSlotElement.name
reflect canonical attributes through the captured attribute mutation operation,
so reassignment by property writes has the same effect as attribute writes and
does not invoke author replacements of getAttribute/setAttribute.

Evidence and validation:

- 23 frozen synthetic groups agree across two fresh headful Chrome
  152.0.7977.82 profiles and Mimic: open/closed roots, unassigned subtrees,
  named/default assignment, duplicate-slot ordering, suppressed fallback, live
  insertion/removal/moves, nested roots and both directions of realm borrowing.
- The frozen regression passes in ordinary and restored bootstrap realms;
  existing computed-style/shadow/stylesheet/cross-realm-node tests pass.
- Replay of the actual saved programs after this correction produces
  **OjmeV1[82] = false in both packet 2 and packet 6**, matching Chrome.
  All 10 input/output packet pairs are complete and match requests, with no
  parsing errors.

Private diagnostics remain in the main checkout under
`.build/style-residual-delegated`: `chrome-shadow/dom-*.json`, `flat-tree-final`
and `replay-after`. The portable synthetic source, oracle metadata and hashes
are retained in `internal/browser/testdata/computed_style_flat_tree_*`.

This changes computed-style observation, not rendering/layout. Manual slot
assignment and the complete slot-event/API subsystem remain existing unsupported
areas; no new claims are made about them. The previously limited computed CSS
property inventory is not expanded into a complete Chrome property catalog.
Performance and Trusted Types behavior are unchanged.
