package textmetrics

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFallbackClustersAndIsolation(t *testing.T) {
	if _, err := os.Stat(filepath.Join(os.Getenv("WINDIR"), "Fonts", "seguiemj.ttf")); err != nil {
		t.Skip("requires Windows Chrome reference fonts")
	}
	a, b := New(), New()
	first, err := a.Shape("A👩‍💻B", "serif", 16, 400, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Glyphs) != 3 || first.Glyphs[0].Cluster != 0 || first.Glyphs[1].Cluster != 1 || first.Glyphs[2].Cluster != 4 {
		t.Fatalf("cluster split: %v", first.Glyphs)
	}
	if len(b.faces) != 0 {
		t.Fatal("cross-engine resource sharing")
	}
	repeated, err := a.Shape("A👩‍💻B", "serif", 16, 400, false, false, false)
	if err != nil || !reflect.DeepEqual(first, repeated) {
		t.Fatal("unstable observations", err)
	}
	independent, err := b.Shape("A👩‍💻B", "serif", 16, 400, false, false, false)
	if err != nil || !reflect.DeepEqual(first, independent) {
		t.Fatal("different independent result", err)
	}
	count := len(a.faces)
	for i := 0; i < 5; i++ {
		if _, err = a.Shape("😀😀😀", "serif", 16, 400, false, false, false); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.faces) != count {
		t.Fatal("repeated fallback grew face cache")
	}
}
func TestMissingFontsRemainExplicit(t *testing.T) {
	e := NewDirectories([]string{t.TempDir()})
	_, err := e.Shape("😀", "serif", 16, 400, false, false, false)
	if err == nil || !strings.Contains(err.Error(), "no usable reference font resource") {
		t.Fatalf("missing font: %v", err)
	}
}
