# Production migration to blitz-dom

Status, 2026-09-20: native production is unconditional for admitted Documents.
There is no environment switch to select the legacy engine. Legacy is only an
internal semantic fallback and test oracle, to be eliminated as migration
coverage grows. This is **not an
unconditional native feature-coverage claim**: unsupported Documents use the
explicit compatibility fallback below. Consult dated reports for the exact
source/binary and correctness/performance receipts; older checkpoint results
predate the default switch. The standalone Blitz PoC is not an E2E result.

The old JavaScript producer is frozen for performance work. Its roles are
correctness comparison and explicit migration fallback. Frozen Chrome 152
remains the reference when old Mimic and native behavior disagree.

Build native changes with `python tools/build_native_layout.py` before Go.
The helper records the archive hash in a generated cgo-package header, making
Rust-only changes invalidate Go's build cache. Raw `cargo build` alone does not
do this: a following cached `go build`/`go test` can otherwise use an old linked
archive. Diagnostic direct-Cargo builds must force a fresh link explicitly.

## Ownership and execution

Canonical identity, attributes, topology and text remain in the Go DOM.
`Realm.blitz` on the main-world owner holds one `layoutblitz.Document`; its
`Owner` holds the native `BaseDocument` and canonical-ID/native-ID association.
Native DOM nodes are a derived engine projection, not browser-visible objects.
Do not give JS wrappers native identity or create an isolated-world producer.

`blitzObserve` resolves the main-world owner and uses the existing nested-runtime
and owner execution boundary before preparation/readback. Go-owned results are
converted into the requesting realm only after the owner call completes; V8
values are not transferred between contexts. One Page event loop serializes
its mutations and native queries. Independent Pages have independent owners;
there is no global browser lock. Native Stylo traversal is explicitly sequential
because the pinned implementation has a concurrent parallel-traversal hazard.

The C ABI copies caller data; no Go pointers survive native calls. Failed native
transactions/panics poison the handle and return an error, not plausible stale
results. A failed observation is not silently changed into a successful native
read. Controlled unsupported-feature admission uses the separate fallback path.

## Current input/invalidation transaction

`Realm.prepareBlitz` checks a key including realm/document revision, style and
resource revisions, viewport, selector target, color scheme, reduced motion and
base URL. An unchanged key is a preparation reuse, not a new producer build.
Font collection mutation already invalidates the canonical observation epoch.

On changed input, `Document.Sync` captures the connected canonical tree under
one DOM read lock. It reconciles attributes, text and topology against its prior
records, reusing native nodes. Moves/removals detach before reparenting. Removed
nodes are retained for identity-safe reinsertion until the bounded tombstone
threshold requires a new derived owner. Only changed values are applied to the
native engine; a changed outer key does not itself force native layout.

This is **snapshot-and-diff input reconciliation**, not a completed selective
mutation-journal transport. The existing journal is not claimed as a complete
native dependency graph. Capture/reconciliation cost must stay in accounting.

Further inputs come from existing owners:

- CSSOM supplies connected owner stylesheet text and order, with canonical
  node IDs and a stable per-sheet URL context captured on sheet creation.
  External sheets use their canonical resource URL, including for later CSSOM
  edits. Source/base changes, removal and reordered sheets update native Stylo;
  a document history transition does not rebase an existing sheet.
- Private control/focus slots supply checked/focus/focus-visible/focus-within
  state without calling overridden author getters; URL target is a state bit.
- The existing font loader exports admitted/normalized resource bytes. Active
  FontFaceSet changes replace native collection membership, including removals;
  they invalidate line products and font-dependent computed lengths. No second
  fetch or independent font resource cache is introduced.
- Canonical image resources supply intrinsic dimensions/completion, without
  native pixels or re-fetch. Viewport, color scheme and base URL reach native
  device/style input before resolve.
- Existing canonical parsed inline declarations retain precision separately
  from serialized attributes. The native declaration block consumes parsed
  values without changing the attribute seen by selectors. Geometry consumes
  numeric transform matrices/origins, not rounded CSSOM matrix strings.

The native generation advances only when native work was required. Its packed
snapshot survives outer-key changes that leave the generation unchanged. When
generation changes, publication is discarded before serving the new state.

## Published products and consumers

`snapshot.go` publishes schema 2: sorted canonical element IDs, eight geometry
values, eligibility flags, and the fixed 17 style columns required by existing
visibility/scroll/IO/input consumers. Strings share a UTF-8 pool. Each world
binary-searches typed bytes rather than reconstructing per-element maps or
document-scale wrapper graphs. The publication cache is generation-bound.

An undisplayed descendant has no primary style from native traversal. The
document packet marks its style columns unresolved rather than eagerly
computing them all. The first demand obtains a single-node packet whose style
batch resolves the hidden computed state once. It remains temporary: it is not
installed as fake primary style and does not create a layout box. Scalar CSSOM
and batch CSSOM share the same native serializer.

