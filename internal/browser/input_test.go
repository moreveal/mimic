package browser

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestProtocolKeyboardAndTextMatchChrome152(t *testing.T) {
	raw, err := os.ReadFile("testdata/cdp_input_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Setup     string
		Scenarios []struct {
			Name     string
			Setup    string
			Commands []struct {
				Method string
				Params map[string]any
			}
			Result any
		}
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		for _, scenario := range fixture.Scenarios {
			t.Run(scenario.Name, func(t *testing.T) {
				ctx := context.Background()
				if _, err := page.Evaluate(ctx, fixture.Setup+scenario.Setup); err != nil {
					t.Fatal(err)
				}
				for _, command := range scenario.Commands {
					var err error
					if command.Method == "Runtime.evaluate" {
						_, err = page.Evaluate(ctx, command.Params["expression"].(string))
					} else {
						err = page.DispatchProtocolInput(ctx, command.Method, command.Params)
					}
					if err != nil {
						t.Fatalf("%s: %v", command.Method, err)
					}
				}
				actual, err := page.Evaluate(ctx, `JSON.stringify({events:inputEvents,value:entry.value,start:entry.selectionStart,end:entry.selectionEnd,active:document.activeElement.id})`)
				if err != nil {
					t.Fatal(err)
				}
				var decoded any
				if err := json.Unmarshal([]byte(actual.(string)), &decoded); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(decoded, scenario.Result) {
					expected, _ := json.Marshal(scenario.Result)
					t.Fatalf("input differs from Chrome\nactual: %s\nexpected: %s", actual, expected)
				}
			})
		}
	})
}

func TestDefaultControlGeometryMatchesChrome152(t *testing.T) {
	raw, err := os.ReadFile("testdata/cdp_input_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		GeometrySetup string
		Geometry      []struct {
			ID            string
			Width, Height float64
		}
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		actual, err := page.Evaluate(context.Background(), fixture.GeometrySetup)
		if err != nil {
			t.Fatal(err)
		}
		var rows []struct {
			ID            string
			Width, Height float64
		}
		if err := json.Unmarshal([]byte(actual.(string)), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != len(fixture.Geometry) {
			t.Fatalf("missing controls: %d", len(rows))
		}
		for i, row := range rows {
			expected := fixture.Geometry[i]
			if row.ID != expected.ID || math.Abs(row.Width-expected.Width) > 1.0/64 || row.Height != expected.Height {
				t.Errorf("%s box %gx%g; Chrome %gx%g", row.ID, row.Width, row.Height, expected.Width, expected.Height)
			}
		}
		value, err := page.Evaluate(context.Background(), `(()=>{const e=document.querySelector('#go'),list=e.getClientRects();return list instanceof DOMRectList&&list.length===1&&Array.from(list)[0]===list.item(0)&&list.item(1)===null&&list[0].width===e.getBoundingClientRect().width})()`)
		if err != nil || value != true {
			t.Fatalf("DOMRectList geometry projection: %v %v", value, err)
		}
	})
}

func TestInputInternalsDoNotReenterAuthorSelectors(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		ctx := context.Background()
		_, err := page.Evaluate(ctx, `document.body.innerHTML='<select id="choice"><option>alpha</option><option>beta</option></select><input id="next">';
		globalThis.selectorCalls=[];
		for(const prototype of [Document.prototype,Element.prototype,DocumentFragment.prototype])for(const name of ['querySelector','querySelectorAll']){
		  const original=prototype[name];prototype[name]=function(...args){selectorCalls.push(name);return Reflect.apply(original,this,args)};
		}
		globalThis.choice=document.getElementById('choice');choice.getBoundingClientRect();choice.getClientRects();choice.focus();`)
		if err != nil {
			t.Fatal(err)
		}
		for _, command := range []struct {
			method string
			params map[string]any
		}{
			{"Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "Tab"}},
			{"Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": 10, "y": 10}},
		} {
			if err := page.DispatchProtocolInput(ctx, command.method, command.params); err != nil {
				t.Fatal(err)
			}
		}
		actual, err := page.Evaluate(ctx, `JSON.stringify({calls:selectorCalls,active:document.activeElement.id,options:choice.options.length})`)
		if err != nil || actual != `{"calls":[],"active":"next","options":2}` {
			t.Fatalf("internal selector traversal: %v %v", actual, err)
		}
	})
}

