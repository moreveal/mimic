# WebAPI realm memory investigation, 2026-09-14

## Scope and decision

This investigation explains the large process-memory step when a CDP client first
connects and evaluates JavaScript, and screens lazy WebAPI publication as the main
optimization candidate. It does not retain a production change.

The current decision is to defer native lazy WebAPI bindings. Mimic already keeps
its strongest memory advantage over Chrome in multi-Page workloads, while the
correct JavaScript-only prototype saved too little to justify its complexity and
risk. Revisit this work if a stable single-Page workload remains above roughly
200--250 MiB private memory after settling, or if a 350--400 MiB case proves to
contain only one realm.

The measurements below are diagnostic observations, not a frozen reportable
comparison. The first control was built from revision `5950d2e` with two unrelated
preview files modified; the bounded prototype was based on clean revision
`74dc43c`. Workloads and harnesses were not edited, but the runs are not a matched
revision pair.

## What the CDP step creates

Mimic starts with a deferred runtime. A real Playwright `connectOverCDP` connection
materialized the initial `about:blank` Page realm even before navigation:

- before Playwright: about 18.7 MiB RSS and 78.9 MiB private memory;
- after connection: about 129.3 MiB RSS and 167.2 MiB private memory;
- one browser context and one Page were present.

This accounts for the observed jump commonly described as approximately 8 MiB at
startup and approximately 150 MiB after CDP connection. It is not merely CDP
transport state: it includes a V8 isolate, context, Window/DOM bindings and the
Chrome 152 WebAPI surface.

A simple raw-CDP navigation added about 12.6 MiB in the observed run. The
investigation did not reproduce 350--400 MiB with one simple top-level document.
That range should first be decomposed into live frames, workers, isolated worlds
and isolates; each realm owns a separate engine instance by design.

## Active Page attribution

The fast gate observed the following ten-Page process memory:

| Workload | Active RSS | Active private | Marginal RSS/Page | Marginal private/Page |
|---|---:|---:|---:|---:|
| static | 500.97 MiB | 551.14 MiB | 47.11 MiB | 46.99 MiB |
| React | 540.57 MiB | 596.16 MiB | 51.07 MiB | 51.48 MiB |

V8 density diagnostics after an explicit V8 collection attributed approximately
16--19 MiB of physical V8 heap to each live isolate. Natural active memory was
higher: collection reclaimed roughly 9--16 MiB private memory per Page, at a
median cost of approximately 6--9 ms per Page. Consequently, forcing a collection
on every Page creation is not a free memory optimization.

After Page teardown and Go collection/scavenging, the diagnostic process returned
to roughly 95--102 MiB RSS and 129--137 MiB private memory with no live isolate.
The main issue in these runs is active-realm cost rather than a retained-isolate
leak.

## Heap evidence

A V8 heap snapshot of a simple Page was approximately 21.9 MiB on disk. The
largest categories were:

- object-property backing arrays: approximately 5.1 MiB;
- strings: approximately 5.2 MiB, including a large bootstrap source string;
- objects: approximately 1.9 MiB;
- closures: approximately 1.4 MiB;
- code: approximately 1.3 MiB.

This supports the WebAPI graph as a real memory target: a small document still
retains thousands of interface constructors, prototypes, methods, accessors and
their metadata.

The Go heap profile after Page close retained approximately 8.3 MiB in parsed
catalog/exposure data and approximately 1.5 MiB in V8 code cache. These are mainly
bounded process-level costs, so reducing them would help the x1 headline but not
the per-Page slope.

## Snapshot counterfactual

Disabling the bootstrap snapshot did not provide a safe general win. At ten Pages,
natural memory varied by workload and was sometimes worse without the snapshot.
After explicit V8 collection, the no-snapshot runs retained about 5--7 MiB less per
realm, but they give up bootstrap reuse and increase transient construction work.
A simple snapshot disable is therefore rejected.

## Lazy WebAPI experiment

Two disposable prototypes were tested in an isolated worktree:

1. An intentionally incompatible upper bound omitted generated fallback members.
   It reduced marginal memory by approximately 8.4--8.8 MiB per Page, around
   17--18%, while the small performance workloads still completed. This establishes
   useful potential, not an acceptable implementation.
2. A JavaScript `Proxy` implementation preserved configurable descriptors,
   reflection, key order, identity on first access and native-looking function
   metadata. Focused surface, exposure, descriptor, prototype and function-source
   tests passed. It was restricted to fully generated prototypes because replacing
   an existing handwritten prototype would break prototype identity, `instanceof`
   and realm behavior.

The correct bounded implementation saved only about 0.5 MiB/Page on static and
about 2.5 MiB/Page on React in the screening runs. The DOM completion median was
approximately 7--8% worse in that run, while static and React did not show a
convincing regression or improvement. Given the unmatched diagnostic revisions,
the exact deltas are not reportable; they are sufficient to reject the JavaScript
approach as a production change.

The prototype also exposed a large penalty in goja when Proxy traps interacted
with its global access observer. The experiment ultimately enabled laziness only
for V8, which further reduced its architectural value. No prototype code or build
artifact was retained.

## Viable future implementation

The pinned `gov8` dependency exposes `Object.SetLazyDataProperty` and native
accessor APIs. A future proof should use those mechanisms on the original
prototype objects instead of substituting Proxy identities. Start with one or two
large interfaces and prove all of the following before expanding the surface:

- exact `getOwnPropertyDescriptor`, `Reflect.ownKeys`, deletion, redefinition and
  `preventExtensions` behavior;
- stable constructor/prototype identity, inheritance and cross-realm behavior;
- native-looking callable metadata and first/subsequent access costs;
- callback/data ownership across bootstrap snapshot creation and restoration;
- complete release after Page teardown;
- matched clean-revision memory and latency gates.

Until that proof exists, native lazy publication belongs in the backlog. It is a
plausible way to recover much of the measured 8--9 MiB/realm upper bound, but it is
not critical to Mimic's present advantage and should not displace compatibility,
stability or reproducible investigation of unexpectedly duplicated realms.
