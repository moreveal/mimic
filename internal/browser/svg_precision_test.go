//go:build (windows || linux) && amd64

package browser

import "testing"

func TestSVGPrecisionMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "svg_precision")
}
