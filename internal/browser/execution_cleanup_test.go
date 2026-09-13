//go:build (windows || linux) && amd64

package browser

import "testing"

// The oracle observes both values and unexpected calls into overridable APIs.
// Ordinary and restored realms must use the same canonical DOM state.
func TestExecutionCleanupMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "execution_cleanup")
}

func TestExecutionReentryMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "execution_reentry")
}

func TestExecutionFontMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "execution_font")
}

func TestExecutionSourceMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "execution_source")
}

func TestExecutionSVGLifecycleMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "execution_svg_lifecycle")
}

func TestExecutionOperationsMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "execution_operations")
}
