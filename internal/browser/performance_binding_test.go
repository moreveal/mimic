//go:build (windows || linux) && amd64

package browser

import "testing"

func TestPerformanceBindingsMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "performance_binding")
}

func TestHTMLAllBindingsMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "html_all_binding")
}
