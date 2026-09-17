# Mimic releases

Release archives are built locally from a clean, committed `moreveal/mimic`
revision. Windows and Linux must use the same source revision.

## Prepare

Requirements: Go 1.26.4+, a platform C compiler, Python 3.12+, Node.js 22+, and
the Linux fonts documented in [getting started](../../docs/getting-started.md).

```powershell
python tools/release/prepare.py --version v0.1.3
```

Run the equivalent command under Ubuntu 24.04/WSL2 for Linux. The tool builds a
stripped binary, assembles the allowlisted package, collects third-party notices,
and verifies the extracted package with V8, QuickJS, goja, Playwright, Puppeteer,
and the concurrency example.

Outputs are written to `.build/releases/VERSION/`. Reusing an existing
version/platform output directory is rejected so stale artifacts cannot be
mistaken for a fresh build.

## Publish

Push the exact release commit to `moreveal/mimic`, then run:

```powershell
python tools/release/publish.py --version v0.1.3
```

Publication requires verified Windows and Linux receipts for the current commit.
It creates a draft release, uploads the archives, manifest, and checksums,
downloads every asset to verify its hash, and only then publishes the release.
Local verification is authoritative; the publisher does not wait for CI.
