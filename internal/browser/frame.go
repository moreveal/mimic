package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"

	"github.com/google/uuid"
	"github.com/moreveal/mimic/internal/dom"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/scheduler"
	"github.com/moreveal/mimic/internal/trace"
)

func (p *Page) frame(id string) *Frame {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if frame := p.frames[id]; frame != nil {
		return frame
	}
	var retained func(*Realm) *Frame
	retained = func(realm *Realm) *Frame {
		if realm == nil {
			return nil
		}
		if frame := realm.retainedFrames[id]; frame != nil {
			return frame
		}
		for _, frame := range realm.retainedFrames {
			if found := retained(frame.Realm); found != nil {
				return found
			}
		}
		return nil
	}
	for _, frame := range p.frames {
		if found := retained(frame.Realm); found != nil {
			return found
		}
	}
	return nil
}

func (r *Realm) ensureChildFrame(elementID int64, connectedThroughShadow ...bool) (*Frame, error) {
	shadowConnected := len(connectedThroughShadow) != 0 && connectedThroughShadow[0]
	return r.ensureChildFrameInternal(elementID, shadowConnected, true)
}

func (r *Realm) ensureChildFrameInternal(elementID int64, shadowConnected, scheduleNavigation bool) (*Frame, error) {
	if !r.document.IsConnected(elementID) && !shadowConnected {
		return nil, nil
	}
	if frame := r.childFrames[elementID]; frame != nil {
		if scheduleNavigation {
			r.scheduleChildFrameNavigation(frame, elementID)
		}
		return frame, nil
	}
	node, ok := r.document.Get(elementID)
	if !ok || node.TagName != "IFRAME" || (node.Parent == 0 && !shadowConnected) {
		return nil, nil
	}
	parent, ok := r.agent.(*Frame)
	if !ok {
		return nil, fmt.Errorf("only a frame realm can own an iframe")
	}
	page := r.agent.Page()
	frame := &Frame{ID: uuid.NewString(), page: page, parent: parent, elementID: elementID, children: map[string]*Frame{}, loadBlockers: map[uint64]string{}}
	document, err := dom.Parse("<!doctype html><html><head></head><body></body></html>")
	if err != nil {
		return nil, err
	}
	blank, _ := url.Parse("about:blank")
	realm, err := newRealm(page, frame, document, blank)
	if err != nil {
		return nil, err
	}
	realm.origin = r.origin
	realm.readyState = "complete"
	frame.Realm = realm
	page.mu.Lock()
	page.frames[frame.ID] = frame
	parent.children[frame.ID] = frame
	page.mu.Unlock()
	r.childFrames[elementID] = frame
	page.trace.Add(trace.Lifecycle, "frameAttached", map[string]any{"frameId": frame.ID, "parentFrameId": parent.ID, "elementNodeId": elementID, "realm": realm.ID, "initialContext": scheduleNavigation})
	if scheduleNavigation {
		r.scheduleChildFrameNavigation(frame, elementID)
	}
	return frame, nil
}

func (r *Realm) scheduleChildFrameNavigation(frame *Frame, elementID int64) {
	if frame.navigationStarted {
		return
	}
	node, ok := r.document.Get(elementID)
	if !ok || node.Attributes["src"] == "" {
		return
	}
	target, err := r.resolveDocument(node.Attributes["src"])
	if err != nil || (target.Scheme != "http" && target.Scheme != "https") {
		return
	}
	if frame.navigationCancel != nil {
		frame.navigationCancel()
	}
	frame.navigationStarted = true
	frame.navigationPending = true
	frame.navigationSequence++
	sequence := frame.navigationSequence
	loaderID := uuid.NewString()
	blockerReason := fmt.Sprintf("iframe:%s:%d", frame.ID, sequence)
	blocksLoad := r.beginLoadBlocker(blockerReason)
	if blocksLoad {
		frame.loadBlockers[sequence] = blockerReason
	}
	r.scheduler.Post(scheduler.Navigation, 0, func(ctx context.Context) error {
		r.startChildFrameNavigation(frame, target, sequence, loaderID, blockerReason, blocksLoad)
		return nil
	})
}

type childNavigation struct {
	embeddingRealm *Realm
	frame          *Frame
	target         *url.URL
	sequence       uint64
	loaderID       string
	blockerReason  string
	blocksLoad     bool
	document       *dom.Document
	realm          *Realm
	scripts        []dom.Node
	nextScript     int
}

