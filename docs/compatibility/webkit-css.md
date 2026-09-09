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

Six oracle fixtures run on both V8 and Goja:

- `values`: accepted and rejected keyword values for appearance and twelve
  related families, including CSS-wide values and specified variable references;
- `aliases`: spelling, existence, priority, invalid-write preservation and removal;
- `computed`: initial values and explicit/implicit inheritance for these families;
- `rules`: the same declaration rules in constructed stylesheets;
- `supports`: declaration queries and boolean support conditions;
- `longhands`: spelling, canonical storage and removal for 135 single-component
  properties using the CSS-wide `initial` value. This is not a value-grammar test
  for every property.

Computed keyword observations are resolved lazily. Reading an unrelated style
must not trigger extra inheritance traversal or geometry work. Declaration
splitting respects quoted strings, comments and nested token blocks, including
semicolons inside custom property values.

## Remaining work

The inventory oracle is diagnostic, not a passing conformance fixture. Its 16
multi-component declarations still need shorthand expansion, indexed component
enumeration, recombination and removal. The grammar and computed-value behavior
of the remaining properties are not established merely by the alias table.
Examples include legacy mask/break value conversion, colors, lengths,
transforms, animations, transitions, border and text shorthands.

CSS escapes, full variable substitution/cycle handling, arbitrary support
conditions, and cascade layers are not yet implemented comprehensively. The
keyword computed-value helper does not claim complete UA stylesheet behavior
for form controls or pseudo-elements. Non-CSS `webkit*` APIs require a separate
behavioral audit; this checkpoint is not evidence of their completeness.

No renderer, GPU or native graphics backend is introduced.
