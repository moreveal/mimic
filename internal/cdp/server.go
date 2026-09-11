package cdp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/moreveal/mimic/internal/browser"
	"github.com/moreveal/mimic/internal/dom"
	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/trace"
)

type Server struct {
	lifecycleMu       sync.Mutex
	connections       map[*websocket.Conn]context.CancelFunc
	pumps             map[*browser.Page]context.CancelFunc
	executions        map[*browser.Page]context.CancelFunc
	pausedPumps       map[*browser.Page]int
	closed            bool
	workers           sync.WaitGroup
	Browser           *browser.Browser
	Context           *browser.Context
	Page              *browser.Page
	http              *http.Server
	listener          net.Listener
	navigationTimeout time.Duration
}

func New(b *browser.Browser) (*Server, error) {
	c := b.NewContext()
	p, err := c.NewPage()
	if err != nil {
		return nil, err
	}
	return &Server{Browser: b, Context: c, Page: p, connections: make(map[*websocket.Conn]context.CancelFunc), pumps: make(map[*browser.Page]context.CancelFunc), executions: make(map[*browser.Page]context.CancelFunc)}, nil
}
func (s *Server) SetNavigationTimeout(timeout time.Duration) {
	if timeout >= 0 {
		s.navigationTimeout = timeout
	}
}
func (s *Server) Serve(listener net.Listener) error {
	s.listener = listener
	mux := http.NewServeMux()
	mux.HandleFunc("/json/version", s.version)
	mux.HandleFunc("/json", s.list)
	mux.HandleFunc("/json/list", s.list)
	mux.HandleFunc("/devtools/page/", s.ws)
	s.lifecycleMu.Lock()
	if s.closed {
		s.lifecycleMu.Unlock()
		return http.ErrServerClosed
	}
	s.http = &http.Server{Handler: mux}
	server := s.http
	s.lifecycleMu.Unlock()
	return server.Serve(listener)
}
func (s *Server) Close(ctx context.Context) error {
	s.lifecycleMu.Lock()
	s.closed = true
	server := s.http
	for _, cancel := range s.pumps {
		cancel()
	}
	for _, cancel := range s.executions {
		cancel()
	}
	for conn, cancel := range s.connections {
		cancel()
		_ = conn.Close()
	}
	s.lifecycleMu.Unlock()
	var err error
	if server != nil {
		err = server.Shutdown(ctx)
	}
	s.Context.Cancel()
	s.workers.Wait()
	_ = s.Context.Close()
	return err
}

func (s *Server) base(r *http.Request) string {
	return "ws://" + r.Host + "/devtools/page/" + s.Page.ID
}
func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"Browser": s.Browser.String(), "Protocol-Version": "1.3", "User-Agent": s.Page.Environment().Navigator().UserAgent, "V8-Version": "virtual", "webSocketDebuggerUrl": s.base(r)})
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	targets := []any{}
	for _, page := range s.Context.Pages() {
		targets = append(targets, map[string]any{"id": page.ID, "type": "page", "title": page.Title(), "url": page.URL(), "webSocketDebuggerUrl": "ws://" + r.Host + "/devtools/page/" + page.ID})
	}
	writeJSON(w, targets)
}
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