func (r *Realm) browserEventLoop() *scheduler.Scheduler {
	p := r.agent.Page()
	if p.Top != nil && p.Top.Realm != nil {
		return p.Top.Realm.scheduler
	}
	return r.scheduler
}

func (r *Realm) childNavigationCurrent(navigation *childNavigation) bool {
	return navigation != nil && navigation.frame != nil &&
		navigation.frame.navigationSequence == navigation.sequence &&
		r.childFrames[navigation.frame.elementID] == navigation.frame
}

func (r *Realm) finishChildNavigation(navigation *childNavigation) {
	if navigation != nil && navigation.blocksLoad {
		navigation.blocksLoad = false
		frame := navigation.frame
		if frame != nil {
			if reason, exists := frame.loadBlockers[navigation.sequence]; exists {
				delete(frame.loadBlockers, navigation.sequence)
				r.endLoadBlocker(reason)
			}
		}
	}
}

// prepareChildFrameRemoval stops an iframe from delaying its owner Document's
// load before the DOM mutation disconnects the element. Chrome synchronously
// dispatches the owner's load when this removes the final blocker, so the
// listener still observes a connected iframe and a live contentWindow.
func (r *Realm) prepareChildFrameRemoval(elementID int64) bool {
	frame := r.childFrames[elementID]
	if frame == nil || len(frame.loadBlockers) == 0 {
		return false
	}
	sequences := make([]uint64, 0, len(frame.loadBlockers))
	for sequence := range frame.loadBlockers {
		sequences = append(sequences, sequence)
	}
	sort.Slice(sequences, func(i, j int) bool { return sequences[i] < sequences[j] })
	for _, sequence := range sequences {
		reason := frame.loadBlockers[sequence]
		delete(frame.loadBlockers, sequence)
		if r.loadBlockers > 0 {
			r.loadBlockers--
		}
		r.agent.Page().trace.Add(trace.Lifecycle, "loadBlockerRemoved", map[string]any{"reason": reason, "count": r.loadBlockers, "realm": r.ID})
	}
	if !r.loadRequested || r.loadCompleted || r.loadScheduled || r.loadBlockers != 0 {
		r.scheduleLoadIfReady()
		return false
	}
	r.readyState = "complete"
	r.loadCompleted = true
	r.agent.Page().trace.Add(trace.Lifecycle, "readyStateComplete", map[string]any{"realm": r.ID})
	return true
}

func (r *Realm) startChildFrameNavigation(frame *Frame, target *url.URL, sequence uint64, loaderID, blockerReason string, blocksLoad bool) {
	p := r.agent.Page()
	navigation := &childNavigation{embeddingRealm: r, frame: frame, target: target, sequence: sequence, loaderID: loaderID, blockerReason: blockerReason, blocksLoad: blocksLoad}
	if !r.childNavigationCurrent(navigation) {
		r.finishChildNavigation(navigation)
		return
	}
	eventLoop := r.browserEventLoop()
	loadContext, cancel := context.WithCancel(r.resourceContext)
	frame.navigationCancel = cancel
	referrer := r.documentURL()
	r.resourceWG.Add(1)
	go func() {
		defer r.resourceWG.Done()
		defer cancel()
		res, err := p.loader.Load(loadContext, network.Request{ID: loaderID, ContextID: frame.ID, URL: target, Referrer: referrer, SourceURL: referrer, Initiator: network.Iframe})
		if loadContext.Err() != nil {
			return
		}
		eventLoop.Post(scheduler.Network, 0, func(ctx context.Context) error {
			return r.commitChildFrameNavigation(ctx, navigation, res, err)
		})
	}()
}

func (r *Realm) commitChildFrameNavigation(ctx context.Context, navigation *childNavigation, res network.Response, loadErr error) error {
	p := r.agent.Page()
	if !r.childNavigationCurrent(navigation) {
		r.finishChildNavigation(navigation)
		return nil
	}
	if loadErr != nil {
		p.trace.Add(trace.Error, "frameLoad", map[string]any{"frameId": navigation.frame.ID, "url": navigation.target.String(), "error": loadErr.Error()})
		r.finishChildNavigation(navigation)
		return nil
	}
	documentURL := navigation.target
	if res.URL != nil {
		documentURL = res.URL
	}
	document, err := dom.Parse(string(res.Body))
	if err != nil {
		r.finishChildNavigation(navigation)
		return err
	}
	realm, err := newRealm(p, navigation.frame, document, documentURL)
	if err != nil {
		r.finishChildNavigation(navigation)
		return err
	}
	old := navigation.frame.Realm
	navigation.frame.Realm = realm
	navigation.frame.loaderID = navigation.loaderID
	if old != nil {
		_ = old.Close()
	}
	navigation.document = document
	navigation.realm = realm
	navigation.scripts = document.Scripts()
	p.trace.Add(trace.Lifecycle, "frameNavigated", map[string]any{"frameId": navigation.frame.ID, "parentFrameId": navigation.frame.parent.ID, "loaderId": navigation.loaderID, "url": documentURL.String(), "realm": realm.ID})
	p.runInitScripts(ctx, realm)
	return r.runChildFrameScripts(ctx, navigation)
}

