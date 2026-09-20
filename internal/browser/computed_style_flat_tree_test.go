//go:build (windows || linux) && amd64

package browser

import "testing"

func TestComputedStyleFlatTreeMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "computed_style_flat_tree")
}
