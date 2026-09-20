//go:build (windows || linux) && amd64

package browser

import "testing"

func TestFontCompositionFrozenChrome152(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "font_composition")
}
