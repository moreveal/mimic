//go:build (windows || linux) && amd64

package browser

import "testing"

func TestHeadersBindingMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "headers_binding")
}

func TestHeadersBytesMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)

	documentAllOracle(t, "headers_bytes")
}

func TestHeadersInitializerMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "headers_init")
}
