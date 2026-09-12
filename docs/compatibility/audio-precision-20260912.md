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
