//go:build (windows || linux) && amd64

package browser

import "testing"

func TestNativeFunctionFinalizationMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "native_function_finalization")
}
