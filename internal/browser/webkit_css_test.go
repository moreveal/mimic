package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestWebkitCSSOracle(t *testing.T) {
	for _, name := range []string{"values", "aliases", "computed", "rules", "supports", "longhands", "shorthand-wide", "shorthand-mutation", "shorthand-rule-mutation", "state", "reflection", "flex", "flex-edges", "numbers", "border-text", "colors", "columns-emphasis", "radius", "ordinary-relations", "column-count", "supports-reflection", "animation-transition", "animation-longhands"} {
		t.Run(name, func(t *testing.T) {
			parallelOracle(t)
			source, err := os.ReadFile("testdata/webkit_css_" + strings.ReplaceAll(name, "-", "_") + "_oracle.js")
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/webkit-css-" + name + "-chrome152.json")
			if err != nil {
				t.Fatal(err)
			}
			var capture struct {
				Result map[string]any `json:"result"`
			}
			if err = json.Unmarshal(data, &capture); err != nil {
				t.Fatal(err)
			}
			historyTestPages(t, func(t *testing.T, p *Page) {
				navigateCapabilityFixture(t, p)
				value, err := p.Evaluate(context.Background(), "JSON.stringify("+string(source)+")")
				if err != nil {
					t.Fatal(err)
				}
				var actual map[string]any
				if err = json.Unmarshal([]byte(value.(string)), &actual); err != nil {
					t.Fatal(err)
				}
				for key, want := range capture.Result {
					if !reflect.DeepEqual(actual[key], want) {
						t.Errorf("%s: got %v want %v", key, actual[key], want)
					}
				}
			})
		})
	}
}
