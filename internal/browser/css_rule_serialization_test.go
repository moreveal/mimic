//go:build (windows || linux) && amd64

package browser

import "testing"

func TestCSSRuleSerializationMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "css_rule_serialization")
}
