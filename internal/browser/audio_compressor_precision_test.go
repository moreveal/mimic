//go:build windows && amd64

package browser

import "testing"

// A buffer source isolates compressor arithmetic from the portable oscillator
// transform. Every PCM sample and observable state is compared exactly.
func TestAudioCompressorPrecisionMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "audio_compressor_precision")
}
