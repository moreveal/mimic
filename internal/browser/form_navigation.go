package browser

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/scheduler"
	"github.com/moreveal/mimic/internal/trace"
)

func (r *Realm) installFormNavigation(host map[string]any) {
	host["submitForm"] = r.fn(func(_ engine.Value, args []engine.Value) (engine.Value, error) {
		frame, ok := r.agent.(*Frame)
		if !ok || frame.parent != nil {
			return r.val("Nested form navigation is not implemented"), nil
		}
		if r.resourceContext.Err() != nil {
			return r.val(""), nil
		}
		target, err := r.resolveDocument(strarg(args, 0))
		if err != nil || target.Scheme != "http" && target.Scheme != "https" {
			return r.val("Only HTTP(S) form navigation is implemented"), nil
		}
		method := strarg(args, 1)
		if method != http.MethodGet && method != http.MethodPost {
			return r.val("Unsupported form method"), nil
		}
		p := r.agent.Page()
		documentURL := r.documentURL()
		allowed := p.cspBypassed() || r.contentPolicy().AllowsFormAction(documentURL, target)
		p.trace.Add(trace.CSP, "formActionDecision", map[string]any{"allowed": allowed, "url": target.String(), "realm": r.ID})
		if !allowed {
			return r.val(""), nil
		}
		request := network.Request{Method: method, SourceURL: documentURL, Referrer: documentURL, ReferrerPolicy: r.referrerPolicy, UserActivation: r.navigationActivated(), OpaqueOrigin: r.origin == "null"}
		if method == http.MethodPost {
			request.Body = []byte(strarg(args, 2))
			request.Headers = http.Header{"Content-Type": {strarg(args, 3)}}
		}
		r.scheduler.Post(scheduler.Navigation, 0, func(ctx context.Context) error {
			return p.beginNavigationRequest(ctx, target.String(), uuid.NewString(), request, 0, true)
		})
		return r.val(""), nil
	})
}
