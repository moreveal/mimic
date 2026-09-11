//go:build windows && amd64

package browser

import "testing"

func TestInterfaceInheritanceMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "interface_inheritance")
}
