# Mimic v0.1.5

Changes since v0.1.4:

- Added browser-owned bootstrap artifacts shared across short-lived contexts,
  including a fingerprinted persistent cache with bounded retention, integrity
  validation, stale-entry cleanup, and deterministic shutdown.
- Reduced Page startup cost by publishing unused isolated-world identity without
  eagerly allocating a second V8/WebAPI realm. The realm materializes on its
  first executable operation while preserving init-script ordering and CDP
  lifecycle visibility.
- Added demand-driven WebAPI implementation domains for WebAudio, WebGL, and
  WebGPU while preserving the complete reflected surface, descriptors,
  constructor and prototype identity, native function shape, and realm-local
  state.
- Reduced memory for concurrent Pages through bounded V8 isolate pooling,
  compact generated shape metadata, lazy DOM attribute storage, and deferred
  image bitmap decoding.
- Fixed V8 context teardown so closed Pages release their native callback
  registrations instead of retaining unreachable realms and DOM state in a hot
  pooled isolate.
- Improved the official 100-page Campfire workload from the previous release
  architecture while keeping the full browser and engine test suites passing.
  Current local compiled runs complete in roughly 5.7--5.9 seconds with about
  222--244 MiB peak memory; results and methodology are recorded in the
  [performance report](https://github.com/moreveal/mimic/blob/main/docs/performance/report.md).

Mimic remains a renderer-free public beta for Windows and Linux amd64. It models
browser-observable state for supported workflows; it does not provide complete
Chrome, rendering, media, or Web API compatibility. See the
[compatibility notes](https://github.com/moreveal/mimic/blob/main/docs/compatibility.md) and
[performance report](https://github.com/moreveal/mimic/blob/main/docs/performance/report.md) for current boundaries and
measured tradeoffs.
