//go:build (windows || linux) && amd64

package browser

import "testing"

func TestNativeFunctionSourcesMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "native_function")
}
