# Architecture

The dependency direction is intentionally one-way:

```text
CDP / future APIs -> browser commands -> canonical state machine
                                      -> scheduler
                                      -> resource loader -> interceptors -> transport
                                      -> Web API semantics -> engine interface
```

Concrete Chrome versions live under `chrome/<milestone>`. The `chrome`
registry returns a version-neutral `compatibility.Bundle` containing the exact
environment profile, generated Web API surface, generated CDP schema and
expectations. Browser realms and the CDP adapter consume that selected bundle;
the runtime core never imports `chrome/152` (or any future version) directly.

`internal/engine` is the only contract visible to browser code. The V8, QuickJS and goja
adapters live below it. Production browser code does not import a concrete engine.

## State

`Environment` stores canonical facts only: product, platform, hardware,
display, graphics, locale, clock, network and permissions. JavaScript-visible
values and request headers are derived projections. Construction validates
cross-field invariants; mutation occurs through commands on `Page`.

Each page owns a top frame, each frame owns a realm, and each realm owns a
separate engine instance and global object. WindowProxy is represented as the
stable frame-facing identity which delegates to the current realm.

## Scheduling

Go concurrency may perform implementation work, but never invokes JavaScript.
All callbacks enter through Scheduler tasks. A microtask checkpoint is made
after every task. Timers use a monotonic virtual clock and deterministic
sequence numbers.

## Networking

Navigation, fetch, XHR, external scripts and dynamically inserted scripts call
the same ResourceLoader. Interceptors receive immutable request/response views
and return explicit decisions. Cookie/header derivation is performed around
that common path.

## Web APIs

`internal/webapi/surface.js` contains handwritten semantic bindings. It is
composed per realm with the selected bundle's generated Blink WebIDL surface.
Generated bindings describe known shape and route operations to semantic hosts;
they never own independent browser state.

## Ownership and execution invariants

- Browser environment is a validated construction template. Page environment is
  its command-owned snapshot; Navigator, Screen, viewport and graphics capability
  bindings derive from it. Returned profiles/environments are read-only to callers.
- Page current URL/history is authoritative for the top document. Realm documentURL
  delegates to it; child document URLs and Worker script URLs are realm-owned.
  Realm origin is the committed security origin, not an independently mutable URL.
- Context owns cookies, origin storage, network/session settings and the transport
  pool. Loader cache/body records and trace data are derived observations. Transport
  connection/TLS/session state is never replaced by a guessed browser identity.
- DOM Document owns nodes/attributes. JavaScript wrappers and cross-realm value
  registries preserve identity; they do not own a second DOM. Removed frames can
  retain a realm for existing WindowProxy references until the owning realm closes.
- Page clock establishes navigation epochs; scheduler time is authoritative during
  a turn, including microtasks. Worker schedulers have their own agent clock.
  Resource/Navigation Performance entries derive from loader/lifecycle records;
  user marks/measures are authored within their own realm.
- CDP sessions share one server command mutex; protocol interception continuation
  remains out of that mutex so a paused network request can be resumed. Library
  users must serialize Page/DOM command access too; a Page is not a concurrently
  callable JavaScript engine. Worker JS may run on its separate agent, never on
  the Window runtime. Cross-agent delivery always queues a receiving-agent task.
- Scheduler runMu serializes a task and its complete checkpoint. V8 uses explicit
  microtasks on its thread-affine owner. Do not restore V8 automatic checkpoints.
- Network goroutines perform external work and enqueue completion; fetch/XHR and
  resource work is cancelled/joined when the realm closes. Cancellation guards
  suppress late Worker delivery. Closing a Context closes pages then its pool.
- V8 cancellation watchers are joined before the next isolate operation; persistent
  values and modules are released on the isolate thread at Close. Values currently
  remain rooted until realm teardown; long-lived realm memory is technical debt.
- Keep runtime behavior target-blind. Domain names, vendor tokens, challenge
  patterns and captured payload structures must never select runtime semantics.

Do not remove retained realms, cross-value registries, explicit checkpoints,
transport session pooling or timer source ordering as apparent duplication.
Any change to these invariants needs a local regression demonstrating the defect.
