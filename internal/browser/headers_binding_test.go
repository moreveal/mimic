//go:build windows && amd64

package browser

import "testing"

func TestHeadersBindingMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "headers_binding")
}

func TestHeadersBytesMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "headers_bytes")
}

func TestHeadersInitializerMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "headers_init")
}
