//go:build windows && amd64

package browser

import "testing"

func TestDocumentDescriptorsMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "document_descriptor")
}
