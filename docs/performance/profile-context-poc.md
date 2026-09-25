# Context/profile PoC — 2026-09-25

This is the earlier PoC checkpoint. The later
[feature-specific RAM report](profile-context-final-20260925.md) supersedes its
memory and diversity limits after the generator and snapshot changes.

This checkpoint implements a single current `Mimic.createContext` contract,
portable generated/manual profiles, explicit CDP import/export and separate
proxy routing. It does not claim the final fingerprint diversity or RAM target.
See [the API](../environment-profiles.md) and [remaining work](../public-context-contract-plan.md).

## Density diagnostic

Windows amd64, Intel Core i7-14700KF, 34,177,138,688 bytes physical RAM.
Fresh test executable SHA-256:
`aa1602cda9cd1d1030114311279953f3dfecb083b175d1c37a424f983cd7cd14`.

`TestProfilePoCDensity` runs three rounds of 100 different seeds against a local
HTML fixture with 200 links. Each job has its own Context, Page and transport.
It checks title, link count, viewport and hardwareConcurrency. Loaded pages
wait at a barrier before measurement and close, proving the concurrent mode
really holds 100 live pages. No proxy, resource policy or forced GC is used.
Each concurrency runs in a separate fresh process. Other correctness tests
were running on the host, so timings are descriptive and not a speed comparison.

| Observation | Concurrency 8 | Concurrency 100 |
| --- | ---: | ---: |
| Maximum sampled RSS, MiB | 533.91 | 4441.53 |
| Maximum sampled private bytes, MiB | 589.29 | 4689.60 |
| RSS after closing first 100, MiB | 198.41 | 425.71 |
| RSS after closing 200, MiB | 193.18 | 394.97 |
| RSS after closing 300, MiB | 195.28 | 404.72 |
| Registered Contexts after every closed wave | 0 | 0 |
| Goroutines at 100/200/300 closed checkpoints | 4 / 4 / 4 | 4 / 6 / 6 |
| Total 300-job elapsed time, ms | 8188 | 8168 |

These are barrier samples, **not continuously measured process peaks**. They
include the Go test executable and its local fixture server. There is no matched
old implementation/Chrome comparison, no ten-wave soak and no JS-heavy fixture.
Residual memory includes allocator/process state; empty Context registries do
not prove complete absence of retained objects. The variable 100-live post-close
samples require a longer attributed soak before claiming a plateau.

The practical result is that bounded concurrency materially limits live memory,
while 100 live pages still cost approximately 4.34 GiB on this small fixture.
No compact-storage implementation or memory reduction is claimed.

Raw receipts are local generated evidence in the repository-ignored diagnostics
directory; the authored summary preserves the results.

Receipts: [8 live](data/profile-context-poc-20260925/concurrency-8.json),
[100 live](data/profile-context-poc-20260925/concurrency-100.json),
[build and environment](data/profile-context-poc-20260925/provenance.json).

Reproduce by compiling `go test -c ./internal/browser` and running
`-test.run=^TestProfilePoCDensity$` in separate processes with
`MIMIC_PROFILE_POC_CONCURRENCY=8` and `100`; set `MIMIC_PROFILE_POC_OUTPUT`
to save JSON. On PowerShell quote the complete `-test.run=...` argument.

## Behavioral coverage and remaining boundaries

- Profile tests cover deterministic/random generation, 100 fixed seeds without
  duplicate IDs, generated/manual roundtrip, changed export rejection, invalid
  inputs and independence of environment identity from proxy routing.
- CDP tests cover stateless generate/import/export, effective observations,
  immutable managed profile setters, invalid-create atomicity, secret redaction,
  proxy navigation/Worker fetch and disposeOnDetach cleanup.
- Focused profile and CDP race tests pass. Browser profile inheritance,
  frame/Worker and bootstrap snapshot tests pass.
- `go test ./... -count=1` passed (browser package: 443.079 s). The final
  mode/transport-independent environment hash and compact lock flag were subsequently
  checked by the focused profile/CDP suite, browser profile/snapshot suite and
  freshly rebuilt density probe; the entire suite was not repeated for that delta.
- Public examples pass against a fresh executable, including 100 generated
  jobs and a separate 100-job explicit-manual run. Puppeteer/Playwright examples
  use ordinary contexts; managed-profile integration is exercised through raw CDP.
- The generated catalog only varies window width/height, theme and reduced
  motion: at most **5,371,192 configurations** on the current baseline. It does
  not provide independent GPU/font/canvas/audio diversity. Different seeds can
  collide and a site's smaller observation set can collapse many configurations.
- The new recipe has not yet been differentially measured against frozen Chrome.
  Checks enforce the current model's invariants, not full physical-device realism.
- Manual mode rejects known incoherence and unvalidated identity/graphics/fonts
  changes. Complete cross-surface validation is unfinished. Profile locks do not
  sandbox arbitrary user scripts or request-interception header replacement.
- Full HTTPS/SOCKS5/WebSocket/UDP routing coverage, cancellation/stall matrices,
  a numerical RAM ceiling and compact storage remain separate work.

The initial smoke run exposed that the old Puppeteer example implicitly depended
on the removed CLI profile. It now sets its intended ordinary CDP emulation
explicitly and retains its original observation assertions.
