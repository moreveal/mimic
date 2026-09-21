# Run real automation against Mimic

These examples use the released executable. No Chrome download or browser
installation is needed. Install Node.js 22+ for the clients.

From this directory:

```sh
npm ci
```

Start Mimic with the included profile in another terminal (adjust the executable path):

```sh
# Linux
/path/to/mimic --profile profile.json --listen 127.0.0.1:9222
```

```powershell
# Windows
C:\path\to\mimic.exe --profile profile.json --listen 127.0.0.1:9222
```

Start the local demo shop and leave it running:

```sh
npm run fixture
```

In another terminal, choose a workflow:

| Command | What it does |
| --- | --- |
| `npm run playwright` | Fills a quantity, clicks a button, waits for fetch and DOM updates; prints `126 USD`. |
| `npm run puppeteer` | Reads content and verifies the configured language, timezone, viewport, and dark theme. |
| `npm run concurrency` | Runs 10 pages concurrently, checks separate window state, collects fetch results, closes every page. |

The fixture uses localhost so the examples are reproducible without a third-party
website. `MIMIC_URL` changes the CDP endpoint. `TARGET_URL` changes the fixture
address; the form examples expect the same demo-shop page structure.

## Verify an installation in one command

This starts its own local fixture and Mimic process on available ports, uses a
fresh native cache, runs all three examples, and cleans up afterward. It does
not need either of the manually started processes above.

```sh
npm run verify -- /absolute/path/to/mimic
```

```powershell
npm run verify -- C:\absolute\path\to\mimic.exe
```

Clients are pinned in `package-lock.json`: Playwright Core 1.63.0 and Puppeteer
Core 25.10.0. These scenarios are verified on Windows amd64 and Ubuntu 24.04
amd64 under WSL2 for the beta release. They do not establish full Playwright or
Puppeteer compatibility. Screenshots and rendered PDFs are not supported.

The files in this directory are [MIT licensed](LICENSE) so you can adapt them.
The Mimic executable is governed by the authoritative project [LICENSE](../LICENSE).

For a separate Crawlee PlaywrightCrawler example and its measured limits, see
the [Crawlee compatibility checkpoint](../docs/compatibility/crawlee-playwright.md).
