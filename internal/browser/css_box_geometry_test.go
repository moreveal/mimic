package browser

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCSSBoxGraphMatchesFrozenChrome(t *testing.T) { testCSSObservation(t, "css_box_geometry") }

func TestCSSBoxStateIsCompleteDuringRecursiveComputedStyle(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		result, err := page.Evaluate(context.Background(), `(()=>{
			document.head.innerHTML='<style>.submenuitem { font-size: 14px }</style>';
			document.body.innerHTML='<div style="display:none"><a class="submenuitem">Home</a></div>';
			const rect=document.querySelector('a').getBoundingClientRect();
			return rect.width===0&&rect.height===0;
		})()`)
		if err != nil || result != true {
			t.Fatalf("hidden descendant geometry: %v %v", result, err)
		}
	})
}

func TestCSSAncestorTransformMovesDescendantClientRect(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		result, err := page.Evaluate(context.Background(), `(()=>{
document.body.style.margin='0';document.body.innerHTML='<div id="parent" style="position:fixed;left:0;top:0;width:100px;height:100px;transform:translateX(-260px)"><div id="child" style="width:50px;height:40px"></div></div>';
const parent=document.getElementById('parent').getBoundingClientRect(),child=document.getElementById('child').getBoundingClientRect();
return JSON.stringify([parent.x,parent.y,child.x,child.y,child.width,child.height,document.elementFromPoint(10,10)?.id||''])
})()`)
		const want = `[-260,0,-260,0,50,40,""]`
		if err != nil || result != want {
			t.Fatalf("ancestor transform geometry: %#v want %#v, err=%v", result, want, err)
		}
	})
}

func TestCSSRowFlexItemsUseIntrinsicBasisAndShrink(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		result, err := page.Evaluate(context.Background(), `(()=>{
document.body.style.margin='0';document.body.innerHTML='<div id="row" style="display:flex;width:300px"><div class="item" style="display:inline-flex"><span style="display:block;width:200px;height:10px"></span></div><div class="item" style="display:inline-flex"><span style="display:block;width:200px;height:10px"></span></div><div class="item" style="display:inline-flex"><span style="display:block;width:200px;height:10px"></span></div></div>';
return JSON.stringify(Array.from(document.querySelectorAll('.item'),item=>{const r=item.getBoundingClientRect();return [r.x,r.width]}))
})()`)
		const want = `[[0,100],[100,100],[200,100]]`
		if err != nil || result != want {
			t.Fatalf("row flex sizing: %v want %s, err=%v", result, want, err)
		}
	})
}

func TestCSSRowFlexAutoMarginConsumesRemainingSpace(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		result, err := page.Evaluate(context.Background(), `(()=>{
document.body.style.margin='0';document.body.innerHTML='<div style="display:flex;width:300px"><div id="start" style="display:inline-flex"><span style="display:block;width:100px;height:10px"></span></div><div id="end" style="display:inline-flex;margin-left:auto"><span style="display:block;width:40px;height:10px"></span></div></div>';
const start=document.getElementById('start').getBoundingClientRect(),end=document.getElementById('end').getBoundingClientRect();
return JSON.stringify([[start.x,start.width],[end.x,end.width]])
})()`)
		const want = `[[0,100],[260,40]]`
		if err != nil || result != want {
			t.Fatalf("row flex auto margin: %v want %s, err=%v", result, want, err)
		}
	})
}

func TestCSSNestedFlexIntrinsicBorderBox(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		result, err := page.Evaluate(context.Background(), `(()=>{
document.body.style.margin='0';document.body.innerHTML='<div style="display:flex;width:300px"><div id="group" style="display:inline-flex;margin-left:auto"><div id="a" style="display:flex;box-sizing:border-box;padding:0 calc(10px)"><span style="width:40px;height:10px;display:block"></span></div><div id="b" style="display:flex;box-sizing:border-box;padding:0 10px"><span style="width:60px;height:10px;display:block"></span></div></div></div>';
return JSON.stringify(['group','a','b'].map(id=>{const r=document.getElementById(id).getBoundingClientRect();return [r.x,r.width]}));
})()`)
		const want = `[[160,140],[160,60],[220,80]]`
		if err != nil || result != want {
			t.Fatalf("nested flex border box: %v want %s, err=%v", result, want, err)
		}
	})
}

func TestCSSComputedCatalogMatchesFrozenChrome(t *testing.T) {
	for _, name := range []string{"css_geometry_integration", "css_geometry_audit", "css_details_query", "css_foreign_owner", "css_shadow_inheritance", "css_line_rounding", "css_wrapper_flow", "css_specified_values", "css_zero_font", "css_computed_initial", "css_computed_catalog", "css_computed_dynamic"} {
		t.Run(name, func(t *testing.T) {
			parallelOracle(t)
			testCSSObservation(t, name)
		})
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
	for _, name := range []string{"css_geometry_integration", "css_geometry_audit", "css_details_query"} {
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
