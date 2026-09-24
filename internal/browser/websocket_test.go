package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWebSocketTextBinaryAndCloseLifecycle(t *testing.T) {
	parallelBrowserTest(t)
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
		return r.Header.Get("Origin") != ""
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetCloseHandler(func(code int, text string) error {
			return conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, text), time.Now().Add(time.Second))
		})
		for {
			messageType, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(messageType, payload); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	p := testPage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	surface, err := p.Evaluate(ctx, `JSON.stringify({constructorLength:WebSocket.length,closeLength:WebSocket.prototype.close.length,sendLength:WebSocket.prototype.send.length,parent:Object.getPrototypeOf(WebSocket.prototype)===EventTarget.prototype,tag:WebSocket.prototype[Symbol.toStringTag],keys:Object.keys(WebSocket.prototype).sort()})`)
	if err != nil {
		t.Fatal(err)
	}
	wantSurface := `{"constructorLength":1,"closeLength":0,"sendLength":1,"parent":true,"tag":"WebSocket","keys":["CLOSED","CLOSING","CONNECTING","OPEN","binaryType","bufferedAmount","close","extensions","onclose","onerror","onmessage","onopen","protocol","readyState","send","url"]}`
	if surface != wantSurface {
		t.Fatalf("WebSocket surface = %v, want %s", surface, wantSurface)
	}
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	value, err := p.Evaluate(ctx, `new Promise((resolve, reject) => {
		const events = [];
		const socket = new WebSocket(`+strconv.Quote(wsURL)+`);
		socket.binaryType = 'arraybuffer';
		socket.onerror = () => reject(Error('websocket error'));
		socket.onopen = () => {
			events.push(['open', socket.readyState]);
			socket.send('hello');
		};
		socket.onmessage = (event) => {
			events.push(['message', typeof event.data, event.data instanceof ArrayBuffer]);
			if (typeof event.data === 'string') {
				socket.send(new Uint8Array([1, 2, 255]));
			} else {
				events.push(['bytes', ...new Uint8Array(event.data)]);
				socket.close(1000, 'done');
			}
		};
		socket.onclose = (event) => resolve(JSON.stringify({
			events,
			code: event.code,
			reason: event.reason,
			clean: event.wasClean,
			state: socket.readyState,
			url: socket.url,
			constants: [WebSocket.CONNECTING, socket.OPEN, socket.CLOSED],
		}));
	})`)
	if err != nil {
		t.Fatalf("%v trace=%v", err, p.Trace().Events())
	}
	want := `{"events":[["open",1],["message","string",false],["message","object",true],["bytes",1,2,255]],"code":1000,"reason":"done","clean":true,"state":3,"url":"` + wsURL + `/","constants":[0,1,3]}`
	if value != want {
		t.Fatalf("WebSocket lifecycle = %v, want %s", value, want)
	}
}
