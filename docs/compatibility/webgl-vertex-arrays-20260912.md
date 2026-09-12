# WebGL vertex arrays and instancing (2026-09-12)

`OES_vertex_array_object`, `OES_element_index_uint`, and `ANGLE_instanced_arrays`
now have behavior on the same resource/observation kernel as core WebGL2 vertex
arrays and instancing. Vertex-array objects own pointer/enabled/divisor state and
the element-buffer binding. Generic attribute values remain context-owned, so they
survive array switches without copied, separately synchronized state.

Indexed draws decode unsigned byte/short/int indices from the selected element
buffer. Instanced draws use gl_InstanceID and divisor-indexed attributes. Pending
commands retain their inputs. The native profile's robust vertex fetch returns
zero for an incomplete enabled attribute, including a partly present tuple; it
does not expose the prefix or return a GL error. Missing-program validation takes
precedence over draw-argument validation, even for zero instances. These behaviors
were established by independent Chrome controls, not inferred from the payload.

Five frozen Chrome 152.0.7977.82 A/B fixtures cover binding identity, fresh/bound/
deleted objects, VAO switches, shared generic attributes, per-VAO elements/divisors,
array/indexed32-bit instancing, divisors1/2, incomplete and partial attributes, and
error precedence. Ordinary and restored bootstrap paths run the same fixtures.
Private evidence remains in main `.build/residual-continuations-delegated/webgl-vao`
and `webgl-instanced*`.

The arithmetic renderer's existing boundaries remain explicit: primitive restart,
nontriangle execution, clip-space crossing and unsupported shader state are not
silently approximated by these extensions. The larger remaining extension
inventory is not advertised by this package.
