# WebGPU render/readback observations in frozen Chrome 152

The remaining saved-program `ZwhIC5` sentinel came from `GPUDevice.createTexture`
throwing an unsupported-operation error. Its producer is a WebGPU texture render
pass, texture-to-buffer copy, mapping, byte decoding and SHA-256 chain. It is not
the adjacent WebGL digest and it is not evidence of a lost Promise reaction.
Earlier localization associated nearby asynchronous chains incorrectly.

The first relevant missing operation was at VM IP 191405. Native execution
continues through a render pass, `draw`, `copyTextureToBuffer`, `mapAsync`,
`getMappedRange` and the result callback. The repaired saved offline replay
matches Chrome's result in both corresponding packets. Raw evidence is retained
outside the removable worktree in
`.build/residual-continuations-delegated/render-fixed` in the original checkout.
No captured shader, expected digest, field name or VM dispatch is implemented.

The implementation extends the existing resource and queue model with bounded
RGBA/BGRA texture storage, render-pass clear/load/store, draw observations,
partial texture writes/copies and padded texture-to-buffer copies. Submission
uses the same texture bytes subsequently observed by mapping or canvas copies.
Resource ownership, destroyed resources, mapped destinations and command encoder
locking participate in validation. Canvas configuration, retained current
texture identity, resizing and unconfiguration use the existing canvas factory
and readback interface.

A WGSL parser lowers arithmetic vertex/fragment functions to the existing
scalar/vector evaluator. It supports local declarations, fixed arrays, indexing,
constructors, conditionals and arithmetic builtins. Triangle coverage uses the
top-left rule. Sources are parsed as data, never evaluated as JavaScript.

The independent checked-in oracle covers a different triangle and color than
the saved program, clear/load/scissor/discard, row padding, partial BGRA writes,
texture copies, foreign-device validation, an unfinished pass and canvas
readback/reset. Frozen Chrome 152.0.7977.82 A/B controls agree, and Mimic agrees
with the matrix. The regression runs both ordinary and restored bootstrap.
Existing WebGPU resources/capabilities and WebGL tests remain checked. The
explicit unsupported-execution regression now tests multisampled textures;
ordinary single-sample texture creation has positive native regression coverage.

This is bounded observation support, not full WGSL or WebGPU rendering. Compute,
bind groups, sampled textures, interpolation of user varyings, clipping,
depth/blend, multisampling, extra color formats and mip/layer observations remain
explicit unsupported boundaries. Shader precision and canvas presentation-time
texture expiration require additional observations outside the tested cases.
These limits are not classified as architectural impossibilities or as harmless
environment variation. The saved-program continuation is closed; broader
graphics groups retain their independent dispositions.

The follow-up alpha oracle confirms that `premultiplied` canvas texture bytes
already have premultiplied meaning; multiplying them again would be wrong. It
compares both alpha modes, translucent and out-of-alpha-range colors, direct
canvas and ImageBitmap copies. Unconfiguration retains the last materialized
canvas snapshot, but does not promote an unread GPU texture to that snapshot.
Reconfiguration and resizing clear the snapshot. Frozen controls and ordinary/
restored regression cover both positive and no-prior-read cases.
