//go:build (windows || linux) && amd64

package browser

import "testing"

func TestAttrNodeBindingsMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "attr_node")
}
