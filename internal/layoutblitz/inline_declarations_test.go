package layoutblitz

import (
	"github.com/moreveal/mimic/internal/dom"
	"math"
	"testing"
)

func TestCanonicalParsedInlinePrecisionSurvivesProjection(t *testing.T) {
	source, err := dom.Parse(`<html><body><div></div></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	id := source.FindAllByTagName("div")[0].ID
	const attr = "transform: scale(1.001);"
	set := func(value string) {
		t.Helper()
		if _, err := source.SetInlineStyle(id, attr, `[{"name":"transform","value":"scale(1.001)","parsedValue":"scale(`+value+`)","priority":""}]`); err != nil {
			t.Fatal(err)
		}
	}
	var projected Document
	defer projected.Close()
	check := func(want float64) {
		t.Helper()
		if err := projected.Sync(source, 800, 600); err != nil {
			t.Fatal(err)
		}
		if err := projected.Owner.Stylesheet(uint64(id), `div[style="transform: scale(1.001);"] {width:123px}`); err != nil {
			t.Fatal(err)
		}
		if _, err := projected.Owner.Resolve(0); err != nil {
			t.Fatal(err)
		}
		matrix, err := projected.Owner.GeometryTransform(uint64(id))
		if err != nil || len(matrix) != 19 || math.Abs(matrix[0]-want) > 1e-12 {
			t.Fatalf("matrix %v err %v want %v", matrix, err, want)
		}
		if width, err := projected.Owner.Style(uint64(id), "width"); err != nil || width != "123px" {
			t.Fatalf("canonical attribute selector lost: %q %v", width, err)
		}
	}
	set("1.000998")
	check(float64(float32(1.000998))) // initial snapshot must publish private input
	set("1.000999")
	check(float64(float32(1.000999))) // same cssText, changed parsed product
	if _, err := source.SetInlineStyle(id, attr, ""); err != nil {
		t.Fatal(err)
	}
	check(float64(float32(1.001))) // clearing parsed state reparses even same cssText
}
