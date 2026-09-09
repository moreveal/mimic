# WebKit CSS observations

Reference: headful Windows Chrome 152.0.7977.82, captured through an isolated
local document by `tools/compatibility/capture_query_oracle.py`. The retained
`webkit-css-*-chrome152.json` files contain fixture hashes and raw observations.

CSS property spelling and JavaScript property spelling are separate namespaces.
The measured table has 151 `webkit*` JavaScript names. Prefixes are not simply
stripped: `webkitBorderBefore` denotes `border-block-start`, for example.
Declarations store canonical names, so aliases share values and priority.
CSSOM method arguments are case-insensitive CSS spellings; camelCase spellings
are only accepted through JavaScript property access.

Eleven oracle fixtures run on both V8 and Goja:

- `values`: accepted and rejected keyword values for appearance and twelve
  related families, including CSS-wide values and specified variable references;
- `aliases`: spelling, existence, priority, invalid-write preservation and removal;
- `computed`: initial values and explicit/implicit inheritance for these families;
- `rules`: the same declaration rules in constructed stylesheets;
- `supports`: declaration queries and boolean support conditions;
- `longhands`: spelling, canonical storage and removal for 135 single-component
  properties using the CSS-wide `initial` value. This is not a value-grammar test
  for every property.
- `shorthand-wide`: all 16 multi-component properties with CSS-wide values and
  pending variable references, including component order, readback and removal;
- `shorthand-mutation` and `shorthand-rule-mutation`: component edits and mixed
  priority in inline and stylesheet declarations;
- `state`: cloning, same-value attribute writes, cssText reparsing, namespace/Attr
  writes, removal and custom-element reaction visibility.
- `reflection`: WebKit name enumeration and indexed/alias property descriptors.

Inline parsed declarations belong to the canonical DOM node, alongside their
attribute serialization. They cannot always be reconstructed from that string:
Chrome exposes empty components after partially replacing a pending shorthand.
An immutable JSON representation crosses the realm boundary; it contains CSS
declarations only, not engine objects. Same-value attribute writes preserve it,
changed attribute writes invalidate it, cssText reparses explicitly, and cloning
copies it. Stylesheet blocks own the corresponding parsed declaration state.

Computed keyword observations are resolved lazily. Reading an unrelated style
must not trigger extra inheritance traversal or geometry work. Declaration
splitting respects quoted strings, comments and nested token blocks, including
semicolons inside custom property values.

## Remaining work

The inventory and ordinary shorthand-values oracles remain diagnostic, not
passing conformance fixtures. The 16 multi-component declarations still need
ordinary value grammar and component-specific recombination beyond CSS-wide and
pending values. The grammar and computed-value behavior
of the remaining properties are not established merely by the alias table.
Examples include legacy mask/break value conversion, colors, lengths,
transforms, animations, transitions, border and text shorthands.

CSS escapes, full variable substitution/cycle handling, arbitrary support
conditions, and cascade layers are not yet implemented comprehensively. The
keyword computed-value helper does not claim complete UA stylesheet behavior
for form controls or pseudo-elements. Non-CSS `webkit*` APIs require a separate
behavioral audit; this checkpoint is not evidence of their completeness.

No renderer, GPU or native graphics backend is introduced.
