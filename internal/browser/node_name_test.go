//go:build (windows || linux) && amd64

package browser

import "testing"

func TestNodeNameBindingsMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "node_name")
}
