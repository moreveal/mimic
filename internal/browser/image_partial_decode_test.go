//go:build windows && amd64

package browser

import "testing"

func TestImagePartialDecodeMatchesFrozenChrome152(t *testing.T) {
	documentAllOracle(t, "image_partial_decode")
}
