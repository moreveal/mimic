//go:build (windows || linux) && amd64

package browser

import "testing"

func TestDocumentGetterOwnershipMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "document_getter_ownership")
}
