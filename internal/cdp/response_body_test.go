package cdp

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/gorilla/websocket"
	"github.com/moreveal/mimic/internal/network"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestResponseBodyPreservesBinaryBytes(t *testing.T) {
	for _, body := range [][]byte{{0, 0, 1, 0, 255, 128, 192}, []byte("plain text")} {
		fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(body)
		}))
		s, addr := runningServer(t)
		u, _ := url.Parse(fixture.URL)
		_, err := s.Page.Loader().Load(context.Background(), network.Request{ID: "binary", URL: u, Method: "GET", Initiator: network.Other})
		if err != nil {
			t.Fatal(err)
		}
		c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		c.WriteJSON(map[string]any{"id": 1, "method": "Network.getResponseBody", "params": map[string]any{"requestId": "binary"}})
		reply := readReply(t, c, 1)
		if reply["error"] != nil {
			t.Fatal(reply)
		}
		result := reply["result"].(map[string]any)
		got := []byte(result["body"].(string))
		if result["base64Encoded"] == true {
			got, err = base64.StdEncoding.DecodeString(string(got))
			if err != nil {
				t.Fatal(err)
			}
		}
		if !bytes.Equal(got, body) {
			t.Fatalf("body changed: %x != %x", got, body)
		}
		c.Close()
		fixture.Close()
	}
}
