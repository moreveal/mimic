//go:build (windows || linux) && amd64

package browser

import "testing"

func TestLocationReflectionMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "location_reflection")
}

func TestAncestorOriginsMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "ancestor_origins")
}

func TestRealmPropertyMutationMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "realm_property_mutation")
}
