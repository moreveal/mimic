//go:build windows && amd64

package browser

import "testing"

func TestCSSTransformPrecisionMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "css_transform_precision")
}
