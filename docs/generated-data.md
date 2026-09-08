# Generated compatibility data

The immutable identifiers are in `chrome/152/target.json`; `generated/lock.json`
records matching Chromium/Blink/WPT metadata and SHA-256 of the normalized WebIDL
catalog and the original browser protocol/JS protocol inputs. The exact target
is 152.0.7977.82, Chromium r1669021, V8 source
`4323497a6a73839e6d5260f6acd7ec0212cb3321`. gov8 v0.1.1 separately packages
V8 15.2.124.1-rusty. Do not equate these two provenance records without evidence.

`tools/generate_compat.py` creates `webapi.json`, `cdp.json`, `surface.js`,
`bundle_data.go` and `lock.json`. `exposure_data.go` is a handwritten embed adapter,
not a generated projection. Window/Worker exposure JSON files are observations
from the exact Chrome product checked by `capture_chrome_surface.py` and
`capture_chrome_worker_surface.py`; they are not reconstructed from IDL. Capture
context includes ephemeral loopback ports/Blob IDs, so recapture is semantically
comparable but not expected to be byte-identical.
The retained exposure inputs are authoritative headful captures and include
mandatory mode/profile/geometry provenance. Headless exposure may be used as a
comparison but never silently replaces these inputs; see the
[oracle policy](oracle-policy.md).

`artifact-hashes.json` covers all retained generated-directory inputs and outputs,
including the exposure captures. Exposure JSON was normalized to LF during cleanup, with no parsed-data changes;
LF is enforced by `.gitattributes`.

```powershell
# No downloads, writes or external browser required:
python tools/generate_compat.py --check
# Explicit maintenance only; contacts Chromium/V8 upstream:
python tools/generate_compat.py --target chrome/152/target.json
```

The offline check verifies metadata, catalog/input hash agreement, every retained
artifact digest, and regenerates JS and Go projections in memory for byte comparison.
Two consecutive checks passed during stabilization. This proves deterministic
projection from the retained normalized inputs, not a fresh download of all raw
Blink IDL or authenticity of historical captures. No upstream was contacted.
Source cache and downloaded browsers are excluded from version control.

After intentional upstream regeneration or exposure recapture, review semantic
changes before refreshing artifact-hashes.json (SHA-256 of each other file in the
generated directory, keyed by filename). The manifest is a drift detector, not a
signature. It deliberately is not silently rewritten by `--check`.

Navigator capability work retains 109 original IDL sources at the pinned commit
in `generated/navigator-idl-sources.json`. The focused recovery command
`python tools/refresh_navigator_idl.py` reproduces those declarations from the
retained sources; it downloads only missing sources. Partial and mixin members
retain their declaring source and conditions. Partial flags no longer overwrite
the primary interface, and Blink's conditional `Exposed(Window Feature, ...)`
syntax is parsed separately from ordinary `Exposed=Window`.
The offline check also verifies these recovered declarations. This is a focused
repair, not a claim that all older catalog declarations were redownloaded.

`window-insecure.json` is an exact-version opaque-context exposure capture.
The [Navigator matrix](navigator-capabilities.md) records additional secure,
isolated and opaque observations, semantic differences and unsupported backends.
