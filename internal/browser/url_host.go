package browser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/moreveal/mimic/internal/engine"
)

func resolveURL(base *url.URL, raw string) (*url.URL, error) {
	reference, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if !reference.IsAbs() && base.Opaque != "" && !strings.HasPrefix(raw, "#") {
		return nil, fmt.Errorf("cannot resolve a relative URL against an opaque base")
	}
	return base.ResolveReference(reference), nil
}

func installURLHost(host map[string]any, runtime engine.Runtime, baseURL func() *url.URL) {
	host["urlParts"] = runtime.Function(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		base := baseURL()
		if len(a) > 1 && strarg(a, 1) == "" {
			reference, err := url.Parse(strarg(a, 0))
			if err != nil || !reference.IsAbs() {
				return nil, fmt.Errorf("invalid absolute URL")
			}
		}
		if len(a) > 1 && strarg(a, 1) != "" {
			var err error
			base, err = url.Parse(strarg(a, 1))
			if err != nil || !base.IsAbs() {
				return nil, fmt.Errorf("invalid base URL")
			}
		}
		// Explicit bases come from URL/Request constructors. The one-argument
		// path is also used by existing element URL reflection, whose fallback
		// semantics are separate from constructor failure behavior.
		var u *url.URL
		var err error
		if len(a) > 1 {
			u, err = resolveURL(base, strarg(a, 0))
		} else {
			u, err = base.Parse(strarg(a, 0))
		}
		if err != nil {
			return nil, err
		}
		search, hash := "", ""
		if u.RawQuery != "" {
			search = "?" + u.RawQuery
		}
		if u.Fragment != "" {
			hash = "#" + u.Fragment
		}
		origin := "null"
		if u.Scheme == "http" || u.Scheme == "https" {
			origin = u.Scheme + "://" + u.Host
		}
		return runtime.Value(map[string]any{"href": u.String(), "origin": origin, "protocol": u.Scheme + ":", "username": usernameOf(u), "password": passwordOf(u), "host": u.Host, "hostname": u.Hostname(), "port": u.Port(), "pathname": u.EscapedPath(), "search": search, "hash": hash}), nil
	})
	host["setURLPart"] = runtime.Function(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		u, err := url.Parse(strarg(a, 0))
		if err != nil {
			return nil, err
		}
		part, value := strarg(a, 1), strarg(a, 2)
		switch part {
		case "protocol":
			u.Scheme = strings.TrimSuffix(value, ":")
		case "username":
			password := passwordOf(u)
			u.User = url.UserPassword(value, password)
		case "password":
			u.User = url.UserPassword(usernameOf(u), value)
		case "host":
			u.Host = value
		case "hostname":
			port := u.Port()
			u.Host = value
			if port != "" {
				u.Host += ":" + port
			}
		case "port":
			u.Host = u.Hostname()
			if value != "" {
				u.Host += ":" + value
			}
		case "pathname":
			u.Path, u.RawPath = value, ""
		case "search":
			u.RawQuery = strings.TrimPrefix(value, "?")
		case "hash":
			u.Fragment = strings.TrimPrefix(value, "#")
		}
		return runtime.Value(u.String()), nil
	})
}
