//go:build windows && amd64

package browser

import "testing"

func TestHeadersBindingMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "headers_binding")
}
