# Mimic v0.1.7

Changes since v0.1.6:

- Added an opt-in BrowserContext resource policy for controlling cache reads and network acquisition by resource kind, URL, and request context. The default behavior is unchanged. See the [resource policy guide](https://github.com/moreveal/mimic/blob/v0.1.7/docs/resource-policy.md).
- Corrected CSS box geometry and hit testing, including stacking and isolated-world style ownership.
- Reduced bootstrap snapshot work and fixed disposal of idle bootstrap V8 isolates.
- Improved Blazor Server Interactive compatibility: server-rendered DOM removal, binary values across typed array views, synthetic events across JavaScript worlds, and pointer input during server-driven layout changes. The [Blazor probe](https://github.com/moreveal/mimic/blob/v0.1.7/tools/compatibility/blazor_server_probe/README.md) passes all 13 comparison steps against Chrome 152.

The Windows and Linux archives are built from the v0.1.7 source revision. Their executables passed CDP runtime checks for V8, QuickJS, and goja, plus the bundled Playwright, Puppeteer, and concurrency examples.

Mimic remains a renderer-free public beta for Windows and Linux amd64. It models browser-observable state for supported workflows; it does not provide complete Chrome, rendering, media, or Web API compatibility. See the [compatibility notes](https://github.com/moreveal/mimic/blob/v0.1.7/docs/compatibility.md) for current boundaries.