func (r *Realm) runChildFrameScripts(ctx context.Context, navigation *childNavigation) error {
	if !r.childNavigationCurrent(navigation) || navigation.frame.Realm != navigation.realm {
		r.finishChildNavigation(navigation)
		return nil
	}
	p := r.agent.Page()
	for navigation.nextScript < len(navigation.scripts) {
		script := navigation.scripts[navigation.nextScript]
		navigation.nextScript++
		code, name := script.Text, navigation.realm.documentURL().String()
		if src := script.Attributes["src"]; src != "" {
			scriptURL, parseErr := navigation.realm.documentURL().Parse(src)
			if parseErr != nil {
				continue
			}
			r.startChildFrameScriptLoad(navigation, scriptURL)
			return nil
		}
		if err := r.executeChildFrameScript(ctx, navigation, code, name); err != nil {
			return err
		}
	}
	navigation.frame.navigationPending = false
	for _, message := range navigation.frame.pendingMessages {
		r.queueFrameMessage(navigation.frame.ID, message)
	}
	navigation.frame.pendingMessages = nil
	navigation.realm.SetReadyState("interactive")
	eventLoop := r.browserEventLoop()
	eventLoop.Post(scheduler.DOM, 0, func(eventContext context.Context) error {
		if !r.childNavigationCurrent(navigation) || navigation.frame.Realm != navigation.realm {
			r.finishChildNavigation(navigation)
			return nil
		}
		// Queue the Window load turn before DOMContentLoaded listeners enqueue
		// posted-message tasks. Chrome completes the child and fires the owner
		// iframe load before those newly posted messages are delivered.
		eventLoop.Post(scheduler.DOM, 0, func(loadContext context.Context) error {
			return r.completeChildFrameLoad(loadContext, navigation)
		})
		_, eventErr := navigation.realm.Evaluate(eventContext, `document.dispatchEvent(new Event('DOMContentLoaded'))`, "mimic:frame-dom-content-loaded")
		if eventErr != nil {
			return eventErr
		}
		p.trace.Add(trace.Lifecycle, "DOMContentLoaded", map[string]any{"frameId": navigation.frame.ID, "loaderId": navigation.loaderID, "url": navigation.realm.documentURL().String(), "realm": navigation.realm.ID})
		return navigation.realm.checkpoint(ctx)
	})
	return nil
}

func (r *Realm) startChildFrameScriptLoad(navigation *childNavigation, scriptURL *url.URL) {
	p := r.agent.Page()
	eventLoop := r.browserEventLoop()
	navigation.realm.resourceWG.Add(1)
	go func() {
		defer navigation.realm.resourceWG.Done()
		response, err := p.loader.Load(navigation.realm.resourceContext, network.Request{ContextID: navigation.frame.ID, URL: scriptURL, Referrer: navigation.realm.documentURL(), SourceURL: navigation.realm.documentURL(), Initiator: network.Script})
		if navigation.realm.resourceContext.Err() != nil {
			return
		}
		eventLoop.Post(scheduler.Network, 0, func(ctx context.Context) error {
			if !r.childNavigationCurrent(navigation) || navigation.frame.Realm != navigation.realm {
				r.finishChildNavigation(navigation)
				return nil
			}
			if err != nil {
				p.trace.Add(trace.Error, "frameScriptLoad", map[string]any{"frameId": navigation.frame.ID, "url": scriptURL.String(), "error": err.Error()})
			} else if executeErr := r.executeChildFrameScript(ctx, navigation, string(response.Body), scriptURL.String()); executeErr != nil {
				return executeErr
			}
			return r.runChildFrameScripts(ctx, navigation)
		})
	}()
}

