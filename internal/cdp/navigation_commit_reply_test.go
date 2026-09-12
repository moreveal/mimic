package cdp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNavigateReplyWaitsForCommitWithoutLockingPage(t *testing.T) {
	entered, mainRelease, scriptRelease := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var mainOnce, scriptOnce sync.Once
	releaseMain := func() { mainOnce.Do(func() { close(mainRelease) }) }
	releaseScript := func() { scriptOnce.Do(func() { close(scriptRelease) }) }
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			close(entered)
			select {
			case <-mainRelease:
			case <-r.Context().Done():
				return
			}
			fmt.Fprint(w, `<title>committed</title><script src=/held.js></script>`)
		case "/held.js":
			select {
			case <-scriptRelease:
			case <-r.Context().Done():
				return
			}
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `globalThis.scriptLoaded=true`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer fixture.Close()
	defer releaseMain()
	defer releaseScript()
	s, addr := runningServer(t)
	c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	c.WriteJSON(map[string]any{"id": 1, "method": "Page.navigate", "params": map[string]any{"url": fixture.URL}})
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("main response barrier was not reached")
	}
	c.WriteJSON(map[string]any{"id": 2, "method": "Runtime.evaluate", "params": map[string]any{"expression": `globalThis.oldMarker={value:42};location.href+':'+oldMarker.value`}})
	for {
		var reply map[string]any
		if err := c.ReadJSON(&reply); err != nil {
			t.Fatal(err)
		}
		if reply["id"] == float64(1) {
			t.Fatal("navigation acknowledged before commit", reply)
		}
		if reply["id"] != float64(2) {
			continue
		}
		if reply["error"] != nil || reply["result"].(map[string]any)["result"].(map[string]any)["value"] != "about:blank:42" {
			t.Fatal(reply)
		}
		break
	}
	releaseMain()
	reply := readReply(t, c, 1)
	if reply["error"] != nil || reply["result"].(map[string]any)["errorText"] != nil {
		t.Fatal(reply)
	}
	if reply["result"].(map[string]any)["loaderId"] != s.Page.LoaderID() {
		t.Fatal("reply belongs to another navigation", reply)
	}
	c.WriteJSON(map[string]any{"id": 3, "method": "Runtime.evaluate", "params": map[string]any{"expression": `document.title==='committed'&&document.readyState!=='complete'&&typeof scriptLoaded==='undefined'&&typeof oldMarker==='undefined'`}})
	if reply := readReply(t, c, 3); reply["error"] != nil || reply["result"].(map[string]any)["result"].(map[string]any)["value"] != true {
		t.Fatal("commit reply waited for load or lost document isolation", reply)
	}
}

func TestPendingNavigateReplyTerminates(t *testing.T) {
	for _, action := range []string{"stop", "replace", "close", "timeout", "failure"} {
		t.Run(action, func(t *testing.T) {
			entered := make(chan struct{})
			var once sync.Once
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/new" {
					fmt.Fprint(w, `<body>replacement</body>`)
					return
				}
				if r.URL.Path != "/" {
					http.NotFound(w, r)
					return
				}
				once.Do(func() { close(entered) })
				if action == "failure" {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						conn.Close()
					}
					return
				}
				<-r.Context().Done()
			}))
			defer fixture.Close()
			s, addr := runningServer(t)
			if action == "timeout" {
				s.SetNavigationTimeout(100 * time.Millisecond)
			}
			c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			c.SetReadDeadline(time.Now().Add(5 * time.Second))
			c.WriteJSON(map[string]any{"id": 1, "method": "Page.navigate", "params": map[string]any{"url": fixture.URL}})
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("request did not start")
			}
			want := 1
			switch action {
			case "stop":
				c.WriteJSON(map[string]any{"id": 2, "method": "Page.stopLoading"})
				want++
			case "replace":
				c.WriteJSON(map[string]any{"id": 2, "method": "Page.navigate", "params": map[string]any{"url": fixture.URL + "/new"}})
				want++
			case "close":
				c.WriteJSON(map[string]any{"id": 2, "method": "Target.closeTarget", "params": map[string]any{"targetId": s.Page.ID}})
				want++
			}
			replies := make(map[float64]map[string]any)
			for len(replies) < want {
				var reply map[string]any
				if err := c.ReadJSON(&reply); err != nil {
					t.Fatal(err)
				}
				if id, ok := reply["id"].(float64); ok {
					if replies[id] != nil {
						t.Fatal("duplicate navigation reply", reply)
					}
					replies[id] = reply
				}
			}
			failed := replies[1]
			if failed["error"] == nil {
				errorText, _ := failed["result"].(map[string]any)["errorText"].(string)
				if errorText == "" {
					t.Fatal("uncommitted navigation reported success", failed)
				}
				if (action == "stop" || action == "replace") && errorText != "net::ERR_ABORTED" {
					t.Fatal(failed)
				}
				if action == "timeout" && errorText != "net::ERR_TIMED_OUT" {
					t.Fatal(failed)
				}
			}
			if want == 2 && replies[2]["error"] != nil {
				t.Fatal(replies[2])
			}
			if action == "replace" && replies[2]["result"].(map[string]any)["errorText"] != nil {
				t.Fatal("replacement did not commit", replies[2])
			}
			if action == "close" && replies[2]["result"].(map[string]any)["success"] != true {
				t.Fatal("target did not close", replies[2])
			}
		})
	}
}
