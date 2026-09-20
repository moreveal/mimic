# Mimic v0.1.4

Changes since v0.1.3:

- Added Blitz as the production style and geometry producer. A native,
  document-owned model now supplies computed styles, boxes, hit testing, and CDP
  geometry from one canonical state shared across worlds and consumers.
- Improved real-page CSS and layout behavior, including stylesheet URL
  resolution, fonts and intrinsic images, SVG sizing, live form controls,
  skipped content, tables, captions, and dependency-driven invalidation.
- Expanded Chrome 152 compatibility across DOM and Web IDL bindings, navigation,
  storage, networking, Performance APIs, Trusted Types, console behavior, media,
  WebGPU, and cross-realm object lifecycles.
- Strengthened Playwright and Puppeteer automation with more complete CDP
  coverage, steadier navigation and frame handling, and isolated concurrent Page
  execution.
- Made the public command a self-contained native executable. Release packages
  require no Go, Rust, Cargo, Chromium, display server, or GPU at runtime.
- Added a clearer startup banner and tightened the reproducible local release
  pipeline for Windows and Linux amd64.

Mimic remains a renderer-free public beta for Windows and Linux amd64. It models
browser-observable state for supported workflows; it does not provide complete
Chrome, rendering, media, or Web API compatibility. See the
[compatibility notes](https://github.com/moreveal/mimic/blob/main/docs/compatibility.md) and [performance report](https://github.com/moreveal/mimic/blob/main/docs/performance/report.md)
for current boundaries and measured tradeoffs.
