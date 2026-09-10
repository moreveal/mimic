package webapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// New explicit boundaries must retain a source site, including worker paths.
func TestSemanticBoundariesHaveSourceSites(t *testing.T) {
	paths, err := filepath.Glob("*.js")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "host.semanticMissing(") {
			t.Errorf("%s: boundary missing source site", path)
		}
	}
}
