//go:build windows && amd64

package browser

import "testing"

func TestWindowNamedPropertiesMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "window_named")
}
