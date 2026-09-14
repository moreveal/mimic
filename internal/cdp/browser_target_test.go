package cdp

import (
	"encoding/base64"
	"testing"
	"time"
)

// Frozen Chrome 152 exposes its attached browser target through discovery,
// not getTargets, and permits an explicit session attached to that target.
func TestBrowserTargetDiscoveryAndSession(t *testing.T) {
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	info := wireCall(t, c, 1, "Target.getTargetInfo", nil)["targetInfo"].(map[string]any)
	id := info["targetId"].(string)
	if id != s.browserID || info["type"] != "browser" || info["attached"] != true {
		t.Fatal(info)
	}
	got := wireCall(t, c, 2, "Target.getTargetInfo", map[string]any{"targetId": id})["targetInfo"].(map[string]any)
	if got["targetId"] != id {
		t.Fatal(got)
	}
	listed := wireCall(t, c, 3, "Target.getTargets", map[string]any{"filter": []any{map[string]any{"type": "browser"}}})["targetInfos"].([]any)
	if len(listed) != 0 {
		t.Fatalf("browser must not be enumerated by getTargets: %v", listed)
	}
	if err := c.WriteJSON(map[string]any{"id": 4, "method": "Target.setDiscoverTargets", "params": map[string]any{"discover": true, "filter": []any{map[string]any{}}}}); err != nil {
		t.Fatal(err)
	}
	seen := 0
	for {
		var msg map[string]any
		if err := c.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		if msg["method"] == "Target.targetCreated" {
			target := msg["params"].(map[string]any)["targetInfo"].(map[string]any)
			if target["type"] == "browser" {
				seen++
				if target["targetId"] != id || target["attached"] != true {
					t.Fatal(target)
				}
			}
		}
		if msg["id"] == float64(4) {
			break
		}
	}
	if seen != 1 {
		t.Fatalf("browser discovery count = %d", seen)
	}
	sid := wireCall(t, c, 5, "Target.attachToTarget", map[string]any{"targetId": id, "flatten": true})["sessionId"].(string)
	got = flatCall(t, c, sid, 6, "Target.getTargetInfo", nil)["targetInfo"].(map[string]any)
	if got["targetId"] != id || got["type"] != "browser" {
		t.Fatal(got)
	}
	// Browser sessions must outlive closing the initial Page.
	wireCall(t, c, 7, "Target.closeTarget", map[string]any{"targetId": s.Page.ID})
	flatCall(t, c, sid, 8, "Browser.getVersion", nil)
	wireCall(t, c, 9, "Target.detachFromTarget", map[string]any{"sessionId": sid})
	wireCall(t, c, 10, "Browser.getVersion", nil)
}

func TestTargetFilterAbsentAndEmptyAreDifferent(t *testing.T) {
	_, addr := runningServer(t)
	c := browserConnection(t, addr)
	defaults := wireCall(t, c, 1, "Target.getTargets", nil)["targetInfos"].([]any)
	if len(defaults) != 1 || defaults[0].(map[string]any)["type"] != "page" {
		t.Fatal(defaults)
	}
	empty := wireCall(t, c, 2, "Target.getTargets", map[string]any{"filter": []any{}})["targetInfos"].([]any)
	if len(empty) != 0 {
		t.Fatal(empty)
	}
	if targetMatches([]any{map[string]any{"type": "browser", "exclude": true}, map[string]any{}}, "browser") {
		t.Fatal("first matching exclusion must win")
	}
}

func TestWindowOpenCreatesTargetWithOpener(t *testing.T) {
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	_ = wireCall(t, c, 1, "Target.setDiscoverTargets", map[string]any{"discover": true})
	sid := wireCall(t, c, 2, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	if err := c.WriteJSON(map[string]any{"id": 3, "sessionId": sid, "method": "Runtime.evaluate", "params": map[string]any{"expression": `window.open('data:text/html,<title>popup</title>', '_blank')`}}); err != nil {
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	seenReply, seenTarget := false, false
	for !seenReply || !seenTarget {
		var msg map[string]any
		if err := c.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		seenReply = seenReply || msg["id"] == float64(3)
		if msg["method"] == "Target.targetCreated" {
			info := msg["params"].(map[string]any)["targetInfo"].(map[string]any)
			if info["type"] == "page" && info["targetId"] != s.Page.ID {
				if info["openerId"] != s.Page.ID || info["openerFrameId"] != s.Page.Top.ID || info["canAccessOpener"] != true {
					t.Fatalf("popup opener metadata: %#v", info)
				}
				seenTarget = true
			}
		}
	}
}

func TestRequestEventIncludesLegacyAndEntryPostData(t *testing.T) {
	s, addr := runningServer(t)
	c := browserConnection(t, addr)
	sid := wireCall(t, c, 1, "Target.attachToTarget", map[string]any{"targetId": s.Page.ID, "flatten": true})["sessionId"].(string)
	flatCall(t, c, sid, 2, "Network.enable", nil)
	if err := c.WriteJSON(map[string]any{"id": 3, "sessionId": sid, "method": "Runtime.evaluate", "params": map[string]any{"expression": `fetch('http://127.0.0.1:1/post',{method:'POST',body:'{"value":123}'}).catch(()=>{})`}}); err != nil {
		t.Fatal(err)
	}
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		var msg map[string]any
		if err := c.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		if msg["method"] != "Network.requestWillBeSent" {
			continue
		}
		request := msg["params"].(map[string]any)["request"].(map[string]any)
		if request["url"] != "http://127.0.0.1:1/post" {
			continue
		}
		if request["postData"] != `{"value":123}` || request["hasPostData"] != true {
			t.Fatalf("legacy post data: %#v", request)
		}
		entries := request["postDataEntries"].([]any)
		if len(entries) != 1 || entries[0].(map[string]any)["bytes"] != base64.StdEncoding.EncodeToString([]byte(`{"value":123}`)) {
			t.Fatalf("entry post data: %#v", entries)
		}
		return
	}
}
