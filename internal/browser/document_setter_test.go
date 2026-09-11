//go:build windows && amd64

package browser

import "testing"

func TestDocumentSettersMatchFrozenChrome(t *testing.T) {
	documentAllOracle(t, "document_setter")
}
