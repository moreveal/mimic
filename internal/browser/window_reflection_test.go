//go:build windows && amd64

package browser

import "testing"

func TestWindowReflectionMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "window_reflection")
}
