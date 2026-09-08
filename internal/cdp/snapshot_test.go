package cdp

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestCaptureSnapshotCDP(t *testing.T) {
	_, addr := runningServer(t)
	response, err := http.Get("http://" + addr + "/json/list")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var targets []discoveryTarget
	if err = json.NewDecoder(response.Body).Decode(&targets); err != nil {
		t.Fatal(err)
	}
	c, _, err := websocket.DefaultDialer.Dial(targets[0].WebSocketURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err = c.WriteJSON(map[string]any{"id": 1, "method": "Mimic.captureSnapshot"}); err != nil {
		t.Fatal(err)
	}
	reply := readReply(t, c, 1)
	if reply["error"] != nil {
		t.Fatal(reply)
	}
	files := reply["result"].(map[string]any)["files"].(map[string]any)
	body, err := base64.StdEncoding.DecodeString(files["index.html"].(string))
	if err != nil || !strings.Contains(string(body), "<!doctype html>") {
		t.Fatalf("invalid document: %s, %v", body, err)
	}
}
