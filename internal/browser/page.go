package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/moreveal/mimic/compatibility"
	"github.com/moreveal/mimic/internal/csp"
	"github.com/moreveal/mimic/internal/dom"
	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/scheduler"
	"github.com/moreveal/mimic/internal/state"
	"github.com/moreveal/mimic/internal/trace"
)

type InitScript struct{ ID, Source string }
type documentSecurity struct {
	secureContext       bool
	crossOriginIsolated bool
	credentialless      bool
	originAgentCluster  bool
	permissionsPolicy   string
}
type Page struct {
	commandMu         sync.Mutex
	mu                sync.RWMutex
	ID                string
	ctx               *Context
	env               state.Environment
	loader            *network.Loader
	trace             *trace.Recorder
	Top               *Frame
	loaderID          string
	clock             time.Time
	performanceOrigin time.Time
	proxy             *WindowProxy
	current           *url.URL
	history           []*sessionHistoryEntry
	historyIndex      int
	initScripts       []InitScript
	sessionStorage    map[string]map[string]string
	policy            csp.PolicySet
	bypassCSP         bool
	frames            map[string]*Frame
	// retiredRealms keeps document realms alive until the browser task which
	// initiated a navigation has unwound. Closing the old JS runtime while one
	// of its callbacks is still on the stack is observably different from
	// Chrome and also makes the scheduler's microtask checkpoint fail.
	retiredRealms    []*Realm
	documentSecurity documentSecurity
	loadEventEnded   bool
	messagePorts     map[string]*messagePortState
}

// LockCommands serializes an external command and its complete event-loop turn.
// The boundary belongs to the Page, so all CDP sessions observe the same loop.
// Library callers must use the same boundary when sharing a Page concurrently.
func (p *Page) LockCommands()   { p.commandMu.Lock() }
func (p *Page) UnlockCommands() { p.commandMu.Unlock() }

func newPage(c *Context) (*Page, error) {
	environment := c.browser.Environment()
	p := &Page{ID: uuid.NewString(), ctx: c, env: environment, trace: trace.New(), historyIndex: -1, clock: environment.Time.WallOrigin, performanceOrigin: environment.Time.WallOrigin, sessionStorage: map[string]map[string]string{}, frames: map[string]*Frame{}, messagePorts: map[string]*messagePortState{}}
	p.loader = network.NewLoaderWithSession(func() state.Environment { p.mu.RLock(); defer p.mu.RUnlock(); return p.env }, c.cookies, c.network, p.trace)
	if c.transport != nil {
		p.loader.SetTransport(c.transport)
	}
	p.Top = &Frame{ID: uuid.NewString(), page: p, children: map[string]*Frame{}}
	p.frames[p.Top.ID] = p.Top
	p.proxy = &WindowProxy{frame: p.Top}
	return p, nil
}
func (p *Page) Compatibility() compatibility.Bundle { return p.ctx.browser.Compatibility() }
func (p *Page) initBlank() error {
	d, err := dom.Parse("<!doctype html><html><head></head><body></body></html>")
	if err != nil {
		return err
	}
	u, _ := url.Parse("about:blank")
	r, err := newRealmState(p, p.Top, d, u, true)
	if err != nil {
		return err
	}
	r.readyState = "complete"
	p.Top.Realm = r
	p.loaderID = uuid.NewString()
	p.current = u
	p.history = []*sessionHistoryEntry{{URL: u, frames: map[string]*historyFrameState{p.Top.ID: {url: u, realmID: r.ID}}}}
	p.historyIndex = 0
	p.loadEventEnded = true
	return nil
}
func (p *Page) Trace() *trace.Recorder                { return p.trace }
func (p *Page) Loader() *network.Loader               { return p.loader }
func (p *Page) Cookies() *network.CookieStore         { return p.ctx.cookies }
func (p *Page) NetworkSession() *network.SessionState { return p.ctx.network }
func (p *Page) Close() error {
	p.mu.Lock()
	realm := p.Top.Realm
	p.Top.Realm = nil
	retired := p.retiredRealms
	p.retiredRealms = nil
	p.history = nil
	p.historyIndex = -1
	p.mu.Unlock()
	if realm != nil {
		_ = realm.Close()
	}
	for _, old := range retired {
		_ = old.Close()
	}
	return nil
}
func (p *Page) URL() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.current == nil {
		return "about:blank"
	}
	return p.current.String()
}
func (p *Page) LoaderID() string { p.mu.RLock(); defer p.mu.RUnlock(); return p.loaderID }
func (p *Page) ClockNow() time.Time {
	p.mu.RLock()
	now := p.clock
	realm := p.Top.Realm
	p.mu.RUnlock()
	if realm != nil {
		if realmNow := realm.scheduler.Now(); realmNow.After(now) {
			return realmNow
		}
	}
	return now
}
func (p *Page) PerformanceOrigin() time.Time {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.performanceOrigin
}
func (p *Page) LoadEventEnded() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.loadEventEnded
}
func (p *Page) Title() string {
	p.mu.RLock()
	r := p.Top.Realm
	p.mu.RUnlock()
	if r == nil || r.document == nil {
		return ""
	}
	return r.document.Title()
}
func (p *Page) AddInitScript(source string) string {
	p.mu.Lock()
	defer p.mu.Unlock()
	id := uuid.NewString()
	p.initScripts = append(p.initScripts, InitScript{id, source})
	return id
}

