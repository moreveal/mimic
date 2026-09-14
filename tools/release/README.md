# Public beta releases

Publish executable releases only to `moreveal/mimic-overview`. Keep implementation
sources, native source, research, raw captures, and private Git history here.
The public repository contains product documentation and runnable client examples.

## Prepare

Commit reviewed source, release tooling, and public documentation first. Both
checkouts must be clean. Use the same private and public revisions for both hosts.
Go 1.26.4+, the platform C compiler, Python 3.12+, Node.js 22+, and installed
example dependencies (`npm ci --prefix PATH_TO_OVERVIEW/examples`) are required.
Linux builds use Ubuntu 24.04 amd64 and installed Liberation/DejaVu fonts.

Run on Windows:

```powershell
python tools/release/prepare.py --version v0.1.0-beta.1 --overview E:/GitHub/mimic-runtime
```

Run on Linux (WSL2 can use the same checkout and output directory):

```sh
python3 tools/release/prepare.py --version v0.1.0-beta.1 --overview /mnt/e/GitHub/mimic-runtime
```

The tool builds with `-trimpath` and stripped debug symbols, assembles a package
from an explicit allowlist, collects dependency license texts, and writes a
build receipt. It runs `runtimecheck` for V8, QuickJS and goja and the actual
public Playwright/Puppeteer examples against the extracted archive. Each run
uses fresh native caches outside the checkout. Failed checks prevent a verified
receipt. Reusing a version/platform output directory is rejected.

The output is `.build/releases/VERSION/`. Platform archives contain only the
executable, license/notices, setup documentation, and public examples without
node_modules. Reproducible here means a repeatable, checked release process;
it is not a claim of bit-for-bit identical compiler output across machines.
Never relabel the retained Windows benchmark checkpoint as this release's
performance or as Linux results.

## Publish

```powershell
python tools/release/publish.py --version v0.1.0-beta.1 --overview E:/GitHub/mimic-runtime
```

To replace the artifacts and notes of an already published release, pass
`--replace-existing`. The release tag must still target the exact clean public
overview revision used by both verified platform builds.

Publishing requires both verified platform receipts with matching private/public
revisions and current archive hashes, clean checkouts, and the public commit
already pushed. The Windows/Linux GitHub Actions workflow must also pass on the
exact private source revision; running, missing, or failed CI blocks publication.
It creates checksums and a public manifest, then creates a draft
**prerelease** in `moreveal/mimic-overview`, uploads all assets, and publishes it.
The public manifest records the overview revision, versions, platform requirements,
and hashes, not local build paths or private source revisions. Private receipts
retain source provenance. An existing published release is never overwritten;
use a new beta version. A failed draft upload can be retried using the same command.

After publishing, download and hash-check the assets from GitHub. Confirm both
working trees are clean and local branches match their remotes. No release or tag
is created in the private repository.
