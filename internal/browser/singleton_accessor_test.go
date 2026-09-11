//go:build windows && amd64

package browser

import "testing"

func TestSingletonAccessorOwnershipMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "singleton_accessor")
}
