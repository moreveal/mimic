package browser

import (
	"context"
	"strings"

	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/trace"
)

// Parser-discovered sheets start together and block subsequent parser scripts.
// Their responses belong to the document, not the loader's bounded CDP response
// history: evicting an old network event must not make an applied sheet vanish.
func (r *Realm) startParserStylesheets() []*resourcePreload {
	if r.stylesheetLoads == nil {
		r.stylesheetLoads = make(map[preloadKey]*resourcePreload)
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

func (r *Realm) retainedStylesheet(rawURL string) (network.Response, bool) {
	for key, pending := range r.stylesheetLoads {
		if key.url != rawURL {
			continue
		}
		select {
		case <-pending.done:
			return pending.response, pending.err == nil
		default:
			return network.Response{}, false
		}
	}
	return r.agent.Page().loader.CompletedURL(rawURL)
}
