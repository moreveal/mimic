//go:build windows && amd64

package browser

import "testing"

func TestNativeFunctionSourcesMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "native_function")
}
