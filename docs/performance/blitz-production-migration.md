# Production migration to blitz-dom

Status: in progress; native owner, C ABI, Go wrapper and canonical snapshot
projection tests pass; browser consumers are not migrated.

The old JavaScript style/layout producer is frozen for performance work. Its
remaining roles are a comparison oracle and an explicit migration fallback.
Compatibility disagreements must ultimately be resolved against Chrome 152;
the old producer is not authoritative when it disagrees with Chrome.

## Ownership

The canonical Go Document owns a persistent native derived Document. Native
node handles are mapped from canonical IDs, never from selectors or synthetic
attributes. Navigation and Document disposal destroy the native owner. V8
worlds borrow observations from that owner and do not own producer instances.

The existing connected mutation journal supplies invalidation hints, but has
structural/state gaps and bounded retention. A missing revision requires an
explicit resynchronization. It must never authorize reuse of stale geometry.
Incremental input transport needs actual canonical attribute, text, topology,
state and resource values; the journal itself is not a replay log.

## Implementation sequence and acceptance

1. Persistent native owner with canonical IDs, mutation application and clean
   readback. Implemented starting in `internal/layoutblitz/native`; initial
   tests cover style mutation, dirty-read rejection and concurrent Documents.
2. Checked C ABI and Go lifetime wrapper. All calls serialize within the Page;
   no global browser lock. Panic/error handling poisons a transaction and
   requires fallback or resynchronization, never mixed-generation readback.
   Initial ABI and Go wrapper implemented and tested. Legacy Taffy is exported
   through the Blitz native archive to link exactly one Rust runtime.
3. Canonical snapshot plus incremental reconciliation. Batch mutations before
   Stylo resolution. Add deletion, reparenting, namespace and overflow tests.
   Initial canonical snapshot projection implemented. Snapshot clones input
   under one DOM read lock. Incremental reconciliation remains outstanding.
4. Feed CSSOM, adopted stylesheets, pseudo states, fonts, decoded resource
   dimensions, viewport and scrolling inputs. Define explicit unsupported
   feature reasons. Do not silently accept missing inputs.
5. Publish geometry and computed values to both worlds. Migrate visibility,
   scrolling, IntersectionObserver and input together to one consistent
   producer generation. Keep existing lifecycle ordering and clipping rules.
6. Run focused Chrome comparisons and the unchanged Wikipedia workload early.
   Record first-build/rebuild/reuse, fallback reasons and producer work by
   stage. Finish with five alternating fresh-process cold/warm pairs, memory,
   teardown and race gates.

The simple standalone Blitz measurement is not a correctness proof or an E2E
speedup. Its HTML parse excludes canonical DOM extraction and serialization.
Its timing does not cover production font loading or shaping compatibility.
The 10k hit-test timing alone does not prove algorithmic complexity; establish
scaling before redesigning it. There is no accepted production performance
result until the complete consumer path has been exercised.

## Outstanding correctness boundaries

The initial native owner is deliberately not connected to browser consumers.
It currently lacks topology validation, removals, external stylesheet/resource
delivery, pseudo state changes, animation lifecycle updates and font setup.
Its clean-read fast path is only valid after those dependencies are wired into
invalidation. It must not be enabled as the default producer in this state.

Blitz sequential style traversal is selected because the pinned implementation
documents an overlapping parallel traversal hazard across Documents. The
initial four-thread independent-Document test passes without a global lock.

No further optimization of the old producer is part of this migration.
