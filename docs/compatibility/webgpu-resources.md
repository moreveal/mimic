# WebGPU resources without a native backend

The navigator getter has a frozen Chrome 152 reflection regression in
`webgpu-navigator-chrome152.json` and its Worker counterpart. Installing the
resource implementation after the original surface's native-function marking
had exposed its JavaScript source and an incorrect getter name. The getter now
uses the shared realm-local native-function registry and validates its owning
navigator, including rejection of objects inheriting from that navigator.
The GPU string tag was already `[object GPU]` before the resource implementation.
These checks establish API agreement; they do not establish an external detector
or challenge result.

The shared Window/Worker implementation owns adapters, device views, queues,
buffers, mapped ranges, command encoders and command buffers. Resource identity
and storage are realm-local; the immutable environment supplies adapter features,
limits and discovery delay. Worker adapter discovery uses its Page scheduler.
Device features include only requested supported features. Device information,
features and limits are independent objects, as measured in Chrome 152.

Buffers support mapped-at-creation storage, writeBuffer, copyBufferToBuffer,
clearBuffer, submission and asynchronous read/write mapping. A submitted command
uses the current resource bytes; commands cannot be submitted twice. Mapping
checks alignment, usage, bounds and ownership. Mapped ranges cannot overlap;
unmap/destroy cancel pending mapping and detach every returned ArrayBuffer.

V8 uses the captured ArrayBuffer transfer intrinsic. Goja supplies a small
engine-neutral ArrayBufferDetacher capability backed by its real buffer detach
operation, called only on the owning realm actor. This does not substitute a
fake byteLength getter. Engines without either primitive reject mapped-at-creation
storage explicitly. Storage is bounded to 64 MiB per buffer.

Validation errors are captured by the matching device scope. An empty scope pop
rejects with OperationError. Uncaptured errors are scheduled as events; destroy
settles device.lost and invalidates mapped state. Async mapping uses the realm's
task scheduler. Native GPU completion timing and all validation ordering across
complex failing submissions have not been established.

The WebIDL generator previously omitted namespace declarations, leaving observed
GPUBufferUsage/GPUMapMode globals as empty exposure placeholders. It now projects
namespace constants and descriptors from retained IDL, with no second flag table.
Generated interface prototypes also carry their own WebIDL toStringTag, including
Worker interfaces without a complete prototype capture. The frozen IDL and
exposure recordings were not changed; deterministic projections and their hash
manifest were regenerated offline.

Texture, sampler, shader-module, render/compute pipeline and pass execution are
still explicit NotSupportedError boundaries. This resource layer does not claim
WGSL execution or successful live challenge compatibility. Several less-used
adapter options, extended limit negotiation, WGSL feature queries and public GPU
error constructors remain incomplete. Invalid resource objects preserve their
queryable descriptor state; they are not valid submission resources.

Evidence is in webgpu-resources, webgpu-flags and webgpu-lifecycle Chrome 152
captures under compatibility/captures/semantic-checkpoints. TestWebGPUResourceOracle
runs their exact sources in Window and Worker using V8 and Goja. The capture tool
uses a disposable context and intercepts a synthetic localhost document for a
secure context; it does not load a remote website. Its fixture hashes use the
LF-normalized UTF-8 source actually sent to Runtime.evaluate.

The older manually written GPUDevice slot test expected adapter/device identity
sharing and a successful empty popErrorScope. Those assertions were corrected
using the new native oracle; readonly descriptors and canonical per-object
identities remain tested.
