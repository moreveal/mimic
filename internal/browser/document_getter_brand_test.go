//go:build (windows || linux) && amd64

package browser

import "testing"

func TestDocumentGetterBrandsMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "document_getter_brand")
}
