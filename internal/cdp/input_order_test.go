package cdp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestPipelinedMouseCommandsPreserveWireOrder(t *testing.T) {
	for _, mode := range []string{"page", "flattened", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			s, addr := runningServer(t)
			_, err := s.Page.Evaluate(context.Background(), `document.body.innerHTML='<button style="position:absolute;left:0;top:0;width:100px;height:100px">open</button>';globalThis.inputOrder='';for(const [type,letter] of [['mousedown','d'],['mouseup','u'],['click','c']])document.addEventListener(type,()=>inputOrder+=letter);`)
			if err != nil {
				t.Fatal(err)
			}
			endpoint := "ws://" + addr + "/devtools/page/" + s.Page.ID
			if mode != "page" {
				endpoint = "ws://" + addr + "/devtools/browser/" + s.browserID
			}
			c, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			sid := ""
			if mode != "page" {
				sid = wireCall(t, c, 1, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": mode == "flattened"})["sessionId"].(string)
			} else {
				wireCall(t, c, 1, "Runtime.evaluate", map[string]any{"expression": "1"})
			}
			const clicks = 32
			id := 2
			// Playwright pipelines move/down/up without waiting for individual replies.
			// Hold the Page while enqueueing so goroutine scheduling cannot hide reordering.
			s.Page.LockCommands()
			for i := 0; i < clicks; i++ {
				for _, kind := range []string{"mouseMoved", "mousePressed", "mouseReleased"} {
					msg := map[string]any{"id": id, "method": "Input.dispatchMouseEvent", "params": map[string]any{"type": kind, "x": 30, "y": 30, "button": "left", "clickCount": 1}}
					if mode == "flattened" {
						msg["sessionId"] = sid
					}
					if mode == "legacy" {
						raw, _ := json.Marshal(msg)
						msg = map[string]any{"id": id, "method": "Target.sendMessageToTarget", "params": map[string]any{"sessionId": sid, "message": string(raw)}}
					}
					if err = c.WriteJSON(msg); err != nil {
						s.Page.UnlockCommands()
						t.Fatal(err)
					}
					id++
				}
			}
			s.Page.UnlockCommands()
			for replies := 0; replies < clicks*3; {
				var reply map[string]any
				if err = c.ReadJSON(&reply); err != nil {
					t.Fatal(err)
				}
				if mode == "legacy" {
					if reply["method"] != "Target.receivedMessageFromTarget" {
						continue
					}
					raw := reply["params"].(map[string]any)["message"].(string)
					if err = json.Unmarshal([]byte(raw), &reply); err != nil {
						t.Fatal(err)
					}
				}
				if reply["id"] == nil {
					continue
				}
				if reply["error"] != nil {
					t.Fatal(reply)
				}
				replies++
			}
			got, err := s.Page.Evaluate(context.Background(), `inputOrder`)
			if err != nil || got != strings.Repeat("duc", clicks) {
				t.Fatalf("pipelined event order: %v err=%v", got, err)
			}
		})
	}
}
