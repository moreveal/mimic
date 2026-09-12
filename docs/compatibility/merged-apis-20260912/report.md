# Trusted Types and Performance integration replay

Local main integrates Trusted Types branch head `1dbe061` through merge
`d53549e` and Performance branch head `d0c8661` through merge `71c080c`.
Conflicts were resolved by combining the new Performance implementation and
Trusted Types enforcement with the existing clone codec, native function source
registry, shared console and computed-style argument validation. Neither branch
was selected wholesale over the other.

## Saved-program results

A fresh executable built at `71c080c` ran the original saved responses twice.
Both runs produced ten complete input/output pairs, all matched to POST bodies,
with zero parser errors. Comparisons use the two existing frozen headful Chrome
152.0.7977.82 controls. They do not contact the live target or establish its next
server decision. The binary hash and field-level results are in
[summary.json](summary.json); [field-registry.csv](field-registry.csv) covers all
1,499 rows. Its review labels refer to historical localization notes; equality
and repeat columns are newly measured.

| Field | Both replay runs, both child executions |
| --- | --- |
| `IGBuA2`, `oHIQ6`, `SbVZ3`, `MlWrD5` | Complete Chrome matches. |
| `OjmeV1` | Only random-derived index 75 differs; 82 and 118/119 remain fixed. |
| `knVv1` | Default-policy accumulator changes from 0 to Chrome's 7. |
| `UYbIv2` | Changes from the old success/error token to the Chrome token. |
| `BMnw0` | Memory API presence changes from false to Chrome's true. |
| `xWWV8` | Changes from null to the 18-entry memory observation sequence. Numerical samples, including the limit projection, still differ from Chrome. |

The absence of performance.memory is repaired; exact memory equivalence is not.
The imported implementation explicitly models an application-heap projection.
Its capacity/limit policy and cached samples are not Chrome's exact heap/GC
policy. Variable byte samples should not be frozen as universal constants.

## What remains

**The complete earlier investigation is not fully fixed.** Run A still has
416 differing rows across 68 field names. 73 rows across 39 names happen to be
stable in both pairs of controls; this is not a count of 73 independent bugs.
These numbers include environment choices, timings and coincidentally repeated
random values as well as actual compatibility gaps.

Previously localized, still differing families include:

- CSS rule serialization and computed property inventory/order (`ZkTjK2`,
  `NNZHC4`), independent of the repaired flat-tree observation.
- Canvas/WebGL/SVG/geometry observations (`XYvy9`, `Jpzg5`, `qydV6`, `RKUE0`,
  `JRzmw6`, `oSIr8`, `HPcn5` and related hashes). The `ZwhIC5` continuation/result
  observation also remains different; its exact root was not established.
- Image decoding/size (`thhSb9`, `ppMls5`, `cHIoX8`), offline audio
  (`MnIr8`, `CzUP6`), and media capabilities.
- Storage-directory/worker-file behavior (`DdIVt1`, `uUOw3`).
- Permission/profile projections (`XCvwf5`, `xVwa3`), collected entries/reentry
  (`rPXg2`, `Pdbt7`), selector-access reentry (`maNnU6`) and other unresolved
  metadata/observation counters.
- WebGPU limits/feature order/WGSL features (`lDUiR4`), WebGL context attributes
  (`etmnR7`) and incompletely reduced capability arrays (`KMUh5`, `OYbs6`).
- WebRTC SDP/ICE (`uHNG9`, `KnOhl5`), with different network restrictions in the
  Chrome diagnostic harness, and RTP codec capabilities (`xkNI3`).
- Observation counters (`lgWCE7`), stacks (`gPuVs8`, `PWGF4`) and numeric root
  metadata (`NnqX6`, `TzZRB1`, `ZMSOw0`, `twvE0`, `myWtu3`, `tZwbF3`,
  `uGyjw9`, `wOvYJ5`, `Blsob5`, `eaaP6`, `poqG1`, `WHTpH6`). Exact generating
  operations for all these values remain unlocalized; they are not assumed
  harmless simply because some vary.

Environment/timing observations also remain: `gsLi5`, connection estimates
(`zYUn5`, `CYsxg7`), visibility (`MfOHt6`, `RlMmu2`), quota (`RPKTR7`), stage
times (`TPpkV4`, `Vtvy6`, `hCfV6`), elapsed callbacks/work/ICE (`qIEN4`, `hCKCP8`,
`jBVrk8`, `qdVjJ1`, `lIEA1`, `uZjT5`) and recorded-response status (`jyDXx2`).
These require matching environment/observation boundaries before calling them
implementation defects.

Some residuals do concern execution paths: storage's rejection replaces a
successful worker/file continuation; the earlier `ZwhIC5` trace failed to enter
the success callback; capability/permission differences select other result
paths, and selector reentry changes the observed call sequence. Other findings
only establish different inputs to hashes or aggregates. We have not traced
every later VM predicate, so neither universal branch equality nor a branch
consequence for every differing hash is established.

The full registry retains additional variable graphics, network, timing and
environment fields. An unequal hash is not treated as proof of a broken hash
algorithm. No newly repeat-stable whole-field difference against a previously
equal Chrome-stable value was found except the already random-derived
`OjmeV1[75]` and a timing `hCfV6` row.

The Performance branch separately documents opaque Fetch completion, streaming
document responseEnd and agent-cluster memory attribution boundaries. Its
reported intermittent snapshot responseEnd failure and throughput regression
signals are not closed merely by merging or by this replay. Trusted Types also
retains its documented unsupported execution/reporting boundaries. See the
[Performance report](../../performance-api/README.md) and
[Trusted Types report](../trusted-types-20260912/report.md).

## Validation

Integrated focused tests pass for Trusted Types, worker enforcement, Performance,
native functions, computed style, console/CDP and structured clone. No frozen
expectations were weakened.

The broader eight-package run **fails** in
`TestPerformanceNetworkAndLifecycleMatchFrozenChrome/snapshot/performance_navigation_events`:
responseEnd remains unavailable at DOMContentLoaded, load and after-load
(rows 1–3). Browser finished in 373.195 seconds; CSP, webapi, engine/v8, CDP,
network, scheduler and DOM packages passed. The same failure was documented on
the original Performance branch before integration, so this is an unclosed
imported defect, not evidence of a newly introduced merge failure. Two circular
JSON teardown diagnostics also appeared. The earlier focused pass does not
override this broader failure. No tests or expectations were modified.

The merged code at `71c080c` was pushed to origin/main. Source branch worktrees
were preserved; no unrelated branch was merged or removed.

Private raw evidence and the executed binary remain in
`.build/merged-api-replay/`, including `run-a` and `run-b`. Original captures,
Chrome controls and earlier replay binaries were preserved.
