//go:build (windows || linux) && amd64

package browser

import "testing"

func TestAttrNodeBindingsMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "attr_node")
}
