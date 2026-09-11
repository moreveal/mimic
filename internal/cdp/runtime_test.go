package cdp

import (
	"github.com/gorilla/websocket"
	"testing"
	"time"
)

func TestCDPAwaitPromiseAllowsSameSessionResolverAndMultipleWaiters(t *testing.T) {
	s, addr := runningServer(t)
	c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(8 * time.Second))
	_ = c.WriteJSON(map[string]any{"id": 1, "method": "Runtime.enable"})
	readReply(t, c, 1)
	_ = c.WriteJSON(map[string]any{"id": 2, "method": "Runtime.evaluate", "params": map[string]any{"expression": `globalThis.sharedPending=new Promise(resolve=>globalThis.finishPending=resolve);console.log('ready');sharedPending`, "awaitPromise": true}})
	for {
		var event map[string]any
		if err := c.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event["method"] == "Runtime.consoleAPICalled" {
			break
		}
	}
	_ = c.WriteJSON(map[string]any{"id": 3, "method": "Runtime.evaluate", "params": map[string]any{"expression": `console.log('second');sharedPending`, "awaitPromise": true}})
	for {
		var event map[string]any
		if err := c.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event["method"] == "Runtime.consoleAPICalled" {
			break
		}
	}
	_ = c.WriteJSON(map[string]any{"id": 4, "method": "Runtime.evaluate", "params": map[string]any{"expression": `finishPending(23);42`}})
	seen := map[float64]bool{}
	for len(seen) < 3 {
		var reply map[string]any
		if err := c.ReadJSON(&reply); err != nil {
			t.Fatal(err)
		}
		id, ok := reply["id"].(float64)
		if !ok {
			continue
		}
		if reply["error"] != nil {
			t.Fatalf("reply: %#v", reply)
		}
		want := float64(23)
		if id == 4 {
			want = 42
		}
		got := reply["result"].(map[string]any)["result"].(map[string]any)["value"]
		if got != want {
			t.Fatalf("id %v got %#v want %v", id, reply, want)
		}
		seen[id] = true
	}
}

func TestCDPConsoleArgumentsHaveObjectHandles(t *testing.T) {
	s, addr := runningServer(t)
	c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	_ = c.WriteJSON(map[string]any{"id": 1, "method": "Runtime.enable"})
	readReply(t, c, 1)
	_ = c.WriteJSON(map[string]any{"id": 2, "method": "Runtime.evaluate", "params": map[string]any{"expression": `console.log({answer:42})`}})
	var objectID string
	for {
		var event map[string]any
		if err := c.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		if event["method"] == "Runtime.consoleAPICalled" {
			args := event["params"].(map[string]any)["args"].([]any)
			objectID, _ = args[0].(map[string]any)["objectId"].(string)
		}
		if event["id"] == float64(2) {
			break
		}
	}
	if objectID == "" {
		t.Fatal("console object was exported instead of retained")
	}
	_ = c.WriteJSON(map[string]any{"id": 3, "method": "Runtime.callFunctionOn", "params": map[string]any{"objectId": objectID, "functionDeclaration": `function(){return this.answer}`}})
	reply := readReply(t, c, 3)
	if reply["result"].(map[string]any)["result"].(map[string]any)["value"] != float64(42) {
		t.Fatal(reply)
	}
}
