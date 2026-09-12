# Offline audio arithmetic follow-up

Frozen headful Chrome 152.0.7977.82, two fresh-context controls, no feature
overrides. The clean local probe uses a 1000-frame mono 44100 Hz graph. A
constant 0.5 buffer isolates the compressor from oscillator synthesis.
Threshold -52, knee 40, ratio 12, attack 0.0001, release 0.25 reproduce the
observed operation contract without any captured output in production code.

The compressor's decibel conversion divided by 20. Chromium's
`audio_utilities::DecibelsToLinear` instead multiplies by the Float32 constant
0.05 before `powf`. These expressions round differently. Preserving that
operation order closes all 736 differing samples of the isolated compressed
buffer: the complete PCM, reduction, context time and final state now compare
exactly. The uncompressed buffer also compares exactly. The frozen receipt
records binary/probe hashes and observed metadata; A/B control has zero
differences. Ordinary and restored-bootstrap regression paths pass.

Independent diagnostic graphs use sine and triangle oscillators at frequency
9998.123456 (stored as Float32). Before compression their portable transform
differs from Chrome in 665/635 samples respectively, maximum absolute error
1.7881393432617188e-7. After this correction triangle-plus-compressor differs in
423 samples with maximum error 5.960464477539063e-8; its reduction now matches
exactly. These remaining differences are deterministic numerical residuals,
not timing or random variation, and are not claimed resolved by this patch.
The existing portable FFT synthesis contract and tolerances remain unchanged.

Backend localization: Chromium's pinned `WebAudioRustFft` feature is stable.
The frozen binary's default output equals an explicitly enabled RustFFT run
for every sample in all six graphs. A diagnostic run disabling that feature
changes 642 uncompressed triangle and 640 sine samples, leaving both constant
buffer graphs unchanged. Those feature-overridden runs are diagnostics only,
not reference oracles. The active backend uses RustFFT 6.4.1 with Float32
half-size complex transforms and reconstruction (`rustfft_ffi.rs`); Mimic uses
a full-size Float64 radix-2 transform. Exact synthesis closure requires matching
that arithmetic rather than adjusting PCM hashes or treating it as randomness.
The two diagnostic runners and passports are retained in `audio-rust` and
`audio-pffft` beside the clean controls.

Validation: `go test ./internal/browser -run
'TestAudioCompressorPrecision|TestOfflineAudio' -count=1` passes. No whole-suite,
race or performance gate was run for this arithmetic correction.

Raw controls, before/after diagnostic PCM, passport and runner are retained in
the main checkout under `.build/residual-media-delegated/audio-before`,
`audio-after`, and `audio-precision`.

Source: pinned Chromium
[`audio_utilities.cc`](https://raw.githubusercontent.com/chromium/chromium/152.0.7977.82/third_party/blink/renderer/platform/audio/audio_utilities.cc).

## Portable FFT closure

The subsequent implementation replaces the Float64 radix-2 transform with a
portable Float32 mixed-radix model of the demonstrated RustFFT path. It models
real-spectrum reconstruction, radix-4/radix-8/radix-32 butterflies, the 256-point
base and the larger power-of-two plans, preserving twiddle multiplication order
and fused arithmetic. Normalization-disabled custom waves retain Chrome's 0.5
default scaling. There is no Rust, native FFT, device or OS backend dependency.
All per-transform arrays remain owned by the local evaluation.

Fresh default-feature Chrome A/B and Mimic now have **zero differing leaves**
in the expanded 21-graph oracle, including every one of its 9840 PCM samples:
the original six isolation graphs and all four built-in waveforms plus custom
unnormalized waves at 22050, 44100 and 96000 Hz. This covers every supported
table size. Reduction and context state/time are also exact. Ordinary and
restored-bootstrap tests compare all fields exactly, with no tolerance or
hash substitution. `TestOfflineAudio` remains unchanged and passes on its
existing backend matrix. Its broader automation/scheduling error budgets are
not a claim of globally bit-identical WebAudio.

The no-feature-override oracle and passport are in `audio-fft` beside the
earlier diagnostic captures. The earlier numerical residual figures above
describe the state before this FFT correction. The original graph's remaining
oscillator numerical residual is now resolved.

Algorithm references: Chromium
[`rustfft_ffi.rs`](https://raw.githubusercontent.com/chromium/chromium/152.0.7977.82/third_party/blink/renderer/platform/audio/rustfft_ffi.rs)
and [RustFFT 6.4.1](https://crates.io/crates/rustfft/6.4.1). The RustFFT MIT notice
is retained under `internal/webapi/vendor/rustfft/NOTICE`.
