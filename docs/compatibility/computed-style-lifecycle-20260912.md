# Computed declaration lifecycle

Chrome 152.0.7977.82 headful resolves an empty CSSStyleDeclaration for a detached
element or a node whose owner document has no active browsing context. The
declaration stays live across insertion, removal and adoption. `display:none`
on the element, an ancestor or the connected embedding iframe does not make
the declaration empty. This was a related lifecycle defect, but subsequent
offline replay showed it did not fix `OjmeV1[82]`; see
`computed-style-flat-tree-20260912.md` for the actual closed-shadow-root cause.

Resolution now checks canonical DOM connectivity and the owning realm's active
document, with the existing synthetic shadow-root ownership supplement. It does
not depend on author replacements of `isConnected`, `ownerDocument` or
`defaultView`. Rule lookup also follows canonical document ownership. Adjacent
CSSOM corrections cover Element argument branding, absent indexed properties
and the empty priority of computed declarations.

The synthetic frozen oracle covers 28 observations, including fragments,
detached subtrees/shadow trees, inert documents, re-adoption, cross-realm
borrowing, iframe removal, display/visibility suppression and live declaration
reads. Two fresh headful profiles agreed; the fresh Mimic binary matched every
observation. Full local launch receipts and raw CDP evidence remain under
`.build/style-lifecycle-compat-delegated/style-final2` in the main checkout.
`internal/browser/testdata/computed_style_lifecycle_chrome152.json` retains
browser/profile metadata, binary and probe hashes and the frozen expectations.

Validation: the focused oracle passes in ordinary and restored bootstrap realms;
existing computed-style, stylesheet, cross-realm-node, document and shadow tests
selected by `TestComputedStyle|Test.*Stylesheet|Test.*CrossRealmNode|Test.*DocumentCompat|Test.*Shadow`
pass. This changes declaration eligibility, not layout, painting or general CSS
property coverage. Noncanonical opacity serialization such as `.5` remains a
separate pre-existing value-serialization limitation; the lifecycle probe uses
the equivalent canonical `0.5` spelling.
