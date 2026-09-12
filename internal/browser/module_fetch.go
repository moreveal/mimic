package browser

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/scheduler"
)

type moduleFetch struct {
	done     chan struct{}
	response network.Response
	err      error
}

// The realm's module map shares fetches (including failures), independently of
// HTTP caching. Only network work runs concurrently; linking, evaluation and
// observable events remain on the Page's existing event loop.
func (r *Realm) fetchModule(request network.Request) *moduleFetch {
	if r.moduleFetches == nil {
		r.moduleFetches = make(map[string]*moduleFetch)
	}
	key := request.URL.String()
	if pending := r.moduleFetches[key]; pending != nil {
		return pending
	}
	pending := &moduleFetch{done: make(chan struct{})}
	r.moduleFetches[key] = pending
	r.resourceWG.Add(1)
	go func() {
		defer r.resourceWG.Done()
		defer close(pending.done)
		pending.response, pending.err = r.loadResource(r.resourceContext, request)
		if pending.err == nil {
			pending.err = scriptResponseError(pending.response)
		}
	}()
	return pending
}

func (m *moduleFetch) wait(ctx context.Context) (network.Response, error) {
	select {
	case <-ctx.Done():
		return network.Response{}, ctx.Err()
	case <-m.done:
		return m.response, m.err
	}
}

func (r *Realm) moduleRequest(target, referrer *url.URL) network.Request {
	headers := make(http.Header)
	headers.Set("Origin", r.origin)
	request := network.Request{ContextID: r.agent.ContextID(), URL: target, Referrer: referrer, SourceURL: referrer, Initiator: network.Script, Mode: "cors", Credentials: "same-origin", Headers: headers}
	r.applyClientHints(&request)
	return request
}

func (r *Realm) preloadModules() {
	base := r.documentURL()
	if r.preloadedModuleLinks == nil {
		r.preloadedModuleLinks = map[int64]bool{}
	}
	for _, link := range r.document.FindAllByTagName("link") {
		if r.preloadedModuleLinks[link.ID] {
			continue
		}
		if !hasLinkRelation(link.Attributes["rel"], "modulepreload") || link.Attributes["href"] == "" {
			continue
		}
		if as := link.Attributes["as"]; as != "" && as != "script" {
			continue
		}
		target, err := base.Parse(link.Attributes["href"])
		if err != nil || !r.allowsScript(target, false, false, link.Attributes["nonce"]) {
			continue
		}
		request := r.moduleRequest(target, base)
		if strings.EqualFold(link.Attributes["crossorigin"], "use-credentials") {
			request.Credentials = "include"
		}
		r.preloadedModuleLinks[link.ID] = true
		request.PerformanceInitiatorType = "link"
		pending := r.fetchModule(request)
		id := link.ID
		r.resourceWG.Add(1)
		go func() {
			defer r.resourceWG.Done()
			_, err := pending.wait(r.resourceContext)
			if r.resourceContext.Err() != nil {
				return
			}
			kind := "load"
			if err != nil {
				kind = "error"
			}
			r.scheduler.Post(scheduler.Network, 0, func(ctx context.Context) error { return r.dispatchResourceEvent(ctx, id, kind) })
		}()
	}
}