type message struct {
	ID     int64           `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
type session struct {
	commandMu         sync.Mutex // protects binding changes against commands and asynchronous navigation
	ctx               context.Context
	work              sync.WaitGroup
	server            *Server
	conn              *websocket.Conn
	page              *browser.Page
	bindMu            sync.RWMutex
	writeMu           sync.Mutex
	routeMu           sync.RWMutex
	activeSession     string
	interceptor       *ControlInterceptor
	unsub             func()
	removeInterceptor func()
	navigationTimeout time.Duration
	navigationMu      sync.Mutex
	navigationCancel  context.CancelFunc
	navigationID      uint64
	contextMu         sync.Mutex
	nextContextID     int64
	contextByFrame    map[string]int64
	frameByContext    map[int64]string
	realmByFrame      map[string]string
}

func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	targetID := strings.TrimPrefix(r.URL.Path, "/devtools/page/")
	page, ok := s.Context.Page(targetID)
	if !ok || r.URL.Path != "/devtools/page/"+targetID {
		http.NotFound(w, r)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.lifecycleMu.Lock()
	if s.closed {
		s.lifecycleMu.Unlock()
		cancel()
		_ = conn.Close()
		return
	}
	s.connections[conn] = cancel
	s.workers.Add(1)
	s.lifecycleMu.Unlock()
	defer s.workers.Done()
	ss := &session{server: s, conn: conn, ctx: ctx, navigationTimeout: s.navigationTimeout}
	page.LockCommands()
	ss.bindPage(page)
	page.UnlockCommands()
	defer func() {
		cancel()
		_ = conn.Close()
		ss.work.Wait()
		ss.unbindPage()
		s.lifecycleMu.Lock()
		delete(s.connections, conn)
		s.lifecycleMu.Unlock()
	}()
	for {
		var m message
		if err := conn.ReadJSON(&m); err != nil {
			return
		}
		ss.work.Add(1)
		go func() { defer ss.work.Done(); ss.handle(m) }()
	}
}

func (s *session) bindPage(page *browser.Page) {
	s.bindMu.Lock()
	defer s.bindMu.Unlock()
	if s.unsub != nil {
		s.unsub()
	}
	if s.removeInterceptor != nil {
		s.removeInterceptor()
	}
	if s.interceptor != nil {
		s.interceptor.Close()
	}
	s.page = page
	s.contextMu.Lock()
	s.nextContextID = 0
	s.contextByFrame = map[string]int64{}
	s.frameByContext = map[int64]string{}
	s.realmByFrame = map[string]string{}
	s.contextMu.Unlock()
	s.interceptor = NewControlInterceptor(s.event)
	s.removeInterceptor = page.Loader().Use(s.interceptor)
	s.unsub = page.Trace().Subscribe(s.traceEvent)
	s.server.ensurePump(page)
}
func (s *session) unbindPage() {
	s.bindMu.Lock()
	defer s.bindMu.Unlock()
	if s.unsub != nil {
		s.unsub()
	}
	if s.removeInterceptor != nil {
		s.removeInterceptor()
	}
	if s.interceptor != nil {
		s.interceptor.Close()
	}
}

// A Page owns one clock regardless of how many debugger connections observe it.
// Its pump survives debugger disconnection and ends with the Page or server.
func (s *Server) ensurePump(page *browser.Page) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if s.closed || s.pumps[page] != nil {
		return
	}
	if live, ok := s.Context.Page(page.ID); !ok || live != page {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.pumps[page] = cancel
	s.workers.Add(1)
	go func() { defer s.workers.Done(); s.pumpEventLoop(ctx, page) }()
}
func (s *Server) stopPump(page *browser.Page) {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if cancel := s.executions[page]; cancel != nil {
		cancel()
	}
	if cancel := s.pumps[page]; cancel != nil {
		cancel()
		delete(s.pumps, page)
	}
}
func (s *Server) pumpEventLoop(lifetime context.Context, page *browser.Page) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	last := time.Now()
	for {
		select {
		case <-ticker.C:
			delta := time.Since(last)
			for turn := 0; turn < 32; turn++ {
				page.LockCommands()
				if lifetime.Err() != nil {
					page.UnlockCommands()
					return
				}
				turnContext, cancelTurn := context.WithCancel(lifetime)
				s.lifecycleMu.Lock()
				if s.pausedPumps[page] > 0 {
					s.lifecycleMu.Unlock()
					cancelTurn()
					page.UnlockCommands()
					break
				}
				s.executions[page] = cancelTurn
				s.lifecycleMu.Unlock()
				// One debugger pump turn must not monopolize the Page when an
				// application continuously posts ready timers/network callbacks.
				// Background JavaScript has the Page lifetime, not the unrelated
				// navigation timeout: terminating a valid hydration callback at 30s
				// leaves an otherwise recoverable committed document half-built.
				more, err := page.AdvanceTimeBudget(turnContext, delta, 1)
				delta = 0
				s.lifecycleMu.Lock()
				delete(s.executions, page)
				s.lifecycleMu.Unlock()
				cancelTurn()
				page.UnlockCommands()
				// Exclude time spent executing or waiting for other Page turns.
				last = time.Now()
				if err != nil && lifetime.Err() == nil {
					page.Trace().Add(trace.Error, "scheduler", map[string]any{"error": err.Error(), "during": "CDP event-loop pump"})
				}
				if !more || err != nil {
					break
				}
			}
		case <-lifetime.Done():
			return
		}
	}
}

// Snapshot serialization owns a consistent task boundary. Prevent the pump
// from starting another task while capture waits for that boundary. Explicit
// interrupted captures may cancel the current task; ordinary captures do not.
func (s *Server) pausePump(page *browser.Page, interrupt bool) func() {
	s.lifecycleMu.Lock()
	if s.pausedPumps == nil {
		s.pausedPumps = make(map[*browser.Page]int)
	}
	s.pausedPumps[page]++
	if interrupt {
		if cancel := s.executions[page]; cancel != nil {
			cancel()
		}
	}
	s.lifecycleMu.Unlock()
	return func() {
		s.lifecycleMu.Lock()
		s.pausedPumps[page]--
		if s.pausedPumps[page] == 0 {
			delete(s.pausedPumps, page)
		}
		s.lifecycleMu.Unlock()
	}
}

func (s *Server) cancelExecution(page *browser.Page) bool {
	s.lifecycleMu.Lock()
	cancel := s.executions[page]
	s.lifecycleMu.Unlock()
	if cancel == nil {
		return false
	}
	cancel()
	return true
}
func (s *session) send(v any) {
	payload, err := json.Marshal(v)
	if err != nil {
		return
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_ = s.conn.WriteMessage(websocket.TextMessage, payload)
}
func (s *session) reply(id int64, result any, err error) {
	s.replyRouted(id, result, err, "")
}
func (s *session) replyRouted(id int64, result any, err error, route string) {
	var response map[string]any
	if err != nil {
		response = map[string]any{"id": id, "error": map[string]any{"code": -32000, "message": err.Error()}}
	} else {
		response = map[string]any{"id": id, "result": result}
	}
	if route == "" {
		s.send(response)
		return
	}
	raw, _ := json.Marshal(response)
	s.rootEvent("Target.receivedMessageFromTarget", map[string]any{"sessionId": route, "message": string(raw), "targetId": s.page.ID})
}
func (s *session) event(method string, params any) {
	s.routeMu.RLock()
	route := s.activeSession
	s.routeMu.RUnlock()
	if route == "" {
		s.rootEvent(method, params)
		return
	}
	raw, _ := json.Marshal(map[string]any{"method": method, "params": params})
	s.rootEvent("Target.receivedMessageFromTarget", map[string]any{"sessionId": route, "message": string(raw), "targetId": s.page.ID})
}
func (s *session) rootEvent(method string, params any) {
	s.page.Trace().Add(trace.CDP, "event", map[string]any{"method": method, "params": params})
	s.send(map[string]any{"method": method, "params": params})
}
func (s *session) traceEvent(e trace.Event) {
	switch e.Kind {
	case trace.Console:
		contextID, ok := s.contextForRealm(stringValue(e.Data["realm"]))
		if !ok {
			// A queued callback can outlive a navigated or detached realm. Chrome
			// does not expose console events for an execution context after sending
			// its destruction, and clients such as Pyppeteer reject such events.
			return
		}
		s.event("Runtime.consoleAPICalled", map[string]any{"type": e.Name, "args": remoteObjects(e.Data["args"]), "executionContextId": contextID, "timestamp": float64(e.Time.UnixMilli())})
	case trace.Exception:
		s.event("Runtime.exceptionThrown", map[string]any{"timestamp": float64(e.Time.UnixMilli()), "exceptionDetails": e.Data})
	case trace.Lifecycle:
		frameID := stringValue(e.Data["frameId"])
		if frameID == "" {
			frameID = s.page.Top.ID
		}
		timestamp := float64(e.Time.UnixMilli()) / 1000
		switch e.Name {
		case "frameAttached":
			parentFrameID := stringValue(e.Data["parentFrameId"])
			s.event("Page.frameAttached", map[string]any{"frameId": frameID, "parentFrameId": parentFrameID})
			s.event("Page.lifecycleEvent", map[string]any{"name": "init", "timestamp": timestamp, "frameId": frameID, "loaderId": s.frameLoaderID(frameID)})
			if initial, _ := e.Data["initialContext"].(bool); initial {
				s.event("Runtime.executionContextCreated", map[string]any{"context": s.ensureContext(frameID, stringValue(e.Data["realm"]), "about:blank")})
			}
		case "frameNavigated":
			isTop := frameID == s.page.Top.ID
			if isTop {
				s.event("Runtime.executionContextsCleared", map[string]any{})
				s.clearContexts()
			} else {
				s.destroyFrameContext(frameID)
			}
			loaderID := stringValue(e.Data["loaderId"])
			if loaderID == "" {
				loaderID = s.frameLoaderID(frameID)
			}
			s.event("Page.lifecycleEvent", map[string]any{"name": "init", "timestamp": timestamp, "frameId": frameID, "loaderId": loaderID})
			s.event("Page.frameNavigated", map[string]any{"frame": s.framePayloadByID(frameID, stringValue(e.Data["url"]), loaderID, stringValue(e.Data["parentFrameId"]))})
			s.event("Runtime.executionContextCreated", map[string]any{"context": s.ensureContext(frameID, stringValue(e.Data["realm"]), stringValue(e.Data["url"]))})
			if isTop {
				s.rootEvent("Target.targetInfoChanged", map[string]any{"targetInfo": targetInfo(s.page, true)})
			}
		case "frameDetached":
			s.event("Page.frameDetached", map[string]any{"frameId": frameID, "reason": "remove"})
			s.destroyFrameContext(frameID)
		case "DOMContentLoaded", "load":
			if frameID == s.page.Top.ID && e.Name == "DOMContentLoaded" {
				s.event("Page.domContentEventFired", map[string]any{"timestamp": timestamp})
			}
			if frameID == s.page.Top.ID && e.Name == "load" {
				s.event("Page.loadEventFired", map[string]any{"timestamp": timestamp})
			}
			s.event("Page.lifecycleEvent", map[string]any{"name": e.Name, "timestamp": timestamp, "frameId": frameID, "loaderId": s.frameLoaderID(frameID)})
		}
	case trace.Network:
		frameID := stringValue(e.Data["context"])
		if frameID == "" {
			frameID = s.page.Top.ID
		}
		loaderID := s.frameLoaderID(frameID)
		if e.Data["initiator"] == network.Iframe {
			loaderID = stringValue(e.Data["id"])
		}
		if e.Name == "request" {
			s.event("Network.requestWillBeSent", map[string]any{"requestId": e.Data["id"], "loaderId": loaderID, "documentURL": e.Data["url"], "request": map[string]any{"url": e.Data["url"], "method": e.Data["method"], "headers": e.Data["headers"], "postData": e.Data["postData"]}, "timestamp": float64(e.Time.UnixMilli()) / 1000, "wallTime": float64(e.Time.Unix()), "initiator": map[string]any{"type": "other"}, "type": resourceTypeFromTrace(e.Data["initiator"]), "frameId": frameID})
		} else if e.Name == "response" {
			s.event("Network.responseReceived", map[string]any{"requestId": e.Data["id"], "loaderId": loaderID, "timestamp": float64(e.Time.UnixMilli()) / 1000, "type": resourceTypeFromTrace(e.Data["initiator"]), "response": map[string]any{"url": e.Data["url"], "status": e.Data["status"], "statusText": "", "headers": e.Data["headers"], "mimeType": e.Data["mimeType"], "connectionReused": e.Data["connectionReused"], "connectionId": e.Data["connectionId"], "protocol": cdpProtocol(e.Data["protocol"]), "timing": cdpResourceTiming(e.Data["transportTiming"]), "encodedDataLength": e.Data["encodedDataLength"], "securityState": "unknown"}, "frameId": frameID})
			s.event("Network.loadingFinished", map[string]any{"requestId": e.Data["id"], "timestamp": float64(e.Time.UnixMilli()) / 1000, "encodedDataLength": e.Data["encodedDataLength"]})
		} else if e.Name == "failed" {
			s.event("Network.loadingFailed", map[string]any{"requestId": e.Data["id"], "timestamp": float64(e.Time.UnixMilli()) / 1000, "type": resourceTypeFromTrace(e.Data["initiator"]), "errorText": e.Data["error"], "canceled": false})
		}
	}
}
func (s *session) handle(m message) {
	s.handleRouted(m, "")
}
func (s *session) handleRouted(m message, route string) {
	s.page.Trace().Add(trace.CDP, "method", map[string]any{"method": m.Method, "sessionId": route})
	var p map[string]any
	_ = json.Unmarshal(m.Params, &p)
	if m.Method == "Target.sendMessageToTarget" {
		s.reply(m.ID, map[string]any{}, nil)
		var inner message
		if json.Unmarshal([]byte(stringValue(p["message"])), &inner) != nil {
			return
		}
		s.handleRouted(inner, stringValue(p["sessionId"]))
		return
	}
	control := m.Method == "Fetch.continueRequest" || m.Method == "Fetch.continueResponse" || m.Method == "Fetch.failRequest" || m.Method == "Fetch.fulfillRequest" || m.Method == "Network.continueInterceptedRequest" || m.Method == "Mimic.getTrace" || m.Method == "Mimic.getStatus" || m.Method == "Mimic.getDiagnostics" || m.Method == "Mimic.cancelExecution" || m.Method == "Page.stopLoading" || m.Method == "Target.closeTarget"
	if !control {
		s.commandMu.Lock()
		defer s.commandMu.Unlock()
		// Target commands operate on the registry or explicitly lock their target.
		// Never hold the control Page while bootstrapping an independent Page.
		if !strings.HasPrefix(m.Method, "Target.") {
			page := s.page
			if m.Method == "Mimic.captureSnapshot" {
				interrupt, _ := p["interrupt"].(bool)
				resume := s.server.pausePump(page, interrupt)
				defer resume()
			}
			page.LockCommands()
			defer page.UnlockCommands()
		}
	}
	var result any = map[string]any{}
	var err error
	switch m.Method {
	case "Page.enable", "Network.enable", "DOM.enable", "Log.enable", "Performance.enable", "Storage.enable", "Security.enable", "Security.setIgnoreCertificateErrors", "Target.setAutoAttach", "Emulation.setTouchEmulationEnabled":
	case "Page.setLifecycleEventsEnabled":
		enabled, _ := p["enabled"].(bool)
		if enabled {
			timestamp := float64(time.Now().UnixMilli()) / 1000
			frameID, loaderID := s.page.Top.ID, s.page.LoaderID()
			s.event("Page.lifecycleEvent", map[string]any{"name": "commit", "timestamp": timestamp, "frameId": frameID, "loaderId": loaderID})
			if s.page.Top.ReadyState() != "loading" {
				s.event("Page.lifecycleEvent", map[string]any{"name": "DOMContentLoaded", "timestamp": timestamp, "frameId": frameID, "loaderId": loaderID})
			}
			if s.page.Top.ReadyState() == "complete" {
				for _, name := range []string{"load", "networkAlmostIdle", "networkIdle"} {
					s.event("Page.lifecycleEvent", map[string]any{"name": name, "timestamp": timestamp, "frameId": frameID, "loaderId": loaderID})
				}
			}
		}
	case "Runtime.enable":
		s.event("Runtime.executionContextCreated", map[string]any{"context": s.ensureContext(s.page.Top.ID, s.page.Top.RealmID(), s.page.URL())})
	case "Target.getBrowserContexts":
		result = map[string]any{"browserContextIds": []string{}}
	case "Target.setDiscoverTargets":
		for _, page := range s.server.Context.Pages() {
			s.rootEvent("Target.targetCreated", map[string]any{"targetInfo": targetInfo(page, page == s.page)})
		}
	case "Target.createTarget":
		page, createErr := s.server.Context.NewPage()
		if createErr != nil {
			err = createErr
			break
		}
		result = map[string]any{"targetId": page.ID}
		s.rootEvent("Target.targetCreated", map[string]any{"targetInfo": targetInfo(page, false)})
	case "Target.attachToTarget":
		targetID := stringValue(p["targetId"])
		page, ok := s.server.Context.Page(targetID)
		if !ok {
			err = fmt.Errorf("unknown target %s", targetID)
			break
		}
		if page != s.page {
			page.LockCommands()
			s.bindPage(page)
			page.UnlockCommands()
		}
		sid := uuid.NewString()
		s.routeMu.Lock()
		s.activeSession = sid
		s.routeMu.Unlock()
		result = map[string]any{"sessionId": sid}
	case "Target.getTargets":
		infos := []any{}
		for _, page := range s.server.Context.Pages() {
			infos = append(infos, targetInfo(page, page == s.page))
		}
		result = map[string]any{"targetInfos": infos}
	case "Target.closeTarget":
		targetID := stringValue(p["targetId"])
		if targetID == "" {
			targetID = s.page.ID
		}
		success := false
		if page, ok := s.server.Context.Page(targetID); ok {
			// Cancel before waiting for the command lock: a running callback
			// can otherwise prevent its own teardown forever.
			s.server.stopPump(page)
			page.LockCommands()
			success = s.server.Context.ClosePage(targetID)
			page.UnlockCommands()
		}
		result = map[string]any{"success": success}
		s.rootEvent("Target.targetDestroyed", map[string]any{"targetId": targetID})
	case "Page.getFrameTree":
		result = map[string]any{"frameTree": s.frameTree(s.page.Top)}
	case "Emulation.setDeviceMetricsOverride":
		err = s.page.SetViewport(intValue(p["width"], 800), intValue(p["height"], 600))
	case "Runtime.evaluate":
		var v any
		v, err = s.evaluateInContext(s.ctx, int64(intValue(p["contextId"], 0)), stringValue(p["expression"]))
		result = map[string]any{"result": remoteObject(v)}
	case "Runtime.callFunctionOn":
		declaration := strings.Split(stringValue(p["functionDeclaration"]), "//# sourceURL=")[0]
		values := []any{}
		if arguments, ok := p["arguments"].([]any); ok {
			for _, item := range arguments {
				if argument, ok := item.(map[string]any); ok {
					values = append(values, argument["value"])
				}
			}
		}
		encoded, _ := json.Marshal(values)
		var v any
		v, err = s.evaluateInContext(s.ctx, int64(intValue(p["executionContextId"], 0)), "("+declaration+")(..."+string(encoded)+")")
		result = map[string]any{"result": remoteObject(v)}
	case "Runtime.releaseObject", "Runtime.releaseObjectGroup":
	case "Page.navigate":
		navigationURL := stringValue(p["url"])
		loaderID := s.page.ReserveNavigation()
		result = map[string]any{"frameId": s.page.Top.ID, "loaderId": loaderID}
		page := s.page
		navigationCtx, navigationCancel := context.WithCancel(s.ctx)
		if s.navigationTimeout > 0 {
			navigationCancel()
			navigationCtx, navigationCancel = context.WithTimeout(s.ctx, s.navigationTimeout)
		}
		s.navigationMu.Lock()
		if s.navigationCancel != nil {
			s.navigationCancel()
		}
		s.navigationID++
		navigationID := s.navigationID
		s.navigationCancel = navigationCancel
		s.navigationMu.Unlock()
		s.work.Add(1)
		go func() {
			defer s.work.Done()
			defer navigationCancel()
			defer func() {
				s.navigationMu.Lock()
				if s.navigationID == navigationID {
					s.navigationCancel = nil
				}
				s.navigationMu.Unlock()
			}()
			s.commandMu.Lock()
			defer s.commandMu.Unlock()
			page.LockCommands()
			defer page.UnlockCommands()
			if live, ok := s.server.Context.Page(page.ID); !ok || live != page {
				return
			}
			s.server.lifecycleMu.Lock()
			if s.server.pumps[page] == nil {
				s.server.lifecycleMu.Unlock()
				return
			}
			s.server.executions[page] = navigationCancel
			s.server.lifecycleMu.Unlock()
			defer func() {
				s.server.lifecycleMu.Lock()
				delete(s.server.executions, page)
				s.server.lifecycleMu.Unlock()
			}()
			if navErr := page.NavigateReserved(navigationCtx, navigationURL, loaderID); navErr != nil {
				page.Trace().Add(trace.Error, "navigation", map[string]any{"url": navigationURL, "error": navErr.Error()})
			}
		}()
	case "Page.stopLoading":
		s.navigationMu.Lock()
		if s.navigationCancel != nil {
			s.navigationCancel()
		}
		s.navigationMu.Unlock()
	case "Browser.close":
	case "Page.addScriptToEvaluateOnNewDocument":
		id := s.page.AddInitScript(stringValue(p["source"]))
		result = map[string]any{"identifier": id}
	case "Page.setBypassCSP":
		bypass, _ := p["enabled"].(bool)
		s.page.SetBypassCSP(bypass)
	case "DOM.getDocument":
		var d *dom.Document
		var ok bool
		if d, ok = s.page.Document(); !ok {
			err = fmt.Errorf("no document")
		} else {
			result = map[string]any{"root": cdpNode(d, d.Root(), intValue(p["depth"], 1))}
		}
	case "DOM.querySelector", "DOM.querySelectorAll":
		var ids []int64
		ids, err = s.page.QueryDOM(s.ctx, int64(intValue(p["nodeId"], 0)), stringValue(p["selector"]), m.Method == "DOM.querySelectorAll")
		if err == nil {
			if m.Method == "DOM.querySelectorAll" {
				result = map[string]any{"nodeIds": ids}
			} else {
				var id int64
				if len(ids) > 0 {
					id = ids[0]
				}
				result = map[string]any{"nodeId": id}
			}
		}
	case "DOM.getOuterHTML":
		var d *dom.Document
		var ok bool
		if d, ok = s.page.Document(); !ok {
			err = fmt.Errorf("no document")
		} else {
			id := int64(intValue(p["nodeId"], 0))
			if id == 0 {
				id = int64(intValue(p["backendNodeId"], 0))
			}
			var markup string
			markup, err = d.OuterHTML(id)
			if err == nil {
				result = map[string]any{"outerHTML": markup}
			}
		}
	case "Network.getAllCookies":
		result = map[string]any{"cookies": s.pageCookies()}
	case "Storage.getCookies":
		result = map[string]any{"cookies": s.pageCookies()}
	case "Network.setCookie":
		err = s.setCookie(p)
		if err == nil {
			result = map[string]any{"success": true}
		}
	case "Network.deleteCookies":
		err = s.deleteCookies(p)
	case "Network.clearBrowserCookies":
		s.page.Cookies().Clear()
	case "Network.setCacheDisabled":
		disabled, _ := p["cacheDisabled"].(bool)
		s.page.NetworkSession().SetCacheDisabled(disabled)
	case "Network.clearBrowserCache":
		s.page.NetworkSession().ClearCache()
	case "Network.setExtraHTTPHeaders":
		s.page.NetworkSession().SetExtraHeaders(headerObject(p["headers"]))
	case "Network.emulateNetworkConditions":
		offline, _ := p["offline"].(bool)
		s.page.NetworkSession().SetOffline(offline)
	case "Network.setRequestInterception":
		patterns, _ := p["patterns"].([]any)
		s.interceptor.EnableNetwork(len(patterns) > 0)
	case "Network.continueInterceptedRequest":
		a := interceptAnswer{action: "continue", url: stringValue(p["url"]), method: stringValue(p["method"]), headers: headerObjectOrNil(p["headers"])}
		if post, ok := p["postData"].(string); ok {
			a.body = []byte(post)
		}
		if stringValue(p["errorReason"]) != "" {
			a.action = "fail"
		}
		if raw := stringValue(p["rawResponse"]); raw != "" {
			a, err = parseRawResponse(raw)
		}
		if err == nil {
			err = s.interceptor.Resolve(stringValue(p["interceptionId"]), a)
		}
	case "Network.getResponseBody":
		if response, ok := s.page.Loader().Completed(stringValue(p["requestId"])); ok {
			result = map[string]any{"body": string(response.Body), "base64Encoded": false}
		} else {
			err = fmt.Errorf("unknown request id")
		}
	case "Network.getCookies":
		result = map[string]any{"cookies": s.pageCookies()}
	case "Network.setCookies":
		if list, ok := p["cookies"].([]any); ok {
			for _, item := range list {
				if c, ok := item.(map[string]any); ok {
					if err = s.setCookie(c); err != nil {
						break
					}
				}
			}
		}
	case "Fetch.enable":
		s.interceptor.Enable(true)
	case "Fetch.disable":
		s.interceptor.Enable(false)
	case "Fetch.continueRequest":
		err = s.interceptor.Resolve(stringValue(p["requestId"]), interceptAnswer{action: "continue", url: stringValue(p["url"]), method: stringValue(p["method"]), headers: headersValue(p["headers"]), body: decodeBody(stringValue(p["postData"]))})
	case "Fetch.continueResponse":
		err = s.interceptor.Resolve(stringValue(p["requestId"]), interceptAnswer{action: "fulfill", status: intValue(p["responseCode"], 0), headers: headersValue(p["responseHeaders"])})
	case "Fetch.failRequest":
		err = s.interceptor.Resolve(stringValue(p["requestId"]), interceptAnswer{action: "fail"})
	case "Fetch.fulfillRequest":
		err = s.interceptor.Resolve(stringValue(p["requestId"]), interceptAnswer{action: "fulfill", status: intValue(p["responseCode"], 200), headers: headersValue(p["responseHeaders"]), body: decodeBody(stringValue(p["body"]))})
	case "Performance.getMetrics":
		result = map[string]any{"metrics": []any{map[string]any{"name": "Timestamp", "value": float64(time.Now().UnixNano()) / 1e9}}}
	case "Mimic.captureSnapshot":
		ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
		defer cancel()
		result, err = s.page.CaptureSnapshot(ctx)
	case "Mimic.getTrace":
		result = map[string]any{"events": s.page.Trace().Events()}
	case "Mimic.getStatus":
		activity := s.page.ExecutionStatus()
		result = map[string]any{
			"execution": map[string]any{
				"running":   activity.Running,
				"taskId":    activity.TaskID,
				"source":    activity.Source,
				"phase":     activity.Phase,
				"elapsedMs": activity.Elapsed.Milliseconds(),
			},
		}
	case "Mimic.getDiagnostics":
		result = s.page.LiveDiagnostics()
	case "Mimic.cancelExecution":
		result = map[string]any{"cancelled": s.server.cancelExecution(s.page)}
	case "Mimic.clearTrace":
		s.page.Trace().Clear()
	case "Mimic.getCompatibilityMatrix":
		schema := s.page.Compatibility().CDP()
		result = map[string]any{"chromeVersion": s.page.Environment().Product.FullVersion, "protocol": protocolMatrix(schema.Methods, schema.Events)}
	case "Mimic.setViewport":
		err = s.page.SetViewport(intValue(p["width"], 0), intValue(p["height"], 0))
	case "Mimic.pause":
		s.page.Pause()
	case "Mimic.resume":
		s.page.Resume()
	default:
		schema := s.page.Compatibility().CDP()
		if _, registered := schema.Methods[m.Method]; registered {
			s.page.Trace().Add(trace.SemanticMissing, "CDP."+m.Method, map[string]any{"sessionId": route})
			err = fmt.Errorf("method %s is registered for the pinned CDP schema but its semantics are not implemented", m.Method)
		} else {
			s.page.Trace().Add(trace.SurfaceMissing, "CDP."+m.Method, map[string]any{"sessionId": route})
			err = fmt.Errorf("method %s is absent from the pinned CDP schema", m.Method)
		}
	}
	s.replyRouted(m.ID, result, err, route)
}
func targetInfo(page *browser.Page, attached bool) map[string]any {
	return map[string]any{"targetId": page.ID, "type": "page", "title": page.Title(), "url": page.URL(), "attached": attached}
}
func (s *session) frameLoaderID(frameID string) string {
	if frameID == "" || frameID == s.page.Top.ID {
		return s.page.LoaderID()
	}
	if frame, ok := s.page.Frame(frameID); ok {
		return frame.LoaderID()
	}
	return ""
}

func (s *session) framePayload(frame *browser.Frame) map[string]any {
	if frame == nil {
		return map[string]any{}
	}
	parentID := ""
	if frame.Parent() != nil {
		parentID = frame.Parent().ID
	}
	return s.framePayloadByID(frame.ID, frame.URL(), s.frameLoaderID(frame.ID), parentID)
}

func (s *session) framePayloadByID(frameID, rawURL, loaderID, parentID string) map[string]any {
	if rawURL == "" {
		if frame, ok := s.page.Frame(frameID); ok {
			rawURL = frame.URL()
		}
	}
	if rawURL == "" && frameID == s.page.Top.ID {
		rawURL = s.page.URL()
	}
	securityOrigin := originURL(rawURL)
	if securityOrigin == "://" && parentID != "" {
		if parent, ok := s.page.Frame(parentID); ok {
			securityOrigin = originURL(parent.URL())
		}
	}
	payload := map[string]any{"id": frameID, "loaderId": loaderID, "url": rawURL, "domainAndRegistry": "", "securityOrigin": securityOrigin, "mimeType": "text/html"}
	if parentID != "" {
		payload["parentId"] = parentID
	}
	return payload
}

func (s *session) frameTree(frame *browser.Frame) map[string]any {
	tree := map[string]any{"frame": s.framePayload(frame)}
	children := frame.Children()
	if len(children) != 0 {
		childTrees := make([]any, 0, len(children))
		for _, child := range children {
			childTrees = append(childTrees, s.frameTree(child))
		}
		tree["childFrames"] = childTrees
	}
	return tree
}

func (s *session) ensureContext(frameID, realmID, rawURL string) map[string]any {
	s.contextMu.Lock()
	if contextID, ok := s.contextByFrame[frameID]; ok && (realmID == "" || s.realmByFrame[frameID] == realmID) {
		s.contextMu.Unlock()
		return s.contextPayload(contextID, frameID, rawURL)
	}
	s.nextContextID++
	contextID := s.nextContextID
	s.contextByFrame[frameID] = contextID
	s.frameByContext[contextID] = frameID
	s.realmByFrame[frameID] = realmID
	s.contextMu.Unlock()
	return s.contextPayload(contextID, frameID, rawURL)
}

func (s *session) contextPayload(contextID int64, frameID, rawURL string) map[string]any {
	origin := originURL(rawURL)
	if origin == "://" {
		if frame, ok := s.page.Frame(frameID); ok && frame.Parent() != nil {
			origin = originURL(frame.Parent().URL())
		}
	}
	return map[string]any{"id": contextID, "origin": origin, "name": "", "auxData": map[string]any{"isDefault": true, "type": "default", "frameId": frameID}}
}

func (s *session) contextForRealm(realmID string) (int64, bool) {
	s.contextMu.Lock()
	defer s.contextMu.Unlock()
	for frameID, currentRealmID := range s.realmByFrame {
		if currentRealmID == realmID {
			contextID, ok := s.contextByFrame[frameID]
			return contextID, ok
		}
	}
	return 0, false
}

func (s *session) clearContexts() {
	s.contextMu.Lock()
	s.contextByFrame = map[string]int64{}
	s.frameByContext = map[int64]string{}
	s.realmByFrame = map[string]string{}
	s.contextMu.Unlock()
}

func (s *session) destroyFrameContext(frameID string) {
	s.contextMu.Lock()
	contextID, ok := s.contextByFrame[frameID]
	if ok {
		delete(s.contextByFrame, frameID)
		delete(s.frameByContext, contextID)
		delete(s.realmByFrame, frameID)
	}
	s.contextMu.Unlock()
	if ok {
		s.event("Runtime.executionContextDestroyed", map[string]any{"executionContextId": contextID, "executionContextUniqueId": ""})
	}
}

func (s *session) evaluateInContext(ctx context.Context, contextID int64, source string) (any, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if contextID == 0 {
		return s.page.EvaluateCommand(ctx, "", source)
	}
	s.contextMu.Lock()
	frameID, ok := s.frameByContext[contextID]
	s.contextMu.Unlock()
	if !ok {
		return nil, fmt.Errorf("cannot find context with specified id")
	}
	if frameID == s.page.Top.ID {
		return s.page.EvaluateCommand(ctx, "", source)
	}
	return s.page.EvaluateCommand(ctx, frameID, source)
}
func originURL(raw string) string {
	u, e := url.Parse(raw)
	if e != nil || u.Host == "" {
		return "://"
	}
	return u.Scheme + "://" + u.Host
}
func resourceTypeFromTrace(v any) string {
	switch fmt.Sprint(v) {
	case "navigation", "iframe":
		return "Document"
	case "script", "worker":
		return "Script"
	case "stylesheet":
		return "Stylesheet"
	case "fetch":
		return "Fetch"
	case "xhr":
		return "XHR"
	case "image":
		return "Image"
	}
	return "Other"
}
func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
func intValue(v any, d int) int {
	if n, ok := v.(float64); ok {
		return int(n)
	}
	return d
}
func remoteObject(v any) map[string]any {
	if v == nil {
		return map[string]any{"type": "undefined"}
	}
	typ := "object"
	switch v.(type) {
	case string:
		typ = "string"
	case bool:
		typ = "boolean"
	case int, int32, int64, float32, float64:
		typ = "number"
	}
	return map[string]any{"type": typ, "value": v, "description": fmt.Sprint(v)}
}

func remoteObjects(value any) []map[string]any {
	items, ok := value.([]any)
	if !ok {
		if value == nil {
			return []map[string]any{}
		}
		items = []any{value}
	}
	objects := make([]map[string]any, 0, len(items))
	for _, item := range items {
		objects = append(objects, remoteObject(item))
	}
	return objects
}

// cdpProtocol translates the transport's factual response protocol into the
// spelling exposed by Chromium's DevTools protocol. The transport trace keeps
// the original value (for example HTTP/3.0); this is only an adapter view.
func cdpProtocol(value any) string {
	protocol := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
	switch {
	case protocol == "h3" || strings.HasPrefix(protocol, "http/3"):
		return "h3"
	case protocol == "h2" || strings.HasPrefix(protocol, "http/2"):
		return "h2"
	case strings.HasPrefix(protocol, "http/1.1"):
		return "http/1.1"
	case strings.HasPrefix(protocol, "http/1.0"):
		return "http/1.0"
	default:
		return protocol
	}
}

func cdpResourceTiming(value any) map[string]any {
	timing, ok := value.(network.TransportTimingSnapshot)
	if !ok {
		return map[string]any{}
	}
	phase := func(name string) float64 {
		if observed, exists := timing.Phases[name]; exists {
			return observed
		}
		return -1
	}
	return map[string]any{
		"requestTime": 0,
		"proxyStart":  -1, "proxyEnd": -1,
		"dnsStart": phase("dnsStart"), "dnsEnd": phase("dnsEnd"),
		"connectStart": phase("tcpConnectStart"), "connectEnd": phase("tcpConnectEnd"),
		"sslStart": phase("tlsHandshakeStart"), "sslEnd": phase("tlsHandshakeEnd"),
		"workerStart": -1, "workerReady": -1, "workerFetchStart": -1, "workerRespondWithSettled": -1,
		"sendStart": phase("requestHeadersSent"), "sendEnd": phase("requestComplete"),
		"pushStart": 0, "pushEnd": 0,
		"receiveHeadersStart": phase("firstResponseByte"), "receiveHeadersEnd": phase("firstResponseByte"),
	}
}
func cdpNode(d *dom.Document, n dom.Node, depth int) map[string]any {
	attrs := []string{}
	for k, v := range n.Attributes {
		attrs = append(attrs, k, v)
	}
	nodeType, nodeName, localName := 1, n.TagName, strings.ToLower(n.TagName)
	switch n.Type {
	case "document":
		nodeType, nodeName, localName = 9, "#document", ""
	case "text":
		nodeType, nodeName, localName = 3, "#text", ""
	case "comment":
		nodeType, nodeName, localName = 8, "#comment", ""
	case "doctype":
		nodeType, localName = 10, ""
	}
	out := map[string]any{"nodeId": n.ID, "backendNodeId": n.ID, "nodeType": nodeType, "nodeName": nodeName, "localName": localName, "nodeValue": n.Text, "attributes": attrs, "childNodeCount": len(n.Children)}
	if depth != 0 {
		children := []any{}
		for _, id := range n.Children {
			if c, ok := d.Get(id); ok {
				children = append(children, cdpNode(d, c, depth-1))
			}
		}
		out["children"] = children
	}
	return out
}
func headersValue(v any) http.Header {
	if a, ok := v.([]any); ok {
		h := make(http.Header)
		for _, x := range a {
			if m, ok := x.(map[string]any); ok {
				h.Add(stringValue(m["name"]), stringValue(m["value"]))
			}
		}
		return h
	}
	return nil
}
func headerObject(v any) http.Header {
	h := make(http.Header)
	if m, ok := v.(map[string]any); ok {
		for k, x := range m {
			h.Set(k, fmt.Sprint(x))
		}
	}
	return h
}
func headerObjectOrNil(v any) http.Header {
	if _, ok := v.(map[string]any); !ok {
		return nil
	}
	return headerObject(v)
}
func parseRawResponse(encoded string) (interceptAnswer, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return interceptAnswer{}, err
	}
	res, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(raw)), &http.Request{Method: http.MethodGet})
	if err != nil {
		return interceptAnswer{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return interceptAnswer{}, err
	}
	return interceptAnswer{action: "fulfill", status: res.StatusCode, headers: res.Header.Clone(), body: body}, nil
}
func (s *session) pageCookies() []any {
	out := []any{}
	for _, snapshot := range s.page.Cookies().Snapshots() {
		c := snapshot.Cookie
		domain := c.Domain
		if !snapshot.HostOnly && !strings.HasPrefix(domain, ".") {
			domain = "." + domain
		}
		expires := float64(-1)
		if !c.Expires.IsZero() {
			expires = float64(c.Expires.UnixNano()) / 1e9
		}
		row := map[string]any{"name": c.Name, "value": c.Value, "domain": domain, "path": c.Path, "secure": c.Secure, "httpOnly": c.HttpOnly,
			"expires": expires, "session": c.Expires.IsZero(), "size": len(c.Name) + len(c.Value)}
		if snapshot.PartitionKey != nil {
			row["partitionKey"] = snapshot.PartitionKey
		}
		switch c.SameSite {
		case http.SameSiteNoneMode:
			row["sameSite"] = "None"
		case http.SameSiteLaxMode:
			row["sameSite"] = "Lax"
		case http.SameSiteStrictMode:
			row["sameSite"] = "Strict"
		}
		out = append(out, row)
	}
	return out
}