Native products feed computed style, box geometry, visibility, existing scrolling,
IntersectionObserver sampling and hit-test/input consumers in both worlds.
Those consumers retain their lifecycle, clipping and event-loop semantics;
this migration does not suppress observers or trust a previous actionability
target in place of hit testing. JS remains the observable API/lifecycle layer,
not an optimized second style/layout producer.

Closed-details geometry and observation eligibility are distinct native products:
CSSOM can expose the retained box while flags exclude skipped content from
visibility, intersection and input. Table row/group rectangles use actual layout
track intervals, not unions that incorrectly include entire rowspan cells.

## Admission and fallback

Current whole-document fallback reasons include shadow/slots, adopted sheets,
quirks mode, dynamic animation lifecycle, child-frame viewport integration,
reduced-motion integration, and unsupported active-font descriptors/indexes or
collection bounds. Keep these reasons explicit in `blitzFallback` traces.
Connected code-unit `TextJSON` representation also requires fallback, before
any native mutation; removal/replacement returns to native production.
Nonempty textarea content, private input value divergence from its default
attribute, and select listbox modes need the live-control content adapter and
also fall back. Empty textarea geometry remains native. Admission reads private
form slots, not author getters. Search typing can therefore incur fallback
before navigation; its cost must remain in the full workload result.

Servo Stylo does not implement authored `content-visibility`. Eligibility reads
the existing parsed stylesheet declarations and parses distinct canonical inline
sources, including escaped CSS property names. Any such declaration sends the
document to fallback. Therefore eligible native documents may safely publish
the initial `visible` value without running legacy cascade per node. CSSOM
removal invalidates eligibility; strings and custom-property payloads are not
mistaken for declarations.

Unsupported scalar property serialization uses the old CSSOM oracle rather than
invented defaults. `alignment-baseline` and `place-items` have explicit property
fallbacks for pinned-engine initial/serialization differences. Chrome's used
`user-select` inheritance and SVG `transform`/`font-size` use selective existing
semantic projections. Other common SVG style columns remain native. Do not interpret
fallback success as native feature coverage or exclude its cost from results.

## Lifetime and bounds

- Native owner is released on owner execution during realm/document teardown;
  close is idempotent. Publication/input handles and Page resources are released
  along with their owners. Navigation must not retain a prior Document's packet.
- Retired-node count above live-node count plus 1024, or estimated retained input
  bytes above 16 MiB, causes reconstruction in the same detaching transaction.
  The byte estimate includes canonical parsed declarations, not total native heap.
- Packed records/string pool are bounded at 256 MiB; oversized publication is an
  explicit error, not truncated state.
- Font collection transfer is bounded at 4096 faces and 64 MiB. Replacement
  discards old collection/source-cache membership; temporary transfer copies
  remain part of memory accounting.
- Hidden style packet/string decoding is scoped to existing observation cache
  generations, not an unbounded cache keyed by arbitrary authored strings.
  Decoded hidden/scalar strings share a 64 MiB budget; scalar keys and numeric
  transform caches have a 4096-entry cap per generation.

These are implementation bounds, not measured zero-retention claims. Keep native
RSS, Go heap, per-world byte copies, retained nodes, and teardown together in the
memory gate. `Builds`, `Updates`, `Reuses`, native generation and invalidation
overhead must be reported separately.

## Build and vendor reproducibility

`tools/build_native_layout.py` builds the committed native manifest/lock with
`--release --locked` for Windows GNU or Linux x86-64. The archive exports both
new Blitz and old Taffy entry points, avoiding two linked Rust runtimes.

Vendored sources carry `MIMIC-VENDOR.md` and original licenses. The enclosing
native `Cargo.lock`, vendor manifests/source and Go/JS adapters must all be in
the commit; binaries and `target` are not source artifacts. No fix requires
editing a shared Cargo checkout or registry. The feature audit used:

```powershell
cargo metadata --manifest-path internal/layoutblitz/native/Cargo.toml --locked --offline --format-version 1 --filter-platform x86_64-pc-windows-gnu
```

It resolved only `system-fonts` for Blitz, `std,system` for Parley, and no
renderer/GPU packages (`blitz-paint`, `wgpu`, `vello`, `winit`, `anyrender`,
`usvg`). `image` has no decoder features. A locked incremental Windows release
build passed at this checkpoint. An empty-target-directory rebuild and Linux
validation were **not** performed by this audit. Unfiltered offline metadata
required an uncached non-Windows dependency; that is not a Windows build failure.

## Acceptance remains end-to-end

The gate is the full unchanged Wikipedia E2E, five alternating fresh-process
cold/warm pairs against the explicit clean production baseline, plus controlled
observation/input chain, stage/accounting displacement checks, correctness,
race, memory and teardown. Stage-local speed, standalone native resolve time,
or a passing document that used fallback cannot substitute for this gate.
