# Mimic v0.1.0-beta.2 — Public Beta

**Run website JavaScript. Keep browser state. Skip the rendering pipeline.**

This second public beta ships ready-to-run **Windows amd64** and **Linux amd64**
builds. Mimic uses V8 and its own browser environment without embedding Chromium.
No browser installation, Go toolchain, display server, or GPU is needed to run it.

## What is new in beta.2

- Fixed several crashes and stalls encountered while loading complex pages,
  including runtime ownership and microtask re-entry failures.
- Completed a major runtime optimization pass across DOM operations, JavaScript
  value lifetimes, Fetch byte transfer, response-body ownership, and bootstrap
  data sharing.
- Integrated **Taffy** as the native CSS layout engine for supported Flex and Grid
  geometry, while preserving Mimic fallbacks for formatting contexts outside the
  current Taffy boundary.
- Fixed element geometry bugs affecting box coordinates, sizing, gaps, alignment,
  scrolling, and automation hit targets in supported layouts.
- Improved Linux teardown memory recovery by reclaiming idle V8 allocator arenas
  after Page disposal.
- Improved Chrome 152 compatibility for `MutationObserver` construction and
  enumeration, frame-import reflection, and cross-realm microtask checkpoints.
- Added repeatable CDP site-journey diagnostics used to reproduce and verify
  page-loading and automation failures.

## Try it

1. Download the archive for your platform from this release and extract it.
2. Read `QUICKSTART.md` and `LICENSE.md` in the archive.
3. Start Mimic with `--profile examples/profile.json --listen 127.0.0.1:9222`.
4. Run the included [examples](examples/README.md) with Node.js 22+.

All three examples were executed against the packaged binary on both platforms:

- **Playwright:** fill a form, click, wait for a fetch-driven DOM update, extract the result.
- **Puppeteer:** read content and check language, timezone, viewport, and theme from a profile.
- **Concurrency:** run ten pages with separate window state, collect fetch results, close pages.

Public clients are pinned to Playwright Core **1.63.0** and Puppeteer Core
**25.10.0**. The packaged executables also pass fresh-cache CDP, DOM, Promise,
text metric, mouse-input, and teardown checks with V8, QuickJS, and goja.

## Platform requirements

| Archive | Requirements |
| --- | --- |
| `mimic-v0.1.0-beta.2-windows-amd64.zip` | Windows x64; verified on Windows 11. |
| `mimic-v0.1.0-beta.2-linux-amd64.tar.gz` | Linux x64, glibc 2.39+, libgcc_s, installed fonts; verified on Ubuntu 24.04 under WSL2. |

On Ubuntu 24.04, install `libgcc-s1 fonts-liberation fonts-dejavu-core` if absent.
ARM64 and musl/Alpine are not packaged. Both hosts expose the current Chrome 152
Windows environment profile; Linux host support does not add a Linux fingerprint.
Native speech synthesis is unavailable on Linux. Builds are not code-signed.

## What is still developing

Browser and CDP coverage are incomplete. Supported workflows do not imply full
Playwright/Puppeteer compatibility. Mimic does not render screenshots or PDFs,
and is not a sandbox for untrusted code. Bind CDP to trusted localhost clients.

The [benchmark charts](BENCHMARKS.md) describe the September 14 Windows development
checkpoint, **not a measurement of the beta release binaries**. The accompanying
Linux 100-Page comparison is reported separately with its memory limitation.

Taffy currently owns final sizing and placement for supported Block, Flex, and
Grid snapshots. Inline text, tables, replaced or shadow-specific geometry,
transforms, and document flow outside those snapshots continue to use Mimic's
existing layout paths.

## License and feedback

Mimic uses **PolyForm Shield 1.0.0**: commercial use is permitted subject to the
license's noncompete provisions and other terms. Examples are MIT licensed;
third-party terms accompany the archives. Implementation sources remain private.

Report reproducible issues here or DM **moreveal** on Discord. Include the release,
platform, client version, and a small example without credentials or private data.

`SHA256SUMS` and `release-manifest.json` accompany the platform archives.
GitHub's automatic source archives contain this product overview and examples,
not the Mimic implementation.
