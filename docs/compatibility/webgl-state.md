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
the implementation. Masked VENDOR/RENDERER and API/GLSL version strings are the
measured Chrome WebGL 1/2 interface identifiers, independent of machine identity;
they do not imply shader execution is implemented. Contexts use a single sample; explicitly requesting
multisampling fails. The state-only layer has no presentation/compositor task and
does not discard a drawing buffer on presentation.

Program/shader ownership, linking, active inputs, uniforms and lazy draw/readback
observations now cover a bounded arithmetic GLSL subset. Renderbuffer attachment
state also has a shared implementation. See `graphics-observations.md` for the
measured cases and explicit limits. This does not make arbitrary shaders or a
native graphics backend available. Texture execution, multisampling, typed-array
offset overloads and non-RGBA/UNSIGNED_BYTE readbacks remain unsupported.

The supported storage/clear/reset fixture is retained in
`internal/browser/testdata/webgl_state_oracle.js`, with a frozen headful Chrome 152
capture and environment metadata in
`compatibility/captures/semantic-checkpoints/webgl-state-chrome152.json`.
Browser tests execute that same source, and separately check ownership, uploads,
scissor changes, repeated reads, Window/Worker profile parity, snapshot orientation
and the unsupported shader boundary. Additional arithmetic and attachment oracles establish only their covered
observations, not arbitrary shader or GPU numerical equivalence.
