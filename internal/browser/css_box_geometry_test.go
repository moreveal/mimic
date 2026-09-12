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
	for _, name := range []string{"css_geometry_audit", "css_details_query", "css_foreign_owner", "css_shadow_inheritance", "css_line_rounding", "css_wrapper_flow", "css_specified_values", "css_zero_font", "css_computed_initial", "css_computed_catalog", "css_computed_dynamic"} {
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

func TestCSSForeignOwnerRestoredRealm(t *testing.T) {
	p := bootstrapSnapshotPage(t)
	bootstrapSnapshotWarm(t, p)
	source, err := os.ReadFile("testdata/css_foreign_owner_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/css_foreign_owner_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct{ Observation any }
	if err := json.Unmarshal(raw, &oracle); err != nil {
		t.Fatal(err)
	}
	actual := bootstrapSnapshotEvaluate(t, p, string(source))
	if !reflect.DeepEqual(actual, oracle.Observation) {
		t.Fatalf("got %v; want %v", actual, oracle.Observation)
	}
	bootstrapSnapshotAssertRestored(t, p, 1)
	if p.crossRealmDepth != 0 {
		t.Fatalf("entry depth leaked: %d", p.crossRealmDepth)
	}
}

func TestCSSGeometryAuditRestoredRealms(t *testing.T) {
	for _, name := range []string{"css_geometry_audit", "css_details_query"} {
		t.Run(name, func(t *testing.T) {
			p := bootstrapSnapshotPage(t)
			bootstrapSnapshotWarm(t, p)
			source, err := os.ReadFile("testdata/" + name + "_oracle.js")
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile("testdata/" + name + "_chrome152.json")
			if err != nil {
				t.Fatal(err)
			}
			var oracle struct{ Observation any }
			if err = json.Unmarshal(raw, &oracle); err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(string(source))
			actual := bootstrapSnapshotEvaluate(t, p, "(()=>{const f=document.createElement('iframe');document.body.append(f);try{return f.contentWindow.eval("+string(encoded)+")}finally{f.remove()}})()")
			if !reflect.DeepEqual(actual, oracle.Observation) {
				t.Fatalf("got %v; want %v", actual, oracle.Observation)
			}
			bootstrapSnapshotAssertRestored(t, p, 1)
		})
	}
}
