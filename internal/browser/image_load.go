package browser

import (
	"context"
	"net/http"
	"strings"

	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/scheduler"
)

type imageLoad struct {
	queued bool
	cancel context.CancelFunc
	blocks bool
}

// Image requests belong to their document even before the element is inserted.
// Attribute changes in one task coalesce; superseded completions cannot fire on
// the replacement request. All observable work stays on the Page event loop.
func (r *Realm) updateImage(id int64, changed bool) {
	if r.imageLoads == nil {
		r.imageLoads = map[int64]*imageLoad{}
	}
	previous := r.imageLoads[id]
	if previous != nil && (!changed || previous.queued) {
		return
	}
	if previous != nil && previous.cancel != nil {
		previous.cancel()
	}
	reason := "image"
	current := &imageLoad{queued: true, blocks: r.beginLoadBlocker(reason)}
	r.imageLoads[id] = current
	if previous != nil && previous.blocks {
		previous.blocks = false
		r.endLoadBlocker(reason)
	}
	finish := func(ctx context.Context, kind string) error {
		if r.imageLoads[id] != current {
			return nil
		}
		defer func() {
			if current.blocks {
				current.blocks = false
				r.endLoadBlocker(reason)
			}
		}()
		if kind == "" {
			return nil
		}
		return r.dispatchResourceEvent(ctx, id, kind)
	}
	// Queue an ordinary resource task: script transport starts still take
	// precedence, but unrelated timers must not starve an image indefinitely.
	r.scheduler.Post(scheduler.Network, 0, func(ctx context.Context) error {
		current.queued = false
		node, ok := r.document.Get(id)
		if !ok {
			return finish(ctx, "")
		}
		src, exists := node.Attributes["src"]
		if !exists {
			return finish(ctx, "")
		}
		if strings.TrimSpace(src) == "" {
			return finish(ctx, "error")
		}
		u, err := r.resolveDocument(src)
		if err != nil {
			return finish(ctx, "error")
		}
		referrer := r.documentURL()
		if strings.EqualFold(node.Attributes["referrerpolicy"], "no-referrer") {
			referrer = nil
		}
		headers := make(http.Header)
		mode := ""
		if _, ok := node.Attributes["crossorigin"]; ok {
			headers.Set("Origin", r.origin)
			mode = "cors"
		}
		request := network.Request{ContextID: r.agent.ContextID(), URL: u, Referrer: referrer, SourceURL: r.documentURL(), Headers: headers, Initiator: network.Image, Mode: mode}
		resourceContext, cancel := context.WithCancel(r.resourceContext)
		current.cancel = cancel
		r.resourceWG.Add(1)
		go func() {
			defer r.resourceWG.Done()
			defer cancel()
			response, err := r.agent.Page().loader.Load(resourceContext, request)
			if resourceContext.Err() != nil {
				return
			}
			kind := "load"
			if err != nil || response.Status < 200 || response.Status >= 300 {
				kind = "error"
			}
			r.scheduler.Post(scheduler.Network, 0, func(ctx context.Context) error { r.notifyPerformanceObservers(ctx); return finish(ctx, kind) })
		}()
		return nil
	})
}
