//go:build (windows || linux) && amd64

package browser

import "testing"

func TestSingletonAccessorOwnershipMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "singleton_accessor")
}
