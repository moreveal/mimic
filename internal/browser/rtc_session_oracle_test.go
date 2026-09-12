//go:build windows && amd64

package browser

import "testing"

func TestRTCSessionMatchesFrozenChrome152(t *testing.T) { documentAllOracle(t, "rtc_session") }
