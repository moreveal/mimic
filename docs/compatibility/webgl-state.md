# WebGL state without rendering

The shared Window/Worker layer owns context identity, dimensions, buffer object
ownership and copied upload bytes, buffer updates/readback, shader source strings,
and color clear/readback state. Scissor rectangles and color masks affect the
stored drawing buffer. Canvas snapshots convert its bottom-up RGBA storage to the
canvas layer's top-down premultiplied representation; transfer clears the source
storage. Resizing preserves WebGL resource and command state. Assigning an
OffscreenCanvas dimension its existing value preserves its WebGL pixels, as
measured in Chrome 152 (this differs from the 2D context reset).

Environment.Graphics remains authoritative for vendor, renderer and maximum
texture size in Window and Worker. No native machine fingerprint was copied into
the implementation. Contexts use a single sample; explicitly requesting
multisampling fails. The state-only layer has no presentation/compositor task and
does not discard a drawing buffer on presentation.

Shader compilation, program linking, drawing, textures and framebuffer attachment
evaluation are explicit NotSupportedError boundaries with semantic diagnostics.
Generated operations outside the implemented state subset also fail explicitly.
The presence of a WebGL context does not assert that arbitrary shaders can run.
In particular, no shader-derived pixels, precision values, extensions or GPU
limits are fabricated. Unmodeled recognized parameter queries fail explicitly;
values outside the generated enum inventory produce INVALID_ENUM and null.
Typed-array offset overloads, non-RGBA/UNSIGNED_BYTE pixel reads and large storage
allocations also remain explicit boundaries.

The supported storage/clear/reset fixture is retained in
`internal/browser/testdata/webgl_state_oracle.js`, with a frozen headful Chrome 152
capture and environment metadata in
`compatibility/captures/semantic-checkpoints/webgl-state-chrome152.json`.
Browser tests execute that same source, and separately check ownership, uploads,
scissor changes, repeated reads, Window/Worker profile parity, snapshot orientation
and the unsupported shader boundary. This work does not establish compatibility
with workloads that consume shader-derived pixels.
