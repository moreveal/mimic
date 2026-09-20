//go:build (windows || linux) && amd64

package browser

import "testing"

func TestFormNameReflectionMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "form_name")
}
