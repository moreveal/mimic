# Snapshot byte ownership experiment, 2026-09-14

The selected change shares immutable serialized Go bytes between snapshot
consumers. Each consumer still owns its own native copy and disposal record;
Pages retain independent V8 isolates and JavaScript graphs. No extra owner
dispatch, global Page lock or mutable browser state is introduced.

The complete prebuilt bootstrap and native-library override experiments were
not selected. Neither demonstrated a twofold improvement including startup.
The override only moved an existing integrity check outside the measured run.

## Method and receipts

The Linux experiment ran in Ubuntu 24.04 on WSL, with no concurrent heavy
work. Production control source is `f07bd2d643eebbadc28d582ef93f01c832a50106`.
The additive `TestBootstrapStartupSpike` was supplied to the control through
a Go overlay. Other opt-in test helpers in the control worktree were not run;
production source and frozen performance fixtures/harnesses were unchanged.

Control binary SHA-256:
`412c71e12918125f703b2d70f38944bb152c266dd2b8d502c59172117356b5bf`.
Candidate binary SHA-256:
`5500c5978a9cd42b738c2e115f0b6e036800d2521ac272dfe960f62012e57bb7`.
The [startup receipt](data/snapshot-byte-ownership-20260914/startup-receipt.json)
records source and artifact hashes; [raw samples](data/snapshot-byte-ownership-20260914/startup-raw.json)
and [aggregates](data/snapshot-byte-ownership-20260914/startup-summary.json) are retained.

Four cold rounds alternate control/prebuilt/prebuilt+shared and reverse that
order on every second round. Every sample starts a fresh process. The parent
timestamps the first successful DOM create/append/query result, including
process startup, compatibility setup, artifact read and first operation.
The workload uses an actual Page with the complete supported WebAPI surface,
normal wrappers and tracing. It is a startup diagnostic, not a replacement
for the frozen CDP benchmark or a Chrome speed comparison.

Separate memory rounds alternate cloned/shared bytes in the same candidate
binary, without disk artifacts. An ordinary warmup builds the existing Context
snapshot, then closes its Page. Ten Pages are created and retained, and every
one must restore successfully. Creation plus the first DOM operation is timed.
Memory is sampled naturally, after explicit V8/Go collection, after Page
closure, and after Context closure. Collection is outside first-use timing.
The diagnostic's whole-process exit interval includes those memory interventions
and must not be reported as ordinary browser completion latency.

## Selected result

Medians across four independent ten-Page runs:

| Metric | Cloned Go bytes | Shared immutable Go bytes |
| --- | ---: | ---: |
| Restored Page creation + first useful operation | 28.054 ms | 23.295 ms |
| Ten Pages: live Go heap after collection | 107.897 MiB | 20.708 MiB |
| Ten Pages: live process private memory after collection | 486.584 MiB | 401.291 MiB |
| Warm ready process private memory | 156.355 MiB | 157.539 MiB |
| Marginal private memory per live Page | 32.888 MiB | 24.067 MiB |
| After Page closure: Go heap | 19.345 MiB | 19.344 MiB |
| After Context closure: Go heap | 10.626 MiB | 10.627 MiB |
| After Context closure: process private memory | 250.484 MiB | 252.367 MiB |

Marginal values are the medians of each run's `(live - warm ready) / 10`,
not a subtraction of independently aggregated process medians.

The shared variant removes 87.188 MiB of retained Go heap for ten Pages and
85.293 MiB of live private memory in this cohort. Setup latency falls 17.0%
(1.20x speedup). The current secure snapshot is 9,141,360 bytes: avoiding ten
Go copies explains approximately 87.18 MiB directly. Native copies remain
per consumer and release on its owner at teardown.

Similar final Go heap establishes removal of the added per-consumer retention,
not immediate return of native allocator arenas to the OS. The approximately
1.9 MiB private-memory difference after full closure is not a demonstrated
new leak. This bounded test does not establish 100-Page throughput or long-session
leak freedom; the integrated campaign must retain those validation gates.

## Rejected expansion: prebuilt complete bootstrap

The spike exported the existing full Window snapshot, matched the source/profile
key, and rebound Page state through the existing restore hook. Artifacts covered
insecure, secure and isolated default profiles, using an OS/architecture/V8/ABI/
flag header and payload checksum. The current native safety setting
`--no-extensible-ro-snapshot` remained enabled. Workers were unchanged.

| First process to first useful DOM result | Median |
| --- | ---: |
| Ordinary bootstrap | 395.675 ms |
| Prebuilt complete bootstrap | 311.026 ms |
| Prebuilt + shared Go bytes | 307.207 ms |

The best integrated startup ratio is 1.288x, below the campaign's twofold
threshold for expanding a radical migration. All ordinary Linux and Windows
fresh-process artifact/descriptor tests passed. An expanded Linux race run later
failed the first insecure artifact restoration assertion; the cause remains
unresolved, and this path is not being promoted. Shipping artifacts would also
require exact native-build receipt binding for alternative builds sharing the
same V8 version/ABI. No production artifact loader, generator or build payload
belongs to the selected change.

## Rejected shortcut: repeating native-library integrity verification

The stock loader revalidates its installed 57,288,784-byte Linux native library
at process startup. A separate experiment used the existing explicit library
override with exactly the same stock library and verified SHA-256
`218b113dbf49d0e7b46b6f009bd9924cc83bf8dbf98e2904891babbc2ef7c140`.
The [receipt](data/snapshot-byte-ownership-20260914/shim-receipt.json),
[raw samples](data/snapshot-byte-ownership-20260914/shim-raw.json) and
[aggregates](data/snapshot-byte-ownership-20260914/shim-summary.json) retain the
verification boundary explicitly.

| Startup variant | Stock path | Previously verified override | Verification included before every override startup |
| --- | ---: | ---: | ---: |
| Ordinary bootstrap | 375.917 ms | 347.736 ms | 403.858 ms |
| Prebuilt + shared | 298.538 ms | 272.502 ms | 311.912 ms |

The excluded one-time verification took 30.548 ms. Repeated checks cost about
25 ms each. Including verification removes the apparent gain; variation also
affects the small differences. No integrity check or installation policy changes
are selected from this experiment.

## Validation of the selected ownership change

Windows focused engine/browser tests passed. Linux focused race tests passed.
Coverage includes artifact source closure while consumers live, separate native
consumer ownership, independent JavaScript objects, one consumer closing while
another remains live, concurrent snapshot creation/closure, complete exposed
descriptor and identity graphs, callbacks and microtasks, document/origin
rebinding, changed locale/display state, and concurrent Page teardown.

The selected [SDK sharing method](../../third_party/gov8/startup_data_sharing.go),
snapshot consumer change and focused ownership test are in production source.
Temporary patches, binaries and the rejected implementation are not retained.
