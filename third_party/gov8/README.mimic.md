# Mimic's gov8 extension

This is a minimal source distribution of `github.com/maclof/gov8` v0.1.1
(https://github.com/maclof/gov8/tree/v0.1.1). The upstream LICENSE and
THIRD_PARTY_NOTICES.md are retained, including Temporal dependency licenses.
Upstream examples, conformance fixtures, and unrelated tests are omitted.

## Changes

- Expose V8 ObjectTemplate::MarkAsUndetectable through the checked Go API
  and a new native export. This supplies real HTMLDDA operator semantics.
- Preserve accessor getter/setter values in property-definer callbacks. Their
  handles occupy existing otherwise-unused kind-specific frame slots; the
  callback frame remains 160 bytes and native ABI remains 44.
- Rebuild the embedded Windows amd64 DLL; its metadata verifies the new asset.

The adapter and browser implementation live in Mimic rather than this binding.
The new export fails explicitly if an old GOV8_SHIM_DLL override is selected.
Normal builds require no override and use the embedded verified binary.

## Rebuild

Use Windows amd64, MSVC x64 tools, Rust 1.98, Go, and PowerShell, then from here:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/setup_windows.ps1
go run ./internal/cmd/package-shim
```

The setup script verifies its pinned V8 library and source archive hashes and
Temporal dependency lock. After packaging, update `Size` and `SHA256` in
`internal/prebuilt/prebuilt_windows_amd64.go` to the size and hash printed by the
packaging command. Do not change the native inputs to make a build pass.

Validate without GOV8_SHIM_DLL using `go test .`; from the Mimic root also run
`go test ./internal/engine/v8 -run TestHTMLDDAObject -count=1`.

Packaged DLL size: 45,930,496 bytes.
DLL SHA-256: `919522b4d4ed80671586a1b7a0efc144a533e91e08319715e76ed27962cee0ff`.
Compressed asset: 17,575,610 bytes. Native engine: V8 15.2.124.1-rusty,
Rust v8 crate 152.2.0, temporal_capi 0.2.6.
