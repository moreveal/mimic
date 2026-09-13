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

## Linux amd64

The same shim and Go binding also run on Linux using the System V ABI.
`internal/native` isolates Windows calls from Linux `dlopen`/`dlsym` and purego
callbacks. Pointer-word exports with more than 15 arguments use a native bridge;
thread affinity remains checked against the owning kernel thread. No global
Page execution lock is introduced.

The packaged Linux library requires glibc 2.39+ and libgcc_s (Ubuntu 24.04+).
It embeds V8, the matching Chromium libc++ and Temporal; no system C++ ABI or
GPU/display service is needed. Its size and digest live in
`internal/prebuilt/prebuilt_linux_amd64.go`. Runtime extraction and integrity
verification are shared with Windows.

From the Mimic root:

```sh
python3 third_party/gov8/scripts/setup_linux.py
go test ./internal/engine/v8
go test ./...
(cd third_party/gov8 && go test ./...)
```

The rebuild requires Python 3.11+, Rust/cargo, Go 1.26.4+, binutils, and glibc
headers. It downloads the hash-pinned v8 152.2.0 crate and Linux static archive,
uses the crate's pinned Chromium Clang updater and libc++ headers, and builds
Temporal with `cargo build --locked`. The downloaded compiler is only needed
for rebuilding the shim. Linux source builds inherit the builder's glibc floor.

Linux archive SHA-256:
`b6683e9afcb77fbd8cb2c3acf45d34d52029ab216b332522c95179c12f0fed3c`.
Both engines report V8 `15.2.124.1-rusty` and shim ABI 44.
`GOV8_SHIM_LIBRARY` is the cross-platform developer override; `GOV8_SHIM_DLL`
remains supported. Neither is needed for normal builds or deployments.
