# Offline PCM audio observations

AudioBuffer and the supported OfflineAudioContext graph are implemented in the
realm. They use Float32Array PCM storage and the Page's task queue, with no audio
device, native audio backend, GPU, or global graph lock. Independent contexts
retain their own nodes, connections, scheduling state, and output buffers.

Implemented observations:

- AudioBuffer dimensions, shared channel view identity, bounded copy operations,
  untouched destination tails, and measured numeric/error boundaries.
- OfflineAudioContext construction, canonical destination, synchronous running
  state, asynchronous statechange/completion, a canonical completion buffer,
  trusted completion events, closed state, and rejection of a second render.
- AudioBufferSourceNode and GainNode construction, branded node/parameter state,
  single-input/output connections, duplicate connection suppression, and basic
  disconnection. Sources and gains can be mixed through an acyclic graph.
- Equal-rate PCM playback at integer sample boundaries, buffer offsets, stop and
  duration limits, valid loop intervals, and source end notifications. The final
  partial render quantum advances currentTime to a 128-frame boundary. A source
  can end within that quantum even if its end is beyond the output buffer length;
  an infinite loop does not end merely because offline rendering completed.
- Constant gain and scheduled setValueAtTime/cancelScheduledValues, with per-frame
  a-rate and 128-frame k-rate evaluation. Mono/stereo speaker conversion and
  discrete channel mapping share the same PCM samples. Stereo downmix scales
  before adding, preserving the measured finite result near Float32 limits.

Seven focused Chrome 152 fixtures cover buffers, empty rendering, connected
graphs, relational sample changes, lifecycle, validation, and channel conversion.
`go test ./internal/browser -run TestOfflineAudio` compares them on V8 and Goja
and checks explicit rejection at unsupported boundaries. Repeat inputs produce
equal output; changing one source sample changes only its dependent output
positions, including loop repetitions. No random or fingerprint-specific sample
generation is used.

## Remaining boundaries

This is not complete Web Audio support. Oscillators, filters, compressors,
convolution, analyzers, ramps/curves/targets, AudioParam connections, decoding,
sample-rate/playback-rate conversion, fractional source timing, feedback graphs,
destination channel reconfiguration and complex speaker conversion remain
unsupported. Node creation or rendering reports NotSupportedError instead of
returning a fabricated successful result. Suspension/resumption and real-time
AudioContext execution are not implemented. All WebIDL overload/conversion
corners and graph changes concurrent with rendering are not yet covered.

The separate `audio-fractional-chrome152.json` fixture is diagnostic evidence for
future interpolation work, not a passing conformance test. Native fractional
start times affect interpolation; rounding them to integer frames would be
incorrect. The saved fixture also records that mutations through a channel view
after source.start and before startRendering are observed by native playback.

PCM allocation is bounded to 16 million samples per buffer and per evaluated
graph; mixing is bounded to 16 million sample contributions. Unsupported resource
limits remain explicit. These guards are implementation limits, not advertised
Chrome hardware limits. More exact floating-point/denormal behavior and additional
graph semantics require further native measurements.

These local API checks do not establish a live Cloudflare pass.
