package browser

import (
	"net/url"

	"github.com/moreveal/mimic/internal/network"
)

func (r *Realm) cookieContext() network.CookieContext {
	context := network.CookieContext{TopLevelSite: network.SchemefulSite(r.requestTopLevelURL())}
	frame, ok := r.agent.(*Frame)
	if !ok {
		return context
	}
	for current := frame; current != nil; current = current.parent {
		if current.Realm == nil {
			context.HasCrossSiteAncestor = true
			break
		}
		// Security origin, not about:blank/srcdoc URL, determines the inherited
		// site's relationship with its ancestors.
		origin, _ := url.Parse(current.Realm.origin)
		if network.SchemefulSite(origin) != context.TopLevelSite {
			context.HasCrossSiteAncestor = true
		}
	}
	return context
}
