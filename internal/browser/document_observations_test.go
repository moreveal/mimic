package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestDocumentCompatibilityOracle(t *testing.T) {
	for _, name := range []string{"document-visibility", "html-enumerated"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile("testdata/" + strings.ReplaceAll(name, "-", "_") + "_oracle.js")
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile("../../compatibility/captures/semantic-checkpoints/" + name + "-chrome152.json")
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
