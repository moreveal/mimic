package textmetrics

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
)

func TestLinuxUserFontDiscovery(t *testing.T) {
	home := t.TempDir()
	data := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", data)
	t.Setenv("MIMIC_FONT_DIR", "")
	directory := filepath.Join(data, "fonts", "truetype", "nested-family")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "Go-Regular.ttf"), goregular.TTF, 0600); err != nil {
		t.Fatal(err)
	}
	engine := New()
	result, err := engine.Shape("Mimic", "Go", 16, 400, false, false, false)
	if err != nil || len(result.Glyphs) != 5 {
		t.Fatalf("user font was not shaped: %+v %v", result, err)
	}
	// An explicit resource directory takes precedence over system/user discovery.
	t.Setenv("MIMIC_FONT_DIR", t.TempDir())
	if _, err = New().Shape("Mimic", "Go", 16, 400, false, false, false); err == nil {
		t.Fatal("override still discovered the user font")
	}
}

func TestLinuxGenericFallbackUsesRealFontMetrics(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "Go-Regular.ttf"), goregular.TTF, 0600); err != nil {
		t.Fatal(err)
	}
	engine := NewDirectories([]string{directory})
	engine.SetGenericFamily("serif", "Unavailable Reference")
	engine.genericFallbacks = map[string][]string{"serif": {"go"}}
	expected, err := engine.Shape("Mimic", "Go", 16, 400, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := engine.Shape("Mimic", "serif", 16, 400, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatal("generic fallback did not use the resource's actual glyph metrics")
	}
}
