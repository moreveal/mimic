# Quick start: one profile, then automate

**Public Beta — Windows and Linux.** [Download v0.1.1](https://github.com/moreveal/mimic-overview/releases/tag/v0.1.1) and
extract the archive. No Go, Rust, Chromium, or GPU is needed. Commands below run
from the extracted directory. Read [PolyForm Shield 1.0.0](LICENSE.md) before use.

Windows: extract the `.zip` and run `mimic.exe`. Linux: extract the `.tar.gz`
and run `./mimic`; Ubuntu 24.04+ with glibc 2.39+ and libgcc_s is required.
For Linux text and input geometry, install fonts if needed:

```sh
sudo apt-get update
sudo apt-get install -y libgcc-s1 fonts-liberation fonts-dejavu-core
```

Verify the downloaded archive against the release's `SHA256SUMS` using
`Get-FileHash -Algorithm SHA256 <archive>` on Windows or `sha256sum <archive>`
on Linux. Node.js 22+ is only needed for the example automation clients.

**Want the shortest path?** The archive already includes `examples/profile.json`
and [three runnable workflows](examples/README.md), including Playwright.

## 1. Describe the browser environment

Save this as `profile.json`:

```json
{
  "schemaVersion": 1,
  "baseProfile": "chrome-152-windows-x64-headful-controlled-v1",
  "display": {
    "width": 1920,
    "height": 1080,
    "availableWidth": 1920,
    "availableHeight": 1040,
    "deviceScaleFactor": 1
  },
  "window": {
    "outerWidth": 1280,
    "outerHeight": 800,
    "viewportWidth": 1280,
    "viewportHeight": 720
  },
  "hardware": { "logicalProcessors": 8, "deviceMemoryGB": 8 },
  "locale": {
    "languages": ["en-US", "en"],
    "reduceAcceptLanguage": false,
    "timezone": "America/New_York",
    "intlLocale": "en-US"
  },
  "preferences": { "colorScheme": "dark", "reducedMotion": false }
}
```

One profile sets the supported browser-visible environment before the first page
loads: screen and viewport, reported hardware, languages, timezone, locale, and
preferences. Its pages, frames, and new workers inherit that environment.

This is broader than a user-agent string change. It is also a bounded interface:
it does not replace every fingerprint surface or turn the runtime into another
OS or browser engine. Hardware values are reported properties, not physical
resource allocation. Profiles are validated; unsupported custom settings return errors.

## 2. Start Mimic

```powershell
.\mimic.exe --profile profile.json --chrome 152 --browser-mode headful --listen 127.0.0.1:9222
```

On Linux, use:

```sh
./mimic --profile profile.json --chrome 152 --browser-mode headful --listen 127.0.0.1:9222
```

`headful` selects the measured environment profile; Mimic still opens no browser
window and renders no pixels. V8 is the default engine. No Chrome installation or
GPU is required to run this build. Keep the local process running while you automate.

## 3. Connect with Puppeteer

With Node.js and npm installed, use another terminal:

```powershell
npm install puppeteer-core@25.10.0
```

Save as `automate.mjs`:

```javascript
import puppeteer from 'puppeteer-core';

const browser = await puppeteer.connect({
  browserURL: 'http://127.0.0.1:9222',
  defaultViewport: null // Keep the viewport from profile.json.
});

try {
  const page = await browser.newPage();
  try {
    await page.goto('https://example.com/', { waitUntil: 'load' });
    await page.waitForSelector('h1');

    console.log(await page.evaluate(() => ({
      heading: document.querySelector('h1').textContent,
      language: navigator.language,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
      viewport: [innerWidth, innerHeight],
      darkMode: matchMedia('(prefers-color-scheme: dark)').matches
    })));
  } finally {
    await page.close();
  }
} finally {
  await browser.disconnect();
}
```

```powershell
node automate.mjs
```

The script reads the page's content and the environment you configured. For your
own workflow, wait for the element or application state that indicates readiness,
then use supported DOM interactions and evaluation. CDP coverage is evolving;
this example does not imply support for every Puppeteer feature.

## Playwright: fill, click, and read the result

The included [Playwright example](examples/playwright.mjs) uses
`chromium.connectOverCDP`, `locator.fill`, and `locator.click` against a local
demo shop. It waits for a fetch-driven DOM update and returns `126 USD`.
Follow [the example instructions](examples/README.md) or run the complete check:

```sh
cd examples
npm ci
# Linux; on Windows replace ../mimic with ../mimic.exe.
npm run verify -- ../mimic
```

This command starts Mimic and the fixture itself, checks Playwright, Puppeteer
profiles, and ten concurrent pages, then cleans up. No Chrome download is needed.

## Native SOCKS5, HTTP, and HTTPS proxies

Add `network` alongside the other top-level fields in `profile.json`:

```json
{
  "network": {
    "proxy": {
      "server": "socks5://127.0.0.1:1080"
    }
  }
}
```

Restart with the updated file and a running SOCKS5 proxy at that address. Resource
requests use the proxy natively, including worker fetches. SOCKS5 passes the
destination hostname to the proxy. HTTP and HTTPS proxy URLs are also supported.
No browser extension is needed.

For an authenticated proxy, add `username` and `password` inside `proxy`, keeping
credentials out of the server URL and public files. A failed proxy connection or
authentication does not fall back to a direct request. Proxy mode disables QUIC;
this setting routes resource-loader HTTP(S), not UDP/WebRTC traffic. Bypass lists
are not currently supported.

A timezone or locale setting does not change your public IP. Use an appropriate
real proxy when the workflow requires a different network location.

## Multiple profiles in one running instance

An advanced CDP client can create a context with its own profile and connection
pool. Here, `browser` is the connected Puppeteer browser from step 3, and
`profile` is the parsed JSON object from step 1:

```javascript
const cdp = await browser.target().createCDPSession();
const { diagnostics } = await cdp.send('Mimic.validateProfile', { profile });
const { browserContextId } = await cdp.send('Mimic.createContext', {
  profile,
  disposeOnDetach: true
});
try {
  const { targetId } = await cdp.send('Target.createTarget', {
    browserContextId,
    url: 'about:blank'
  });
  // Attach a page session to targetId to navigate and automate it.
  await cdp.send('Mimic.updateProfile', {
    targetId,
    patch: { window: { viewportWidth: 900, viewportHeight: 600 } }
  });
  const effective = await cdp.send('Mimic.getProfile', { targetId });
} finally {
  await cdp.send('Target.disposeBrowserContext', { browserContextId });
  await cdp.detach();
}
```

Use `Mimic.getProfileSchema` to discover valid base profiles and supported fields.
Viewport, language, and theme have dynamic overrides; full environment replacement,
including hardware, timezone, and proxy settings, requires a new context. Existing
cookies and documents are not migrated. Contexts are not a security sandbox.

The aim: describe your environment once, connect your automation, and focus on
the workflow. **[Public Beta downloads and feedback →](BETA.md)**
