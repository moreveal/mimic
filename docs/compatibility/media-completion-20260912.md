# Media completion and RTC cache lifetime, frozen Chrome 152

Reference: headful Chrome 152.0.7977.82, separate fresh processes/profiles A/B,
no feature overrides, local HTTP fixture. The committed oracle retains version,
origin/window metadata and source/binary hashes. Both controls and Mimic agree
on all 16 named observations, including ordinary and restored bootstrap runs.

Supported file/media-source video decodingInfo resolves outside the initiating
microtask checkpoint. Audio, unsupported video and the tested RTP configuration
resolve during it. Mimic previously resolved every successful query in a
microtask. The decoder query now completes through the owning realm's host
scheduler, without depending on author-replaced timers. Repeated and concurrent
queries preserve results. This is a task boundary, not an invented decoder delay;
ordering against independent timer tasks and hardware service latency varies.

Adjacent measured validation: zero and unsigned-wrapped dimensions do not reject
the codec query, but cannot form a positive signed decoder extent and are not
power efficient. Negative bitrate is accepted by the unsigned dictionary contract.
Nonpositive framerate still rejects. No device-specific maximum decoder dimensions
or decoder performance estimator is introduced by this patch.

## Private capture producer localization

Only manual-20260912-045102 was investigated. The labels HDEX5/Aqcaj8 previously
attributed to media capability support were inaccurate. They consume a helper's
RTC cache, written on Window as cyUJp9. Its Gamg7 array contains candidates and
iYEgs4 records successful offer completion. The helper schedules a 5000 ms cleanup
(IP 3230/3791; first bound callback entry 635). Cleanup reads the cache at IP656,
closes the peer at 714, and writes null at725. The main producer reads it at152390.
Chrome reaches the object before cleanup; Mimic reaches null and starts a third
peer. This explains HDEX5=3 versus0 and Aqcaj8=true versus null without changing
ICE candidate counts or introducing a captured configuration special case.

Clock receipt from mimic-clock: first timeout scheduled at Date1789189561971 /
performance40.575, cache cleared at Date1789189567743 / performance5812.08
(5771.505 ms). Second timer elapsed5099.94ms. Main cache read was performance
7495.205 in the second realm, after its deadline. There is no premature timer.
A bounded diagnostic replay with advance1ms/sleep1ms also reached the main read
at performance6646.745 after expiration. Neither this replay nor the historical
saved VM diagnostics supplies generic Chrome expectations; their instrumentation
and replay environment are explicitly diagnostic.

XqEQ3 combines canPlayType support with audio/video decodingInfo bitmasks. The
last video component can still be null when the producer samples before the
external decoder answer. Frozen final codec values match; the separately proven
microtask completion defect is fixed. A slower path which waits on the fallback
RTC peer can still observe the completed array; equal payload bytes are not a
valid requirement across these deadline races.

Raw receipts, runner/probe sources and diagnostic traces are retained outside the
worktree at .build/residual-media-final-delegated. Focused checks:
`go test ./internal/browser -run 'TestMedia(Completion|Support)MatchesFrozenChrome' -count=1`.
No full suite, race or performance gate was run for this package.