func TestProtocolInputSharesFocusAndFormStateAcrossWorlds(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		ctx := context.Background()
		if _, err := page.Evaluate(ctx, `document.body.innerHTML='<input id="text"><input id="other">';globalThis.mainEvents=[];document.addEventListener('input',e=>mainEvents.push([e.target===document.querySelector('#text'),e.isTrusted,e.target.value]));`); err != nil {
			t.Fatal(err)
		}
		world, err := page.isolatedWorld(ctx, page.Top.Realm, "input-test")
		if err != nil {
			t.Fatal(err)
		}
		d := NewDebugger(page)
		defer d.Close()
		result, err := d.Evaluate(ctx, "", world.ID, `globalThis.isolatedEvents=[];let field=document.querySelector('#text');field.value='abc';field.focus();field.setSelectionRange(1,2);document.addEventListener('input',e=>isolatedEvents.push([e.target===field,e.isTrusted,e.target.value]));document.activeElement===field`, DebuggerOptions{ReturnByValue: true})
		if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != true {
			t.Fatalf("isolated form/focus setup: %#v %v", result, err)
		}
		if err := page.DispatchProtocolInput(ctx, "Input.insertText", map[string]any{"text": "X"}); err != nil {
			t.Fatal(err)
		}
		value, err := page.Evaluate(ctx, `JSON.stringify({value:document.querySelector('#text').value,focused:document.activeElement.id,events:mainEvents})`)
		if err != nil || value != `{"value":"aXc","focused":"text","events":[[true,true,"aXc"]]}` {
			t.Fatalf("main world did not observe canonical input: %v %v", value, err)
		}
		result, err = d.Evaluate(ctx, "", world.ID, `JSON.stringify({value:field.value,events:isolatedEvents})`, DebuggerOptions{ReturnByValue: true})
		if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != `{"value":"aXc","events":[[true,true,"aXc"]]}` {
			t.Fatalf("isolated world did not observe canonical input: %#v %v", result, err)
		}
	})
}

func TestProtocolSelectAndContentClickAcrossWorlds(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		ctx := context.Background()
		if _, err := page.Evaluate(ctx, `document.body.innerHTML='<select id="choice"><option value="a">A</option><option value="b">B</option></select><button id="button" type="button" onclick="window.clicks=(window.clicks||0)+1">Go</button>';`); err != nil {
			t.Fatal(err)
		}
		world, err := page.isolatedWorld(ctx, page.Top.Realm, "form-test")
		if err != nil {
			t.Fatal(err)
		}
		d := NewDebugger(page)
		defer d.Close()
		result, err := d.Evaluate(ctx, "", world.ID, `(()=>{const select=document.querySelector('#choice');select.value=undefined;for(const option of select.options){option.selected=option.value==='b';if(option.selected&&!select.multiple)break}document.querySelector('#button').click();return JSON.stringify({value:select.value,selected:Array.from(select.selectedOptions,option=>option.value)})})()`, DebuggerOptions{ReturnByValue: true})
		if err != nil || result["exceptionDetails"] != nil || result["result"].(map[string]any)["value"] != `{"value":"b","selected":["b"]}` {
			t.Fatalf("isolated selection: %#v %v", result, err)
		}
		value, err := page.Evaluate(ctx, `JSON.stringify({value:document.querySelector('#choice').value,clicks:window.clicks,handler:typeof document.querySelector('#button').onclick})`)
		if err != nil || value != `{"value":"b","clicks":1,"handler":"function"}` {
			t.Fatalf("main canonical selection/content handler: %v %v", value, err)
		}
		point, err := page.Evaluate(ctx, `(()=>{const r=document.querySelector('#button').getBoundingClientRect();return [r.x+r.width/2,r.y+r.height/2]})()`)
		if err != nil {
			t.Fatal(err)
		}
		coordinates := point.([]any)
		for _, kind := range []string{"mouseMoved", "mousePressed", "mouseReleased"} {
			if err := page.DispatchProtocolInput(ctx, "Input.dispatchMouseEvent", map[string]any{"type": kind, "x": coordinates[0], "y": coordinates[1], "button": "left", "clickCount": float64(1)}); err != nil {
				t.Fatal(err)
			}
		}
		value, err = page.Evaluate(ctx, `window.clicks`)
		if err != nil || numberValue(value) != 2 {
			t.Fatalf("trusted pointer did not use same content handler: %v %v", value, err)
		}
	})
}

