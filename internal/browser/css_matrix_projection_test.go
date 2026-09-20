package browser

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestCSSMatrixProjectionMatchesFrozenChrome(t *testing.T) {
	parallelBrowserTest(t)
	source, err := os.ReadFile("testdata/css_matrix_projection_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/css_matrix_projection_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	type observation struct {
		Rects []struct {
			Transform           string
			X, Y, Width, Height float64
		}
		Parsed            int
		PrivateProjection bool
		Calls             []string
		Error             string
	}
	var oracle struct{ Observation observation }
	if err := json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		result, err := page.Evaluate(context.Background(), "JSON.stringify("+string(source)+")")
		if err != nil {
			t.Fatal(err)
		}
		var actual observation
		if err := json.Unmarshal([]byte(result.(string)), &actual); err != nil {
			t.Fatal(err)
		}
		if actual.Error != "" || len(actual.Calls) != 0 || actual.Parsed != oracle.Observation.Parsed || actual.PrivateProjection != oracle.Observation.PrivateProjection {
			t.Fatalf("private matrix path: %s", result)
		}
		if len(actual.Rects) != len(oracle.Observation.Rects) {
			t.Fatal("missing geometry rows")
		}
		for i, r := range actual.Rects {
			want := oracle.Observation.Rects[i]
			if r.Transform != want.Transform {
				t.Fatal("transform order")
			}
			// Blink stores projected bounds at float precision; the CPU model keeps
			// doubles. This tolerance is below a layout unit (1/64 CSS px).
			for name, pair := range map[string][2]float64{"x": {r.X, want.X}, "y": {r.Y, want.Y}, "width": {r.Width, want.Width}, "height": {r.Height, want.Height}} {
				if math.IsNaN(pair[0]) || math.Abs(pair[0]-pair[1]) > 0.0001 {
					t.Errorf("%s %s: %.12g want %.12g", r.Transform, name, pair[0], pair[1])
				}
			}
		}
	})
}
