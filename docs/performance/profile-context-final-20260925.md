# Generated Contexts: fingerprint-specific RAM checkpoint (2026-09-25)

The new profile feature initially made each distinct environment select a
different V8 bootstrap snapshot key. A cold burst of generated identities then
compiled and retained separate JavaScript graphs even though the bootstrap API
exposure was identical. The fix keys snapshots on the exposure-shaping state
and makes the first managed Page for each security/exposure graph wait for one
Browser-owned preparation. Host callbacks rebind locale, window, graphics,
fonts and audio to the consuming Page. Pages still have separate isolates and
event loops. Browser cache admission remains bounded to four artifacts and
32 MiB of snapshot data.

Context construction also no longer resolves the same CDP generation twice or
copies the complete normalized environment several times. `ApplyOwned` transfers
the private normalized document into Context state; public getters and `Apply`
retain defensive-copy semantics. The performance work is restricted to storage
and construction introduced by profiles; DOM or general Page storage was not
rewritten.

The bootstrap key also hashes its immutable multi-megabyte source without
copying it into a byte slice for every Page. Empty collections are canonicalized
once when a profile is constructed, letting the typed fingerprint hash avoid a
large reflected map per lookup while preserving identical IDs across generated
and manual import modes.

## Matched live-memory comparison

Windows amd64, i7-14700KF, 34,177,138,688 bytes physical RAM. Each receipt is
a fresh process. A local HTTP page contains 200 links; each job owns a distinct
generated profile, Context and Page. The test verifies title, links, viewport and
hardware observations, then holds all admitted Pages at a barrier. Three rounds
of 100 jobs run in each process. No proxy, resource policy or forced GC is used.
The control retains the full-environment snapshot key and cold navigated Pages;
the candidate uses exposure-only keys and prepares the Page's actual exposure
graph. Their executable hashes and exact construction are in
[provenance](data/profile-context-final-20260925/provenance.json).

| Concurrency | Control max sampled live RSS | Candidate max sampled live RSS | Candidate improvement |
| --- | ---: | ---: | ---: |
| 100, three alternating fresh-process pairs | 3972, 4004, 4037 MiB; median **4004** | 3135, 3137, 2995 MiB; median **3135** | **869 MiB (21.7%)** |
| 8, two alternating fresh-process pairs | 539, 535 MiB; midpoint **537** | 397, 398 MiB; midpoint **398** | **140 MiB (26.0%)** |

The 100-Page candidate restored **100/100** navigated realms from one cached
artifact in every run; the control restored zero and had no matching navigated
artifact. The eight-Page candidate restored **8/8**. This is the measured
feature-specific mechanism behind the RSS change. Candidate first-live barrier
times were 1.77–1.91 s versus 3.15 s for the first control run; these are
diagnostic timings, not an end-to-end throughput benchmark.

The process RSS remains approximately 3 GiB for 100 active Pages because every
Page retains an independent V8 isolate, DOM and network state. Sharing one
isolate between concurrent Pages is not an acceptable shortcut: native teardown
races have been observed. Snapshot reuse removes duplicate bootstrap work, not
the cost of the Pages themselves.

## Bounded 1000-job soak

One fresh candidate process ran 1000 distinct identities with concurrency 8.
The largest sampled live RSS was **403 MiB**. After each completed hundred,
RSS was **149, 169, 169, 163, 186, 174, 189, 153, 180, 173 MiB**; every
checkpoint had zero registered Contexts and three or four goroutines. This
supports bounded reuse over ten waves, although allocator/process memory is
not expected to return to startup RSS after every close.

## Attribution and limits

A separate in-use Go heap capture at the first 100-Page barrier held about
75 MiB of sampled Go objects, versus multi-GiB process RSS. Before ownership
transfer, the environment clone path accounted for approximately 1.5 MiB
sampled in-use space across 100 profiles. After removing the redundant retained
clone, that path fell below the heap sampler's resolution; profile normalization
still accounts for about 1.5 MiB sampled. A second, matched-fixture heap capture
after replacing reflection-heavy profile hashing and repeated resolved-profile
normalization reduced **profile-attributed sampled allocation** from about
156 to 34 MiB across the first 100 Pages. Those are allocation samples, not
additional retained RSS. The large retained groups in this fixture are DOM
records, one shared snapshot blob, and host callback bindings. Native V8 memory
is outside Go heap profiles. A later pair of the same 100-Page fixture, before
and after removing the bootstrap-source copy and canonicalizing profile hashes,
sampled **408 → 334 MiB** of total Go allocation. A final compiled-binary
sample restored 100/100 navigated realms with **2978 MiB RSS** and 333 MiB
sampled Go allocation at its first live barrier; this
single barrier is a sanity check, not part of the paired RSS comparison. The
[heap captures](data/profile-context-final-20260925/) and
[raw receipts](data/profile-context-final-20260925/) are available for RSS,
private bytes, Go heap, cache state, restored realm count, teardown checkpoints
and elapsed time.

Measurements are barrier samples, **not continuous process peaks**. The 1000-job
soak is queued, not 1000 simultaneous Pages. This local fixture does not cover
JS-heavy sites, real proxy transports or a Chrome comparison. It is a profile
feature regression checkpoint, not a universal RAM ceiling.
