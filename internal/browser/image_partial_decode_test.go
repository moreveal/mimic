//go:build (windows || linux) && amd64

package browser

import "testing"

func TestImagePartialDecodeMatchesFrozenChrome152(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "image_partial_decode")
}
