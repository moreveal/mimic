//go:build (windows || linux) && amd64

package browser

import "testing"

func TestSelectorBindingsMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "selector_binding")
}
