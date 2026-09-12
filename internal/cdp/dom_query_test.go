package cdp

import (
	"context"
	"testing"

	"github.com/gorilla/websocket"
)

func TestInputHitHintUsesOnlyCoordinatesInsideQuad(t *testing.T) {
	s := &session{hitNode: 42, hitDX: 100, hitDY: 200, hitQuad: []any{110.0, 220.0, 150.0, 220.0, 150.0, 260.0, 110.0, 260.0}}
	inside := map[string]any{"x": 130.0, "y": 240.0}
	s.applyInputHitHint(inside)
	if inside["_mimicNodeId"] != int64(42) || inside["_mimicLocalX"] != 30.0 || inside["_mimicLocalY"] != 40.0 {
		t.Fatalf("inside hint: %#v", inside)
	}
	outside := map[string]any{"x": 151.0, "y": 240.0}
	s.applyInputHitHint(outside)
	if outside["_mimicNodeId"] != nil {
		t.Fatalf("outside point received hint: %#v", outside)
	}
}

func TestDOMQueriesShareRealmSelectorEngine(t *testing.T) {
	s, addr := runningServer(t)
	if _, err := s.Page.Evaluate(context.Background(), `document.body.innerHTML='<main id="root"><i class="item"></i><i class="item" id="second"></i></main>';Element.prototype.querySelectorAll=()=>[]`); err != nil {
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
	if _, err := s.Page.Evaluate(context.Background(), `document.body.innerHTML='<button id="login" style="font-size:var(--missing)">Log in</button>'`); err != nil {
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
	if _, err := s.Page.Evaluate(context.Background(), `document.getElementById('login').style.fontSize='2ex'`); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteJSON(map[string]any{"id": 2, "method": "DOM.getContentQuads", "params": map[string]any{"nodeId": button.ID}}); err != nil {
		t.Fatal(err)
	}
	if reply := readReply(t, c, 2); reply["error"] == nil {
		t.Fatalf("unsupported geometry became a fabricated quad: %#v", reply)
	}
}
