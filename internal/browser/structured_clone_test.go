//go:build windows && amd64

package browser

import "testing"

func TestStructuredCloneMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "structured_clone")
}
