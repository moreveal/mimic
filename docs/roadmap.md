# Remaining technical debt

This is a backlog, not authorization to add features in a cleanup pass.

- Workers, ES modules/dynamic import, origin storage and synthetic CSS layout
  already exist. Remaining work is conformance and lifecycle depth.
- Keep generated surfaces separate from handwritten semantics; many generated
  operations still report missing behavior.
- QuickJS checkpoints use a bounded marker chain; goja automatically drains jobs
  at engine boundaries. Neither is equivalent to the primary V8 loop.
- Persistent engine values are rooted for the lifetime of a realm. Fine-grained
  handle release, bounded long-lived traces and CDP remote-object lifecycle need
  a deliberate ownership contract.
- CORS/cookies/cache, layout, graphics/media and Streams algorithms are incomplete.
  Scheduler source priorities and retained WindowProxy realms are intentional.
- Fresh upstream regeneration and exposure recapture were outside this offline
  stabilization pass; keep exact version checks in future maintenance.

Runtime behavior must remain generic and derived from canonical state. Never
patch target JavaScript or add website/vendor-specific decisions to the runtime.
