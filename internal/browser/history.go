package browser

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/scheduler"
)

// Session history is joint across the frame tree. Each entry records the URL
// and state of the participating documents; a child update must never replace
// the Page's top-level URL. The embedded URL is the top-level entry URL.
type sessionHistoryEntry struct {
	*url.URL
	frames map[string]*historyFrameState
}

type historyFrameState struct {
	navigationKey   string
	navigationID    string
	navigationState string
	storageData     string
	url             *url.URL
	realmID         string
	state           engine.Value
	storageState    engine.Value // private clone, never exposed to application code
}

// Chrome disables session history for the complete auxiliary PiP tree.
// Its documents still own History state and fragment URLs, but never append
// entries to the opener Page's joint history.
func disabledSessionHistory(frame *Frame) bool {
	for frame != nil {
		if frame.auxiliaryOpener != nil {
			return true
		}
		frame = frame.parent
	}
	return false
}

func historyOrigin(u *url.URL) string {
	port := u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	return strings.ToLower(u.Scheme + "://" + u.Hostname() + ":" + port)
}

func historyURLAllowed(current, target *url.URL) bool {
	if historyOrigin(current) != historyOrigin(target) || usernameOf(current) != usernameOf(target) || passwordOf(current) != passwordOf(target) {
		return false
	}
	if current.Scheme == "http" || current.Scheme == "https" {
		return true
	}
	// Non-network URLs have narrower rewrite permissions. Opaque documents
	// cannot use an inherited origin to turn about:blank into a network URL.
	if current.Path != target.Path || current.Opaque != target.Opaque {
		return false
	}
	return current.Scheme == "file" || (current.RawQuery == target.RawQuery && current.ForceQuery == target.ForceQuery)
}

func (r *Realm) historyPush(raw string, replace bool, state engine.Value) (string, error) {
	frame, ok := r.agent.(*Frame)
	if !ok || !r.activeHistoryDocument() {
		return "The document is not fully active.", nil
	}
	stored, err := r.runtime.Call(context.Background(), r.historyCloneFunction, nil, state)
	if err != nil {
		return "", err
	}
	exposed, err := r.runtime.Call(context.Background(), r.historyCloneFunction, nil, stored)
	if err != nil {
		return "", err
	}
	current := r.documentURL()
	target := current
	if raw != "" {
		target, err = r.resolveDocument(raw)
	}
	if err != nil || !historyURLAllowed(current, target) {
		return "A history state object cannot be created with this URL in the current document.", nil
	}
	if disabledSessionHistory(frame) {
		r.auxiliaryState, r.auxiliaryStorage = exposed, stored
		r.url = target
		return "", nil
	}
	if !r.navigationStart(target, replace, "history") {
		return "", nil
	}
	r.agent.Page().commitHistory(frame, target, replace, exposed)
	p := r.agent.Page()
	p.mu.Lock()
	p.history[p.historyIndex].frames[frame.ID].storageState = stored
	committedState := p.history[p.historyIndex].frames[frame.ID]
	p.mu.Unlock()
	if r.navigationEncoder != nil {
		v, e := r.runtime.Call(context.Background(), r.navigationEncoder, nil, stored)
		if e != nil {
			return "", e
		}
		data := v.Export().([]any)
		if data[0] == true {
			p.mu.Lock()
			committedState.storageData = data[1].(string)
			p.mu.Unlock()
		}
	}
	r.navigationCommitted(map[bool]string{true: "replace", false: "push"}[replace])
	return "", nil
}

