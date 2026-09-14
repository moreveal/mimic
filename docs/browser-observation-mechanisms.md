# Browser observation mechanisms

Worktree: `mimic-browser-mechanisms`; branch:
`codex/browser-observation-mechanisms`. The behavioral reference is headful
Chrome 152.0.7977.82 on Windows. Every differential fixture records its source
hash, Chrome/V8 revision, origin, presentation mode, viewport and isolation
state. Site traces are discovery inputs and are not regression expectations.

## Implemented contract

### Web Audio

Offline and realtime contexts share one Page-owned, 128-frame-quantum graph and
parameter timeline. The model never opens an audio device and has no native
playback pipeline.

- AudioNode connections preserve input/output ports, reject cross-context and
  invalid connections, deduplicate identical edges, support AudioParam inputs,
  fan-in, fan-out, splitters, mergers and delayed feedback cycles.
- Buffer, oscillator and constant sources retain quantum state and implement
  scheduled start, stop and trusted ended events. Gain, delay, stereo panning,
  equal-power spatial panning, listener parameters, biquad and IIR filters,
  dynamics compression, convolution, waveshaping and analyser observations feed
  the same coherent PCM state.
- AudioParam implements value assignment and the automation timeline, including
  ramps, target curves, value curves, cancellation and stable same-time ordering.
- Offline rendering supports quantum-accurate suspension and continuation,
  state transitions, completion events and repeated-call failures. Realtime
  contexts implement resume, suspend and close, pending promise settlement,
  currentTime, baseLatency, outputLatency and getOutputTimestamp from the virtual
  Page clock.
- RIFF/WAVE PCM and float decoding detaches the input ArrayBuffer, runs
  asynchronously, preserves callback/promise ordering and resamples into the
  context rate. Invalid input rejects with EncodingError.
- AudioWorklet exposes a canonical per-context scope, asynchronous module
  registration, duplicate and missing processor errors, AudioParamMap identity,
  port identity and AudioWorkletNode topology. Processor output remains
  deterministic silence because author worklet code is not run on an audio
  rendering thread.
- Media element/stream sources, media-stream destinations and ScriptProcessor
  nodes expose their Chrome topology and object identity without connecting to
  media devices. Audio destination streams share MediaStreamTrack state with the
  RTC implementation.

### WebGPU

Adapter requests return fresh wrappers over a coherent selected profile. Default,
low-power, high-performance and fallback policies keep adapter info, features and
limits together, and a missing forced fallback resolves to null. Selection does
not mutate Page environment state. Adapter/device/info/limits/features identity,
receiver checks, device loss, second-device failure and Window/iframe/Worker
projection are covered by Chrome differential tests. Rendering remains the
existing deterministic observable GPU model without a native GPU backend.

### Fonts

Decoded faces use bounded LRU storage. Registered binary fonts are copied to
Page/Worker-owned reloadable backing files, so they can be evicted under pressure
without disappearing from a FontFaceSet or changing later metrics. Registration,
deletion, re-addition and cache invalidation retain the existing document-owned
lifecycle. Page and Worker teardown removes backing storage. Artificial limits on
registered byte size, face count, decoded registered bytes and catalog entries
were removed; parser bounds that protect malformed table traversal remain.

### Focus

The Page owns focused-frame identity and external focus state. document.hasFocus,
activeElement, focus/blur/focusin/focusout, same-origin nested frame ownership and
cross-frame event ordering match the retained Chrome probes. Ordinary script
calls to top-level Window.focus/blur are receiver-checked no-ops, as observed in
the reference browser. CDP focus emulation projects into every document in the
active frame ancestry without duplicating document state.

### CSS, Canvas and Navigator

CSS.supports uses the same value parsers as CSSOM for implemented property
families, selector() delegates to the selector grammar, nested conditions enforce
operator grammar, and custom properties validate top-level priority, semicolons
and balanced blocks. Common sizing, positioning, overflow, visibility,
white-space, object-fit, cursor and box model properties now have typed paths.
System colors and fonts come from the environment profile, and computed color
resolution follows the owning color-scheme. Canvas uses that color projection and
adds coherent separable/nonseparable blend observations; rasterization remains
approximate by design. Navigator Window/Worker values share one projection with
canonical UAData, frozen brand data and Worker storage identity.

## Deliberate boundaries

- Compressed audio codecs (MP3, AAC, Opus and platform containers) are not
  decoded. Implementing them correctly requires a portable codec library and is
  separate from audio playback; returning invented PCM would violate the
  observable contract. RIFF/WAVE is implemented fully enough for deterministic
  graph and decoding probes.
- AudioWorklet processor JavaScript is registered but not executed, and
  ScriptProcessor callbacks are not driven by a native realtime thread. A real
  worklet agent and deadline scheduler would be a new execution subsystem.
  Surface, identity, registration, parameters, messaging and failure semantics
  are modeled; output is explicit deterministic silence.
- Convolver normalization, HRTF spatialization and waveshaper oversampling use
  bounded deterministic approximations. Exact Chrome kernels depend on Blink's
  DSP tables and host sample-rate implementation. Local changes, graph flow and
  readbacks remain mutually consistent.
- Media element and incoming MediaStream audio sources expose silence until a
  media decoder supplies PCM. No microphone, speaker, audio device or native
  media pipeline is opened.
- CSS properties without a grammar used elsewhere in Mimic retain the explicit
  unsupported/fallback boundary. Building every CSS grammar would amount to a
  style engine; this batch covers shared validation paths and the properties
  exercised by the analyzed browser gate.
- Canvas curves, text and blend edges retain the repository's approximate CPU
  rasterization boundary. Pixel-identical Skia output and a GPU renderer are
  outside the requested architecture.
- WebGPU command execution remains the deterministic observation model. Native
  shader execution, driver validation and GPU timing would require a real GPU
  backend and would compromise deterministic Page isolation.

## Validation

The retained differential suite runs every fixture on Goja and V8. Targeted
audio, focus, fonts, CSS, Canvas, WebGPU, RTC, state, textmetrics and webapi tests
are run in addition to the full `internal/browser` package. The final command
results are recorded in the completing change set.
