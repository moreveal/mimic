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

func TestCSSFixedInsetsAcceptTailwindCalcProducts(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		result, err := page.Evaluate(context.Background(), `(()=>{
document.documentElement.style.setProperty('--spacing','.25rem');
document.body.style.margin='0';
document.body.innerHTML='<button id="target" style="position:fixed;right:calc(var(--spacing) * 6);bottom:calc(var(--spacing)*24);width:calc(50px*2);height:calc(40px/2)"></button>';
const target=document.getElementById('target'),rect=target.getBoundingClientRect(),style=getComputedStyle(target);
return JSON.stringify({rect:[rect.x,rect.y,rect.width,rect.height],insets:[style.right,style.bottom],hit:document.elementFromPoint(rect.x+50,rect.y+10)===target});
})()`)
		const want = `{"rect":[1148,537,100,20],"insets":["24px","96px"],"hit":true}`
		if err != nil || result != want {
			t.Fatalf("fixed Tailwind calc-product insets: %v want %s, err=%v", result, want, err)
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

func TestCSSTaffyNestedFlexBasisKeepsSearchControlVisible(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		result, err := page.Evaluate(context.Background(), `(()=>{
document.body.style.margin='0';document.body.innerHTML='<div style="display:flex;width:416px"><div style="width:40px"></div><div style="display:flex;flex:1 1 0%;min-width:0"><textarea id="q" name="q" style="display:flex;flex:1 1 100%;min-width:0"></textarea></div><div style="width:40px"></div></div>';
const q=document.getElementById('q'),r=q.getBoundingClientRect();return JSON.stringify([r.x,r.width,r.height,q.offsetWidth,q.clientWidth]);
})()`)
		const want = `[40,336,36,336,336]`
		if err != nil || result != want {
			t.Fatalf("nested flex-basis search control: %v want %s, err=%v", result, want, err)
		}
	})
}

func TestCSSTaffyGridTracksGapAndPlacement(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		result, err := page.Evaluate(context.Background(), `(()=>{
document.body.style.margin='0';document.body.innerHTML='<div id="grid" style="display:grid;width:300px;height:40px;grid-template-columns:repeat(2,50px) 1fr;column-gap:10px"><div id="a" style="width:50px;grid-column-start:1"></div><div id="b" style="width:240px;grid-column-start:2;grid-column-end:span 2"></div></div>';
return JSON.stringify(['grid','a','b'].map(id=>{const e=document.getElementById(id),r=e.getBoundingClientRect();return [r.x,r.width,e.offsetWidth,e.clientWidth]}));
})()`)
		const want = `[[0,300,300,300],[0,50,50,50],[60,240,240,240]]`
		if err != nil || result != want {
			t.Fatalf("grid tracks and placement: %v want %s, err=%v", result, want, err)
		}
	})
}

func TestCSSTaffyGeometryAPIsShareBorderAndOffsetBoxes(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		result, err := page.Evaluate(context.Background(), `(()=>{
document.body.style.margin='10px';document.body.innerHTML='<div id="root" style="display:flex;position:relative;box-sizing:border-box;width:200px;height:40px;border:4px solid"><div id="child" style="box-sizing:border-box;width:100px;height:20px;border:3px solid"></div></div>';
const root=document.getElementById('root'),child=document.getElementById('child'),r=child.getBoundingClientRect();return JSON.stringify({rect:[r.x,r.y,r.width,r.height],root:[root.offsetWidth,root.clientWidth],child:[child.offsetWidth,child.clientWidth,child.offsetLeft,child.offsetTop]});
})()`)
		const want = `{"rect":[14,14,100,20],"root":[200,192],"child":[100,94,0,0]}`
		if err != nil || result != want {
			t.Fatalf("Taffy geometry APIs: %v want %s, err=%v", result, want, err)
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

func TestRangeTextGeometryUsesElementLayout(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		result, err := page.Evaluate(context.Background(), `(()=>{
document.body.style.margin='0';document.body.innerHTML='<div id="label" style="display:block;width:120px;height:24px">Sign in with Steam</div>';
const text=document.getElementById('label').firstChild,range=document.createRange();range.selectNode(text);
const rect=range.getBoundingClientRect(),rects=range.getClientRects(),clone=range.cloneRange();
range.setStart(text,0);range.setEnd(text,text.textContent.length);
return JSON.stringify({rect:[rect.x,rect.y,rect.width,rect.height],rectType:rect instanceof DOMRect,rectsType:rects instanceof DOMRectList,rectsLength:rects.length,start:range.startContainer===text,end:range.endContainer===text,offsets:[range.startOffset,range.endOffset],collapsed:range.collapsed,ancestor:range.commonAncestorContainer===text,text:String(clone)});
})()`)
		const want = `{"rect":[0,0,120,24],"rectType":true,"rectsType":true,"rectsLength":1,"start":true,"end":true,"offsets":[0,18],"collapsed":false,"ancestor":true,"text":"Sign in with Steam"}`
		if err != nil || result != want {
			t.Fatalf("range text geometry: %v want %s, err=%v", result, want, err)
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
