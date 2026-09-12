//go:build windows && amd64

package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"
)

func TestCanvasColorAndCoverageFrozenChrome(t *testing.T) {
	for _, name := range []string{"canvas_color_state", "canvas_path_coverage", "canvas_color_space"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/" + name + "_oracle.js")
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile("testdata/" + name + "_chrome152.json")
			if err != nil {
				t.Fatal(err)
			}
			var capture struct {
				Observation map[string]any `json:"observation"`
			}
			if err = json.Unmarshal(data, &capture); err != nil {
				t.Fatal(err)
			}
			for _, restored := range []bool{false, true} {
				t.Run(strconv.FormatBool(restored), func(t *testing.T) {
					t.Setenv("MIMIC_DISABLE_BOOTSTRAP_SNAPSHOT", map[bool]string{false: "1", true: "0"}[restored])
					seed := bootstrapSnapshotPage(t)
					navigateCapabilityFixture(t, seed)
					if restored {
						bootstrapSnapshotWarm(t, seed)
					}
					p, err := seed.ctx.NewPage()
					if err != nil {
						t.Fatal(err)
					}
					defer p.Close()
					navigateCapabilityFixture(t, p)
					actual, err := p.Evaluate(context.Background(), string(source))
					if err != nil {
						t.Fatal(err)
					}
					if name == "canvas_color_space" {
						// Only color-converted paint pixels have a renderer precision
						// boundary: one premultiplied byte at the fixture's alpha >= .5.
						// Metadata/styles/types and alpha remain exact. The separate
						// state oracle above compares HDR writes/copies/resets exactly.
						if err := compareCanvasColorPrecision(capture.Observation, actual); err != nil {
							t.Fatal(err)
						}
					} else if diff := bootstrapSnapshotDifference(name, capture.Observation, actual); diff != "" {
						t.Fatal(diff)
					}
				})
			}
		})
	}
}

func compareCanvasColorPrecision(want, got any) error {
	w, ok := want.(map[string]any)
	if !ok {
		if diff := bootstrapSnapshotDifference("color", want, got); diff != "" {
			return fmt.Errorf("%s", diff)
		}
		return nil
	}
	g, ok := got.(map[string]any)
	if !ok || len(w) != len(g) {
		return fmt.Errorf("color object shape differs")
	}
	for key, value := range w {
		actual, exists := g[key]
		if !exists {
			return fmt.Errorf("missing %s", key)
		}
		if key == "data" {
			wp, wok := value.([]any)
			gp, gok := actual.([]any)
			if !wok || !gok || len(wp) != len(gp) {
				return fmt.Errorf("pixel array shape differs")
			}
			tolerance := 2.0
			if w["pixelFormat"] == "rgba-float16" {
				tolerance = 2.0 / 255
			}
			for i := range wp {
				a, aok := wp[i].(float64)
				b, bok := gp[i].(float64)
				limit := tolerance
				if i%4 == 3 {
					limit = 0
				}
				if !aok || !bok || math.Abs(a-b) > limit {
					return fmt.Errorf("pixel %d: Chrome %v, model %v (precision bound %g)", i, wp[i], gp[i], limit)
				}
			}
		} else if err := compareCanvasColorPrecision(value, actual); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	return nil
}