func (r *Realm) executeChildFrameScript(ctx context.Context, navigation *childNavigation, code, name string) error {
	if code == "" {
		return nil
	}
	p := r.agent.Page()
	p.trace.Add(trace.JS, "scriptStart", map[string]any{"frameId": navigation.frame.ID, "url": name, "realm": navigation.realm.ID})
	if _, evalErr := navigation.realm.Evaluate(ctx, code, name); evalErr != nil {
		p.trace.Add(trace.Exception, "frameScript", map[string]any{"frameId": navigation.frame.ID, "url": name, "error": evalErr.Error()})
		return nil
	}
	return navigation.realm.checkpoint(ctx)

}

func (r *Realm) completeChildFrameLoad(ctx context.Context, navigation *childNavigation) error {
	if !r.childNavigationCurrent(navigation) || navigation.frame.Realm != navigation.realm {
		r.finishChildNavigation(navigation)
		return nil
	}
	p := r.agent.Page()
	navigation.realm.SetReadyState("complete")
	if _, eventErr := navigation.realm.Evaluate(ctx, `dispatchEvent(new Event('load'))`, "mimic:frame-load"); eventErr != nil {
		r.finishChildNavigation(navigation)
		return eventErr
	}
	if checkpointErr := navigation.realm.checkpoint(ctx); checkpointErr != nil {
		r.finishChildNavigation(navigation)
		return checkpointErr
	}
	p.trace.Add(trace.Lifecycle, "load", map[string]any{"frameId": navigation.frame.ID, "loaderId": navigation.loaderID, "url": navigation.realm.documentURL().String(), "realm": navigation.realm.ID})
	if r.frameLoadDispatcher != nil {
		p.trace.Add(trace.Lifecycle, "iframeOwnerLoad", map[string]any{"frameId": navigation.frame.ID, "elementNodeId": navigation.frame.elementID, "realm": r.ID})
		if _, err := r.runtime.Call(ctx, r.frameLoadDispatcher, r.runtime.Get("window"), r.runtime.Value(navigation.frame.elementID)); err != nil {
			r.finishChildNavigation(navigation)
			return err
		}
	}
	r.finishChildNavigation(navigation)
	return nil
}

func (r *Realm) canAccess(frame *Frame) bool {
	return frame != nil && frame.Realm != nil && frame.Realm.origin == r.origin
}

func (r *Realm) detachChildFrame(elementID int64) {
	frame := r.childFrames[elementID]
	if frame == nil {
		return
	}
	if frame.navigationCancel != nil {
		frame.navigationCancel()
	}
	if frame.Realm != nil {
		frame.Realm.cancelResources()
	}
	delete(r.childFrames, elementID)
	page := r.agent.Page()
	page.mu.Lock()
	delete(page.frames, frame.ID)
	if frame.parent != nil {
		delete(frame.parent.children, frame.ID)
	}
	page.mu.Unlock()
	// Removing an iframe detaches its browsing context from the active frame
	// tree, but references to its WindowProxy/functions keep the Window/Realm
	// alive. Retain it until the owning Realm is closed.
	r.retainedFrames[frame.ID] = frame
	page.trace.Add(trace.Lifecycle, "frameDetached", map[string]any{"frameId": frame.ID, "elementNodeId": elementID})
}

func (r *Realm) evalInFrame(ctx context.Context, frameID, source string) (engine.Value, error) {
	frame := r.agent.Page().frame(frameID)
	if !r.canAccess(frame) {
		return nil, fmt.Errorf("SecurityError: Blocked cross-origin frame access")
	}
	r.agent.Page().trace.Add(trace.JS, "frameEvalStart", map[string]any{"frameId": frameID, "realm": frame.Realm.ID, "sourceLength": len(source)})
	value, err := frame.Realm.Evaluate(ctx, source, "frame-eval")
	if err != nil {
		return nil, err
	}
	r.agent.Page().trace.Add(trace.JS, "frameEvalResult", map[string]any{"frameId": frameID, "realm": frame.Realm.ID, "type": frame.Realm.runtime.TypeOf(value), "sourceLength": len(source)})
	if err := frame.Realm.RunReady(ctx); err != nil {
		return nil, err
	}
	if resolved, done, awaitErr := frame.Realm.runtime.Await(value); awaitErr != nil {
		return nil, awaitErr
	} else if done {
		return resolved, nil
	}
	if err := frame.Realm.RunUntilIdle(ctx); err != nil {
		return nil, err
	}
	if resolved, done, awaitErr := frame.Realm.runtime.Await(value); awaitErr != nil {
		return nil, awaitErr
	} else if done {
		return resolved, nil
	}
	return value, nil
}

