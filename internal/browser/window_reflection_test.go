//go:build (windows || linux) && amd64

package browser

import "testing"

func TestWindowReflectionMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "window_reflection")
}
