# Mimic releases

Release archives are built locally from a clean, committed `moreveal/mimic`
revision. Windows and Linux must use the same source revision.

## Prepare

Requirements: Go 1.26.4+, a platform C compiler, Python 3.12+, Node.js 22+, and
the Linux fonts documented in [getting started](../../docs/getting-started.md).
Rust/Cargo is a build-time requirement for the native Blitz producer.

```powershell
python tools/release/prepare.py --version v0.1.3
```

Run the equivalent command under Ubuntu 24.04/WSL2 for Linux. The tool builds a
content-addressed, locked native producer first and then builds the public
`./cmd/mimic` command as a stripped standalone binary. It assembles the
allowlisted package, collects third-party notices, and verifies the extracted
package with V8, QuickJS, goja, Playwright, Puppeteer, and the concurrency
example.

The release entry point is deliberately the single command above. Do not package
`tools/runmimic`: it is a source-checkout bootstrap that may invoke Cargo. Do not
invoke `go build` directly for a clean release checkout without first running
`go run ./tools/buildnative`; the release tool performs both operations in the
required order. The resulting `mimic`/`mimic.exe` contains the statically linked
native producer and needs neither Go, Rust nor Cargo on an end-user machine.

For a local unpackaged release-equivalent build:

```powershell
go run ./tools/buildnative
go build -trimpath -ldflags="-s -w" -o .build/mimic.exe ./cmd/mimic
```

On Linux, use `.build/mimic` as the output path. This binary is the same command
the release packager builds; `go run ./tools/runmimic` is only the convenient
clean-checkout development path.

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
