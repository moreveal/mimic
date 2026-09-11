//go:build windows && amd64

package browser

import "testing"

func TestDocumentBodySetterMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "document_body")
}
