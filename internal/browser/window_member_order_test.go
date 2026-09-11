//go:build windows && amd64

package browser

import "testing"

func TestWindowMemberOrderMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "window_member_order")
}
