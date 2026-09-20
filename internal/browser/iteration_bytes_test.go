//go:build (windows || linux) && amd64

package browser

import "testing"

func TestLivePairIteratorsMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)

	documentAllOracle(t, "iteration_pairs")
}

func TestFormUTF8MatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "iteration_form")
}

func TestTextEncoderBindingsMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)

	documentAllOracle(t, "iteration_encoder")
}

func TestIterationAndEncodingInWorkersMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "iteration_worker")
}