func TestProtocolMouseClickNavigatesThroughAnchorDescendant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/next" {
			_, _ = w.Write([]byte(`<!doctype html><title>next</title><body>arrived</body>`))
			return
		}
		_, _ = w.Write([]byte(`<!doctype html><a href="/next" style="display:block;width:120px;height:40px"><span style="display:block;width:120px;height:40px">continue</span></a>`))
	}))
	defer server.Close()

	page := testPage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := page.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"mouseMoved", "mousePressed", "mouseReleased"} {
		if err := page.DispatchProtocolInput(ctx, "Input.dispatchMouseEvent", map[string]any{"type": kind, "x": 60, "y": 20, "button": "left", "clickCount": 1}); err != nil {
			t.Fatal(err)
		}
	}
	for page.URL() != server.URL+"/next" || !page.LoadEventEnded() {
		realm := page.Top.Realm
		if err := realm.scheduler.Wait(ctx); err != nil {
			t.Fatal(err)
		}
		if err := realm.RunReady(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if page.URL() != server.URL+"/next" {
		t.Fatalf("anchor descendant click did not navigate: %s", page.URL())
	}
}

func TestProtocolMouseClickCannotBypassOverlayWithHitHint(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		ctx := context.Background()
		before := page.URL()
		if _, err := page.Evaluate(ctx, `document.body.innerHTML='<a id="target" href="#arrived" style="display:block;width:100px;height:40px">go</a><div style="position:absolute;left:0;top:0;width:200px;height:100px;z-index:2"></div>'`); err != nil {
			t.Fatal(err)
		}
		document, ok := page.Document()
		target, ok := document.Find("#target")
		if !ok {
			t.Fatal("missing target node")
		}
		for _, kind := range []string{"mousePressed", "mouseReleased"} {
			params := map[string]any{"type": kind, "x": 20, "y": 20, "button": "left", "clickCount": 1, "_mimicNodeId": target.ID, "_mimicLocalX": 20, "_mimicLocalY": 20}
			if err := page.DispatchProtocolInput(ctx, "Input.dispatchMouseEvent", params); err != nil {
				t.Fatal(err)
			}
		}
		if page.URL() != before {
			t.Fatalf("non-protocol hint bypassed overlay: %s", page.URL())
		}
	})
}

func TestProtocolMouseInputRoutesThroughIframeCoordinates(t *testing.T) {
	historyTestPages(t, func(t *testing.T, page *Page) {
		navigateCapabilityFixture(t, page)
		ctx := context.Background()
		value, err := page.Evaluate(ctx, `(()=>{
			document.body.style.margin='0';
			const frame=document.createElement('iframe');
			frame.style.cssText='display:block;margin-left:40px;margin-top:30px;width:200px;height:100px;border:0';
			document.body.append(frame);
			frame.contentDocument.body.innerHTML='<button style="margin-left:10px;margin-top:5px;width:80px;height:30px" onclick="window.clicks=(window.clicks||0)+1">Go</button>';
			const outer=frame.getBoundingClientRect(),inner=frame.contentDocument.querySelector('button').getBoundingClientRect();
			return [outer.x+inner.x+inner.width/2,outer.y+inner.y+inner.height/2];
		})()`)
		if err != nil {
			t.Fatal(err)
		}
		coordinates := value.([]any)
		for _, kind := range []string{"mouseMoved", "mousePressed", "mouseReleased"} {
			if err := page.DispatchProtocolInput(ctx, "Input.dispatchMouseEvent", map[string]any{"type": kind, "x": coordinates[0], "y": coordinates[1], "button": "left", "clickCount": float64(1)}); err != nil {
				t.Fatal(err)
			}
		}
		clicked, err := page.Evaluate(ctx, `document.querySelector('iframe').contentWindow.clicks`)
		if err != nil || numberValue(clicked) != 1 {
			t.Fatalf("iframe button was not clicked: %v %v", clicked, err)
		}
	})
}

func TestProtocolInputSuppressionSurvivesNavigation(t *testing.T) {
	page := testPage(t)
	ctx := context.Background()
	navigateCapabilityFixture(t, page)
	if err := page.DispatchProtocolInput(ctx, "Input.setIgnoreInputEvents", map[string]any{"ignore": true}); err != nil {
		t.Fatal(err)
	}
	navigateCapabilityFixture(t, page)
	if _, err := page.Evaluate(ctx, `document.body.innerHTML='<input>';document.querySelector('input').focus()`); err != nil {
		t.Fatal(err)
	}
	if err := page.DispatchProtocolInput(ctx, "Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "a", "text": "a"}); err != nil {
		t.Fatal(err)
	}
	value, err := page.Evaluate(ctx, `document.querySelector('input').value===''&&!navigator.userActivation.isActive`)
	if err != nil || value != true {
		t.Fatalf("suppression was lost or granted activation: %v %v", value, err)
	}
	if err := page.DispatchProtocolInput(ctx, "Input.setIgnoreInputEvents", map[string]any{"ignore": false}); err != nil {
		t.Fatal(err)
	}
	if err := page.DispatchProtocolInput(ctx, "Input.insertText", map[string]any{"text": "b"}); err != nil {
		t.Fatal(err)
	}
	value, err = page.Evaluate(ctx, `document.querySelector('input').value`)
	if err != nil || value != "b" {
		t.Fatalf("reenabled input: %v %v", value, err)
	}
}
