package textmetrics

import (
	fontcontainer "github.com/tdewolff/font"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestWebFontContainersAndIsolation(t *testing.T) {
	source, _ := runtime.FuncForPC(reflect.ValueOf(fontcontainer.ToSFNT).Pointer()).FileLine(0)
	root := filepath.Join(filepath.Dir(source), "resources")
	var reference Result
	for _, extension := range []string{"ttf", "woff", "woff2"} {
		t.Run(extension, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, "DejaVuSerif."+extension))
			if err != nil {
				t.Fatal(err)
			}
			e := NewDirectories(nil)
			id, err := e.RegisterFont(data)
			if err != nil {
				t.Fatal(err)
			}
			same, err := e.RegisterFont(data)
			if err != nil || same != id || len(e.resources) != 1 {
				t.Fatal("resource identity changed")
			}
			choices := []FontReference{{ID: id, Family: "Alias", Weight: 400, Style: "normal"}}
			got, err := e.ShapeWithFonts("ABCD", "Alias", 20, 400, false, false, false, choices)
			if err != nil {
				t.Fatal(err)
			}
			if extension == "ttf" {
				reference = got
			} else if !reflect.DeepEqual(got, reference) {
				t.Fatalf("container changed metrics: %v vs %v", got, reference)
			}
			if _, err = e.Shape("ABCD", "Alias", 20, 400, false, false, false); err == nil {
				t.Fatal("unregistered alias became globally visible")
			}
			other := NewDirectories(nil)
			if _, err = other.ShapeWithFonts("ABCD", "Alias", 20, 400, false, false, false, choices); err == nil {
				t.Fatal("resource leaked to independent engine")
			}
		})
	}
}
func TestWebFontInvalidResources(t *testing.T) {
	e := NewDirectories(nil)
	for _, data := range [][]byte{nil, []byte("not an actual font container"), append([]byte("wOF2"), make([]byte, 50)...)} {
		if _, err := e.RegisterFont(data); err == nil {
			t.Fatal("accepted invalid font")
		}
	}
	if len(e.resources) != 0 || len(e.faces) != 0 || e.bytes != 0 {
		t.Fatal("failed decode retained state")
	}
}