func (p *Page) runInitScripts(ctx context.Context, realm *Realm) {
	p.mu.RLock()
	scripts := append([]InitScript(nil), p.initScripts...)
	p.mu.RUnlock()
	for _, script := range scripts {
		if _, err := realm.Evaluate(ctx, script.Source, "mimic:init-script"); err != nil {
			p.trace.Add(trace.Exception, "initScript", map[string]any{"scriptId": script.ID, "error": err.Error(), "realm": realm.ID})
			continue
		}
		if err := realm.checkpoint(ctx); err != nil {
			p.trace.Add(trace.Error, "initScriptMicrotaskCheckpoint", map[string]any{"scriptId": script.ID, "error": err.Error(), "realm": realm.ID})
		}
	}
}
func (p *Page) SetBypassCSP(bypass bool) { p.mu.Lock(); p.bypassCSP = bypass; p.mu.Unlock() }
func (p *Page) allowsScript(resource *url.URL, inline, dynamic bool, nonce string) bool {
	p.mu.RLock()
	bypass, policy, documentURL := p.bypassCSP, p.policy, p.current
	p.mu.RUnlock()
	allowed, reason := true, "CSP bypass enabled"
	if !bypass {
		allowed, reason = policy.AllowsScript(documentURL, resource, inline, dynamic, nonce)
	}
	p.trace.Add(trace.CSP, "scriptDecision", map[string]any{"allowed": allowed, "inline": inline, "dynamic": dynamic, "nonce": nonce != "", "resource": urlString(resource), "reason": reason})
	return allowed
}
func (p *Page) SetViewport(width, height int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("viewport dimensions must be positive")
	}
	if width > p.env.Window.OuterWidth || height > p.env.Window.OuterHeight {
		return fmt.Errorf("viewport cannot exceed outer window")
	}
	p.env.Window.ViewportWidth = width
	p.env.Window.ViewportHeight = height
	p.trace.Add(trace.Lifecycle, "viewportChanged", map[string]any{"width": width, "height": height})
	return nil
}
func (p *Page) Navigate(ctx context.Context, raw string) error {
	if err := p.navigate(ctx, raw, uuid.NewString()); err != nil {
		return err
	}
	// The direct Go API is synchronous through the document load boundary.
	// CDP uses NavigateReserved and observes the independently emitted lifecycle
	// events, matching Page.navigate's asynchronous protocol contract.
	for !p.LoadEventEnded() {
		p.mu.RLock()
		realm := p.Top.Realm
		p.mu.RUnlock()
		if realm == nil {
			return fmt.Errorf("page has no realm while waiting for load")
		}
		if err := realm.scheduler.Wait(ctx); err != nil {
			return err
		}
		if err := realm.RunReady(ctx); err != nil {
			return err
		}
	}
	return nil
}
func (p *Page) ReserveNavigation() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.loaderID = uuid.NewString()
	return p.loaderID
}
func (p *Page) NavigateReserved(ctx context.Context, raw, loaderID string) error {
	return p.navigate(ctx, raw, loaderID)
}
func (p *Page) navigate(ctx context.Context, raw, loaderID string, replace ...bool) error {
	return p.navigateRequest(ctx, raw, loaderID, network.Request{UserActivation: true}, replace...)
}
func (p *Page) navigateRequest(ctx context.Context, raw, loaderID string, request network.Request, replace ...bool) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported navigation scheme %q", u.Scheme)
	}
	performanceOrigin := p.ClockNow()
	// A controllable CDP process runs against the real wall clock by default.
	// Network operations may hold the browser state-machine lock while Go's
	// monotonic clock continues; catch the canonical clock up at the navigation
	// boundary rather than letting Date/performance drift behind the server.
	if wallNow := time.Now().UTC(); wallNow.After(performanceOrigin) {
		performanceOrigin = wallNow
	}
	p.mu.Lock()
	p.loaderID = loaderID
	p.clock = performanceOrigin
	p.performanceOrigin = performanceOrigin
	p.loadEventEnded = false
	p.mu.Unlock()
	// CDP defines the main resource request id as the navigation loader id.
	// Puppeteer/Pyppeteer use this equality (together with type=Document) to
	// recognize the navigation request and return its Response from goto().
	request.ID, request.ContextID, request.URL, request.Method, request.Initiator = loaderID, p.Top.ID, u, http.MethodGet, network.Navigation
	res, err := p.loader.Load(ctx, request)
	if err != nil {
		return err
	}
	// The response URL is the committed document URL after redirects. Use it
	// for the realm origin, history, policy checks and relative resource URLs.
	if res.URL != nil {
		u = res.URL
	}
	navigationScale := p.Environment().Time.NavigationScale
	p.mu.Lock()
	responseTime := performanceOrigin.Add(time.Duration(float64(res.Duration) * navigationScale))
	if responseTime.After(p.clock) {
		p.clock = responseTime
	}
	p.mu.Unlock()
	doc, err := dom.Parse(string(res.Body))
	if err != nil {
		return err
	}
	navigationMetaPolicies := doc.MetaHTTPEquiv("content-security-policy")
	doc, err = dom.Parse("")
	if err != nil {
		return err
	}
	coep := strings.ToLower(strings.TrimSpace(res.Headers.Get("Cross-Origin-Embedder-Policy")))
	coop := strings.ToLower(strings.TrimSpace(res.Headers.Get("Cross-Origin-Opener-Policy")))
	secureContext := potentiallyTrustworthyURL(u)
	p.mu.Lock()
	p.documentSecurity = documentSecurity{
		secureContext:       secureContext,
		crossOriginIsolated: secureContext && strings.HasPrefix(coop, "same-origin") && (strings.HasPrefix(coep, "require-corp") || strings.HasPrefix(coep, "credentialless")),
		credentialless:      strings.HasPrefix(coep, "credentialless"),
		originAgentCluster:  strings.EqualFold(strings.TrimSpace(res.Headers.Get("Origin-Agent-Cluster")), "?1"),
		permissionsPolicy:   res.Headers.Get("Permissions-Policy"),
	}
	p.mu.Unlock()
	realm, err := newRealm(p, p.Top, doc, u)
	if err != nil {
		return err
	}
	realm.referrerPolicy = res.Headers.Get("Referrer-Policy")
	realm.initializeClientHints(res.Headers.Get("Permissions-Policy"))
	p.mu.Lock()
	old := p.Top.Realm
	p.removeDescendantFramesLocked(p.Top)
	p.Top.Realm = realm
	p.current = u
	entry := &sessionHistoryEntry{URL: u, frames: map[string]*historyFrameState{p.Top.ID: {url: u, realmID: realm.ID}}}
	if len(replace) > 0 && replace[0] && p.historyIndex >= 0 {
		p.history[p.historyIndex] = entry
	} else {
		p.history = p.history[:p.historyIndex+1]
		p.history = append(p.history, entry)
		p.historyIndex++
	}
	policies := append([]string(nil), res.Headers.Values("Content-Security-Policy")...)
	policies = append(policies, navigationMetaPolicies...)
	p.policy = csp.Parse(policies...)
	p.mu.Unlock()
	if old != nil {
		p.mu.Lock()
		p.retiredRealms = append(p.retiredRealms, old)
		p.mu.Unlock()
	}
	p.trace.Add(trace.Lifecycle, "frameNavigated", map[string]any{"url": u.String(), "realm": realm.ID})
	streamState, err := realm.initializeNavigationStream()
	if err != nil {
		return err
	}
	p.runInitScripts(ctx, realm)
	type deferredModule struct {
		code string
		name string
	}
	modules := make([]deferredModule, 0)
	streamState.onScript = func(s dom.Node) error {
		realm.preloadModules()
		kind := scriptExecutionKind(s.Attributes["type"], s.Attributes["language"])
		if kind == "" {
			return nil
		}
		doc.MarkScriptStarted(s.ID)
		code := doc.TextContent(s.ID)
		name := u.String()
		if src := s.Attributes["src"]; src != "" {
			su, err := u.Parse(src)
			if err != nil {
				p.trace.Add(trace.Error, "scriptURL", map[string]any{"src": src, "error": err.Error()})
				return nil
			}
			if !p.allowsScript(su, false, false, s.Attributes["nonce"]) {
				return nil
			}
			request := network.Request{ContextID: p.Top.ID, URL: su, Referrer: u, SourceURL: u, Initiator: network.Script}
			realm.applyClientHints(&request)
			if _, crossOrigin := s.Attributes["crossorigin"]; crossOrigin || kind == "module" {
				request.Mode = "cors"
				request.Headers = make(http.Header)
				request.Headers.Set("Origin", originOf(u.String()))
			}
			var rr network.Response
			if kind == "module" {
				rr, err = realm.fetchModule(request).wait(ctx)
			} else {
				rr, err = p.loader.Load(ctx, request)
			}
			if err == nil {
				err = scriptResponseError(rr)
			}
			if err != nil {
				p.trace.Add(trace.Error, "scriptLoad", map[string]any{"url": su.String(), "error": err.Error()})
				scriptID := s.ID
				realm.scheduler.Post(scheduler.DOM, 0, func(eventContext context.Context) error {
					return realm.dispatchResourceEvent(eventContext, scriptID, "error")
				})
				if !streamState.insideScript {
					if eventErr := realm.RunReady(ctx); eventErr != nil {
						p.trace.Add(trace.Error, "scriptErrorEvent", map[string]any{"url": su.String(), "error": eventErr.Error()})
					}
				}
				return nil
			}
			code = string(rr.Body)
			name = su.String()
		} else if !p.allowsScript(nil, true, false, s.Attributes["nonce"]) {
			return nil
		}
		if kind == "module" {
			if code != "" {
				modules = append(modules, deferredModule{code: code, name: name})
			}
			return nil
		}
		if code != "" {
			// Parser scripts are browser-observable tasks too. Running them directly
			// from navigation freezes the canonical monotonic clock and lets engine
			// microtasks escape the scheduler. Queue each parser-blocking script and
			// drain the ready turn before the parser proceeds to the next script.
			scriptCode, scriptName, scriptID := code, name, s.ID
			runScript := func(taskContext context.Context) error {
				p.trace.Add(trace.JS, "scriptStart", map[string]any{"url": scriptName, "realm": realm.ID})
				previous := streamState.insideScript
				streamState.insideScript = true
				defer func() { streamState.insideScript = previous }()
				evalErr := realm.evaluateClassicScript(taskContext, scriptCode, scriptName, scriptID)
				if evalErr != nil {
					p.trace.Add(trace.Exception, "script", map[string]any{"url": scriptName, "error": evalErr.Error()})
					p.trace.Add(trace.JS, "scriptEnd", map[string]any{"url": scriptName, "realm": realm.ID, "error": evalErr.Error()})
					return evalErr
				}
				p.trace.Add(trace.JS, "scriptEnd", map[string]any{"url": scriptName, "realm": realm.ID})
				return nil
			}
			if streamState.insideScript {
				// document.write executes inserted classic scripts synchronously
				// within this parser task; the outer task owns its checkpoint.
				_ = runScript(ctx)
			} else {
				realm.scheduler.Post(scheduler.DOM, 0, runScript)
				if err := realm.RunReady(ctx); err != nil {
					p.trace.Add(trace.Error, "parserScriptTask", map[string]any{"url": scriptName, "error": err.Error()})
				}
			}
		}
		return nil
	}
	if err := realm.writeDocumentStream(realm, string(res.Body)); err != nil {
		return err
	}
	if err := realm.closeDocumentStream(); err != nil {
		return err
	}
	realm.preloadModules()
	// Module scripts are deferred by default: fetch begins at parser discovery,
	// while evaluation happens after parsing and before DOMContentLoaded.
	for _, module := range modules {
		module := module
		realm.scheduler.Post(scheduler.DOM, 0, func(taskContext context.Context) error {
			p.trace.Add(trace.JS, "scriptStart", map[string]any{"url": module.name, "realm": realm.ID, "module": true})
			_, evalErr := realm.EvaluateModule(taskContext, module.code, module.name, func(specifier, referrer string) (string, string, error) {
				base, parseErr := url.Parse(referrer)
				if parseErr != nil {
					return "", "", parseErr
				}
				dependency, parseErr := base.Parse(specifier)
				if parseErr != nil {
					return "", "", parseErr
				}
				request := network.Request{ContextID: p.Top.ID, URL: dependency, Referrer: base, SourceURL: base, Initiator: network.Script, Mode: "cors", Headers: make(http.Header)}
				realm.applyClientHints(&request)
				request.Headers.Set("Origin", originOf(u.String()))
				// Dynamic import callbacks may run in a later browser task, after this
				// module-evaluation task has completed. Network work belongs to the
				// document realm and remains live until that realm is discarded.
				response, loadErr := realm.fetchModule(request).wait(realm.resourceContext)
				if loadErr == nil {
					loadErr = scriptResponseError(response)
				}
				if loadErr != nil {
					return "", "", loadErr
				}
				return string(response.Body), dependency.String(), nil
			})
			if evalErr != nil {
				p.trace.Add(trace.Exception, "script", map[string]any{"url": module.name, "error": evalErr.Error(), "module": true})
				p.trace.Add(trace.JS, "scriptEnd", map[string]any{"url": module.name, "realm": realm.ID, "module": true, "error": evalErr.Error()})
				return evalErr
			}
			p.trace.Add(trace.JS, "scriptEnd", map[string]any{"url": module.name, "realm": realm.ID, "module": true})
			return nil
		})
		if err := realm.RunReady(ctx); err != nil {
			p.trace.Add(trace.Error, "moduleScriptTask", map[string]any{"url": module.name, "error": err.Error()})
		}
	}
	// Parser-discovered external stylesheets are real, load-blocking browser
	// resources. Fetch them through the shared loader before DOMContentLoaded;
	// their CSSOM projection remains derived from the owning LINK element.
	for _, link := range doc.FindAllByTagName("link") {
		if !hasLinkRelation(link.Attributes["rel"], "stylesheet") || strings.TrimSpace(link.Attributes["href"]) == "" {
			continue
		}
		stylesheetURL, parseErr := u.Parse(link.Attributes["href"])
		if parseErr != nil {
			p.trace.Add(trace.Error, "stylesheetURL", map[string]any{"href": link.Attributes["href"], "error": parseErr.Error()})
			continue
		}
		request := network.Request{ContextID: p.Top.ID, URL: stylesheetURL, Referrer: u, SourceURL: u, Initiator: network.Stylesheet}
		realm.applyClientHints(&request)
		if _, loadErr := p.loader.Load(ctx, request); loadErr != nil {
			p.trace.Add(trace.Error, "stylesheetLoad", map[string]any{"url": stylesheetURL.String(), "error": loadErr.Error()})
		}
	}
	// The HTML parser creates browsing contexts for iframe elements without
	// waiting for script to read contentWindow.  Attach those contexts now, but
	// queue their network navigations from the DOMContentLoaded turn below. This
	// preserves parser iframe order relative to frames inserted by that event's
	// handlers while keeping all child execution on browser scheduler tasks.
	type parserFrame struct {
		elementID int64
		frame     *Frame
	}
	parserFrames := make([]parserFrame, 0)
	for _, iframe := range doc.FindAllByTagName("iframe") {
		frame, frameErr := realm.ensureChildFrameInternal(iframe.ID, false, false)
		if frameErr != nil {
			p.trace.Add(trace.Error, "parserFrameAttach", map[string]any{"elementNodeId": iframe.ID, "error": frameErr.Error(), "realm": realm.ID})
			continue
		}
		if frame != nil {
			parserFrames = append(parserFrames, parserFrame{elementID: iframe.ID, frame: frame})
		}
	}
	realm.SetReadyState("interactive")
	// DOMContentLoaded is a browser task, not merely a CDP notification.  Page
	// scripts observe it on Document and its Promise jobs checkpoint before the
	// next lifecycle/resource task is selected.
	realm.scheduler.Post(scheduler.DOM, 0, func(taskContext context.Context) error {
		for _, parserFrame := range parserFrames {
			realm.scheduleChildFrameNavigation(parserFrame.frame, parserFrame.elementID)
		}
		_, eventErr := realm.Evaluate(taskContext, `document.dispatchEvent(new Event('DOMContentLoaded'))`, "mimic:dom-content-loaded")
		if eventErr == nil {
			p.trace.Add(trace.Lifecycle, "DOMContentLoaded", map[string]any{"url": u.String(), "frameId": p.Top.ID, "realm": realm.ID})
		}
		return eventErr
	})
	if err := realm.RunReady(ctx); err != nil {
		p.trace.Add(trace.Error, "domContentLoaded", map[string]any{"url": u.String(), "error": err.Error(), "realm": realm.ID})
	}
	// Chrome's browser-owned favicon discovery begins once the parser has a
	// complete document. It is not load-blocking, but starting it from the load
	// event itself is observably too late: Resource Timing can contain its entry
	// before page challenge/application work performed around load completes.
	iconURLs := documentIconURLs(doc, u)
	iconInitiatorType := "link"
	if len(iconURLs) == 0 {
		iconURLs = []*url.URL{u.ResolveReference(&url.URL{Path: "/favicon.ico"})}
		iconInitiatorType = "img"
	}
	for _, favicon := range iconURLs {
		favicon := favicon
		realm.scheduler.Post(scheduler.ResourceLow, 0, func(taskContext context.Context) error {
			request := network.Request{ContextID: p.Top.ID, URL: favicon, Referrer: u, SourceURL: u, Initiator: network.Other, PerformanceInitiatorType: iconInitiatorType}
			realm.applyClientHints(&request)
			realm.resourceWG.Add(1)
			go func() {
				defer realm.resourceWG.Done()
				_, loadErr := p.loader.Load(realm.resourceContext, request)
				if realm.resourceContext.Err() != nil {
					return
				}
				if loadErr != nil {
					p.trace.Add(trace.Error, "browserResourceLoad", map[string]any{"url": favicon.String(), "error": loadErr.Error()})
				}
				realm.scheduler.Post(scheduler.Network, 0, func(callbackContext context.Context) error {
					realm.notifyPerformanceObservers(callbackContext)
					return nil
				})
			}()
			return nil
		})
		// Begin the non-blocking transport at this parser boundary. Network work
		// remains concurrent, and its completion is observed by the realm only
		// through the normal scheduler/Performance notification path.
		if err := realm.RunReady(ctx); err != nil {
			p.trace.Add(trace.Error, "faviconScheduler", map[string]any{"url": favicon.String(), "error": err.Error()})
		}
	}
	// Load is scheduled only after parser-time and transitively inserted
	// load-blocking resources complete. The realm owns that accounting so DOM,
	// resource loading, readyState and NavigationTiming share one lifecycle.
	realm.requestLoad(func(taskContext context.Context) {
		p.trace.Add(trace.Lifecycle, "load", map[string]any{"url": u.String()})
		p.mu.Lock()
		p.loadEventEnded = true
		p.mu.Unlock()
		// NavigationTiming is a live entry while the document loads. Chrome
		// delivers it to observers again when loadEventEnd finalizes duration;
		// resource-only notifications cannot represent that transition.
		realm.notifyPerformanceObservers(taskContext)
	})
	if err := realm.RunReady(ctx); err != nil {
		p.trace.Add(trace.Error, "scheduler", map[string]any{"error": err.Error()})
	}
	return nil
}
func documentIconURLs(document *dom.Document, base *url.URL) []*url.URL {
	var result []*url.URL
	for _, link := range document.FindAllByTagName("link") {
		if hasLinkRelation(link.Attributes["rel"], "icon") || hasLinkRelation(link.Attributes["rel"], "shortcut") {
			if href := strings.TrimSpace(link.Attributes["href"]); href != "" {
				if resolved, err := base.Parse(href); err == nil {
					result = append(result, resolved)
				}
			}
		}
	}
	return result
}
func hasLinkRelation(value, wanted string) bool {
	for _, relation := range strings.Fields(strings.ToLower(value)) {
		if relation == wanted {
			return true
		}
	}
	return false
}

