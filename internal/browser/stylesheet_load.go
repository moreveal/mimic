package browser

import (
	"context"
	"strings"

	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/scheduler"
	"github.com/moreveal/mimic/internal/trace"
)

// Parser-discovered sheets start together and block subsequent parser scripts.
// Their responses belong to the document, not the loader's bounded CDP response
// history: evicting an old network event must not make an applied sheet vanish.
func (r *Realm) startParserStylesheets() []*resourcePreload {
	if r.stylesheetLoads == nil {
		r.stylesheetLoads = make(map[preloadKey]*resourcePreload)
		r.parserStylesheetEvents = make(map[int64]preloadKey)
		r.stylesheetFetchSlots = make(chan struct{}, 32)
	}
	var pending []*resourcePreload
	for _, link := range r.document.FindAllByTagName("link") {
		if !hasLinkRelation(link.Attributes["rel"], "stylesheet") || strings.TrimSpace(link.Attributes["href"]) == "" {
			continue
		}
		u, err := r.resolveDocument(link.Attributes["href"])
		if err != nil {
			continue
		}
		request := r.elementRequest(u, link.Attributes, network.Stylesheet)
		key := preloadRequestKey(request)
		load := r.stylesheetLoads[key]
		if load == nil {
			load = &resourcePreload{done: make(chan struct{})}
			r.stylesheetLoads[key] = load
			r.resourceWG.Add(1)
			go func(load *resourcePreload) {
				defer r.resourceWG.Done()
				defer func() {
					close(load.done)
					r.resourceRevision.Add(1)
				}()
				select {
				case r.stylesheetFetchSlots <- struct{}{}:
					defer func() { <-r.stylesheetFetchSlots }()
				case <-r.resourceContext.Done():
					load.err = r.resourceContext.Err()
					return
				}
				load.response, load.err = r.loadResource(r.resourceContext, request)
			}(load)
		}
		pending = append(pending, load)
		if previous, exists := r.parserStylesheetEvents[link.ID]; !exists || previous != key {
			r.parserStylesheetEvents[link.ID] = key
			id := link.ID
			r.resourceWG.Add(1)
			go func() {
				defer r.resourceWG.Done()
				response, err := load.wait(r.resourceContext)
				if r.resourceContext.Err() != nil {
					return
				}
				kind := "load"
				if err != nil || response.Status < 200 || response.Status >= 300 {
					kind = "error"
				}
				// The retained response/sheet is visible before load, including to
				// inline handlers. Parser scripts and these events share one loop.
				r.scheduler.Post(scheduler.Network, 0, func(ctx context.Context) error {
					if r.parserStylesheetEvents[id] != key {
						return nil
					}
					return r.dispatchResourceEvent(ctx, id, kind)
				})
			}()
		}
	}
	return pending
}

func (r *Realm) waitParserStylesheets(ctx context.Context, pending []*resourcePreload) {
	for _, load := range pending {
		if _, err := load.wait(ctx); err != nil {
			r.agent.Page().trace.Add(trace.Error, "stylesheetLoad", map[string]any{"realm": r.ID, "error": err.Error()})
		}
	}
}

func (r *Realm) retainedStylesheet(request network.Request) (network.Response, bool) {
	// Isolated worlds share the document's stylesheet loads with its owner.
	if r.mainWorld != nil {
		return r.mainWorld.retainedStylesheet(request)
	}
	for key, pending := range r.stylesheetLoads {
		if key != preloadRequestKey(request) {
			continue
		}
		select {
		case <-pending.done:
			return pending.response, pending.err == nil
		default:
			return network.Response{}, false
		}
	}
	return r.agent.Page().loader.CompletedURL(request.URL.String())
}
