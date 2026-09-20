//go:build (windows || linux) && amd64

package browser

import "testing"

func TestDOMExceptionStateMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "domexception_state")
	documentAllOracle(t, "domexception_worker")
}
