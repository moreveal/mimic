//go:build (windows || linux) && amd64

package browser

import "testing"

func TestPerformanceBindingsMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)

	documentAllOracle(t, "performance_binding")
}

func TestHTMLAllBindingsMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "html_all_binding")
}
