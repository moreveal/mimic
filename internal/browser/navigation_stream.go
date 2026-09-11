package browser

import (
	"context"
	"errors"
	"github.com/moreveal/mimic/internal/dom"
	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/scheduler"
	"github.com/moreveal/mimic/internal/trace"
)

// Navigation owns its existing lifecycle and history commit. It shares the
// document insertion stream with document.write without implicitly opening or
// replacing the already established Document/Window.
func (r *Realm) initializeNavigationStream(navigationContext ...context.Context) (*documentStream, error) {
	parser, err := r.document.NewStream()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(r.resourceContext)
	stopNavigationCancel := func() bool { return false }
	if len(navigationContext) > 0 && navigationContext[0] != nil {
		stopNavigationCancel = context.AfterFunc(navigationContext[0], cancel)
	}
	s := &documentStream{
		parser: parser,
		ctx:    ctx,
		cancel: func() {
			stopNavigationCancel()
			cancel()
		},
		scripts:    map[int64]*streamScriptResponse{},
		navigation: true,
	}
	r.documentStream = s
	return s, nil
}

func (r *Realm) executeChildNavigationScript(ctx context.Context, navigation *childNavigation, stream *documentStream, node dom.Node) error {
	realm := navigation.realm
	realm.preloadResources()
	stylesheets := realm.startParserStylesheets()
	if !r.childNavigationCurrent(navigation) || realm.documentStream != stream || realm.document.ScriptStarted(node.ID) {
		return nil
	}
	if scriptExecutionKind(node.Attributes["type"], node.Attributes["language"]) == "" {
		return nil
	}
	code, name := realm.document.TextContent(node.ID), realm.documentURL().String()
	if src := node.Attributes["src"]; src != "" {
		target, err := realm.resolveDocument(src)
		if err != nil {
			return err
		}
		loaded := stream.scripts[node.ID]
		if loaded == nil {
			loaded = &streamScriptResponse{}
			stream.scripts[node.ID] = loaded
			eventLoop := r.browserEventLoop()
			request := realm.elementRequest(target, node.Attributes, network.Script)
			realm.resourceWG.Add(1)
			go func() {
				defer realm.resourceWG.Done()
				response, loadErr := realm.loadResource(stream.ctx, request)
				if loadErr == nil {
					loadErr = scriptResponseError(response)
				}
				if stream.ctx.Err() != nil {
					return
				}
				eventLoop.Post(scheduler.Network, 0, func(taskContext context.Context) error {
					if !r.childNavigationCurrent(navigation) || realm.documentStream != stream {
						r.finishChildNavigation(navigation)
						return nil
					}
					loaded.response, loaded.err, loaded.ready = response, loadErr, true
					stream.depth++
					resumeErr := stream.parser.Resume(stream.onScript)
					stream.depth--
					if errors.Is(resumeErr, dom.ErrStreamPaused) {
						return nil
					}
					if resumeErr != nil {
						return resumeErr
					}
					if stream.closing {
						return realm.finishDocumentStream(stream)
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
			realm.document.MarkScriptStarted(node.ID)
			r.agent.Page().trace.Add(trace.Error, "frameScriptLoad", map[string]any{"frameId": navigation.frame.ID, "url": target.String(), "error": loaded.err.Error()})
			return realm.dispatchResourceEvent(ctx, node.ID, "error")
		}
		code, name = string(loaded.response.Body), target.String()
	}
	realm.document.MarkScriptStarted(node.ID)
	if code == "" {
		return nil
	}
	realm.waitParserStylesheets(ctx, stylesheets)
	previous := stream.insideScript
	stream.insideScript = true
	defer func() { stream.insideScript = previous }()
	r.agent.Page().trace.Add(trace.JS, "scriptStart", map[string]any{"frameId": navigation.frame.ID, "url": name, "realm": realm.ID})
	if err := realm.evaluateClassicScript(ctx, code, name, node.ID); err != nil {
		r.agent.Page().trace.Add(trace.Exception, "frameScript", map[string]any{"frameId": navigation.frame.ID, "url": name, "error": err.Error()})
	}
	return nil
}
