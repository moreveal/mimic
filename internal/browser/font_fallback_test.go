package browser

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestFontFallbackChrome152(t *testing.T) {
	fixture, err := os.ReadFile("testdata/font_fallback_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/font-fallback-chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	type measurement struct {
		Length float64   `json:"length"`
		Box    []float64 `json:"box"`
	}
	var capture struct {
		Result map[string]measurement `json:"result"`
	}
	if err = json.Unmarshal(data, &capture); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		navigateCapabilityFixture(t, p)
		value, err := p.Evaluate(context.Background(), "JSON.stringify("+string(fixture)+")")
		if err != nil {
			t.Fatal(err)
		}
		var actual map[string]measurement
		if err = json.Unmarshal([]byte(value.(string)), &actual); err != nil {
			t.Fatal(err)
		}
		if len(actual) != len(capture.Result) {
			t.Fatal("missing observations")
		}
		for name, want := range capture.Result {
			got := actual[name]
			if got.Length != want.Length || len(got.Box) != 4 {
				t.Errorf("%s: got %v want %v", name, got, want)
				continue
			}
			if got.Box[1] != want.Box[1] || got.Box[3] != want.Box[3] {
				t.Errorf("%s: line metrics got %v want %v", name, got.Box, want.Box)
			}
			// The outline observation model does not rasterize Windows hinted ink.
			// Keep the native capture intact; only horizontal ink uses a bounded check.
			if math.Abs(got.Box[0]-want.Box[0]) > 1 || math.Abs((got.Box[0]+got.Box[2])-(want.Box[0]+want.Box[2])) > 1 {
				t.Errorf("%s: ink bounds got %v want %v", name, got.Box, want.Box)
			}
		}
		for _, e := range p.Trace().Events() {
			if e.Name == "Text.shapeFailure" {
				t.Errorf("shaping failure: %v", e.Data)
			}
		}
	})
}
