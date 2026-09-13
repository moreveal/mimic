# Intermittent Google browser rejection — 2026-09-13

Investigation following `ea585bb`, the parser-defer correction, and `8065bbd`,
the preceding browser/network compatibility fixes. The observed JavaScript
exceptions are gone. Google's decision after identifier submission remains
intermittent. No new production correction is established by this report.

Subsequent work implemented and validated a general bridge optimization;
see [the transaction notes](../performance/frame-transactions-20260913.md) and
the performance report. The diagnosis below remains the historical evidence;
it does not establish Google's server-side decision rule.

## Where rejection occurs

The initial account RPC (`UEkKwb`) returns `[2]` in both the accepted and
rejected captures. The identifier RPC (`MI613e`) returns HTTP 200 in both.
The accepted response contains the account-not-found result; the rejected
response supplies a navigation to `/v3/signin/rejected` with `rrk=46` and
`rhlk=le`. The page follows that server instruction. These parameters do not
expose the server's decision rule, and their internal meaning is not inferred.

All four corrected Mimic runs below recorded zero Runtime.exceptionThrown
and zero Network.loadingFailed. This rules out the original uncaught script
failure and an observed transport failure in these runs. It does not rule out
server classification using browser observations, session history or network
characteristics. HTTP success alone cannot establish fingerprint equivalence.

## What differs

Comparing the first two Mimic submissions after decoding the ordinary JSON
envelope, there were no differing leaves outside a connection-check value,
session identifiers and one opaque generated payload. Their initial
`browserinfo` arrays were identical. Ordinary request headers matched except
session-bearing URLs. The generated payload is opaque here: its meaning and
which observations determine the verdict have not been established. Payload
length alone is not evidence of a missing capability.

The connection-check iframe returns JavaScript that posts an elapsed-time
measurement to its parent. Its HTTP response arrived promptly, but queued
execution in Mimic was delayed by long synchronous timer callbacks.

| Capture suffix | Runtime | Connection-check value sent | Identifier result |
| --- | --- | --- | --- |
| `defer-final-mimic` | Mimic before final iframe timing guard | empty | Account not found |
| `defer-verified-mimic` | Mimic final production build | `youtube:8814` | Browser rejected |
| `rejection-trace-mimic` | Same production code, diagnostic HTTP endpoints | `youtube:10795` | Account not found |
| `rejection-profile-mimic` | Same diagnostic build, CPU sampling enabled | `youtube:13539` | Browser rejected |
| `defer-verified-chrome` | Frozen Chrome 152.0.7977.82 | `youtube:525` | Account not found |

All captures have prefix `google-signin-20260913-` under private-captures.
The accepted 10.795 s observation and rejected 8.814 s observation refute a
simple monotonic delay-threshold explanation across these sessions. There
was no randomized experiment isolating a single variable. The first two runs
overlapped a full local test run; subsequent ones did not. The Chrome window
geometry was not matched to Mimic, so this is a control observation, not a
claim that all Chrome inputs were equivalent.

## A reproduced runtime bottleneck

The unprofiled diagnostic trace contains consecutive timer tasks lasting
5.334 s and 5.214 s. The connection-check response was available at 9.923 s,
but its frame navigation/script began at 20.260 s. The profiled repetition
contains corresponding timer tasks of 6.392 s and 6.892 s. Sampling overhead
and different application execution make those durations non-interchangeable.

A 15 s CPU profile spanning account initialization collected 10.67 CPU-seconds.
`crossFrameData` appears in 4.74 s cumulative owner-operation samples;
`callFrameReflection`, `callFrameReference`, reflected value encoding and V8
host callbacks account for substantial nested work. Cumulative samples overlap
and are not additive. This identifies cross-realm bridging as a measured
runtime cost, not the server's rejection predicate.

A separate loopback fixture, excluding frame creation and warming each case,
confirmed that ordinary cross-realm operations carry large overhead:

| 1000 operations | Mimic | Frozen Chrome |
| --- | ---: | ---: |
| Parent-local function calls | 0.1 ms | Below timer resolution |
| Calls to an iframe function | 115.4 ms | Below timer resolution |
| Reads of an iframe object's property | 61.8 ms | Below timer resolution |
| One iframe call containing a local 1000-iteration loop | 0.1 ms | Below timer resolution |

Both engines produced the same arithmetic results. These are single warmed
diagnostic samples, not a reportable benchmark matrix. The difference isolates
bridge crossings rather than JavaScript arithmetic. The fixture contains no
Google source, identifiers, checks or generated payloads.

## Consequence for a future correction

The concrete runtime target is to reduce owner transitions, reflection and
value encoding for ordinary cross-realm reads/calls. Any implementation must
preserve canonical identity, getters/proxies, receiver and exception realms,
origin checks, navigation invalidation and the Page event loop. Replacing a
foreign function with a local copy, fabricating measurements, or changing a
site payload would not be a browser compatibility correction.

A bridge optimization needs local semantic regressions and measurements before
a new live observation. Even if that eliminates the long tasks, it must not be
called a Google-rejection fix without fresh evidence. The exact server-side
reason remains unproven; the response does not disclose it.

## Reproduction records

Ignored helpers: `.build/google_signin_rejection_trace.py`,
`.build/google_signin_rejection_profile.py`, `.build/mimic-diagnostic.go` and
`.build/frame_bridge_oracle.py`. The diagnostic executable exposes trace and
standard Go profiling only on loopback; production sources were not changed.
All owned browser processes were terminated after each run.

Raw traces, responses, profiles and summaries remain private and untracked.
The loopback captures are `frame-bridge-oracle-20260913-{mimic,chrome}`.
No real account password, successful login, or protected account data was used.