func (r *Realm) crossRealmValue(value engine.Value) map[string]any {
	if value == nil || value.String() == "undefined" {
		return map[string]any{"__mimicCrossRealm": "undefined"}
	}
	if value.String() == "null" && r.runtime.TypeOf(value) == "object" {
		return map[string]any{"__mimicCrossRealm": "null"}
	}
	if r.runtime.StrictEqual(value, r.runtime.Get("globalThis")) {
		return map[string]any{"__mimicCrossRealm": "window", "frame": r.agent.ContextID()}
	}
	if r.runtime.StrictEqual(value, r.runtime.Get("parent")) {
		frame, _ := r.agent.(*Frame)
		if frame != nil && frame.parent != nil {
			return map[string]any{"__mimicCrossRealm": "window", "frame": frame.parent.ID}
		}
	}
	typeName := r.runtime.TypeOf(value)
	if typeName != "object" && typeName != "function" {
		return map[string]any{"__mimicCrossRealm": "value", "value": value.Export()}
	}
	for id, existing := range r.crossValues {
		if r.runtime.StrictEqual(value, existing) {
			return map[string]any{"__mimicCrossRealm": typeName, "realm": r.ID, "handle": id}
		}
	}
	r.crossValueSeq++
	r.crossValues[r.crossValueSeq] = value
	return map[string]any{"__mimicCrossRealm": typeName, "realm": r.ID, "handle": r.crossValueSeq}
}

func (r *Realm) postToFrame(frameID string, data any, targetOrigin string, portIDs []string) error {
	page := r.agent.Page()
	frame := page.frame(frameID)
	if frame == nil || frame.Realm == nil {
		return nil
	}
	if targetOrigin != "*" && targetOrigin != "/" && targetOrigin != frame.Realm.origin {
		return nil
	}
	sourceID := r.agent.ContextID()
	encoded, _ := json.Marshal(data)
	page.trace.Add(trace.Lifecycle, "frameMessagePosted", map[string]any{"sourceFrameId": sourceID, "targetFrameId": frameID, "valueType": fmt.Sprintf("%T", data), "valueShape": messageValueShape(data, 0), "encodedSize": len(encoded), "portCount": len(portIDs)})
	message := frameMessage{sourceFrameID: sourceID, origin: r.origin, data: data, portIDs: append([]string(nil), portIDs...)}
	if frame.navigationPending {
		frame.pendingMessages = append(frame.pendingMessages, message)
		return nil
	}
	r.queueFrameMessage(frameID, message)
	return nil
}

func messageValueShape(value any, depth int) any {
	if depth >= 2 {
		return fmt.Sprintf("%T", value)
	}
	switch value := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		shape := make(map[string]any, len(keys))
		for _, key := range keys {
			shape[key] = messageValueShape(value[key], depth+1)
		}
		return shape
	case []any:
		shape := make([]any, 0, min(len(value), 8))
		for index := 0; index < len(value) && index < 8; index++ {
			shape = append(shape, messageValueShape(value[index], depth+1))
		}
		return map[string]any{"length": len(value), "items": shape}
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64, float32, int, int32, int64, uint, uint32, uint64:
		return "number"
	default:
		return fmt.Sprintf("%T", value)
	}
}

type frameMessage struct {
	sourceFrameID string
	origin        string
	data          any
	portIDs       []string
}

func (r *Realm) queueFrameMessage(frameID string, message frameMessage) {
	page := r.agent.Page()
	eventLoop := r.scheduler
	if page.Top != nil && page.Top.Realm != nil {
		eventLoop = page.Top.Realm.scheduler
	}
	eventLoop.Post(scheduler.PostedMessage, 0, func(ctx context.Context) error {
		// postMessage targets a WindowProxy, not the Window which happened to
		// back it when the call was made. Resolve the active realm at delivery
		// time so a navigation commit cannot strand the task in about:blank.
		target := page.frame(frameID)
		if target == nil || target.Realm == nil {
			return nil
		}
		page.transferMessagePorts(message.portIDs, target.Realm)
		fn := target.Realm.messageReceiver
		_, err := target.Realm.runtime.Call(ctx, fn, target.Realm.runtime.Get("window"), target.Realm.runtime.Value(message.data), target.Realm.runtime.Value(message.origin), target.Realm.runtime.Value(message.sourceFrameID), target.Realm.runtime.Value(message.portIDs))
		if err != nil {
			return err
		}
		// Delivery runs on the shared browser loop, but Promise jobs belong
		// to the receiving realm. Drain them before the next message task.
		return target.Realm.checkpoint(ctx)
	})
}
