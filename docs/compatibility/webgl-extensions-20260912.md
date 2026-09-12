# WebGL extension lifecycle and queries (2026-09-12)

The extension registry now couples advertised extensions to their private objects,
constants and state. This package adds `KHR_parallel_shader_compile`, timer queries
and `WEBGL_lose_context`; it does not claim the frozen device's full inventory.
Compilation is synchronous in the software model, so completion queries are true
for valid shader/program resources after the extension is enabled.

Timer queries have context-owned resource identity, binding, active/pending/result
states and deletion. Flush/finish submits pending work; availability changes in a
later task. The software elapsed interval comes from the runtime clock, not a
hardware GPU clock. The frozen profile reports zero timestamp counter bits and
accepts timestamp queries. Occlusion results come from covered, non-discarded
samples in the same observation kernel used for readPixels, including scissoring.
Transform-feedback commands remain explicitly unsupported. No synthetic positive
query result is substituted for a draw.

Context loss changes observation state immediately, delivers a cancelable loss
event asynchronously, and allows restoration only after preventDefault. Restoration
resets context state and invalidates old resources through a context generation.
Retained extension objects preserve identity. WebGL1 and WebGL2 query differences
(fresh query result and counter binding) were measured independently.

Frozen Chrome 152.0.7977.82 A/B controls cover no-flush queries, explicit submission,
query errors/deletion, visible and scissored draws, loss/restoration, old resource
validity, retained extension identity, early restore, and uncanceled loss. Fixtures
are ordinary local scripts, not captured workload code. Tests run ordinary and
restored bootstrap paths. Private raw data and harness passports remain under
`.build/residual-continuations-delegated/webgl-query*` and `webgl-loss*` in the main
checkout. No private captures are committed.

The saved workload's remaining extension-derived values are not environment-only
noise: `KMUh5[0]` consists of thirteen WebGL1 inventory flags, `[1]` three WebGL2
flags, and `[3]` seven renderbuffer/FBO format checks (already matching). The WebGL1
flags include compressed texture families and debug shader translation. `qydV6`
also includes the inventory and count. Those require further coherent texture and
shader translation support; merely returning their extension names would be false
coverage. This package is a bounded correction, not closure of that inventory gap.
