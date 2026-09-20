# Pinned Stylo/Taffy interoperability source

Copied from `packages/stylo_taffy` at Blitz revision
`e7bf7bca452bc9194ac5c9bcf2ac3c0ebcf02e09`.
Package version: `0.3.0-beta.2`; repository
https://github.com/DioxusLabs/blitz. Manifest license expression:
`MIT OR Apache-2.0 OR MPL-2.0`; preserve file-level notices. Original MIT
and Apache-2.0 license files are included.
The manifest expands upstream workspace versions and retains the identical
pinned Taffy source. See the included upstream licenses.
That Taffy revision is `4863877b9cab21645b160d7f247901f90214fe27` from
https://github.com/DioxusLabs/taffy. Stylo is locked at `0.21.0`.

The local change replaces unconditional containing-block claims with CSS
position/transform/perspective/filter/containment and will-change claims.
The paired blitz-dom implementation retains actual hoisted-child ownership,
propagates inline candidates, and supplies the initial viewport area.
Canonical DOM and inline layout ancestry are not rewritten by hoisting.

Enabled adapter features are `std`, `block`, `flexbox`, `grid` (the default
adapter feature set). No rasterizer or GPU backend is included. The path
dependency applies the patch from this tree; no Cargo cache edits are needed.