func scriptExecutionKind(typeValue, language string) string {
	typeValue = strings.ToLower(strings.TrimSpace(typeValue))
	if typeValue == "module" {
		return "module"
	}
	if typeValue == "" && strings.TrimSpace(language) != "" {
		typeValue = "text/" + strings.ToLower(strings.TrimSpace(language))
	}
	switch typeValue {
	case "", "text/javascript", "application/javascript", "application/ecmascript", "text/ecmascript",
		"application/x-javascript", "text/javascript1.0", "text/javascript1.1", "text/javascript1.2",
		"text/javascript1.3", "text/javascript1.4", "text/javascript1.5", "text/jscript", "text/livescript":
		return "classic"
	default:
		return ""
	}
}
func urlString(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.String()
}
func (p *Page) Evaluate(ctx context.Context, source string) (any, error) {
	p.mu.RLock()
	r := p.Top.Realm
	p.mu.RUnlock()
	return p.evaluateRealm(ctx, r, source)
}

func (p *Page) Frame(frameID string) (*Frame, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	frame, ok := p.frames[frameID]
	return frame, ok
}

func (p *Page) EvaluateFrame(ctx context.Context, frameID, source string) (any, error) {
	frame, ok := p.Frame(frameID)
	if !ok || frame.Realm == nil {
		return nil, fmt.Errorf("unknown frame %q", frameID)
	}
	return p.evaluateRealm(ctx, frame.Realm, source)
}

