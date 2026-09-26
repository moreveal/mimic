//go:build (windows || linux) && amd64

package browser

import "testing"

func TestPlatformMemberOrderMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "platform_member_order")
}
