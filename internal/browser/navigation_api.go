package browser

import (
	"context"
	"github.com/google/uuid"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/scheduler"
	"github.com/moreveal/mimic/internal/trace"
	"net/url"
)

func (s *historyFrameState) ensureNavigationIdentity() {
	if s.navigationKey == "" {
		s.navigationKey = uuid.NewString()
		s.navigationID = uuid.NewString()
	}
}

// Navigation is a projection of the frame's entries in joint session history.
// Keys survive replacement; IDs identify a particular version of an entry.
func (r *Realm) navigationEntries() map[string]any {
	if frame, ok := r.agent.(*Frame); ok && disabledSessionHistory(frame) {
		return map[string]any{"entries": []any{}, "current": ""}
	}
	if !r.activeHistoryDocument() {
		return map[string]any{"entries": []any{}, "current": ""}
	}
	documentURL := r.documentURL()
	if documentURL.Scheme != "http" && documentURL.Scheme != "https" {
		return map[string]any{"entries": []any{}, "current": ""}
	}
	currentOrigin := historyOrigin(documentURL)
	p := r.agent.Page()
	p.mu.Lock()
	defer p.mu.Unlock()
	entries := []any{}
	current := ""
	seen := map[*historyFrameState]bool{}
	if p.historyIndex < 0 {
		result := map[string]any{"entries": entries, "current": current, "activationType": r.navigationActivationType}
		if from := r.navigationActivationFrom; from != nil {
			from.ensureNavigationIdentity()
			index := -1
			for i, row := range entries {
				if row.(map[string]any)["id"] == from.navigationID {
					index = i
				}
			}
			result["activationFrom"] = map[string]any{"url": from.url.String(), "id": from.navigationID, "key": from.navigationKey, "index": index, "sameDocument": from.realmID == r.ID, "state": from.navigationState}
		}
		return result
	}
	active := p.history[p.historyIndex].frames[r.agent.ContextID()]
	for _, entry := range p.history {
		s := entry.frames[r.agent.ContextID()]
		if s == nil || seen[s] || historyOrigin(s.url) != currentOrigin {
			continue
		}
		seen[s] = true
		s.ensureNavigationIdentity()
		entries = append(entries, map[string]any{"key": s.navigationKey, "id": s.navigationID, "url": s.url.String(), "sameDocument": s.realmID == r.ID, "state": s.navigationState, "index": len(entries)})
		if s == active {
			current = s.navigationID
		}
	}
	result := map[string]any{"entries": entries, "current": current, "activationType": r.navigationActivationType}
	if from := r.navigationActivationFrom; from != nil {
		from.ensureNavigationIdentity()
		index := -1
		for i, row := range entries {
			if row.(map[string]any)["id"] == from.navigationID {
				index = i
			}
		}
		result["activationFrom"] = map[string]any{"url": from.url.String(), "id": from.navigationID, "key": from.navigationKey, "index": index, "sameDocument": from.realmID == r.ID, "state": from.navigationState}
	}
	return result
}

func (r *Realm) navigationStart(target *url.URL, replace bool, reason string, keys ...string) bool {
	if r.navigationCallback == nil {
		return true
	}
	kind := "push"
	if replace {
		kind = "replace"
	}
	if reason == "reload" || reason == "traverse" {
		kind = reason
	}
	data := map[string]any{"url": target.String(), "navigationType": kind, "reason": reason}
	if len(keys) > 0 {
		data["key"] = keys[0]
	}
	v, err := r.runtime.Call(context.Background(), r.navigationCallback, nil, r.val("start"), r.val(data))
	if err != nil {
		r.agent.Page().trace.Add(trace.Error, "navigationEvent", map[string]any{"error": err.Error()})
		return false
	}
	return v == nil || v.Export() != false
}
func (r *Realm) navigationCommitted(kind string) {
	if r.navigationCallback == nil {
		return
	}
	_, err := r.runtime.Call(context.Background(), r.navigationCallback, nil, r.val("commit"), r.val(kind))
	if err != nil {
		r.agent.Page().trace.Add(trace.Error, "navigationEvent", map[string]any{"error": err.Error()})
	}
}
func addNavigationHosts(r *Realm, h map[string]any) {
	h["installNavigation"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		r.navigationCallback = a[0]
		r.navigationEncoder = a[1]
		r.navigationDecoder = a[2]
		return nil, nil
	})
	h["navigationEncodeReference"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		target, err := r.referenceRealm(strarg(a, 0), strarg(a, 1))
		if err != nil {
			return nil, err
		}
		handle := int64(numarg(a, 2))
		return r.crossFrameData(target, func(ctx context.Context) (any, error) {
			v, e := target.runtime.Call(ctx, target.navigationEncoder, nil, target.crossValues[handle])
			if e != nil {
				return nil, e
			}
			return v.Export(), nil
		})
	})
	h["navigationEntries"] = r.fn(func(engine.Value, []engine.Value) (engine.Value, error) { return r.val(r.navigationEntries()), nil })
	h["navigationState"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		p := r.agent.Page()
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.historyIndex >= 0 {
			if s := p.history[p.historyIndex].frames[r.agent.ContextID()]; s != nil {
				s.navigationState = strarg(a, 0)
			}
		}
		return nil, nil
	})
	h["navigationDefaultReplace"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		target, err := r.resolveDocument(strarg(a, 0))
		if err != nil {
			return r.val(false), nil
		}
		current := r.documentURL()
		frame, ok := r.agent.(*Frame)
		return r.val(ok && frame.parent != nil && r.activationAt.IsZero() && (target.Path != current.Path || target.RawQuery != current.RawQuery || target.Scheme != current.Scheme || target.Host != current.Host)), nil
	})
	h["navigationCommit"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		target, err := r.resolveDocument(strarg(a, 0))
		if err != nil {
			return nil, err
		}
		frame, ok := r.agent.(*Frame)
		if !ok || !r.activeHistoryDocument() {
			return nil, nil
		}
		replace, _ := arg(a, 1).(bool)
		previousURL := r.documentURL()
		r.agent.Page().commitHistory(frame, target, replace, nil)
		p := r.agent.Page()
		p.mu.Lock()
		p.history[p.historyIndex].frames[frame.ID].navigationState = strarg(a, 2)
		p.mu.Unlock()
		r.updateSelectorTarget(target.Fragment)
		if previousURL.Scheme == target.Scheme && previousURL.Host == target.Host && previousURL.Path == target.Path && previousURL.RawQuery == target.RawQuery && previousURL.Fragment != target.Fragment {
			r.browserEventLoop().Post(scheduler.Navigation, 0, func(ctx context.Context) error {
				if !r.activeHistoryDocument() {
					return nil
				}
				return r.dispatchHashChange(ctx, previousURL, target)
			})
		}
		return nil, nil
	})
	h["navigationTraverse"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		p := r.agent.Page()
		p.mu.Lock()
		delta := 0
		found := false
		for i, e := range p.history {
			if s := e.frames[r.agent.ContextID()]; s != nil {
				s.ensureNavigationIdentity()
				if s.navigationKey == strarg(a, 0) {
					delta = i - p.historyIndex
					found = true
					break
				}
			}
		}
		p.mu.Unlock()
		if found && delta != 0 {
			r.historyGo(delta)
		}
		return r.val(found), nil
	})
}