func (p *Page) evaluateRealm(ctx context.Context, r *Realm, source string) (any, error) {
	if r == nil {
		return nil, fmt.Errorf("page has no realm; navigate first")
	}
	v, err := r.Evaluate(ctx, source, "__pyppeteer_evaluation_script__")
	if err != nil {
		return nil, err
	}
	// Runtime.evaluate is itself a browser-observable task boundary. Promise
	// reactions queued by its synchronous body run before any timer task.
	if err := r.checkpoint(ctx); err != nil {
		return nil, err
	}
	if err := p.runEvaluationTasks(ctx, r); err != nil {
		p.trace.Add(trace.Error, "scheduler", map[string]any{"error": err.Error(), "during": "Runtime.evaluate"})
	}
	if resolved, done, err := r.runtime.Await(v); err != nil {
		return nil, err
	} else if done {
		return resolved.Export(), nil
	}
	// Some engines expose Promise state by registering a reaction rather than
	// reading an internal slot. Give that reaction the same microtask checkpoint
	// that Chrome performs at the end of Runtime.evaluate before deciding that
	// external work is required.
	if err := r.checkpoint(ctx); err != nil {
		return nil, err
	}
	if resolved, done, err := r.runtime.Await(v); err != nil {
		return nil, err
	} else if done {
		return resolved.Export(), nil
	}
	for {
		realms := p.evaluationRealms(r)
		queues := make([]*scheduler.Scheduler, 0, len(realms))
		for _, realm := range realms {
			queues = append(queues, realm.scheduler)
		}
		if err := scheduler.WaitAny(ctx, queues); err != nil {
			return nil, err
		}
		if err := p.runEvaluationTasks(ctx, r); err != nil {
			p.trace.Add(trace.Error, "scheduler", map[string]any{"error": err.Error(), "during": "Runtime.awaitPromise wake"})
		}
		if resolved, done, err := r.runtime.Await(v); err != nil {
			return nil, err
		} else if done {
			return resolved.Export(), nil
		}
	}
}

