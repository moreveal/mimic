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
- PCM playback with constant or scheduled rates, detune and source/context rate conversion,
  fractional starts, rounded buffer offsets, stop/duration limits, supported loop
  intervals, and source end notifications. Zero rate holds the source sample;
  negative rates support non-looping reverse playback. The final
  partial render quantum advances currentTime to a 128-frame boundary. A source
  can end within that quantum even if its end is beyond the output buffer length;
  an infinite loop does not end merely because offline rendering completed.
- Constant gain, scheduled values, linear/exponential ramps, copied value curves,
  cancellation and hold, with per-frame a-rate and 128-frame k-rate evaluation.
  Mono/stereo speaker conversion and
  discrete channel mapping share the same PCM samples. Stereo downmix scales
  before adding, preserving the measured finite result near Float32 limits.

Nineteen focused Chrome 152 fixtures cover buffers, empty rendering, connected
graphs, relational sample changes, lifecycle, validation, channel conversion,
interpolation, playback rates, reverse playback, phase accumulation and source
completion, automation validation/cancellation, scheduled rates and durations.
Disconnected sources are not pulled by the destination and do not
produce a synthetic ended event merely because the context completes.
`go test ./internal/browser -run TestOfflineAudio` compares them on V8 and Goja
and checks explicit rejection at unsupported boundaries. Repeat inputs produce
equal output; changing one source sample changes only its dependent output
positions, including loop repetitions. No random or fingerprint-specific sample
generation is used.

## Remaining boundaries

This is not complete Web Audio support. Oscillators, filters, compressors,
convolution, analyzers, setTargetAtTime, AudioParam connections, decoding,
reverse loops, fractional loop boundaries,
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

Automation events retain stable ordering for different event kinds at equal
times; a new event replaces the same kind at the same time. Curves copy their
input values and reject overlapping events. Native cancellation inside a curve
removes the complete curve, whereas cancelAndHold preserves its preceding
observations and freezes the value at the requested time. Curve interpolation
uses sample coordinates to preserve exact zeros at measured sample boundaries.
The active segment is found with binary search rather than rescanning the entire
event history for every output sample.

Source playbackRate and detune automation are sampled at 128-frame boundaries.
Playback phase and consumed source duration advance with the effective rate;
changing the rate to zero holds the source without exhausting a finite grain.
Faster and slower scheduled playback therefore change the measured end time.

The broader `audio-automation-chrome152.json` capture retains the native
setTargetAtTime output as diagnostic evidence. Its rounding differs from a naive
scalar exponential implementation. That operation remains explicitly unsupported
and is not included among the passing conformance fixtures; its expectations
have not been replaced by approximate results.

PCM allocation is bounded to 16 million samples per buffer and per evaluated
graph; source evaluation steps and mixing contributions share a 16-million work
budget. Effective resampling ratios above 1024 in magnitude, nonfinite ratios,
and unsupported loop intervals are rejected explicitly. Unsupported resource
limits remain explicit. These guards are implementation limits, not advertised
Chrome hardware limits. More exact floating-point/denormal behavior and additional
graph semantics require further native measurements.

These local API checks do not establish a live Cloudflare pass.
