//go:build (windows || linux) && amd64

package browser

import "testing"

func TestLocationReflectionMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "location_reflection")
}

func TestAncestorOriginsMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)

	documentAllOracle(t, "ancestor_origins")
}

func TestRealmPropertyMutationMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "realm_property_mutation")
}
