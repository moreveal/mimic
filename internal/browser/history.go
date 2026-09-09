package browser

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/scheduler"
	"github.com/moreveal/mimic/internal/trace"
)

// Session history is joint across the frame tree. Each entry records the URL
// and state of the participating documents; a child update must never replace
// the Page's top-level URL. The embedded URL is the top-level entry URL.
type sessionHistoryEntry struct {
	*url.URL
	frames map[string]*historyFrameState
}

type historyFrameState struct {
	url     *url.URL
	realmID string
	state   engine.Value
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

func (r *Realm) historyPush(raw string, replace bool, state engine.Value) string {
	frame, ok := r.agent.(*Frame)
	if !ok || !r.activeHistoryDocument() {
		return "The document is not fully active."
	}
	current := r.documentURL()
	target := current
	var err error
	if raw != "" {
		target, err = r.resolveDocument(raw)
	}
	if err != nil || !historyURLAllowed(current, target) {
		return "A history state object cannot be created with this URL in the current document."
	}
	r.agent.Page().commitHistory(frame, target, replace, state)
	return ""
}

func (p *Page) commitHistory(frame *Frame, target *url.URL, replace bool, state engine.Value) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry := &sessionHistoryEntry{URL: p.current, frames: make(map[string]*historyFrameState)}
	if p.historyIndex >= 0 {
		for id, value := range p.history[p.historyIndex].frames {
			entry.frames[id] = value
		}
	}
	entry.frames[frame.ID] = &historyFrameState{url: target, realmID: frame.Realm.ID, state: state}
	if frame == p.Top {
		entry.URL = target
		p.current = target
	}
	frame.Realm.url = target
	if replace && p.historyIndex >= 0 {
		p.history[p.historyIndex] = entry
	} else {
		p.history = p.history[:p.historyIndex+1]
		p.history = append(p.history, entry)
		p.historyIndex++
	}
}

func (r *Realm) historyState() engine.Value {
	p := r.agent.Page()
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.historyIndex >= 0 {
		state := p.history[p.historyIndex].frames[r.agent.ContextID()]
		if state != nil && state.realmID == r.ID && state.state != nil {
			return state.state
		}
	}
	return r.val(nil)
}

func (r *Realm) historyGo(delta int) {
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
		// Restoring a different Document needs a real navigation/BFCache path.
		// Do not silently relabel a live document as an unrelated history entry.
		for id, state := range entry.frames {
			if frame := p.frames[id]; frame != nil && frame.Realm != nil && frame.Realm.ID != state.realmID {
				p.mu.Unlock()
				p.trace.Add(trace.Unsupported, "history.crossDocumentTraversal", map[string]any{"frameId": id})
				return nil
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
	r.updateSelectorTarget(target.Fragment)
	r.agent.Page().commitHistory(frame, target, replace, nil)
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
