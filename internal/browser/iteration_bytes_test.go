//go:build windows && amd64

package browser

import "testing"

func TestLivePairIteratorsMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "iteration_pairs")
}
