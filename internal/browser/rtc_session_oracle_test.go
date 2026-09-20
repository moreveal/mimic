//go:build (windows || linux) && amd64

package browser

import "testing"

func TestRTCSessionMatchesFrozenChrome152(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "rtc_session")
}
