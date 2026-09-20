//go:build (windows || linux) && amd64

package browser

import "testing"

func TestStructuredCloneMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "structured_clone")
}
