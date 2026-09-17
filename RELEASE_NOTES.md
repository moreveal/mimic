# Mimic v0.1.3

Changes since v0.1.2:

- Reduced repeated DOM, style, geometry, and inspector work by retaining validated
  projections and batching browser-visible observations.
- Added document-owned flat layout records and a JSON producer so CDP geometry
  reads share one revisioned layout state instead of rebuilding equivalent data.
- Improved iframe automation by preserving positioned descendant hit testing,
  mouse input, and focus transitions across frame boundaries.
- Reduced page startup, navigation, and wait overhead while tightening V8 realm,
  snapshot, module-error, and worker-handle ownership.
- Expanded the local runtime verification and release gates used for Windows and
  Linux packages.

Mimic remains a renderer-free public beta for Windows and Linux amd64. It models
browser-observable state for supported workflows; it does not provide complete
Chrome, rendering, media, or Web API compatibility. See the
[compatibility notes](https://github.com/moreveal/mimic/blob/main/docs/compatibility.md) and [performance report](https://github.com/moreveal/mimic/blob/main/docs/performance/report.md)
for current boundaries and measured tradeoffs.
