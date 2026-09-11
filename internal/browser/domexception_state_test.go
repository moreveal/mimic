//go:build windows && amd64

package browser

import "testing"

func TestDOMExceptionStateMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "domexception_state")
	documentAllOracle(t, "domexception_worker")
}
