# WebGPU capability projections

Frozen headful Chrome 152.0.7977.82 local controls and raw passports are retained
under `.build/residual-gpu`; the reusable probe and full observed values are in
`internal/browser/testdata/webgpu_capability_projection_*`.

The confirmed browser contract gaps were five absent limits
(`maxImmediateSize` and the four per-vertex/per-fragment storage limits), and
the missing `GPU.wgslLanguageFeatures` setlike projection. Baseline device
limits now come from a shared Go model. Adapter limits are independently
configurable through Environment; Window and Worker use the same projection.
Language capabilities belong to the versioned profile. Their object identity,
iteration, membership and callback semantics use one private set.

Independent fresh Chrome processes changed the insertion order of both adapter
features and WGSL features. Those raw differences are retained; the focused
contract comparison checks set membership, not a particular process's order.
Adapter capacity values are hardware/profile observations, not replacements for
the baseline device limits. Tests check the entire limit inventory and all
baseline values while allowing Environment to select adapter capacities.

Focused ordinary/restored Window/Worker capability and existing WebGPU resource,
lifecycle and navigator oracles passed. This change adds no native GPU backend
and makes no claim about unsupported shader/texture execution outside these
measured capability observations.
