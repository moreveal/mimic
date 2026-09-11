//go:build windows && amd64

package browser

import "testing"

func TestFormNameReflectionMatchesFrozenChrome(t *testing.T) { documentAllOracle(t, "form_name") }
