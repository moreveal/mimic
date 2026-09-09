package cdp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestCDPInitialBlankFramesCompleteLifecycle(t *testing.T) {
	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<body><iframe></iframe><iframe src="about:blank"></iframe><script>window.blankOrder=[];const f=document.createElement('iframe');f.onload=()=>blankOrder.push('owner');document.body.append(f);blankOrder.push('after:'+f.contentDocument.readyState)</script>`)
	}))
	defer pageServer.Close()
	s, addr := runningServer(t)
	c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_ = c.WriteJSON(map[string]any{"id": 1, "method": "Page.setLifecycleEventsEnabled", "params": map[string]any{"enabled": true}})
	_ = readReply(t, c, 1)
	_ = c.WriteJSON(map[string]any{"id": 2, "method": "Page.navigate", "params": map[string]any{"url": pageServer.URL}})
	attached := map[string]bool{}
	navigated := map[string]string{}
	events := map[string]map[string]bool{}
	acknowledged, topLoaded := false, false
	for !acknowledged || !topLoaded {
		var raw map[string]any
		if err := c.ReadJSON(&raw); err != nil {
			t.Fatal(err)
		}
		if raw["id"] == float64(2) {
			acknowledged = true
			continue
		}
		p, _ := raw["params"].(map[string]any)
		switch raw["method"] {
		case "Page.frameAttached":
			attached[p["frameId"].(string)] = true
		case "Page.frameNavigated":
			f := p["frame"].(map[string]any)
			if f["parentId"] == s.Page.Top.ID {
				if f["url"] != "about:blank" {
					t.Fatalf("blank URL: %v", f)
				}
				navigated[f["id"].(string)] = f["loaderId"].(string)
			}
		case "Page.lifecycleEvent":
			id, name := p["frameId"].(string), p["name"].(string)
			if events[id] == nil {
				events[id] = map[string]bool{}
			}
			events[id][name] = true
			if name == "load" && id == s.Page.Top.ID {
				topLoaded = true
			}
		}
	}
	if len(attached) != 3 {
		t.Fatalf("attached=%v", attached)
	}
	for id := range attached {
		if navigated[id] == "" || !events[id]["DOMContentLoaded"] || !events[id]["load"] {
			t.Fatalf("child %s prevents recursive load completion: navigation=%q lifecycle=%v", id, navigated[id], events[id])
		}
	}
	_ = c.WriteJSON(map[string]any{"id": 3, "method": "Runtime.evaluate", "params": map[string]any{"expression": "JSON.stringify(blankOrder)"}})
	reply := readReply(t, c, 3)
	value := reply["result"].(map[string]any)["result"].(map[string]any)["value"]
	if value != `["owner","after:complete"]` {
		t.Fatalf("Chrome 152 blank insertion ordering: %v", value)
	}
}