// Evaluation awaits work across the Page, not just the realm owning its
// Promise. Keep the existing realm queues and their microtask checkpoints,
// but include siblings and newly created frames at each task boundary.
func (p *Page) evaluationRealms(evaluating *Realm) []*Realm {
	p.mu.RLock()
	defer p.mu.RUnlock()
	realms := make([]*Realm, 0, len(p.frames)+1)
	found := false
	for _, frame := range p.frames {
		if frame.Realm != nil {
			realms = append(realms, frame.Realm)
			found = found || frame.Realm == evaluating
		}
	}
	if !found {
		realms = append(realms, evaluating)
	}
	return realms
}

func (p *Page) runEvaluationTasks(ctx context.Context, evaluating *Realm) error {
	p.mu.RLock()
	top := p.Top.Realm
	active := false
	for _, frame := range p.frames {
		active = active || frame.Realm == evaluating
	}
	p.mu.RUnlock()
	// Preserve the established parent/child/parent drain order, including
	// messages posted back to the parent during a child's task.
	if top != nil {
		if err := top.RunReady(ctx); err != nil {
			return err
		}
	}
	if !active {
		return evaluating.RunReady(ctx)
	}
	return nil
}
func (p *Page) Document() (*dom.Document, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.Top.Realm == nil {
		return nil, false
	}
	return p.Top.Realm.document, true
}
func (p *Page) Environment() state.Environment { p.mu.RLock(); defer p.mu.RUnlock(); return p.env }
func (p *Page) Pause() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.Top.Realm != nil {
		p.Top.Realm.scheduler.Pause()
	}
}
func (p *Page) AdvanceTime(ctx context.Context, delta time.Duration) error {
	p.mu.Lock()
	p.clock = p.clock.Add(delta)
	realms := make([]*Realm, 0, len(p.frames))
	for _, frame := range p.frames {
		if frame.Realm != nil {
			realms = append(realms, frame.Realm)
		}
	}
	p.mu.Unlock()
	for _, realm := range realms {
		// A task from an earlier snapshot entry can commit navigation and
		// retire later entries. Only drive realms still in the active tree.
		p.mu.RLock()
		frame := p.frames[realm.agent.ContextID()]
		active := frame != nil && frame.Realm == realm
		p.mu.RUnlock()
		if !active {
			continue
		}
		if err := realm.AdvanceBy(ctx, delta); err != nil {
			p.closeRetiredRealms()
			return err
		}
	}
	p.closeRetiredRealms()
	return nil
}

