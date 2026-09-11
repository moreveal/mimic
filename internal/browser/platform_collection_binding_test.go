//go:build windows && amd64

package browser

import "testing"

func TestCollectionPrototypeBindingsMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "collection_binding")
}
