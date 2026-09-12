# WebGL extension operation audit

Frozen headful Chrome 152.0.7977.82, two separate fresh processes/profiles, no feature overrides. Independent local fixture only. A/B: **0 differences**. Chrome exposes 35 WebGL1 and 32 WebGL2 names on this selected GPU environment; Mimic now exposes 20 and 13 respectively. All advertised extension operation observations match the corresponding native observations. Full inventory parity is **not** claimed.

## Structural correction

Framebuffer component-type and color-encoding queries now use the authoritative renderbuffer storage record. The record distinguishes the accepted requested internal format from the successfully allocated storage format: WebGL1 half-float requests can update the former while a driver-level validation error preserves the previous allocation. Zero-sized successful allocations retain their component type, while never-allocated storage reports no components. Invalid queries return the measured WebGL error instead of throwing an unsupported-operation exception.

WebGL1 half-float color allocation requires both color-buffer and half-float texture capabilities. `WEBGL_color_buffer_float` exposes the existing genuine float renderbuffer allocation/query path; `OES_texture_float` implies that color-buffer capability. WebGL2 format gating remains separate. The allocation/reallocation oracle has 14 groups with **0 Chrome/Mimic differences**, including failed/zero-sized allocations, packed depth-stencil errors, extension enable combinations and format/component queries.

Existing explicit boundaries still apply to attachment pixel execution, multisampling and texture attachments. The newly exposed float color-buffer name has real allocation/status/component behavior, not empty registration, but does not claim complete framebuffer rendering.

## Complete inventory and minimal operation

“Pending” means not implemented in this model, not an architectural impossibility. Compile checks for language extensions validate pragma acceptance only; buffer uploads, limit queries and validation-only draws are deliberately minimal and do not prove full rendering support. The float blend probe validates a draw against a float attachment, not its numerical blended pixels.

| Extension | Native modes | Mimic modes | Minimal native operation/query |
|---|---|---|---|
| `ANGLE_instanced_arrays` | 1 | 1 | attribute divisor mutation/query |
| `EXT_blend_minmax` | 1 | Pending | blend equation mutation/query |
| `EXT_clip_control` | 1/2 | Pending | clip origin/depth mutation/query |
| `EXT_color_buffer_float` | 2 | 2 | float renderbuffer and attachment component query |
| `EXT_color_buffer_half_float` | 1/2 | 1/2 | half-float renderbuffer and attachment component query |
| `EXT_conservative_depth` | 2 | Pending | required GLSL extension compilation |
| `EXT_depth_clamp` | 1/2 | Pending | depth clamp enable/query |
| `EXT_disjoint_timer_query` | 1 | 1 | query counter bits, elapsed begin/end, completion availability |
| `EXT_disjoint_timer_query_webgl2` | 2 | 2 | query counter bits, elapsed begin/end, completion availability |
| `EXT_float_blend` | 1/2 | Pending | float framebuffer + linked program + blended draw validation |
| `EXT_frag_depth` | 1 | Pending | required GLSL extension compilation |
| `EXT_polygon_offset_clamp` | 1/2 | Pending | polygon offset factor/units/clamp mutation/query |
| `EXT_render_snorm` | 2 | Pending | signed-normalized renderbuffer allocation/query |
| `EXT_sRGB` | 1 | Pending | sRGB texture allocation |
| `EXT_shader_texture_lod` | 1 | Pending | required GLSL extension compilation |
| `EXT_texture_compression_bptc` | 1/2 | 1/2 | compressed format enum query |
| `EXT_texture_compression_rgtc` | 1/2 | 1/2 | compressed format enum query |
| `EXT_texture_filter_anisotropic` | 1/2 | 1/2 | anisotropy mutation/query |
| `EXT_texture_mirror_clamp_to_edge` | 1/2 | Pending | texture wrap mutation/query |
| `EXT_texture_norm16` | 2 | Pending | 16-bit normalized renderbuffer allocation/query |
| `KHR_parallel_shader_compile` | 1/2 | 1/2 | shader completion query |
| `NV_shader_noperspective_interpolation` | 2 | Pending | required GLSL extension compilation |
| `OES_draw_buffers_indexed` | 2 | Pending | indexed color write mask mutation/query |
| `OES_element_index_uint` | 1 | 1 | Uint32 element-buffer upload/size query |
| `OES_fbo_render_mipmap` | 1 | Pending | nonzero-mip framebuffer attachment/status |
| `OES_sample_variables` | 2 | Pending | required GLSL extension compilation |
| `OES_shader_multisample_interpolation` | 2 | Pending | interpolation limits query |
| `OES_standard_derivatives` | 1 | 1 | derivative hint mutation/query |
| `OES_texture_float` | 1 | 1 | float/half-float texture allocation |
| `OES_texture_float_linear` | 1/2 | 1/2 | texture linear filter mutation/query |
| `OES_texture_half_float` | 1 | 1 | float/half-float texture allocation |
| `OES_texture_half_float_linear` | 1 | 1 | texture linear filter mutation/query |
| `OES_vertex_array_object` | 1 | 1 | VAO creation/binding/identity/deletion |
| `OVR_multiview2` | 2 | Pending | maximum views query |
| `WEBGL_blend_func_extended` | 1/2 | Pending | dual-source blend factors and limit query |
| `WEBGL_clip_cull_distance` | 2 | Pending | clip distance enable and clip/cull limits |
| `WEBGL_color_buffer_float` | 1 | 1 | float renderbuffer and attachment component query |
| `WEBGL_compressed_texture_s3tc` | 1/2 | 1/2 | compressed format enum query |
| `WEBGL_compressed_texture_s3tc_srgb` | 1/2 | 1/2 | compressed format enum query |
| `WEBGL_debug_renderer_info` | 1/2 | 1/2 | unmasked identity query types |
| `WEBGL_debug_shaders` | 1/2 | 1/2 | compiled translated-source nonemptiness |
| `WEBGL_depth_texture` | 1 | Pending | depth texture allocation |
| `WEBGL_draw_buffers` | 1 | Pending | draw buffer selection and limit query |
| `WEBGL_lose_context` | 1/2 | 1/2 | context loss and error transition |
| `WEBGL_multi_draw` | 1/2 | Pending | zero-count multi-draw validation |
| `WEBGL_polygon_mode` | 1/2 | Pending | polygon mode mutation/query |
| `WEBGL_provoking_vertex` | 2 | Pending | provoking vertex mutation/query |
| `WEBGL_stencil_texturing` | 2 | Pending | texture depth/stencil mode mutation/query |

## Regression and evidence

`TestWebGLColorAttachmentMatchesFrozenChrome` compares all 14 allocation/query groups exactly in ordinary and restored-bootstrap realms. `TestWebGLExtensionOperationsFrozenChrome` pins the implemented inventory and compares every exposed extension operation against the full native oracle; removing a name cannot silently skip its regression. The native inventory and its pending names remain intact in the retained full audit JSON.

Focused neighboring capability/state, float-texture/error, timer-query and context-loss tests pass. No full suite, race or performance gate was run. Raw A/B outputs, separate color/all-extension passports and runners are outside the disposable worktree at main `.build/webgl-extension-audit`. Retained expectations include exact Chrome/Chromium/V8 versions, origin/security and viewport state, profile ID, and probe/binary digests.
