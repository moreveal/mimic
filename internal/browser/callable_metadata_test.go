//go:build (windows || linux) && amd64

package browser

import "testing"

func TestCallableMetadataMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "callable_metadata")
}
