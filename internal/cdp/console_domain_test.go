package cdp

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestConsolePreviewFollowsRuntimeDomain(t *testing.T) {
	s, addr := runningServer(t)
	c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	id := 0
	var previews []map[string]any
	call := func(method string, params map[string]any) map[string]any {
		id++
		if err := c.WriteJSON(map[string]any{"id": id, "method": method, "params": params}); err != nil {
			t.Fatal(err)
		}
		for {
			var reply map[string]any
			if err := c.ReadJSON(&reply); err != nil {
				t.Fatal(err)
			}
			if reply["method"] == "Runtime.consoleAPICalled" {
				args := reply["params"].(map[string]any)["args"].([]any)
				if len(args) > 0 {
					if preview, ok := args[0].(map[string]any)["preview"].(map[string]any); ok {
						previews = append(previews, preview)
					}
				}
			}
			if reply["id"] == float64(id) {
				return reply
			}
		}
	}
	probe := `(()=>{let reads=[];const e=new Error('x');e.toString=()=>{reads.push('text');return 'custom'};Object.defineProperty(e,'name',{get(){reads.push('name');return 'Error'}});console.log(e);return reads.join(',')})()`
	for _, step := range []struct{ method, want string }{{"", "text"}, {"Runtime.enable", "text,name"}, {"Runtime.disable", "text"}, {"Runtime.enable", "text,name"}} {
		if step.method != "" {
			call(step.method, map[string]any{})
		}
		reply := call("Runtime.evaluate", map[string]any{"expression": probe, "returnByValue": true})
		value := reply["result"].(map[string]any)["result"].(map[string]any)["value"]
		if value != step.want {
			t.Fatalf("%s got %v want %s", step.method, value, step.want)
		}
	}
	if len(previews) != 2 {
		t.Fatalf("got %d Error previews", len(previews))
	}
	for _, preview := range previews {
		property := preview["properties"].([]any)[0].(map[string]any)
		if property["name"] != "stack" || property["type"] != "string" || property["value"] == "" {
			t.Fatalf("missing materialized stack: %v", property)
		}
	}
	// Frozen Chrome leaves author stack getters and object stack values untouched.
	reply := call("Runtime.evaluate", map[string]any{"expression": `(()=>{let reads=[];for(const mode of ['getter','object']){const e=new Error('x');e.toString=()=> 'custom';if(mode==='getter')Object.defineProperty(e,'stack',{get(){reads.push('stack');return 'author'}});else e.stack={toString(){reads.push('coerce');return 'object'}};console.log(e)}return reads.join(',')})()`, "returnByValue": true})
	if value := reply["result"].(map[string]any)["result"].(map[string]any)["value"]; value != "" {
		t.Fatalf("author stack hook ran: %v", value)
	}
}
