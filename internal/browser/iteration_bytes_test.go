//go:build windows && amd64

package browser

import "testing"

func TestLivePairIteratorsMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "iteration_pairs")
}

func TestFormUTF8MatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "iteration_form")
}

func TestTextEncoderBindingsMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "iteration_encoder")
}
