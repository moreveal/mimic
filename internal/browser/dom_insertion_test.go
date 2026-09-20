//go:build (windows || linux) && amd64

package browser

import "testing"

func TestDOMInsertionMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "dom_insertion")
}
