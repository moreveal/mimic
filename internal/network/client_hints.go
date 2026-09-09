package network

import "net/url"

// ClientHintsContext is a request-owned projection of its document's effective
// policy. The opt-in belongs to the top-level origin; permissions belong to the
// initiating document. Nil keeps the top-level navigation lookup behavior.
type ClientHintsContext struct {
	TopLevelURL *url.URL
	Allowed     map[string][]string
}

func (l *Loader) acceptedClientHints(r Request) map[string]bool {
	if r.OmitClientHints {
		return nil
	}
	if r.ClientHints == nil {
		return l.session.ClientHints(r.URL)
	}
	accepted := l.session.ClientHints(r.ClientHints.TopLevelURL)
	origin := r.URL.Scheme + "://" + r.URL.Host
	for hint := range accepted {
		allowed := false
		for _, value := range r.ClientHints.Allowed[hint] {
			if value == "*" || value == origin {
				allowed = true
				break
			}
		}
		if !allowed {
			delete(accepted, hint)
		}
	}
	return accepted
}
