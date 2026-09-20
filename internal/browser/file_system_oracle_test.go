//go:build (windows || linux) && amd64

package browser

import "testing"

func TestOPFSMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "opfs")
}
