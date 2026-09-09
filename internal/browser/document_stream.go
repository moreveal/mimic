package browser

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/moreveal/mimic/internal/dom"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/scheduler"
	"github.com/moreveal/mimic/internal/trace"
)

// The stream replaces input, not the realm or canonical Document. The parser
// owns insertion state; this layer owns script execution and load events.
type documentStream struct {
	parser                          *dom.Stream
	depth                           int
	closing, finished, insideScript bool
	ctx                             context.Context
	cancel                          context.CancelFunc
	scripts                         map[int64]*streamScriptResponse
	navigation                      bool
	onScript                        func(dom.Node) error
	onFinished                      func() error
}

type streamScriptResponse struct {
	response network.Response
	err      error
	ready    bool
}

func (r *Realm) installDocumentStream(host map[string]any) {
	host["documentStream"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		frame := r.agent.Page().frame(strarg(args, 0))
		if !r.canAccess(frame) {
			return r.val(map[string]any{"name": "SecurityError", "error": "Blocked cross-origin document access"}), nil
		}
		target := frame.Realm
		caller := r
		if r.documentEntry != nil {
			caller = r.documentEntry
		}
		var err error
		switch strarg(args, 1) {
		case "open":
			err = target.openDocumentStream(caller)
		case "write":
			err = target.writeDocumentStream(caller, strarg(args, 2))
		case "close":
			err = target.closeDocumentStream()
		}
		return r.val(nil), err
	})
	host["documentBaseURI"] = r.fn(func(engine.Value, []engine.Value) (engine.Value, error) {
		return r.val(r.documentBaseURL().String()), nil
	})
}

func (r *Realm) documentBaseURL() *url.URL {
	base := r.documentURL()
	if base.Scheme == "about" {
		if frame, ok := r.agent.(*Frame); ok && frame.parent != nil && frame.parent.Realm != nil {
			base = frame.parent.Realm.documentBaseURL()
		}
	}
	for _, node := range r.document.FindAllByTagName("base") {
		if raw, exists := node.Attributes["href"]; exists {
			if ref, err := url.Parse(raw); err == nil {
				return base.ResolveReference(ref)
			}
			break
		}
	}
	return base
}

func (r *Realm) openDocumentStream(caller *Realm) error {
	if r.documentStream != nil && r.documentStream.insideScript {
		return nil
	}
	if r.documentStream != nil {
		r.documentStream.cancel()
		r.documentStream.parser.Abort()
	}
	// Reset before detaching: previously detached nodes keep their listeners.
	if _, err := r.runtime.Call(r.resourceContext, r.documentStreamReset, nil); err != nil {
		return err
	}
	for id := range r.childFrames {
		r.detachChildFrame(id)
	}
	parser, err := r.document.NewStream()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(r.resourceContext)
	r.documentStream = &documentStream{parser: parser, ctx: ctx, cancel: cancel, scripts: map[int64]*streamScriptResponse{}}
	r.readyState = "loading"
	r.loadEpoch++
	r.loadRequested, r.loadScheduled, r.loadCompleted = false, false, false
	r.navigationLoadEnd = time.Time{}
	if frame, ok := r.agent.(*Frame); ok {
		if frame == r.agent.Page().Top {
			p := r.agent.Page()
			p.mu.Lock()
			p.loadEventEnded = false
			p.mu.Unlock()
		}
		if frame.navigationCancel != nil {
			frame.navigationCancel()
		}
		frame.navigationSequence++
		frame.navigationPending = false
		u := caller.documentURL()
		if caller != r {
			u.Fragment, u.RawFragment = "", ""
		}
		r.agent.Page().commitHistory(frame, u, true, r.historyState())
	}
	return nil
}

func (r *Realm) writeDocumentStream(caller *Realm, source string) error {
	if r.documentStream == nil || r.documentStream.finished {
		if err := r.openDocumentStream(caller); err != nil {
			return err
		}
	}
	s := r.documentStream
	s.depth++
	err := s.parser.Write(source, func(node dom.Node) error { return r.executeStreamScript(s, node) })
	s.depth--
	if errors.Is(err, dom.ErrStreamPaused) {
		return nil
	}
	if err != nil {
		return err
	}
	if s.closing && s.depth == 0 && r.documentStream == s {
		return r.finishDocumentStream(s)
	}
	return nil
}

func (r *Realm) closeDocumentStream() error {
	s := r.documentStream
	if s == nil || s.finished {
		return nil
	}
	s.closing = true
	if s.depth != 0 {
		return nil
	}
	return r.finishDocumentStream(s)
}

