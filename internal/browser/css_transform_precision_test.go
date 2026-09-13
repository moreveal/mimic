//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
)

func TestCSSTransformPrecisionMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "css_transform_precision")
}

func TestCSSTransformPrecisionGojaMatchesFrozenChrome(t *testing.T) {
	testTransformGojaOracle(t, "css_transform_precision")
}
func TestSVGPercentTransformGojaMatchesFrozenChrome(t *testing.T) {
	testTransformGojaOracle(t, "svg_percent_transform")
}
func testTransformGojaOracle(t *testing.T, name string) {
	t.Helper()
	browser, err := New(gojaengine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	p, err := browser.NewContext().NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	navigateCapabilityFixture(t, p)
	source, err := os.ReadFile("testdata/" + name + "_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/" + name + "_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var frozen struct {
		Result struct{ Result struct{ Value any } }
	}
	if err = json.Unmarshal(data, &frozen); err != nil {
		t.Fatal(err)
	}
	result, err := p.Evaluate(context.Background(), "(async()=>JSON.stringify(await "+string(source)+"))()")
	if err != nil {
		t.Fatal(err)
	}
	var actual any
	if err = json.Unmarshal([]byte(result.(string)), &actual); err != nil {
		t.Fatal(err)
	}
	// Goja and V8 use different transcendental implementations. Preserve exact
	// CSS serialization and all non-matrix observations; allow at most four ULP
	// for the unrounded rotation matrix only.
	expected := frozen.Result.Result.Value.(map[string]any)
	rows := actual.(map[string]any)
	for key, wantRow := range expected {
		if !strings.Contains(strings.ToLower(key), "rotat") {
			continue
		}
		want, wok := wantRow.(map[string]any)
		got, gok := rows[key].(map[string]any)
		if !wok || !gok {
			continue
		}
		wm, wok := want["matrix"].([]any)
		gm, gok := got["matrix"].([]any)
		if !wok || !gok || len(wm) != len(gm) {
			continue
		}
		for i, w := range wm {
			wf, wok := w.(float64)
			gf, gok := gm[i].(float64)
			if !wok || !gok || wf == gf {
				continue
			}
			ulp := math.Abs(math.Nextafter(wf, math.Inf(1)) - wf)
			if math.Abs(wf-gf) <= 4*ulp {
				gm[i] = wf
			}
		}
	}
	if !reflect.DeepEqual(actual, frozen.Result.Result.Value) {
		t.Fatal(bootstrapSnapshotDifference("Goja transform precision", frozen.Result.Result.Value, actual))
	}
}

func TestSVGPercentTransformMatchesFrozenChrome(t *testing.T) {
	documentAllOracle(t, "svg_percent_transform")
}
