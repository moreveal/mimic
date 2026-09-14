<p align="center">
  <img src="assets/readme-hero.png" alt="Mimic — Run the web. Skip the rendering. A lightweight runtime between HTTP and a full browser." width="1200">
</p>

<p align="center">
  <strong>Browser logic, without the rendering pipeline.</strong><br>
  Execute website JavaScript and work with Chrome-visible browser state in a lightweight Go runtime.
</p>

<p align="center">
  <a href="QUICKSTART.md"><img src="https://img.shields.io/badge/platforms-Windows%20%7C%20Linux-5988C7?style=flat-square" alt="Windows and Linux amd64"></a>
  <a href="QUICKSTART.md"><img src="https://img.shields.io/badge/engine-V8-81B5FF?style=flat-square&amp;logo=v8&amp;logoColor=white" alt="V8 engine"></a>
  <a href="FAQ.md"><img src="https://img.shields.io/badge/runs-JavaScript-F7DF1E?style=flat-square&amp;logo=javascript&amp;logoColor=black" alt="Runs JavaScript"></a>
  <a href="FAQ.md"><img src="https://img.shields.io/badge/automation-CDP-AC9FFF?style=flat-square" alt="CDP automation"></a>
  <a href="FAQ.md"><img src="https://img.shields.io/badge/status-public_beta-FFCA91?style=flat-square" alt="Public beta"></a>
</p>

<p align="center">
  <a href="QUICKSTART.md">Quick start</a> ·
  <a href="#measured-not-assumed">Benchmarks</a> ·
  <a href="FAQ.md">CDP support</a> ·
  <a href="#let-the-website-do-the-work">Product vision</a> ·
  <a href="FAQ.md">FAQ</a>
</p>

## Let the website do the work

**The website's JavaScript. Browser state. A lighter runtime.**

Mimic is a browser execution runtime built around V8, with its own browser
environment and no rendering engine. Run website logic, work with the DOM,
and automate through familiar CDP tools without embedding Chromium.

**Our ambition: direct HTTP lightness with browser compatibility.**

