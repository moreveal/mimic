package browser

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/moreveal/mimic/internal/imageresource"
	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/scheduler"
	"github.com/moreveal/mimic/internal/trace"
)

type imageLoad struct {
	complete    bool
	currentSrc  string
	decoded     *imageresource.Image
	originClean bool
	queued      bool
	cancel      context.CancelFunc
	blocks      bool
}

func (r *Realm) startDocumentImages() {
	for _, node := range r.document.FindAllByTagName("img") {
		r.updateImage(node.ID, false)
	}
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
	current := &imageLoad{originClean: true, queued: true, blocks: r.beginLoadBlocker(reason)}
	if previous != nil {
		current.decoded = previous.decoded
		current.currentSrc = previous.currentSrc
		current.originClean = previous.originClean
	}
	r.imageLoads[id] = current
	if previous != nil && previous.blocks {
		previous.blocks = false
		r.endLoadBlocker(reason)
	}
	finish := func(ctx context.Context, kind string) error {
		if r.imageLoads[id] != current {
			return nil
		}
		current.complete = true
		r.resourceRevision.Add(1)
		if kind != "load" {
			current.decoded = nil
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
		request := r.elementRequest(u, node.Attributes, network.Image)
		resourceContext, cancel := context.WithCancel(r.resourceContext)
		current.cancel = cancel
		r.resourceWG.Add(1)
		go func() {
			defer r.resourceWG.Done()
			defer cancel()
			response, err := r.loadResource(resourceContext, request)
			if resourceContext.Err() != nil {
				return
			}
			var decoded *imageresource.Image
			kind := "load"
			originClean, corsErr := r.imageResponseOrigin(request, response)
			if err == nil {
				err = corsErr
			}
			// Chrome decodes an image response even with an HTTP error status.
			// Transport/CORS failures and invalid image bytes determine failure.
			if err != nil {
				kind = "error"
			} else {
				decoded, err = imageresource.Decode(response.Body, response.Headers.Get("Content-Type"))
				if err != nil {
					kind = "error"
					r.agent.Page().trace.Add(trace.Error, "imageDecode", map[string]any{"url": u.String(), "error": err.Error(), "realm": r.ID})
				}
			}
			r.scheduler.Post(scheduler.Network, 0, func(ctx context.Context) error {
				if r.imageLoads[id] != current {
					return nil
				}
				current.currentSrc = u.String()
				current.decoded = decoded
				current.originClean = originClean
				r.notifyPerformanceObservers(ctx)
				return finish(ctx, kind)
			})
		}()
		return nil
	})
}

func (r *Realm) imageResponseOrigin(request network.Request, response network.Response) (bool, error) {
	finalURL := response.URL
	if finalURL == nil {
		finalURL = request.URL
	}
	originURL := finalURL
	if finalURL.Scheme == "blob" {
		if parsed, err := url.Parse(strings.TrimPrefix(finalURL.String(), "blob:")); err == nil {
			originURL = parsed
		}
	}
	clean := finalURL.Scheme == "data" || originURL.Scheme == request.SourceURL.Scheme && originURL.Host == request.SourceURL.Host
	if request.Mode == "cors" && !clean {
		allow := response.Headers.Get("Access-Control-Allow-Origin")
		clean = allow == request.Headers.Get("Origin") || allow == "*" && request.Credentials != "include"
		if request.Credentials == "include" {
			clean = clean && response.Headers.Get("Access-Control-Allow-Credentials") == "true"
		}
		if !clean {
			return false, fmt.Errorf("image CORS response disallowed")
		}
	}
	return clean, nil
}
