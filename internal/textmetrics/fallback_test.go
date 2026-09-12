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

func TestMissingGlyphDoesNotRetainFallbackCandidates(t *testing.T) {
	if _, err := os.Stat(filepath.Join(os.Getenv("WINDIR"), "Fonts", "arial.ttf")); err != nil {
		t.Skip("requires reference fonts")
	}
	e := New()
	if _, err := e.Shape("A", "Arial", 16, 400, false, false, false); err != nil {
		t.Fatal(err)
	}
	faces, bytes := len(e.faces), e.bytes
	for _, text := range []string{"\u0098", "\u0378"} {
		value, err := e.Shape(text, "Arial", 16, 400, false, false, false)
		if err != nil || len(value.Glyphs) != 1 || value.Glyphs[0].Advance <= 0 {
			t.Fatalf("missing glyph: %v %v", value, err)
		}
		if len(e.faces) != faces || e.bytes != bytes {
			t.Fatalf("unselected candidate retained: faces=%d bytes=%d", len(e.faces), e.bytes)
		}
	}
	if _, err := e.Shape("Later ordinary text", "serif", 16, 400, true, false, false); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalCompositionPreservesPrimaryFace(t *testing.T) {
	if _, err := os.Stat(filepath.Join(os.Getenv("WINDIR"), "Fonts", "seguiemj.ttf")); err != nil {
		t.Skip("requires reference font")
	}
	e := New()
	e.SetFallbackFamilies([]string{"Tahoma", "Segoe UI"})
	for _, pair := range [][2]string{{"e\u0301", "\u00e9"}, {"a\u0308", "\u00e4"}, {"n\u0303", "\u00f1"}} {
		a, err := e.Shape(pair[0], "Segoe UI Emoji", 16, 400, false, false, false)
		if err != nil {
			t.Fatal(err)
		}
		b, err := e.Shape(pair[1], "Segoe UI Emoji", 16, 400, false, false, false)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("canonical composition changed face/metrics: %q: %v != %v", pair, a, b)
		}
	}
}
