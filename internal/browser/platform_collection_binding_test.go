//go:build (windows || linux) && amd64

package browser

import "testing"

func TestCollectionPrototypeBindingsMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "collection_binding")
}
