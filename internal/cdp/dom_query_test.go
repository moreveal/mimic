package cdp

import (
	"context"
	"reflect"
	"testing"

	"github.com/gorilla/websocket"
)

// Chrome 152.0.7977.82 returns border quads here, even when the
// DOM.getBoxModel content rectangle has zero area.
func TestDOMContentQuadsIncludePaddingAndBorder(t *testing.T) {
	s, addr := runningServer(t)
	if _, err := evaluatePageFixture(s.Page, `document.body.innerHTML='<input id="entry" style="width:0;height:28px;padding:12px 14px;border:2px solid;box-sizing:content-box">'`); err != nil {
		t.Fatal(err)
	}
	d, _ := s.Page.Document()
	entry, _ := d.Find("#entry")
	c := browserConnection(t, addr)
	sid := wireCall(t, c, 1, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	params := map[string]any{"nodeId": entry.ID}
	model := flatCall(t, c, sid, 2, "DOM.getBoxModel", params)["model"].(map[string]any)
	quads := flatCall(t, c, sid, 3, "DOM.getContentQuads", params)["quads"].([]any)
	if len(quads) != 1 || !reflect.DeepEqual(quads[0], model["border"]) {
		t.Fatalf("quads must include padding and border: %v; model: %v", quads, model)
	}
	content := model["content"].([]any)
	if content[0] != content[2] {
		t.Fatalf("fixture must have zero content width: %v", content)
	}
	quad := quads[0].([]any)
	x, y := (coordinateValue(quad[0])+coordinateValue(quad[2]))/2, (coordinateValue(quad[1])+coordinateValue(quad[5]))/2
	flatCall(t, c, sid, 4, "Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": x, "y": y, "button": "left", "clickCount": 1})
	flatCall(t, c, sid, 5, "Input.dispatchMouseEvent", map[string]any{"type": "mouseReleased", "x": x, "y": y, "button": "left", "clickCount": 1})
	if got, err := evaluatePageFixture(s.Page, `document.activeElement.id`); err != nil || got != "entry" {
		t.Fatalf("quad center did not focus input: %v %v", got, err)
	}
}

func TestDOMQueriesShareRealmSelectorEngine(t *testing.T) {
	s, addr := runningServer(t)
	if _, err := evaluatePageFixture(s.Page, `document.body.innerHTML='<main id="root"><i class="item"></i><i class="item" id="second"></i></main>';Element.prototype.querySelectorAll=()=>[]`); err != nil {
		t.Fatal(err)
	}
	d, _ := s.Page.Document()
	root, _ := d.Find("#root")
	second, _ := d.Find("#second")
	c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for index, method := range []string{"DOM.querySelector", "DOM.querySelectorAll"} {
		_ = c.WriteJSON(map[string]any{"id": index + 1, "method": method, "params": map[string]any{"nodeId": root.ID, "selector": ":scope > i:nth-child(2)"}})
		reply := readReply(t, c, float64(index+1))
		if reply["error"] != nil {
			t.Fatal(reply)
		}
		result := reply["result"].(map[string]any)
		if index == 0 {
			if result["nodeId"] != float64(second.ID) {
				t.Fatal(result)
			}
		} else {
			ids := result["nodeIds"].([]any)
			if len(ids) != 1 || ids[0] != float64(second.ID) {
				t.Fatal(result)
			}
		}
	}
	_ = c.WriteJSON(map[string]any{"id": 3, "method": "DOM.querySelector", "params": map[string]any{"nodeId": root.ID, "selector": "["}})
	if reply := readReply(t, c, 3); reply["error"] == nil {
		t.Fatal("accepted invalid selector", reply)
	}
	_ = c.WriteJSON(map[string]any{"id": 4, "method": "DOM.querySelector", "params": map[string]any{"nodeId": root.ID, "selector": "main"}})
	if reply := readReply(t, c, 4); reply["result"].(map[string]any)["nodeId"] != float64(0) {
		t.Fatal("included query root", reply)
	}
}

func TestDOMQueryInitialDocumentBootstrapsRealm(t *testing.T) {
	s, _ := runningServer(t)
	d, _ := s.Page.Document()
	ids, err := s.Page.QueryDOM(context.Background(), d.Root().ID, "html", false)
	if err != nil || len(ids) != 1 {
		t.Fatalf("initial query: %v %v", ids, err)
	}
}

func TestDOMContentQuadsUseResolvedControlFontGeometry(t *testing.T) {
	s, addr := runningServer(t)
	_, err := evaluatePageFixture(s.Page, `document.body.innerHTML='<button id="login" style="font-size:var(--missing)">Log in</button>'`)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := s.Page.Document()
	button, _ := d.Find("#login")
	c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.WriteJSON(map[string]any{"id": 1, "method": "DOM.getContentQuads", "params": map[string]any{"nodeId": button.ID}}); err != nil {
		t.Fatal(err)
	}
	reply := readReply(t, c, 1)
	if reply["error"] != nil {
		t.Fatal(reply)
	}
	quads := reply["result"].(map[string]any)["quads"].([]any)
	if len(quads) != 1 || len(quads[0].([]any)) != 8 {
		t.Fatalf("unexpected quads: %#v", quads)
	}
	quad := quads[0].([]any)
	if width := coordinateValue(quad[2]) - coordinateValue(quad[0]); width <= 16 {
		t.Fatalf("fabricated control geometry: %#v", quad)
	}
	_, err = evaluatePageFixture(s.Page, `document.getElementById('login').style.fontSize='2ex'`)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.WriteJSON(map[string]any{"id": 2, "method": "DOM.getContentQuads", "params": map[string]any{"nodeId": button.ID}}); err != nil {
		t.Fatal(err)
	}
	if reply := readReply(t, c, 2); reply["error"] == nil {
		t.Fatalf("unsupported geometry became a fabricated quad: %#v", reply)
	}
}
