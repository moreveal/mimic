//go:build (windows || linux) && amd64

package browser

import "testing"

func TestComputedStyleLifecycleMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "computed_style_lifecycle")
}
