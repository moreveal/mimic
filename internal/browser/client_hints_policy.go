package browser

import (
	"net/url"
	"strings"

	"github.com/moreveal/mimic/internal/network"
)

var clientHintFeatures = []string{"ch-ua-arch", "ch-ua-bitness", "ch-ua-full-version", "ch-ua-full-version-list", "ch-ua-model", "ch-ua-platform-version", "ch-ua-form-factors", "ch-ua-wow64"}

// Only the high-entropy hints implemented by Environment are projected here.
// Policies are captured when a document commits, so network goroutines never
// read live DOM or a replacement realm's state.
type clientHintsDocument struct {
	top     *url.URL
	origin  string
	policy  map[string][]string
	enabled map[string]bool
}

func hintAllowlist(value, self, src string) []string {
	var out []string
	for _, item := range strings.Fields(strings.Trim(value, "()")) {
		item = strings.Trim(item, "\"'")
		switch item {
		case "self":
			item = self
		case "src":
			item = src
		case "none":
			continue
		}
		if item == "*" || strings.Contains(item, "://") {
			out = append(out, item)
		}
	}
	return out
}

func hintPolicy(header, self string) map[string][]string {
	result := map[string][]string{}
	for _, directive := range strings.Split(header, ",") {
		name, value, ok := strings.Cut(directive, "=")
		if ok {
			result[strings.ToLower(strings.TrimSpace(name))] = hintAllowlist(strings.TrimSpace(value), self, "")
		}
	}
	return result
}

func hintContainer(header, self, src string) map[string][]string {
	result := map[string][]string{}
	for _, directive := range strings.Split(header, ";") {
		parts := strings.Fields(directive)
		if len(parts) == 0 {
			continue
		}
		value := "src"
		if len(parts) > 1 {
			value = strings.Join(parts[1:], " ")
		}
		result[parts[0]] = hintAllowlist(value, self, src)
	}
	return result
}

func hintAllows(list []string, origin string) bool {
	for _, allowed := range list {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

func (r *Realm) initializeClientHints(header string) {
	security := r.securityState()
	security.permissionsPolicy = header
	if allow, declared := hintPolicy(header, r.origin)["cross-origin-isolated"]; declared && !hintAllows(allow, r.origin) {
		security.crossOriginIsolated = false
	}
	r.documentSecurity = &security
	origin := r.origin
	state := &clientHintsDocument{top: r.url, origin: origin, policy: hintPolicy(header, origin), enabled: map[string]bool{}}
	for _, feature := range clientHintFeatures {
		state.enabled[feature] = true
	}
	if frame, ok := r.agent.(*Frame); ok && frame.parent != nil && frame.parent.Realm != nil {
		parent := frame.parent.Realm
		if parent.clientHints != nil {
			state.top = parent.clientHints.top
			container := parent.frameClientHintsContainer(frame, origin)
			for _, feature := range clientHintFeatures {
				policy, declared := parent.clientHints.policy[feature]
				allow, specified := container[feature]
				delegated := origin == parent.origin || declared && hintAllows(policy, origin)
				if specified {
					delegated = hintAllows(allow, origin)
				}
				state.enabled[feature] = parent.clientHints.enabled[feature] && (!declared || hintAllows(policy, origin)) && delegated
			}
		}
	}
	r.clientHints = state
}

func (r *Realm) frameClientHintsContainer(frame *Frame, target string) map[string][]string {
	if node, ok := r.document.Get(frame.elementID); ok {
		src := target
		if raw := node.Attributes["src"]; raw != "" {
			if u, err := r.resolveDocument(raw); err == nil && u.Host != "" {
				src = originOf(u.String())
			}
		}
		return hintContainer(node.Attributes["allow"], r.origin, src)
	}
	return map[string][]string{}
}

func (r *Realm) applyClientHints(request *network.Request) {
	request.TopLevelURL = r.requestTopLevelURL()
	request.OpaqueOrigin = r.origin == "null"
	if !request.OpaqueOrigin {
		request.SourceOrigin, _ = url.Parse(r.origin)
	}
	request.HasCrossSiteAncestor = r.cookieContext().HasCrossSiteAncestor
	if r.clientHints == nil {
		return
	}
	state := r.clientHints
	projection := &network.ClientHintsContext{TopLevelURL: state.top, Allowed: map[string][]string{}}
	for _, feature := range clientHintFeatures {
		if !state.enabled[feature] {
			continue
		}
		list, specified := state.policy[feature]
		if !specified {
			list = []string{state.origin}
		}
		projection.Allowed["sec-"+feature] = append([]string(nil), list...)
	}
	request.ClientHints = projection
}

func (r *Realm) applyChildNavigationClientHints(request *network.Request, frame *Frame, target *url.URL) {
	r.applyClientHints(request)
	if request.ClientHints == nil {
		return
	}
	// Chrome applies the iframe container opt-in to the navigation request;
	// ancestor header restrictions still apply to the committed document.
	for feature, allowed := range r.frameClientHintsContainer(frame, originOf(target.String())) {
		request.ClientHints.Allowed["sec-"+feature] = allowed
	}
}

// Capture the committed document context before dispatching network work.
func (r *Realm) requestTopLevelURL() *url.URL {
	if r.clientHints != nil {
		return r.clientHints.top
	}
	return r.documentURL()
}
