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
- PCM playback with constant rates, detune and source/context rate conversion,
  fractional starts, rounded buffer offsets, stop/duration limits, supported loop
  intervals, and source end notifications. Zero rate holds the source sample;
  negative rates support non-looping reverse playback. The final
  partial render quantum advances currentTime to a 128-frame boundary. A source
  can end within that quantum even if its end is beyond the output buffer length;
  an infinite loop does not end merely because offline rendering completed.
- Constant gain and scheduled setValueAtTime/cancelScheduledValues, with per-frame
  a-rate and 128-frame k-rate evaluation. Mono/stereo speaker conversion and
  discrete channel mapping share the same PCM samples. Stereo downmix scales
  before adding, preserving the measured finite result near Float32 limits.

Thirteen focused Chrome 152 fixtures cover buffers, empty rendering, connected
graphs, relational sample changes, lifecycle, validation, channel conversion,
interpolation, playback rates, reverse playback, phase accumulation and source
completion. Disconnected sources are not pulled by the destination and do not
produce a synthetic ended event merely because the context completes.
`go test ./internal/browser -run TestOfflineAudio` compares them on V8 and Goja
and checks explicit rejection at unsupported boundaries. Repeat inputs produce
equal output; changing one source sample changes only its dependent output
positions, including loop repetitions. No random or fingerprint-specific sample
generation is used.

## Remaining boundaries

This is not complete Web Audio support. Oscillators, filters, compressors,
convolution, analyzers, ramps/curves/targets, AudioParam connections, decoding,
scheduled playback-rate/detune changes, reverse loops, fractional loop boundaries,
feedback graphs,
destination channel reconfiguration and complex speaker conversion remain
unsupported. Node creation or rendering reports NotSupportedError instead of
returning a fabricated successful result. Suspension/resumption and real-time
AudioContext execution are not implemented. All WebIDL overload/conversion
corners and graph changes concurrent with rendering are not yet covered.

The formerly diagnostic `audio-fractional-chrome152.json` capture is now a passing
conformance test; its original source and expectations are unchanged. Native
fractional start times affect interpolation, while buffer offsets are rounded to
the nearest source sample. Interpolation at a non-looping trailing edge uses the
last pair of samples to extrapolate; loops interpolate toward the loop start.
The source-rate ratio is formed before scaling and source phase is accumulated
frame by frame, including across 128-frame quanta. Replacing that accumulation
with a direct elapsed-time formula fails exact native comparisons at 44.1/48 kHz.
The saved fixture also records that mutations through a channel view after
source.start and before startRendering are observed by native playback.

PCM allocation is bounded to 16 million samples per buffer and per evaluated
graph; source evaluation steps and mixing contributions share a 16-million work
budget. Effective resampling ratios above 1024 in magnitude, nonfinite ratios,
and unsupported loop intervals are rejected explicitly. Unsupported resource
limits remain explicit. These guards are implementation limits, not advertised
Chrome hardware limits. More exact floating-point/denormal behavior and additional
graph semantics require further native measurements.

These local API checks do not establish a live Cloudflare pass.
