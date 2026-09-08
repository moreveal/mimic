# Next bounded ownership experiment

Status: the reserved-ID importer and host bridge are implemented in the
detached spike; automatic JS ownership and boundary hooks are not implemented.
No performance gain is claimed for this foundation.
Production reference: catalog filter 52a2aa2. Fresh profile:
`catalog/profile/phases.json.gz`. The earlier independent JS kernel demonstrates
that removing host calls can help, but is not a compatible replacement DOM.

The bridge's Go tests cover atomic rejection of cycles, duplicate/unreserved
IDs, missing/duplicate children, inconsistent parents and resource elements;
failed imports preserve reservations for a valid retry. Input slices/maps cannot
alias the canonical Go nodes. V8 and Goja tests create the actual existing WebAPI
wrappers before importing their records, then verify the same wrapper references
through child/parent/query lookups and insertion. Go mutations after import remain
visible to the retained JS object. This test uses a test-only closure probe;
normal `document.createElement` is deliberately not switched to deferred storage.

Reproduction patch and validation are under `detached-bridge/`. The next code
step is a private pending-node store plus comprehensive engine entry/exit hooks;
the correctness conditions below still apply before measuring a complete fast path.

The next useful experiment should keep newly created, detached, plain DOM
subtrees in JavaScript and import their state into Go in one operation when a
Go consumer needs them. Reuse existing wrappers, events, and WebIDL bindings.
Do not introduce a second public document or specialize the frozen workload.

## Boundary found in the current implementation

`Realm.Evaluate` is not the only JavaScript entry point. Timer, posted-message,
performance-observer, transport, and frame callbacks call `runtime.Call`
directly; modules and microtask checkpoints are separate entries. Flushing only
after `Evaluate` would silently lose observable mutations in these paths.
Exceptions must also leave completed mutations visible. Resource insertion and
cross-realm access can re-enter Go before a top-level evaluation finishes.

The Go `Page.Document()` API exposes the canonical document. Existing tests
mutate its attributes between evaluations and require previously obtained JS
objects/classList instances to see those writes. `Document.Find` also searches
detached nodes in the current implementation. A JS-only cache cannot silently
replace either behavior. Library callers sharing a Page already need its command
boundary; that boundary must remain per Page, not become a global runtime lock.

## Minimal candidate

1. Reserve stable node IDs in blocks. Existing wrappers keep their IDs and
   identity when their data is materialized. Never replace an object on import.
2. Keep pending node records private to the realm. Initially include only plain
   HTML elements, text, and comments. Resource-bearing elements use the existing
   eager path. Unknown operations first materialize pending state.
3. Handle local attribute writes and subtree insertion with synchronous JS
   validation. Validate cycle/reference errors before mutating either tree.
   Keep the existing synthetic fragment membership rules.
4. Import pending records atomically into Go, validating IDs, parent/child
   consistency, and node types first. Clear pending data only after success.
5. Materialize before any host query/operation which consumes the nodes and at
   every outermost JS/task/microtask boundary, including error returns. Once
   materialized, use the canonical Go path. Do not cache externally mutable
   attributes without an explicit invalidation mechanism.

A correct entry/exit hook must cover all engine entry points before expanding
the fast path. Audit reentrancy and navigation/retired-realm teardown separately.
If safe hooks require a larger ownership rewrite, keep the experiment detached
and report that limitation rather than hiding it behind a benchmark wrapper.

## Evidence required

Use the unchanged frozen DOM/static/React workloads and hash-verified binaries.
Compare completion, execution, crossings, throughput, active/recovered memory,
and close tails. Include the cost of import and boundary synchronization.
Correctness must cover stable references before/after materialization, fragments,
moves, cycle errors, Unicode, Go queries after JS writes, JS reads after Go writes,
timer/promise/module/throw paths, resource insertion, and independent Pages.

This candidate leaves connected attribute writes on the Go path and is not
expected to reproduce the full independent kernel's speedup. Predicting a
whole-browser Chrome lead from the old kernel CDP timing is invalid: Chrome's
native in-page DOM time was lower than the JS kernel's. Only a measured end-to-end
result can justify expanding ownership further.
