package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCSSBoxGraphMatchesFrozenChrome(t *testing.T) { testCSSObservation(t, "css_box_geometry") }

func TestCSSComputedCatalogMatchesFrozenChrome(t *testing.T) {
	for _, name := range []string{"css_line_rounding", "css_wrapper_flow", "css_specified_values", "css_zero_font", "css_computed_initial", "css_computed_catalog", "css_computed_dynamic"} {
		t.Run(name, func(t *testing.T) { testCSSObservation(t, name) })
	}
}

func testCSSObservation(t *testing.T, name string) {
	source, err := os.ReadFile("testdata/" + name + "_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/" + name + "_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct{ Observation any }
	if err := json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		result, err := page.Evaluate(context.Background(), "document.body.replaceChildren();JSON.stringify("+string(source)+")")
		if err != nil {
			t.Fatal(err)
		}
		var actualValue any
		if err := json.Unmarshal([]byte(result.(string)), &actualValue); err != nil {
			t.Fatal(err)
		}
		expectedFields, object := oracle.Observation.(map[string]any)
		if !object {
			if !reflect.DeepEqual(actualValue, oracle.Observation) {
				t.Errorf("got %v; want %v", actualValue, oracle.Observation)
			}
			return
		}
		actual, _ := actualValue.(map[string]any)
		for name, expected := range expectedFields {
			if !reflect.DeepEqual(actual[name], expected) {
				if want, ok := expected.(map[string]any); ok {
					got, _ := actual[name].(map[string]any)
					for key, value := range want {
						if !reflect.DeepEqual(got[key], value) {
							t.Errorf("%s.%s: got %v; want %v", name, key, got[key], value)
						}
					}
				} else {
					t.Errorf("%s: got %v; want %v", name, actual[name], expected)
				}
			}
		}
	})
}
