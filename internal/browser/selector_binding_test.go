//go:build windows && amd64

package browser

import "testing"

func TestSelectorBindingsMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "selector_binding")
}
