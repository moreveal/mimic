# Mimic

Mimic is a lightweight browser execution runtime focused on reproducing
observable Chrome behavior without embedding Chromium. Frozen Chrome 152 is
the current behavioral reference.

## Core Principles

- Preserve observable Chrome behavior. When behavior is uncertain, measure it
  against frozen Chrome 152 rather than guessing from specifications or other
  browser implementations.

- Prefer behavioral compatibility over superficial API coverage. An API that
  exists but behaves incorrectly is not considered implemented.

- Preserve canonical object identity, realm boundaries, lifecycle ordering,
  task/microtask semantics, and one event loop per Page.

- Keep Page state isolated. Independent Pages must be able to execute
  concurrently and must not depend on a global runtime lock or shared mutable
  browser state unless the behavior explicitly requires it.

- Maintain a single authoritative state model. CDP, JavaScript bindings, DOM,
  navigation, networking, and other projections should observe the same
  underlying state rather than maintain independently synchronized copies.

- Mimic does not render images and must not require a GPU or a native graphics
  backend. Model the observations scripts can make, including consistent
  metrics, readbacks, local changes, resets and copies. Do not substitute
  independent random buffers or a fingerprint derived from the entire command
  sequence; repeated and overlapping observations must agree. See
  `docs/architecture.md` for the graphics observation boundary.

- Do not implement behavior specifically for individual websites, tests, or
  benchmarks. Fix the underlying browser semantic or architectural cause.

- Do not add Web APIs merely to increase API counts. Implement capabilities
  when required by real workloads, compatibility evidence, or architectural
  dependencies.

- Prefer the smallest implementation that reproduces the required observable
  behavior. Do not introduce layout, media, device, or other browser
  subsystems unless their observable behavior is actually required.

## Correctness

- Treat compatibility regressions as correctness regressions, even when the
  affected behavior appears obscure.

- When fixing a compatibility issue, add or extend a focused regression test
  whenever practical.

- Distinguish unsupported behavior from incorrectly implemented behavior.
  Prefer an explicit unsupported boundary over plausible but incorrect browser
  behavior.

- Do not weaken existing tests, frozen harnesses, or reference expectations to
  make an implementation pass.

- Preserve useful failures and diagnostics. Avoid silently swallowing errors
  solely to make workloads continue.

## Architecture

- Prefer structural fixes over accumulating special cases.

- Avoid duplicated state, unnecessary Go↔V8 crossings, hidden global
  synchronization, and ownership that makes Page teardown incomplete.

- Keep boundaries between runtime, DOM, navigation, networking, CDP, and
  platform APIs explicit. Do not bypass ownership boundaries for convenience
  without a demonstrated architectural reason.

- Be conservative with new abstractions. Introduce them when they remove real
  duplication or enforce an invariant, not in anticipation of hypothetical
  future requirements.

## Performance

- Optimize measured architectural overhead rather than benchmark-specific
  behavior. Profile before undertaking substantial performance work.

- Use `tools/performance/fast_gate.py` for iterative performance validation
  when appropriate. Reserve the full benchmark matrix for substantial
  optimization batches, regression investigation, and reportable checkpoints.

- Evaluate latency, throughput, memory usage, allocation pressure, concurrency,
  and retained state after teardown together.

- Performance changes must preserve correctness and observable semantics. Do
  not add APIs, weaken compatibility, or introduce workload-specific shortcuts
  solely to improve benchmark results.

- Do not modify frozen performance harnesses, workloads, or original baseline
  data. For reportable comparisons, ensure the executed binary corresponds to
  the intended fresh build.

- Record significant results, regressions, remaining bottlenecks, and known
  limitations in `docs/performance/report.md`.

## Changes

- Keep changes scoped to the problem being solved. Avoid unrelated refactors
  unless they are necessary for correctness or materially simplify the fix.

- Inspect and understand the existing implementation before replacing it.
  Preserve intentional invariants unless evidence shows they are wrong.

- Keep substantial changes reviewable and logically separated.

- Document non-obvious architectural decisions and important limitations close
  to the code or in the relevant project documentation.

- Keep experimental rewrites, including Rust spikes, separate from the
  production implementation until they have been evaluated against equivalent
  correctness and workload requirements.
