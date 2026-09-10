# Remaining frame bridge cost and ownership alternatives

The remaining cost is primarily the browser/engine ownership boundary, not a
language-imposed minimum for Go. A native two-context experiment using the
same Go/V8 binding executes the same operations without Mimic's remote proxy
bridge orders of magnitude faster. This is evidence for an architecture
change, not a completed iframe implementation or a Chrome-parity claim.

## Operation breakdown

These are new measurements of the same 100-iteration checksum-6050 probe from
the [earlier investigation](../compatibility/task-profile-2026-09-10.md).
The earlier whole-probe result was roughly 195–205 ms; the fresh decomposition
is approximately **222 ms**. Do not rescale it to force a 195 ms total.
Reported numbers are medians of five rounds after one initial round. Windows
QueryPerformanceCounter measures real elapsed time, including sub-millisecond
operations, independently of the page's performance clock.

| Phase | Work | Median ms |
| --- | --- | ---: |
| Setup | Create/append iframe, initialize its realm, seed child objects | 68.922 |
| Global lookup | Read `w.chain` 100 times | 12.960 |
| Prototype traversal | 300 `Object.getPrototypeOf` calls, plus one root lookup | 51.177 |
| Property reads | 100 `w.obj.x` evaluations | 34.488 |
| Calls | 100 `w.fn(i)` evaluations | 53.973 |
| Removal | Remove iframe | 0.168 |

The complete mixed loop, measured separately, is **153.126 ms**, close to
the sum of the four loop components, **152.598 ms**. The empty evaluation
boundary is 0.096 ms. A local equivalent in the same Mimic realm is 0.123 ms.
The isolated phase timings include their evaluation boundaries; they are an
approximate decomposition, not a partition from one instrumented execution.

In a separate host-instrumented run, the fresh child's surface compile and
execution cost approximately **20.0 + 45.7 ms**. They explain most setup time.
The bare V8 isolate/context allocation is much cheaper, as measured below.
Do not add nested host timings to the surface execution total.

## Layer accounting

Twenty repetitions on already created realms, with before/after counters,
give per 100-iteration loop:

* Approximately **2,500 Go host callbacks** across caller and child.
* Approximately **4,609 owner-command dispatches**, excluding almost all setup
  but including a small diagnostic/evaluation boundary contribution.
* Approximately **17,101 additional persistent handles** in the two adapters.
  These are retained handles at the snapshots, not 17,101 distinct live JS
  objects or a claim of a post-teardown leak. Many observations persist native
  values until runtime close rather than using a short-lived scope.

A separately measured empty owner command takes about **10.97 microseconds**
(10,000 commands in 109.745 ms). Multiplying by the observed dispatch count
gives a scale of **50.6 ms** per mixed loop. This is a model estimate, not an
exclusive timing slice: nested dispatch, scheduling, native calls and useful
work overlap differently in the actual workload.

The Go CPU profile of 20 mixed loops records 3.29 s elapsed and 2.22 s sampled
CPU. Flat samples include 0.64 s `runtime.cgocall` and 0.52 s
`runtime.semawakeup`. These establish substantial native-call and wakeup
cost, but must not be subtracted from elapsed time to invent an exact
Go-versus-V8 remainder. Proxy dispatch, reflection, argument/result envelopes,
cross-realm identity bookkeeping and handle allocation are additional work.

## Alternative models and measured limits

### Larger operations within the existing isolate-per-realm model

A temporary candidate grouped related target operations on its owner thread.
The five-iteration benchmark improved from **204.323 ms to 174.106 ms** per
probe, about **15%**. Focused frame/nested/cancellation tests passed. This
experiment was withdrawn from production sources and retained as a private
patch; it does not justify a new ownership interface as the preferred route
to a forty-fold improvement. Earlier reference-encoder improvements remain.

### One Page owner thread, but separate isolates for each realm

This could remove many inter-thread handoffs while retaining cross-isolate
value encoding, proxy reflection and separate bootstrap work. It was not
implemented here. The ~50 ms handoff scale suggests a useful but insufficient
gain; it does not establish an upper bound or a guaranteed saving.

### One isolate for a compatible realm group, distinct native contexts

This is the preferred model to investigate next. Same-origin synchronous
access can use direct V8 references, with contexts retaining their own globals,
constructors and prototypes. Each Page must retain independent ownership and
execution; workers remain separate. Origin/agent-cluster rules determine which
realms may share an isolate. Cross-origin WindowProxy restrictions, navigation,
document replacement, context disposal and microtask ordering still require
browser-level implementation and regression evidence.

The bounded engine experiment uses two native contexts in one isolate, a
shared same-origin security token and direct child-global references. It checks
that `Array` and `Object` constructors differ between realms and that the child
object's prototype belongs to the child. Both runs preserve checksum results.

| Bare engine operation | Median ms |
| --- | ---: |
| Create isolate | 0.794 |
| Create two contexts | 0.558 |
| Same 100-iteration cross-context loop, including evaluation | 0.147 |
| 100,000 iterations of the same loop | 12.397 |
| Dispose isolate | 0.945 |

The 100-iteration result is an existence proof that this kernel does not need
153 ms in Go+V8. It omits the DOM, WindowProxy lifetime/security machinery,
navigation and bootstrap; it is not a measured speed for a production iframe.

If loop overhead approached native-context cost while current setup stayed
unchanged, the measured probe would fall from approximately **222 ms to
69–70 ms**, roughly **3.2x** overall. Reaching the earlier Chrome observation
of **4.5–5.1 ms** for the full probe additionally requires reducing bootstrap
cost. Compiled bootstrap reuse, per-context templates or snapshots are
candidates, but were not evaluated here. Realm-local mutable prototypes and
host bindings must not be shared accidentally to obtain that result.

A Go-to-Rust rewrite with the same thousands of transfers, proxy envelopes and
retained handles is not supported by these measurements as the preferred fix.
The next useful experiment is a shared-isolate, two-context **browser** slice
with real DOM identities and teardown, followed by context bootstrap reuse.

## Artifacts and reproducibility

Private directory: `compatibility/private-captures/owner-batch-20260910`.
`breakdown-qpc/phases.json`, `shared-owner-qpc/model.json`, `profile` and
`dispatch-profile` retain phase measurements, host counters and Go profiles.
`owner-batch-candidate.patch` is the withdrawn owner-batching experiment.
Production source changes from that experiment were restored before the final
measurements. The retained tests are opt-in diagnostics, not changes to frozen
benchmarks or expected Chrome behavior:

* `TestFrameBridgeBreakdown`: set `MIMIC_BRIDGE_BREAKDOWN` to an output directory;
  optionally enable `MIMIC_PROFILE_HOSTS=1` for host counters and CPU sampling.
* `TestOwnerModelProbe`: set `MIMIC_OWNER_MODEL_PROBE` to an output directory.
* `BenchmarkFrameBridgeOperations`: runs the unchanged checksum probe through
  the production browser implementation.

The dispatch counter was temporary diagnostic instrumentation, restored after
capture. Host timings and counters are separate from the uninstrumented QPC
phase measurements. No new production optimization is retained from this
ownership experiment.
