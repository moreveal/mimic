//go:build windows && amd64

package browser

import "testing"

func TestMissingGlyphsFrozenChrome(t *testing.T) { documentAllOracle(t, "font_missing_glyph") }
