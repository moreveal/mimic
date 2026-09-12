package cdp

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/moreveal/mimic/internal/trace"
)

func TestInterruptAndCloseChildDOMContentLoadedMicrotask(t *testing.T) {
	for _, closeTarget := range []bool{false, true} {
		t.Run(fmt.Sprintf("close=%v", closeTarget), func(t *testing.T) {
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/":
					fmt.Fprint(w, `<iframe src=/child></iframe>`)
				case "/child":
					fmt.Fprint(w, `<p id=before>before</p><script>
document.addEventListener('DOMContentLoaded',()=>{
  Promise.resolve().then(()=>{
    document.getElementById('before').textContent='running child DCL microtask';
    console.log('child DCL microtask entered');
    while(true){}
  });
});
</script>`)
				default:
					http.NotFound(w, r)
				}
			}))
			defer fixture.Close()
			s, addr := runningServer(t)
			s.SetNavigationTimeout(0)
			entered := make(chan struct{}, 1)
			unsubscribe := s.Page.Trace().Subscribe(func(event trace.Event) {
				// The fixture has one console call, inside the Promise reaction.
				// This proves the precise checkpoint was entered without polling JS.
				if event.Kind == trace.Console {
					select {
					case entered <- struct{}{}:
					default:
					}
				}
			})
			defer unsubscribe()
			c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			defer s.stopPump(s.Page)
			c.SetReadDeadline(time.Now().Add(5 * time.Second))
			c.WriteJSON(map[string]any{"id": 1, "method": "Page.navigate", "params": map[string]any{"url": fixture.URL}})
			readReply(t, c, 1)
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("child DCL Promise reaction did not start")
			}
			c.SetReadDeadline(time.Now().Add(3 * time.Second))
			if closeTarget {
				c.WriteJSON(map[string]any{"id": 2, "method": "Target.closeTarget", "params": map[string]any{"targetId": s.Page.ID}})
			} else {
				c.WriteJSON(map[string]any{"id": 2, "method": "Mimic.captureSnapshot", "params": map[string]any{"interrupt": true}})
			}
			reply := readReply(t, c, 2)
			if reply["error"] != nil {
				t.Fatal(reply)
			}
			if closeTarget {
				if reply["result"].(map[string]any)["success"] != true {
					t.Fatal(reply)
				}
				return
			}
			files := reply["result"].(map[string]any)["files"].(map[string]any)
			found := false
			for _, encoded := range files {
				data, err := base64.StdEncoding.DecodeString(encoded.(string))
				if err == nil && strings.Contains(string(data), "running child DCL microtask") {
					found = true
				}
			}
			if !found {
				t.Fatal("snapshot lost interrupted child microtask state")
			}
			c.WriteJSON(map[string]any{"id": 3, "method": "Runtime.evaluate", "params": map[string]any{
				"expression":    `frames[0].document.getElementById('before').textContent+':'+frames[0].eval('6*7')`,
				"returnByValue": true,
			}})
			recovery := readReply(t, c, 3)
			if recovery["error"] != nil {
				t.Fatal(recovery)
			}
			if result := recovery["result"].(map[string]any)["result"].(map[string]any); result["value"] != "running child DCL microtask:42" {
				t.Fatalf("child runtime did not recover: %#v", recovery)
			}
		})
	}
}
