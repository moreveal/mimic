//go:build windows && amd64

package browser

import "testing"

func TestComputedStyleFlatTreeMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "computed_style_flat_tree")
}
