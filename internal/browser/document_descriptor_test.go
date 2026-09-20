//go:build (windows || linux) && amd64

package browser

import "testing"

func TestDocumentDescriptorsMatchFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	documentAllOracle(t, "document_descriptor")
}
