package cdp

import (
	"time"

	"github.com/moreveal/mimic/internal/browser"
	"github.com/moreveal/mimic/internal/trace"
)

type pageIdle struct {
	loaded   time.Time
	loaderID string
	emitted  [2]time.Time
}

func (s *session) replayIdle() {
	active, quiet := s.page.Loader().Activity()
	var events []map[string]any
	s.server.lifecycleMu.Lock()
	state := s.server.idlePages[s.page]
	if state != nil && !state.loaded.IsZero() {
		for i, name := range []string{"networkIdle", "networkAlmostIdle"} {
			start := quiet[i]
			if start.Before(state.loaded) {
				start = state.loaded
			}
			if active > i*2 || state.emitted[i].IsZero() || !start.Equal(state.emitted[i]) {
				continue
			}
			events = append(events, map[string]any{"frameId": s.page.Top.ID, "loaderId": state.loaderID, "name": name, "timestamp": float64(start.UnixNano()) / 1e9})
		}
	}
	s.server.lifecycleMu.Unlock()
	for _, params := range events {
		s.event("Page.lifecycleEvent", params)
	}
}

func (s *Server) observePageLifecycle(page *browser.Page, e trace.Event) {
	if e.Kind != trace.Lifecycle || stringValue(e.Data["frameId"]) != page.Top.ID {
		return
	}
	s.lifecycleMu.Lock()
	state := s.idlePages[page]
	if state != nil {
		switch e.Name {
		case "frameNavigated":
			state.loaded = time.Time{}
			state.loaderID = stringValue(e.Data["loaderId"])
			state.emitted = [2]time.Time{}
		case "load":
			state.loaded = time.Now()
			state.loaderID = page.LoaderID()
			state.emitted = [2]time.Time{}
		}
	}
	s.lifecycleMu.Unlock()
	if e.Name == "frameNavigated" || e.Name == "navigatedWithinDocument" {
		s.targetChanged(page)
	}
}

func (s *Server) emitIdle(page *browser.Page) {
	active, quiet := page.Loader().Activity()
	var events []map[string]any
	s.lifecycleMu.Lock()
	state := s.idlePages[page]
	if state != nil && !state.loaded.IsZero() {
		for i, threshold := range []int{0, 2} {
			if active > threshold {
				continue
			}
			start := quiet[i]
			if start.Before(state.loaded) {
				start = state.loaded
			}
			if time.Since(start) < 500*time.Millisecond || start.Equal(state.emitted[i]) {
				continue
			}
			state.emitted[i] = start
			name := "networkIdle"
			if i == 1 {
				name = "networkAlmostIdle"
			}
			events = append(events, map[string]any{"frameId": page.Top.ID, "loaderId": state.loaderID, "name": name, "timestamp": float64(start.UnixNano()) / 1e9})
		}
	}
	s.lifecycleMu.Unlock()
	for _, params := range events {
		for _, c := range s.clientSnapshot() {
			for _, session := range c.snapshot() {
				if session.page == page && session.targetType == "page" {
					session.event("Page.lifecycleEvent", params)
				}
			}
		}
	}
}