func (r *Realm) executeStreamScript(s *documentStream, node dom.Node) error {
	if s.onScript != nil {
		return s.onScript(node)
	}
	if r.documentStream != s || r.document.ScriptStarted(node.ID) {
		return nil
	}
	kind := scriptExecutionKind(node.Attributes["type"], node.Attributes["language"])
	if kind == "" {
		return nil
	}
	ctx := s.ctx
	code, name := r.document.TextContent(node.ID), r.documentURL().String()
	src := node.Attributes["src"]
	if src != "" {
		u, err := r.resolveDocument(src)
		if err != nil {
			return err
		}
		if !r.agent.Page().allowsScript(u, false, false, node.Attributes["nonce"]) {
			return nil
		}
		loaded := s.scripts[node.ID]
		if loaded == nil {
			loaded = &streamScriptResponse{}
			s.scripts[node.ID] = loaded
			request := network.Request{ContextID: r.agent.ContextID(), URL: u, Referrer: r.documentURL(), SourceURL: r.documentURL(), Initiator: network.Script}
			r.applyClientHints(&request)
			eventLoop := r.browserEventLoop()
			r.resourceWG.Add(1)
			go func() {
				defer r.resourceWG.Done()
				response, loadErr := r.agent.Page().loader.Load(ctx, request)
				if loadErr == nil {
					loadErr = scriptResponseError(response)
				}
				if ctx.Err() != nil {
					return
				}
				eventLoop.Post(scheduler.Network, 0, func(taskContext context.Context) error {
					if ctx.Err() != nil || r.documentStream != s {
						return nil
					}
					loaded.response, loaded.err, loaded.ready = response, loadErr, true
					s.depth++
					resumeErr := s.parser.Resume(func(node dom.Node) error { return r.executeStreamScript(s, node) })
					s.depth--
					if errors.Is(resumeErr, dom.ErrStreamPaused) {
						return nil
					}
					if resumeErr != nil {
						return resumeErr
					}
					if s.closing {
						return r.finishDocumentStream(s)
					}
					return nil
				})
			}()
			return dom.ErrStreamPaused
		}
		if !loaded.ready {
			return dom.ErrStreamPaused
		}
		if loaded.err != nil {
			r.document.MarkScriptStarted(node.ID)
			r.agent.Page().trace.Add(trace.Error, "scriptLoad", map[string]any{"url": u.String(), "error": loaded.err.Error()})
			return r.dispatchResourceEvent(ctx, node.ID, "error")
		}
		code, name = string(loaded.response.Body), u.String()
	} else if !r.agent.Page().allowsScript(nil, true, false, node.Attributes["nonce"]) {
		return nil
	}
	if kind != "classic" {
		r.agent.Page().trace.Add(trace.Unsupported, "document.stream.module", map[string]any{"realm": r.ID})
		return nil
	}
	previous := s.insideScript
	r.document.MarkScriptStarted(node.ID)
	s.insideScript = true
	err := r.evaluateClassicScript(ctx, code, name, node.ID)
	s.insideScript = previous
	if err != nil {
		r.agent.Page().trace.Add(trace.Exception, "script", map[string]any{"url": name, "error": err.Error(), "realm": r.ID})
	}
	if src != "" {
		return r.dispatchResourceEvent(ctx, node.ID, "load")
	}
	return nil
}

func (r *Realm) finishDocumentStream(s *documentStream) error {
	s.depth++
	err := s.parser.Close(func(node dom.Node) error { return r.executeStreamScript(s, node) })
	s.depth--
	if errors.Is(err, dom.ErrStreamPaused) {
		return nil
	}
	if err != nil {
		return err
	}
	if r.documentStream != s || s.finished || !s.parser.Closed() {
		return nil
	}
	s.finished = true
	if s.navigation {
		r.updateSelectorTarget(r.documentURL().Fragment)
		if s.onFinished != nil {
			return s.onFinished()
		}
		return nil
	}
	ctx := r.resourceContext
	for _, iframe := range r.document.FindAllByTagName("iframe") {
		if _, err := r.ensureChildFrame(iframe.ID); err != nil {
			return err
		}
	}
	r.readyState = "interactive"
	if _, err := r.runtime.Call(ctx, r.documentStreamEvent, nil, r.val("DOMContentLoaded")); err != nil {
		return err
	}
	if r.documentStream != s {
		return nil
	}
	if r.loadBlockers != 0 {
		r.requestLoad(func(ctx context.Context) { r.finishStreamLoad(ctx, s) })
		return nil
	}
	r.readyState = "complete"
	r.loadCompleted = true
	if _, err := r.runtime.Call(ctx, r.documentStreamEvent, nil, r.val("load")); err != nil {
		return err
	}
	r.finishStreamLoad(ctx, s)
	return nil
}

func (r *Realm) finishStreamLoad(ctx context.Context, s *documentStream) {
	if r.documentStream != s {
		return
	}
	r.navigationLoadEnd = r.scheduler.Now()
	p := r.agent.Page()
	p.trace.Add(trace.Lifecycle, "load", map[string]any{"frameId": r.agent.ContextID(), "url": r.documentURL().String(), "realm": r.ID})
	frame, ok := r.agent.(*Frame)
	if ok && frame.parent != nil && frame.parent.Realm != nil {
		parent := frame.parent.Realm
		if _, err := parent.runtime.Call(ctx, parent.frameLoadDispatcher, nil, parent.val(frame.elementID)); err != nil {
			p.trace.Add(trace.Error, "frameLoad", map[string]any{"error": err.Error()})
		}
	} else {
		p.mu.Lock()
		p.loadEventEnded = true
		p.mu.Unlock()
	}
	r.notifyPerformanceObservers(ctx)
}
