//go:build (windows || linux) && amd64

package browser

import "testing"

func TestRTCSessionMatchesFrozenChrome152(t *testing.T) {
	// The frozen oracle samples ICE events after a fixed 250 ms. Keep this
	// timing comparison out of the parallel isolate pool so unrelated tests
	// cannot delay its chained timers past the sample.
	serialBrowserTest(t)
	documentAllOracle(t, "rtc_session")
}
