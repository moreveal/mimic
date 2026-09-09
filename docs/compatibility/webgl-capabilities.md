# WebGL capability observations

The Chrome 152 oracle records both WebGL versions with antialiasing disabled,
60 parameter queries each, renderbuffer sample-count queries and extension
activation. It preserves scalar/typed-array distinctions and GL errors. See
`compatibility/captures/semantic-checkpoints/webgl-capabilities-chrome152.json`
and its hashed fixture in `internal/browser/testdata`.

The immutable `Graphics.WebGLCapabilitiesJSON` profile is projected to Window
and Worker once when their WebGL bindings initialize. Empty selects the measured
Windows/D3D11 profile in `internal/state/webgl_capabilities.json`. Those measured
limits are an explicit default profile, not a claim about every physical GPU;
other environments can select their own profile. No native GPU is consulted.
Returned typed arrays are fresh copies. Extensions, hints, masks and context
attributes are owned by each context and remain separate from immutable limits.

`getInternalformatParameter` validates target, format and pname. Float formats
require the corresponding enabled color-buffer extension; integer formats may
return a valid empty Int32Array. Unsupported enums return null and INVALID_ENUM,
not a JavaScript exception. The exposed extension subset covers the implemented
query semantics. It is intentionally smaller than the native extension list.

Shader execution, framebuffer/texture operations and multisampled drawing remain
explicit unsupported boundaries. Querying supported sample counts does not add
a multisampled renderer. Default context antialiasing remains the existing
disabled boundary; this oracle does not establish default native context parity.
