package cdp

import (
	"bytes"
	"context"
	_ "embed"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed dev_preview.html
var previewHTML []byte

//go:embed dev_preview_dom.js
var previewDOM []byte

func (s *Server) registerPreview(mux *http.ServeMux) {
	mux.HandleFunc("/debug/preview/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/debug/preview/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline' http: https:; img-src http: https: data:; font-src http: https: data:; media-src http: https: data:; connect-src 'self'; frame-src 'self' about:; object-src 'none'; form-action 'none'")
		_, _ = w.Write(bytes.Replace(previewHTML, []byte("/* preview_dom */"), previewDOM, 1))
	})
	mux.HandleFunc("/debug/preview/ws", s.previewWS)
}

func (s *Server) previewWS(w http.ResponseWriter, r *http.Request) {
	page, ok := s.page(r.URL.Query().Get("target"))
	if !ok {
		http.Error(w, "unknown target", http.StatusNotFound)
		return
	}
	// Default upgrader enforces same-origin requests (unlike the CDP endpoint).
	u := websocket.Upgrader{}
	conn, err := u.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(r.Context())
	s.lifecycleMu.Lock()
	if s.closed {
		s.lifecycleMu.Unlock()
		cancel()
		conn.Close()
		return
	}
	s.connections[conn] = cancel
	s.workers.Add(1)
	s.lifecycleMu.Unlock()
	defer s.workers.Done()
	defer func() {
		cancel()
		conn.Close()
		s.lifecycleMu.Lock()
		delete(s.connections, conn)
		s.lifecycleMu.Unlock()
	}()
	page.LockCommands()
	sub, err := page.SubscribePreview()
	page.UnlockCommands()
	if err != nil {
		return
	}
	defer func() { page.LockCommands(); page.UnsubscribePreview(sub); page.UnlockCommands() }()
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer cancel()
		conn.SetReadLimit(1024)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	defer func() { conn.Close(); <-done }()
	for {
		select {
		case wire := <-sub.Updates:
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if conn.WriteMessage(websocket.TextMessage, wire) != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
