//go:build windows && amd64

package browser

import "testing"

func TestCSSRuleSerializationMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "css_rule_serialization")
}
