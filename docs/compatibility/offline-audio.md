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
- OscillatorNode and PeriodicWave construction, parameter bounds, sine/square/
  sawtooth/triangle Fourier synthesis, copied custom coefficients, normalization,
  DC removal, band limiting, negative/zero/Nyquist frequencies, detune, scheduling,
  and ended notifications. Wavetable banks and phase belong to the context/node.
- DynamicsCompressorNode with bounded k-rate parameters, mono/stereo linked peak
  detection, nonlinear knee/ratio, 6 ms lookahead, makeup gain, 32-frame envelope
  divisions, adaptive attack/release, and reduction metering. Its output derives
  from delayed input samples and local envelope state.
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

This is not complete Web Audio support. Filters, convolution, analyzers, setTargetAtTime, AudioParam connections, decoding,
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

## Portable oscillator and compressor DSP

Six additional frozen Chrome 152 captures cover oscillator parameter shape,
error boundaries, all four built-in waveforms, custom periodic waves, scheduling,
automation, 8/44.1/48/96 kHz sample rates, and oscillator/compressor composition.
They compare every captured PCM sample on V8 and Goja. Existing buffer-source,
gain and automation captures still compare exactly and are unchanged.

The portable radix-2 transform computes band-limited waveform tables from Fourier
coefficients. It follows Chrome 152's three pitch ranges per octave, table sizes,
normalization and interpolation, but is not its native FFT implementation. New
synthesis tests allow an explicit absolute numeric error of 1e-5; API states,
errors, dimensions and types are checked exactly. The final 5000-frame triangle/compressor graph has measured maximum error
1.20e-7; other synthesis graph cases remain below 2.39e-7. The separately measured
4096-frame compressor cases match within 8.95e-8 (three cases match exactly).
These results establish a measured approximate PCM model, not arbitrary or
precomputed samples and not bit-identical native FFT/libm output.

The DSP algorithm references are the Chrome 152.0.7977.82 sources
[periodic_wave.cc](https://github.com/chromium/chromium/blob/152.0.7977.82/third_party/blink/renderer/modules/webaudio/periodic_wave.cc),
[oscillator_handler.cc](https://github.com/chromium/chromium/blob/152.0.7977.82/third_party/blink/renderer/modules/webaudio/oscillator_handler.cc), and
[dynamics_compressor.cc](https://github.com/chromium/chromium/blob/152.0.7977.82/third_party/blink/renderer/platform/audio/dynamics_compressor.cc).
The applicable source notice is retained in the implementation.

Fractional oscillator starts expose a distinction between initial constructor
values and AudioParam changes: a parameter setter is an initial automation event.
When the oscillator starts in that first quantum its phase begins at zero;
a constant parameter carries the sub-frame remainder. The dedicated schedule
capture records both the first quantum and starts beyond it. Oscillators and
compressors still participate in the same canonical graph and asynchronous
completion lifecycle as buffer sources; they do not require an audio device.

The synthesis relation capture additionally checks repeated runs, exact gain
scaling, stereo channel agreement, disconnection, stop-before-start events,
partial-quantum compressor state and PeriodicWave reuse across contexts. Native
waves retain their originating sample rate when reused by another context.
Internal graph PCM covers the full final render quantum before clipping to the
requested AudioBuffer length, so compressor state and readback share the same
source samples. Lazy wavetable banks have a per-context 16-million-sample limit.
Constant-rate oscillator phase uses four Float32 lanes with a double-precision
origin restored at each quantum, preserving Chrome's phase rounding near steep
waveform edges. Native FFT implementation differences still remain bounded by
the explicit synthesis tolerance.

The extreme-parameter capture also checks finite detune limits: native DSP flushes
subnormal detune multipliers to zero, and a NaN effective frequency (zero times
overflowed detune) clamps to Nyquist. These cases produce measured silence.

## Chromium oscillator source notice

The oscillator phase and interpolation algorithms reference code copyright
2020, 2022 The Chromium Authors, under the following license:

```text
// Copyright 2015 The Chromium Authors
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google LLC nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```
