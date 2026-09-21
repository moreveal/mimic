# Mimic v0.1.6

Changes since v0.1.5:

- Added a browser WebSocket transport and import-map resolution for JavaScript
  modules.
- Expanded CDP support for downloads, timezone, locale, and core window
  overrides. Concurrent Playwright pages now handle runtime lifecycle more
  reliably.
- Improved Chrome-compatible network request headers, floated inline layout,
  media `controlsList`, and Canvas observations, including conic gradients,
  patterns, focus rings, and zero-size readback errors.
- Added a runnable Crawlee CDP connection example and documented its verified
  scope. The example connects upstream Crawlee to Mimic; it does not imply
  complete Crawlee or Chrome compatibility.

Mimic remains a renderer-free public beta for Windows and Linux amd64. It models
browser-observable state for supported workflows; it does not provide complete
Chrome, rendering, media, or Web API compatibility. See the
[compatibility notes](https://github.com/moreveal/mimic/blob/main/docs/compatibility.md)
for current boundaries.
