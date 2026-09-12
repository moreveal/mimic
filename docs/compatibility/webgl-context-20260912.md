# WebGL context and multisample observations

Frozen Chrome 152.0.7977.82 enables antialiasing by default, preserves the
requested power preference and desynchronized flag, and includes xrCompatible
in the returned attribute dictionary. The frozen machine has no compatible XR
provider and returns false even when that attribute was requested. Boolean
dictionary values use conversion rather than comparison with the literal false.

Mimic now models these context observations, including the actual four-sample
drawing storage used by this frozen graphics profile. Each sample retains its
own coverage/color state across overlapping draws. Clears update all covered
samples, and readback resolves the stored samples consistently. Resizing and
canvas transfers reset sample storage. Reporting antialiasing is therefore tied
to the corresponding behavior, rather than being an isolated metadata change.
Single-sample and multisample triangle edge coverage follow the measured edge
inclusion rule. The selected sample positions and resolve rounding are frozen
backend observations, not a claim that all GPU profiles choose those positions.

Two independent native A/B matrices cover WebGL1/2 attribute defaults, requested
attributes, Boolean/string conversions, invalid power preference, translucent
capability metadata, and single/multisample triangle readback with repeated
reads, overlapping draws, scissored clear and resize. Both matrices have zero
control differences and zero Mimic differences. Checked-in regressions cover
ordinary and restored bootstrap; existing WebGL/WebGPU focused tests also pass.
Private matrices remain under `.build/residual-continuations-delegated/webgl-*`
in the original checkout.

This localizes the known `etmnR7` antialias/powerPreference observations and the
same context/sample parameters in `OYbs6`. It does not claim that the remaining
extension inventory in `qydV6` or the nested `KMUh5` producer group is fixed.
Unimplemented extension semantics and unsupported drawing operations remain
independent work, rather than being advertised solely to change capability bits.