**Public Beta · Windows & Linux.** [Download the beta →](https://github.com/moreveal/mimic-overview/releases/tag/v0.1.0-beta.1) · [Run the examples →](examples/README.md)

Actively developed. Feedback and workflow help: **`moreveal`** on Discord.

## Working browser behavior

These development checkpoints exercise execution and observable browser behavior:

| Capability | Verified checkpoint |
| :--- | :--- |
| **React ✓** | 200 cards, effects, fetch, and state transitions. |
| **WebAssembly ✓** | Module instantiation and 100,000 integer-add calls. |
| **Workers ✓** | Messaging and worker fetch in supported workflows. |
| **DOM mutations ✓** | 3,000 elements with validated final structure and text. |
| **Networking ✓** | Fetch and XHR in deterministic local fixtures. |
| **Playwright / Puppeteer ✓** | [Verified examples](examples/README.md): form input, clicks, fetch, profile readback, and ten concurrent pages on Windows and Linux. |
| **100 concurrent pages ✓** | Completed Linux static, CPU, and React comparison with Chrome. |

These are scoped, verified workflows; compatibility continues to expand.

## Measured, not assumed

Fresh-build Windows checkpoint from **September 14, 2026**, against **Chrome 152.0.7977.82** in headless mode, on Windows 11 x64 (Intel i7-14700KF, 31.83 GiB RAM).

<p align="center">
  <a href="BENCHMARKS.md"><img src="assets/benchmark-startup-20260914.svg" alt="Mimic and Chrome: CDP readiness and startup memory" width="1200"></a>
</p>

<p align="center">
  <a href="BENCHMARKS.md"><img src="assets/benchmark-scaling-20260914.svg" alt="Static concurrency: active memory and throughput through 50 pages" width="1200"></a>
</p>

**92% lower ready RSS. 62% less active RAM and 3.32× throughput at 50 static pages.**
Measured on these fixtures and this machine; startup RSS is not per-page memory.

Mimic is actively evolving toward direct HTTP lightness with browser compatibility.
These are selected strengths from an early checkpoint; see Known limitations and
the complete benchmark report for the current tradeoffs.

[Full results, concurrency, CPU, memory, and methodology →](BENCHMARKS.md)

## Real-world compatibility checkpoints

Complex compatibility checkpoints have also included client-side challenge flows,
browser diagnostics, and production storefront content. They help test how the
runtime handles demanding combinations of browser behavior.

| Development checkpoint | Observed result |
| :--- | :--- |
| **Cloudflare challenge laboratory** · September 12 | Mimic moved from the challenge response to the explicit success page on ScrapingCourse. The transition also succeeded in a subsequent paired check. |
| **BrowserScan** · September 12 | The tested session received the explicit **Normal** verdict. |
| **Amazon storefront** · September 11 | Storefront navigation and campaign cards were captured with available assets; the exported snapshot was checked offline in Chrome. |

These are specific recorded site/session outcomes, not a measured pass rate across
anti-bot systems or proof of why a service admitted a session. Amazon's checkpoint
covers storefront content, not login or checkout. Compatibility continues to evolve;
site policies and server decisions still apply.

## Configure once. Then automate.

Designed to simplify automation: set the environment up front, start Mimic, and
connect your CDP client. No Chromium installation is required.

After receiving a beta build, save this as `profile.json`:

```json
{
  "schemaVersion": 1,
  "baseProfile": "chrome-152-windows-x64-headful-controlled-v1",
  "hardware": { "logicalProcessors": 8, "deviceMemoryGB": 8 },
  "locale": {
    "languages": ["en-US", "en"],
    "timezone": "America/New_York",
    "intlLocale": "en-US"
  },
  "preferences": { "colorScheme": "dark" }
}
```

```powershell
.\mimic.exe --profile profile.json --chrome 152 --browser-mode headful --listen 127.0.0.1:9222
```

`headful` selects an environment profile; Mimic still renders no pixels. Profiles
configure supported language, timezone, hardware, and preference observations.
Use separate profiles per context and native proxies when your workflow needs them.

**[Complete quick start: Puppeteer, native proxies, and multiple profiles →](QUICKSTART.md)**

## Public beta

**[Download Windows or Linux builds →](https://github.com/moreveal/mimic-overview/releases/tag/v0.1.0-beta.1)**

Extract, start Mimic, and connect your automation. [Quick start](QUICKSTART.md) ·
[Runnable examples](examples/README.md) · [Release notes](RELEASE_NOTES.md).

DM **`moreveal`** on Discord for workflow help, or [report a reproducible issue](BETA.md).
This repository is the public product overview, with binary releases and client
examples. Implementation sources and internal research remain private.

## Known limitations

Warm execution is faster in two of six fixtures and still trails Chrome most sharply
for DOM mutations. The Windows host stopped Mimic's 100-page levels at its memory-pressure
guard; the completed Linux 100-page comparison shows higher marginal memory per Mimic Page.

Mimic is in active development. The beta supports **Windows amd64** and
**Linux amd64 with glibc 2.39+** (validated on Ubuntu 24.04 under WSL2). The
browser environment remains **Chrome 152 on Windows** on either host. ARM64 and
musl are not packaged. Linux needs installed fonts and has no native speech synthesis.
The benchmark charts are Windows measurements; the separate 100-Page density results are Linux measurements.

It implements a subset of browser APIs and CDP. It does not render screenshots,
PDFs, Canvas/WebGL output, or video, and does not provide full CSS layout or complete
Chrome compatibility. Supported automation workflows do not imply that every
Playwright or Puppeteer feature works. See the [FAQ](FAQ.md) for the practical boundaries.

## License

Mimic is licensed under [PolyForm Shield 1.0.0](LICENSE.md). Commercial use is
permitted; the license restricts competing products. Third-party components
retain their own licenses. See the full license for its scope and conditions. [Client examples](examples/) are MIT licensed.
