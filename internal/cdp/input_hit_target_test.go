package cdp

import (
	"context"
	"testing"
)

func TestContentQuadQueryDoesNotOverrideMouseHitTarget(t *testing.T) {
	s, addr := runningServer(t)
	if _, err := s.Page.Evaluate(context.Background(), `document.body.innerHTML='<a id="link" style="position:absolute;left:0;top:0;width:100px;height:100px"><span id="child" style="display:block;width:100px;height:100px">open</span></a>';globalThis.clicked='';document.addEventListener('click',e=>clicked=e.target.id);`); err != nil {
		t.Fatal(err)
	}
	d, _ := s.Page.Document()
	link, _ := d.Find("#link")
	c := browserConnection(t, addr)
	sid := wireCall(t, c, 1, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	flatCall(t, c, sid, 2, "DOM.getContentQuads", map[string]any{"nodeId": link.ID})
	flatCall(t, c, sid, 3, "Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": 30, "y": 30, "button": "left", "clickCount": 1})
	flatCall(t, c, sid, 4, "Input.dispatchMouseEvent", map[string]any{"type": "mouseReleased", "x": 30, "y": 30, "button": "left", "clickCount": 1})
	if got, err := s.Page.Evaluate(context.Background(), `clicked`); err != nil || got != "child" {
		t.Fatalf("quad query changed event target: %v %v", got, err)
	}
	if _, err := s.Page.Evaluate(context.Background(), `document.body.insertAdjacentHTML('beforeend','<div id="cover" style="position:fixed;left:0;top:0;width:100px;height:100px;z-index:10"></div>')`); err != nil {
		t.Fatal(err)
	}
	flatCall(t, c, sid, 5, "Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": 30, "y": 30, "button": "left", "clickCount": 1})
	flatCall(t, c, sid, 6, "Input.dispatchMouseEvent", map[string]any{"type": "mouseReleased", "x": 30, "y": 30, "button": "left", "clickCount": 1})
	if got, err := s.Page.Evaluate(context.Background(), `clicked`); err != nil || got != "cover" {
		t.Fatalf("quad query bypassed overlay: %v %v", got, err)
	}
}
