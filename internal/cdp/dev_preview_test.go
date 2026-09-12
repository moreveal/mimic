package cdp

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/browser"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestDevPreviewRoutesAndWebSocket(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		b, err := browser.NewWithOptions(v8engine.Factory{}, chrome152.New(), browser.Options{DevPreview: enabled})
		if err != nil {
			t.Fatal(err)
		}
		s, err := New(b)
		if err != nil {
			t.Fatal(err)
		}
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		go s.Serve(l)
		defer s.Close(context.Background())
		base := "http://" + l.Addr().String()
		resp, err := http.Get(base + "/debug/preview/")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		want := 200
		if !enabled {
			want = 404
		}
		if resp.StatusCode != want {
			t.Fatalf("route: %d", resp.StatusCode)
		}
		if !enabled {
			resp, err = http.Get(base + "/debug/preview/ws")
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != 404 {
				t.Fatal("disabled WS exposed")
			}
			continue
		}
		conn, _, err := websocket.DefaultDialer.Dial("ws://"+l.Addr().String()+"/debug/preview/ws?target="+s.Page.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var packet map[string]any
		if err := conn.ReadJSON(&packet); err != nil {
			t.Fatal(err)
		}
		if packet["error"] != nil || packet["target"] != s.Page.ID || packet["html"] == nil {
			t.Fatalf("packet: %v", packet)
		}
		conn.Close()
	}
}
