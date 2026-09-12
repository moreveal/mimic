//go:build windows && amd64

package browser

import "testing"

func TestComputedStyleLifecycleMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "computed_style_lifecycle")
}
