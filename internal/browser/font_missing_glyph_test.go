//go:build (windows || linux) && amd64

package browser

import "testing"

func TestMissingGlyphsFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "font_missing_glyph")
}
