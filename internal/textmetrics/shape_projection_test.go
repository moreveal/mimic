package textmetrics

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHorizontalRTLAdvancesMatchChrome152(t *testing.T) {
	if _, err := os.Stat(filepath.Join(os.Getenv("WINDIR"), "Fonts", "arial.ttf")); err != nil {
		t.Skip("requires Windows reference fonts")
	}
	e := New()
	// Canvas measureText, Arial 12px, frozen Chrome 152.0.7977.82.
	for _, row := range []struct {
		text  string
		width float64
	}{
		{"العربية", 25.921875}, {"עברית", 29.63671875}, {"فارسی", 25.259765625},
	} {
		shaped, err := e.Shape(row.text, "Arial", 12, 400, false, false, false)
		if err != nil {
			t.Fatal(err)
		}
		var width float64
		for _, glyph := range shaped.Glyphs {
			width += glyph.Advance
		}
		if math.Abs(width-row.width) > 1e-9 {
			t.Fatalf("%q advance: %g, want %g", row.text, width, row.width)
		}
	}
}

func TestAggregateShapingMatchesFullAcrossScratchReuse(t *testing.T) {
	if _, err := os.Stat(filepath.Join(os.Getenv("WINDIR"), "Fonts", "arial.ttf")); err != nil {
		t.Skip("requires Windows reference fonts")
	}
	reused, fresh := New(), New()
	texts := []string{"", "AV ffi ffl", "A\u0301B", "A👩‍💻B", "Привет κόσμε", "عربى", strings.Repeat("A", shapeScratchRunes+1)}
	for round := 0; round < 2; round++ {
		for _, family := range []string{"Arial", "Courier New", "Times New Roman"} {
			for flags := 0; flags < 4; flags++ {
				for _, text := range texts {
					fresh.shapeScratch, fresh.shapePlans = nil, nil
					full, fullErr := fresh.ShapeWithFonts(text, family, 19.37, 400, false, flags&1 != 0, flags&2 != 0, nil)
					metrics, err := reused.MeasureWithFonts(text, family, 19.37, 400, false, flags&1 != 0, flags&2 != 0, nil)
					if fmt.Sprint(err) != fmt.Sprint(fullErr) {
						t.Fatalf("error differs: %v / %v", err, fullErr)
					}
					if err != nil {
						continue
					}
					want := Metrics{Ascent: full.Ascent, Descent: full.Descent, LineGap: full.LineGap}
					for _, glyph := range full.Glyphs {
						want.Advance += glyph.Advance
					}
					if metrics != want {
						t.Fatalf("projection differs %q %q flags%d: %+v / %+v", family, text, flags, metrics, want)
					}
					actual, err := reused.ShapeWithFonts(text, family, 19.37, 400, false, flags&1 != 0, flags&2 != 0, nil)
					if err != nil || !reflect.DeepEqual(actual, full) {
						t.Fatalf("scratch changed full glyphs: %q %q flags%d %v", family, text, flags, err)
					}
					if len(reused.shapePlans) > shapeScratchPlans {
						t.Fatal("unbounded shape plans")
					}
					if reused.shapeScratch != nil && (len(reused.shapeScratch.Info) != 0 || len(reused.shapeScratch.Pos) != 0) {
						t.Fatal("completed scratch retains active text")
					}
				}
			}
		}
	}
	if fresh.shapeScratch == reused.shapeScratch {
		t.Fatal("Pages share scratch buffer")
	}
}

func TestShapeScratchBoundaries(t *testing.T) {
	e := New()
	var first = &loaded{}
	a := e.shapingBuffer([]rune("hello"), 0, 5, first, false, false)
	b := e.shapingBuffer([]rune("world"), 0, 5, first, false, false)
	if a != b || len(e.shapePlans) != 1 {
		t.Fatal("same shaping plan was not reused")
	}
	large := []rune(strings.Repeat("x", shapeScratchRunes+1))
	temporary := e.shapingBuffer(large, 0, len(large), first, false, false)
	if temporary == b || e.shapeScratch != b {
		t.Fatal("large scratch was retained")
	}
	for i := 0; i < shapeScratchPlans+1; i++ {
		e.shapingBuffer([]rune("x"), 0, 1, &loaded{}, false, false)
		if len(e.shapePlans) > shapeScratchPlans {
			t.Fatal("shaping plan limit exceeded")
		}
	}
	if e.shapeScratch == b {
		t.Fatal("full plan cache was not replaced")
	}
}

func BenchmarkAggregateShapingScratch(b *testing.B) {
	if _, err := os.Stat(filepath.Join(os.Getenv("WINDIR"), "Fonts", "arial.ttf")); err != nil {
		b.Skip("requires Windows reference fonts")
	}
	for _, mode := range []string{"full-fresh-buffer", "aggregate-fresh-buffer", "aggregate-reused-buffer"} {
		b.Run(mode, func(b *testing.B) {
			e := New()
			texts := make([]string, 128)
			for i := range texts {
				texts[i] = fmt.Sprintf("Distinct %d: %s", i, strings.Repeat("AV ffi geometry observations ", 4))
			}
			measure := func() {
				for _, text := range texts {
					if mode != "aggregate-reused-buffer" {
						e.shapeScratch, e.shapePlans = nil, nil
					}
					var err error
					if mode == "full-fresh-buffer" {
						_, err = e.ShapeWithFonts(text, "Arial", 16, 400, false, false, false, nil)
					} else {
						_, err = e.MeasureWithFonts(text, "Arial", 16, 400, false, false, false, nil)
					}
					if err != nil {
						b.Fatal(err)
					}
				}
			}
			measure()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				measure()
			}
		})
	}
}
