# Floating texture observations (2026-09-12)

The existing texture-level model now stores float and half-float uploads as
normalized floating values. WebGL1 exposes OES_texture_float,
OES_texture_half_float and their linear-filter extensions with the corresponding
upload and completeness rules. WebGL2 uses core half-float filtering and the
OES_texture_float_linear gate for float32 filtering. Filtering depends on internal
storage format, not merely the source array type.

RGBA16F storage quantizes Float32 uploads to IEEE half precision before later
sampling, including subuploads. RGBA32F retains Float32 precision. Pixel-type array
mismatches produce INVALID_OPERATION rather than a JavaScript exception. WebGL2
unsized RGBA/FLOAT combinations are rejected; its sized float formats are accepted.

Three independent frozen Chrome 152.0.7977.82 A/B fixtures cover extension gating,
linear sampling, Float32/Uint16 type errors, unsized/sized uploads and shader-
amplified half-storage quantization. Both bootstrap modes use the frozen fixtures.
Private evidence is under main `.build/residual-continuations-delegated/webgl-float*`.
This extends the bounded 2D texture observations; unsupported framebuffer execution
and advanced filtering retain the boundaries documented in webgl-textures-20260912.md.
