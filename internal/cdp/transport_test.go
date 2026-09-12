package cdp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func browserConnection(t *testing.T, addr string) *websocket.Conn {
	t.Helper()
	response, err := http.Get("http://" + addr + "/json/version")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var version map[string]any
	if err := json.NewDecoder(response.Body).Decode(&version); err != nil {
		t.Fatal(err)
	}
	c, _, err := websocket.DefaultDialer.Dial(version["webSocketDebuggerUrl"].(string), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	return c
}

func wireCall(t *testing.T, c *websocket.Conn, id int, method string, params map[string]any) map[string]any {
	t.Helper()
	if err := c.WriteJSON(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		t.Fatal(err)
	}
	reply := readReply(t, c, float64(id))
	if reply["error"] != nil {
		t.Fatalf("%s: %v", method, reply)
	}
	return reply["result"].(map[string]any)
}
func flatCall(t *testing.T, c *websocket.Conn, sessionID string, id int, method string, params map[string]any) map[string]any {
	t.Helper()
	_ = c.WriteJSON(map[string]any{"id": id, "sessionId": sessionID, "method": method, "params": params})
	for {
		var r map[string]any
		if err := c.ReadJSON(&r); err != nil {
			t.Fatal(err)
		}
		if r["id"] == float64(id) && r["sessionId"] == sessionID {
			if r["error"] != nil {
				t.Fatalf("%s: %v", method, r)
			}
			return r["result"].(map[string]any)
		}
	}
}

func TestFlattenedSessionsKeepIndependentPagesAndDuplicateCommandIDs(t *testing.T) {
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	target := wireCall(t, c, 1, "Target.createTarget", map[string]any{"url": "about:blank"})["targetId"].(string)
	a := wireCall(t, c, 2, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	b := wireCall(t, c, 3, "Target.attachToTarget", map[string]any{"targetId": target, "flatten": true})["sessionId"].(string)
	for i, sid := range []string{a, b} {
		_ = c.WriteJSON(map[string]any{"id": 9, "sessionId": sid, "method": "Runtime.evaluate", "params": map[string]any{"expression": fmt.Sprintf("globalThis.owned=%d;owned", i+1)}})
	}
	seen := map[string]float64{}
	for len(seen) < 2 {
		var r map[string]any
		if err := c.ReadJSON(&r); err != nil {
			t.Fatal(err)
		}
		if r["id"] == float64(9) {
			if r["error"] != nil {
				t.Fatal(r)
			}
			seen[r["sessionId"].(string)] = r["result"].(map[string]any)["result"].(map[string]any)["value"].(float64)
		}
	}
	if seen[a] != 1 || seen[b] != 2 {
		t.Fatalf("crossed sessions: %v", seen)
	}
	got := flatCall(t, c, a, 10, "Runtime.evaluate", map[string]any{"expression": "owned"})
	if got["result"].(map[string]any)["value"] != float64(1) {
		t.Fatal(got)
	}
	wireCall(t, c, 11, "Target.detachFromTarget", map[string]any{"sessionId": a})
	got = flatCall(t, c, b, 12, "Runtime.evaluate", map[string]any{"expression": "owned"})
	if got["result"].(map[string]any)["value"] != float64(2) {
		t.Fatal(got)
	}
}

func TestBrowserContextsOwnPagesAndAreDisposed(t *testing.T) {
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	contextID := wireCall(t, c, 1, "Target.createBrowserContext", map[string]any{})["browserContextId"].(string)
	targetID := wireCall(t, c, 2, "Target.createTarget", map[string]any{"url": "about:blank", "browserContextId": contextID})["targetId"].(string)
	ctx, ok := s.Browser.Context(contextID)
	if !ok {
		t.Fatal("missing context")
	}
	if _, ok := ctx.Page(targetID); !ok {
		t.Fatal("wrong owner")
	}
	info := wireCall(t, c, 3, "Target.getTargetInfo", map[string]any{"targetId": targetID})["targetInfo"].(map[string]any)
	if info["browserContextId"] != contextID {
		t.Fatal(info)
	}
	wireCall(t, c, 4, "Target.disposeBrowserContext", map[string]any{"browserContextId": contextID})
	if _, ok := s.Browser.Context(contextID); ok {
		t.Fatal("context retained")
	}
	if _, ok := s.page(targetID); ok {
		t.Fatal("page retained")
	}
	if _, ok := s.Context.Page(s.Page.ID); !ok {
		t.Fatal("disposed unrelated context")
	}
}

func TestTargetDestroyedRespectsDiscoveryFilter(t *testing.T) {
	for _, tc := range []struct {
		name       string
		filter     []any
		wantPageID bool
	}{
		{name: "default page discovery", wantPageID: true},
		{name: "explicit tab discovery", filter: []any{map[string]any{"type": "tab"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, addr := runningServer(t)
			c := browserConnection(t, addr)
			discoveryParams := map[string]any{"discover": true}
			if tc.filter != nil {
				discoveryParams["filter"] = tc.filter
			}
			wireCall(t, c, 1, "Target.setDiscoverTargets", discoveryParams)
			targetID := wireCall(t, c, 2, "Target.createTarget", map[string]any{"url": "about:blank"})["targetId"].(string)

			if err := c.WriteJSON(map[string]any{"id": 3, "method": "Target.closeTarget", "params": map[string]any{"targetId": targetID}}); err != nil {
				t.Fatal(err)
			}
			var destroyed []string
			for {
				var message map[string]any
				if err := c.ReadJSON(&message); err != nil {
					t.Fatal(err)
				}
				if message["method"] == "Target.targetDestroyed" {
					destroyed = append(destroyed, message["params"].(map[string]any)["targetId"].(string))
				}
				if message["id"] == float64(3) {
					break
				}
			}
			if len(destroyed) != 1 {
				t.Fatalf("targetDestroyed IDs = %v, want exactly one discovered target", destroyed)
			}
			if got := destroyed[0] == targetID; got != tc.wantPageID {
				t.Fatalf("targetDestroyed ID = %q (page ID %q), page match = %v, want %v", destroyed[0], targetID, got, tc.wantPageID)
			}
		})
	}
}

func TestProtocolDiscoveryAndErrorCodes(t *testing.T) {
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	version := wireCall(t, c, 1, "Browser.getVersion", nil)
	if version["product"] != s.Browser.String() {
		t.Fatal(version)
	}
	for i, tc := range []struct {
		method string
		params map[string]any
		code   float64
	}{{"Page.navigate", map[string]any{"url": 5}, -32602}, {"Unknown.command", nil, -32601}} {
		_ = c.WriteJSON(map[string]any{"id": i + 2, "method": tc.method, "params": tc.params})
		r := readReply(t, c, float64(i+2))
		e, ok := r["error"].(map[string]any)
		if !ok || e["code"] != tc.code {
			t.Fatal(r)
		}
	}
	response, err := http.Get("http://" + addr + "/json/protocol")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var schema map[string]any
	if err := json.NewDecoder(response.Body).Decode(&schema); err != nil {
		t.Fatal(err)
	}
	if len(schema["domains"].([]any)) < 50 {
		t.Fatal("incomplete protocol discovery")
	}
}
