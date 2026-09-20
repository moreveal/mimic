//go:build (windows || linux) && amd64

package browser

import "testing"

func TestDocumentSettersMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "document_setter")
}
