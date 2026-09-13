//go:build (windows || linux) && amd64

package browser

import "testing"

func TestWindowMemberOrderMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "window_member_order")
}
