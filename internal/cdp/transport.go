package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/moreveal/mimic/internal/browser"
	"github.com/moreveal/mimic/internal/trace"
)

// A connection owns the wire, not a Page. Every attachment has an immutable
// Page binding and its own domain/Runtime state. Flattened and legacy sessions
// share the same registry and write lock; command IDs are scoped to a session.
type connection struct {
	server   *Server
	conn     *websocket.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	writeMu  sync.Mutex
	mu       sync.RWMutex
	root     *session
	sessions map[string]*session
	contexts map[string]bool
	work     sync.WaitGroup
}

func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	isBrowser := r.URL.Path == "/devtools/browser/"+s.browserID
	page := s.Page
	if !isBrowser {
		var ok bool
		page, ok = s.page(strings.TrimPrefix(r.URL.Path, "/devtools/page/"))
		if !ok {
			http.NotFound(w, r)
			return
		}
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &connection{server: s, conn: conn, ctx: ctx, cancel: cancel, sessions: make(map[string]*session), contexts: make(map[string]bool)}
	s.lifecycleMu.Lock()
	if s.closed {
		s.lifecycleMu.Unlock()
		cancel()
		_ = conn.Close()
		return
	}
	s.connections[conn] = cancel
	s.clients[c] = struct{}{}
	s.workers.Add(1)
	s.lifecycleMu.Unlock()
	defer s.workers.Done()
	c.root = c.newSession(page, "", nil, false, isBrowser)
	defer func() {
		cancel()
		_ = conn.Close()
		for _, ss := range c.snapshot() {
			ss.interceptor.Close()
		}
		s.refreshCertificatePolicies()
		c.work.Wait()
		for _, ss := range c.snapshot() {
			ss.cancel()
			ss.unbindPage()
		}
		// disposeOnDetach belongs to the connection that created the Context.
		c.disposeContexts()
		s.lifecycleMu.Lock()
		delete(s.connections, conn)
		delete(s.clients, c)
		s.lifecycleMu.Unlock()
	}()
	for {
		var raw json.RawMessage
		if err := conn.ReadJSON(&raw); err != nil {
			return
		}
		var m message
		if err := json.Unmarshal(raw, &m); err != nil || m.Method == "" {
			c.write(map[string]any{"id": m.ID, "error": map[string]any{"code": -32600, "message": "Invalid request"}})
			continue
		}
		ss := c.root
		if m.SessionID != "" {
			c.mu.RLock()
			ss = c.sessions[m.SessionID]
			c.mu.RUnlock()
			if ss == nil || !ss.flat {
				c.write(map[string]any{"id": m.ID, "sessionId": m.SessionID, "error": map[string]any{"code": -32001, "message": "Session with given id not found."}})
				continue
			}
		}
		c.dispatch(ss, m)
	}
}

func (c *connection) newSession(page *browser.Page, id string, parent *session, flat, browserSession bool, targetTypes ...string) *session {
	ctx, cancel := context.WithCancel(c.ctx)
	s := &session{server: c.server, transport: c, conn: c.conn, ctx: ctx, cancel: cancel, page: page, id: id, parent: parent, flat: flat, browserSession: browserSession, navigationTimeout: c.server.navigationTimeout, domains: make(map[string]bool)}
	s.targetID = page.ID
	s.targetType = "page"
	if browserSession {
		s.targetID = c.server.browserID
		s.targetType = "browser"
	}
	if len(targetTypes) > 0 && targetTypes[0] == "tab" {
		s.targetType = "tab"
		s.targetID = c.server.tabID(page)
	}
	page.LockCommands()
	s.bindPage(page)
	page.UnlockCommands()
	c.mu.Lock()
	c.sessions[id] = s
	c.mu.Unlock()
	return s
}

func (c *connection) snapshot() []*session {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]*session, 0, len(c.sessions))
	for _, s := range c.sessions {
		out = append(out, s)
	}
	return out
}

func (c *connection) dispatch(s *session, m message) {
	c.work.Add(1)
	go func() {
		defer c.work.Done()
		if s.ctx.Err() != nil {
			s.reply(m.ID, nil, fmt.Errorf("Session closed"))
			return
		}
		s.handle(m)
	}()
}

func (c *connection) write(v any) {
	payload, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.WriteMessage(websocket.TextMessage, payload)
}

func (s *session) send(v map[string]any) {
	if s.id != "" {
		if s.flat {
			v["sessionId"] = s.id
		} else {
			raw, err := json.Marshal(v)
			if err != nil {
				return
			}
			s.parent.event("Target.receivedMessageFromTarget", map[string]any{"sessionId": s.id, "message": string(raw), "targetId": s.targetID})
			return
		}
	}
	s.transport.write(v)
}

func (s *session) reply(id int64, result any, err error) {
	if err != nil {
		code := -32000
		// Generated validation errors retain Chrome's protocol error class.
		if e, ok := err.(interface{ ProtocolCode() int }); ok {
			code = e.ProtocolCode()
		}
		payload := map[string]any{"code": code, "message": err.Error()}
		if e, ok := err.(interface{ ProtocolData() any }); ok {
			if data := e.ProtocolData(); data != nil {
				payload["data"] = data
			}
		}
		s.send(map[string]any{"id": id, "error": payload})
		return
	}
	if result == nil {
		result = map[string]any{}
	}
	s.send(map[string]any{"id": id, "result": result})
}

func (s *session) event(method string, params any) {
	if s.ctx.Err() != nil {
		return
	}
	domain := strings.SplitN(method, ".", 2)[0]
	switch domain {
	case "Page", "Runtime", "Network", "DOM", "Log", "Performance", "Audits", "WebMCP":
		if !s.domainEnabled(domain) {
			return
		}
	}
	if method == "Page.lifecycleEvent" {
		s.stateMu.RLock()
		enabled := s.lifecycleEvents
		s.stateMu.RUnlock()
		if !enabled {
			return
		}
	}
	s.page.Trace().Add(trace.CDP, "event", map[string]any{"method": method, "params": params, "sessionId": s.id})
	s.send(map[string]any{"method": method, "params": params})
}

func (s *session) rootEvent(method string, params any) { s.event(method, params) }

func (s *session) setDomain(domain string, enabled bool) {
	s.stateMu.Lock()
	s.domains[domain] = enabled
	s.stateMu.Unlock()
}
func (s *session) domainEnabled(domain string) bool {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.domains[domain]
}
