//go:build (windows || linux) && amd64

package browser

import "testing"

func TestNodeNameBindingsMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "node_name")
}