func (p *Page) closeRetiredRealms() {
	p.mu.Lock()
	retired := p.retiredRealms
	p.retiredRealms = nil
	p.mu.Unlock()
	for _, realm := range retired {
		_ = realm.Close()
	}
}
func (p *Page) Resume() {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.Top.Realm != nil {
		p.Top.Realm.scheduler.Resume()
	}
}
func (p *Page) locationPart(k string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.current == nil {
		return "about:blank"
	}
	u := p.current
	switch k {
	case "href":
		return u.String()
	case "origin":
		return originOf(u.String())
	case "protocol":
		return u.Scheme + ":"
	case "host":
		return u.Host
	case "hostname":
		return u.Hostname()
	case "port":
		return u.Port()
	case "pathname":
		return u.Path
	case "search":
		if u.RawQuery != "" {
			return "?" + u.RawQuery
		}
	case "hash":
		return u.Fragment
	}
	return ""
}
func (p *Page) resolve(raw string) (*url.URL, error) {
	p.mu.RLock()
	base := p.current
	p.mu.RUnlock()
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if base != nil {
		return base.ResolveReference(u), nil
	}
	return u, nil
}
func (p *Page) historyLength() int { p.mu.RLock(); defer p.mu.RUnlock(); return len(p.history) }
func headerMap(v any) http.Header {
	h := make(http.Header)
	if m, ok := v.(map[string]any); ok {
		for k, x := range m {
			h.Set(k, fmt.Sprint(x))
		}
	}
	return h
}
func originOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "null"
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "null"
	}
	return strings.ToLower(u.Scheme + "://" + u.Host)
}
