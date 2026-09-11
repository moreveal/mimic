package cdp

import (
	"context"
	"fmt"
)

type runtimeWorldContext struct {
	FrameID, RealmID, MainRealmID, Name string
}

func (s *session) ensureRuntimeWorldContext(frameID, realmID, mainRealmID, name, url string) map[string]any {
	s.contextMu.Lock()
	defer s.contextMu.Unlock()
	if s.worldContexts == nil {
		s.worldContexts = make(map[int64]runtimeWorldContext)
	}
	var id int64
	for contextID, world := range s.worldContexts {
		if world.RealmID == realmID {
			id = contextID
			break
		}
	}
	if id == 0 {
		s.nextContextID++
		id = s.nextContextID
		s.worldContexts[id] = runtimeWorldContext{frameID, realmID, mainRealmID, name}
	}
	return map[string]any{"id": id, "origin": originURL(url), "name": name, "uniqueId": realmID, "auxData": map[string]any{"isDefault": false, "type": "isolated", "frameId": frameID}}
}

func (s *session) createIsolatedWorld(ctx context.Context, params map[string]any) (any, error) {
	frameID, name := stringValue(params["frameId"]), stringValue(params["worldName"])
	frame, ok := s.page.Frame(frameID)
	if !ok || frame.Realm == nil {
		return nil, fmt.Errorf("No frame for given id found")
	}
	realmID, err := s.page.IsolatedWorld(ctx, frameID, name)
	if err != nil {
		return nil, err
	}
	context := s.ensureRuntimeWorldContext(frameID, realmID, frame.Realm.ID, name, s.page.URL())
	return map[string]any{"executionContextId": context["id"]}, nil
}

func (s *session) clearRuntimeWorldContexts() {
	s.contextMu.Lock()
	s.worldContexts = make(map[int64]runtimeWorldContext)
	s.contextMu.Unlock()
}

func (s *session) destroyRuntimeWorldContexts(frameID string) {
	s.contextMu.Lock()
	var destroyed []int64
	for id, world := range s.worldContexts {
		if world.FrameID == frameID {
			destroyed = append(destroyed, id)
			delete(s.worldContexts, id)
		}
	}
	s.contextMu.Unlock()
	for _, id := range destroyed {
		s.event("Runtime.executionContextDestroyed", map[string]any{"executionContextId": id})
	}
}