func (p *Page) commitHistory(frame *Frame, target *url.URL, replace bool, state engine.Value, traversal ...int) {
	if disabledSessionHistory(frame) {
		frame.Realm.url = target
		frame.Realm.auxiliaryState = state
		frame.Realm.auxiliaryStorage = nil
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	entry := &sessionHistoryEntry{URL: p.current, frames: make(map[string]*historyFrameState)}
	if p.historyIndex >= 0 {
		for id, value := range p.history[p.historyIndex].frames {
			entry.frames[id] = value
		}
	}
	key := uuid.NewString()
	if replace && p.historyIndex >= 0 {
		if old := p.history[p.historyIndex].frames[frame.ID]; old != nil {
			old.ensureNavigationIdentity()
			key = old.navigationKey
		}
	}
	entry.frames[frame.ID] = &historyFrameState{url: target, realmID: frame.Realm.ID, state: state, navigationKey: key, navigationID: uuid.NewString()}
	if frame == p.Top {
		entry.URL = target
		p.current = target
	}
	frame.Realm.url = target
	if len(traversal) > 0 && traversal[0] > 0 {
		target := traversal[0] - 1
		previous := p.history[target].frames[frame.ID]
		previous.ensureNavigationIdentity()
		entry.frames[frame.ID].navigationKey = previous.navigationKey
		entry.frames[frame.ID].navigationID = previous.navigationID
		entry.frames[frame.ID].navigationState = previous.navigationState
		entry.frames[frame.ID].storageData = previous.storageData
		oldRealmID := previous.realmID
		for _, oldEntry := range p.history {
			if oldState := oldEntry.frames[frame.ID]; oldState != nil && oldState.realmID == oldRealmID {
				oldState.realmID = frame.Realm.ID
				oldState.state = nil
				oldState.storageState = nil
			}
		}
		p.history[target] = entry
		p.historyIndex = target
	} else if replace && p.historyIndex >= 0 {
		p.history[p.historyIndex] = entry
	} else {
		p.history = p.history[:p.historyIndex+1]
		p.history = append(p.history, entry)
		p.historyIndex++
	}
}

func (r *Realm) historyState() engine.Value {
	if f, ok := r.agent.(*Frame); ok && disabledSessionHistory(f) {
		if r.auxiliaryState != nil {
			return r.auxiliaryState
		}
		return r.val(nil)
	}
	p := r.agent.Page()
	p.mu.RLock()
	var state *historyFrameState
	if p.historyIndex >= 0 {
		state = p.history[p.historyIndex].frames[r.agent.ContextID()]
	}
	p.mu.RUnlock()
	if state == nil || state.realmID != r.ID {
		return r.val(nil)
	}
	if state.state == nil && state.storageData != "" && r.navigationDecoder != nil {
		value, err := r.runtime.Call(context.Background(), r.navigationDecoder, nil, r.val(state.storageData))
		if err == nil {
			p.mu.Lock()
			state.state = value
			p.mu.Unlock()
		}
	}
	if state.state != nil {
		return state.state
	}
	return r.val(nil)
}

func (r *Realm) historyGo(delta int) {
	if f, ok := r.agent.(*Frame); ok && disabledSessionHistory(f) {
		return
	}
	p := r.agent.Page()
	r.browserEventLoop().Post(scheduler.Navigation, 0, func(ctx context.Context) error {
		if !r.activeHistoryDocument() {
			return nil
		}
		if delta == 0 {
			return r.postNavigate(r.documentURL().String(), true, true)
		}
		p.mu.Lock()
		n := p.historyIndex + delta
		if n < 0 || n >= len(p.history) {
			p.mu.Unlock()
			return nil
		}
		entry := p.history[n]
		if target := entry.frames[r.agent.ContextID()]; target != nil && target != p.history[p.historyIndex].frames[r.agent.ContextID()] {
			target.ensureNavigationIdentity()
			key := target.navigationKey
			targetURL := target.url
			p.mu.Unlock()
			if !r.navigationStart(targetURL, false, "traverse", key) {
				return nil
			}
			p.mu.Lock()
		}
		// A different Document is restored through the existing network/parser
		// lifecycle. The target index travels with that navigation, so another
		// frame or canceled request cannot consume shared traversal state.
		for id, state := range entry.frames {
			if frame := p.frames[id]; frame != nil && frame.Realm != nil && frame.Realm.ID != state.realmID {
				frame.Realm.historyTraversalTarget = n + 1
				destination := state.url.String()
				realm := frame.Realm
				p.mu.Unlock()
				return realm.postNavigate(destination, true, true)
			}
		}
		previous := p.history[p.historyIndex]
		p.historyIndex = n
		p.current = entry.URL
		type historyChange struct {
			realm  *Realm
			oldURL *url.URL
			newURL *url.URL
		}
		var changed []historyChange
		for id, state := range entry.frames {
			if frame := p.frames[id]; frame != nil && frame.Realm != nil {
				if previous.frames[id] != state {
					changed = append(changed, historyChange{frame.Realm, frame.Realm.url, state.url})
				}
				frame.Realm.url = state.url
			}
		}
		p.mu.Unlock()
		for _, change := range changed {
			realm := change.realm
			if !realm.activeHistoryDocument() {
				continue
			}
			state := entry.frames[realm.agent.ContextID()]
			if state.storageState != nil {
				cloned, err := realm.runtime.Call(ctx, realm.historyCloneFunction, nil, state.storageState)
				if err != nil {
					return err
				}
				p.mu.Lock()
				state.state = cloned
				p.mu.Unlock()
			}
			realm.navigationCommitted("traverse")
			// Dispatch in the document whose active history entry changed, even
			// when traversal was requested by its parent or a sibling.
			if _, err := realm.Evaluate(ctx, `(()=>{const event=new Event('popstate');Object.defineProperty(event,'state',{value:history.state});dispatchEvent(event)})()`, "mimic:history"); err != nil {
				return fmt.Errorf("history popstate: %w", err)
			}
			if change.oldURL.Fragment != change.newURL.Fragment {
				if !realm.activeHistoryDocument() {
					continue
				}
				realm.updateSelectorTarget(change.newURL.Fragment)
				if err := realm.dispatchHashChange(ctx, change.oldURL, change.newURL); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (r *Realm) dispatchHashChange(ctx context.Context, oldURL, newURL *url.URL) error {
	_, err := r.Evaluate(ctx, `(()=>{const event=new Event('hashchange');Object.defineProperties(event,{oldURL:{value:`+strconv.Quote(oldURL.String())+`},newURL:{value:`+strconv.Quote(newURL.String())+`}});dispatchEvent(event)})()`, "mimic:hashchange")
	return err
}

func (r *Realm) navigateFragment(target *url.URL, replace bool) {
	frame, ok := r.agent.(*Frame)
	if !ok || !r.activeHistoryDocument() {
		return
	}
	current := r.documentURL()
	if current.String() == target.String() {
		return
	}
	if !r.navigationStart(target, replace, "fragment") {
		return
	}
	r.updateSelectorTarget(target.Fragment)
	r.agent.Page().commitHistory(frame, target, replace, nil)
	r.navigationCommitted(map[bool]string{true: "replace", false: "push"}[replace])
	r.browserEventLoop().Post(scheduler.Navigation, 0, func(ctx context.Context) error {
		if !r.activeHistoryDocument() {
			return nil
		}
		return r.dispatchHashChange(ctx, current, target)
	})
}

func (r *Realm) activeHistoryDocument() bool {
	p := r.agent.Page()
	p.mu.RLock()
	defer p.mu.RUnlock()
	frame := p.frames[r.agent.ContextID()]
	return frame != nil && frame.Realm == r
}
