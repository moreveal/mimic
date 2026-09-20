//go:build (windows || linux) && amd64

package browser

import "testing"

func TestWindowMemberOrderMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "window_member_order")
}
