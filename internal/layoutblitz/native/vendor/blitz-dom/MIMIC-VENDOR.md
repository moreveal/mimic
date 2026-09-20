# Pinned Blitz DOM source

Source: https://github.com/DioxusLabs/blitz

Revision: `e7bf7bca452bc9194ac5c9bcf2ac3c0ebcf02e09`

Package version: `0.3.0-beta.2`. Preserve individual upstream source/asset
notices, including the MPL notice in the UA stylesheet, as well as license files.

`packages/blitz-dom` and its `stylo_taffy` interoperability dependency are
vendored from this revision. The upstream MIT and Apache-2.0
licenses are included. No renderer, compositor, shell, or GPU backend is
enabled by Mimic's dependency features.

The package manifest expands the upstream workspace dependency declarations
without changing their version requirements or features. Local upstream
dependencies other than `stylo_taffy` remain git dependencies at the same
revision, including `blitz-traits`, so public trait types have one Cargo identity.
Parley 0.11.1 is vendored separately for the CSS strut/baseline contract. The containing
native crate's committed Cargo.lock pins the resolved dependency versions.
The lock resolves Stylo `0.21.0`; the Blitz path pins Taffy revision
`4863877b9cab21645b160d7f247901f90214fe27`. The legacy fallback's Taffy
dependency remains separately identified in that lock.

Local compatibility changes:

- `:focus-within`, `:focus-visible`, and `:target` match the authoritative
  element-state flags instead of returning false.
- Author stylesheets are inserted by connected DOM tree order instead of
  node allocation order. The owner integration must re-register sheets when
  their tree order changes, even if source text is unchanged.
- `make_stylesheet_with_url` takes an independent validated stylesheet URL
  context. External relative URLs and CSSOM edits resolve against the canonical
  sheet URL rather than the document's current history/base URL. It does not
  temporarily mutate document URL state.
- Native layout records actual out-of-flow containing-block ownership and
  propagates inline candidates without changing canonical DOM ancestry.
- Closed details groups non-summary content in a retained anonymous skipped
  layout subtree. Observation eligibility is separate from real CSSOM boxes.
- Form-control intrinsic sizes use actual font metrics; inline intrinsic
  widths use LayoutUnit precision instead of whole-pixel rounding.
- CSS parent struts and atomic inline baselines are preserved in the shared
  Parley line product, including subsequent geometry/selection consumers.
- `resolved_style.rs`: shared scalar/batch serialization, a single temporary
  undisplayed-style resolution per batch, logical/alias mapping, CSSOM
  shorthands and resolved percentage origins. Initial content-visibility
  publication depends on embedding admission rejecting every authored
  declaration of that unsupported property; it is not authored-property support.
- `stylo_device.rs` and `document.rs`: reference-profile initial named font,
  color-scheme and writing-mode input, and internal-table geometry dispatch.
- `fonts.rs`: replacement of the active collection from canonical admitted
  resources, including removal and computed/inline invalidation.
- `intrinsic_images.rs` and image node data: canonical dimensions/completion
  without network re-fetch or native pixel allocation.
- Table layout and `table_geometry.rs`: captions contribute to actual layout;
  row/group rectangles come from actual tracks rather than spanning-cell unions.

## Enabled dependency surface

Mimic disables Blitz default features and enables only `system-fonts`.
Windows-target `cargo metadata --locked --offline` verifies Blitz
`system-fonts`; Parley `std,system`; adapter `std,block,flexbox,grid`; and
`image 0.25.10` with no decoder features. The resolved target graph does not
enable `blitz-paint`, `wgpu`, `vello`, `winit`, `anyrender` or `usvg`.
Geometric/color/path types and OS font discovery are still dependencies;
they do not require a renderer or GPU. The owner selects sequential style
traversal, and the `parallel-construct` feature is disabled.

Keep changes scoped and reviewable against the pinned revision. Never edit
Cargo's shared checkout/cache to apply compatibility fixes.
