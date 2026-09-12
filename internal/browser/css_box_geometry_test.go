package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCSSBoxGraphMatchesFrozenChrome(t *testing.T) {
	source, err := os.ReadFile("testdata/css_box_geometry_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/css_box_geometry_chrome152.json")
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
		var actual map[string]any
		if err := json.Unmarshal([]byte(result.(string)), &actual); err != nil {
			t.Fatal(err)
		}
		for name, expected := range oracle.Observation.(map[string]any) {
			if !reflect.DeepEqual(actual[name], expected) {
				t.Errorf("%s: got %v; want %v", name, actual[name], expected)
			}
		}
	})
}
