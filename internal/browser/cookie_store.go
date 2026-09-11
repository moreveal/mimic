package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/moreveal/mimic/internal/engine"
	"github.com/moreveal/mimic/internal/network"
	"github.com/moreveal/mimic/internal/scheduler"
)

func cookieStoreRow(row network.CookieSnapshot, deleted bool) map[string]any {
	c := row.Cookie
	var domain, expires any
	if !row.HostOnly {
		domain = c.Domain
	}
	if !c.Expires.IsZero() {
		expires = float64(c.Expires.UnixMilli())
	}
	sameSite := "lax"
	if c.SameSite == http.SameSiteStrictMode {
		sameSite = "strict"
	} else if c.SameSite == http.SameSiteNoneMode {
		sameSite = "none"
	}
	value := map[string]any{"name": c.Name, "domain": domain, "path": c.Path, "secure": c.Secure, "sameSite": sameSite, "partitioned": c.Partitioned}
	if !deleted {
		value["value"] = c.Value
		value["expires"] = expires
	}
	return value
}

func addCookieStoreHosts(r *Realm, h map[string]any) {
	h["cookieStoreRead"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		out := []map[string]any{}
		for _, row := range r.agent.Page().ctx.cookies.DocumentSnapshots(r.documentURL(), r.cookieContext()) {
			out = append(out, cookieStoreRow(row, false))
		}
		return r.val(out), nil
	})
	h["cookieStoreWrite"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		var data struct {
			Name, Value, Domain, Path, SameSite string
			Expires                             *float64
			Partitioned, Delete                 bool
		}
		if err := json.Unmarshal([]byte(strarg(a, 0)), &data); err != nil {
			return nil, err
		}
		cookie := http.Cookie{Name: data.Name, Value: data.Value, Domain: data.Domain, Path: data.Path, Secure: true, Partitioned: data.Partitioned, SameSite: http.SameSiteStrictMode}
		if data.SameSite == "lax" {
			cookie.SameSite = http.SameSiteLaxMode
		} else if data.SameSite == "none" {
			cookie.SameSite = http.SameSiteNoneMode
		}
		if data.Expires != nil {
			cookie.Expires = time.UnixMilli(int64(*data.Expires))
		}
		if data.Delete {
			cookie.MaxAge = -1
		}
		r.agent.Page().ctx.cookies.SetFromCookieStore(r.documentURL(), &cookie, r.cookieContext())
		return nil, nil
	})
	h["installCookieStoreObserver"] = r.fn(func(_ engine.Value, a []engine.Value) (engine.Value, error) {
		if r.cookieUnsubscribe != nil {
			r.cookieUnsubscribe()
		}
		r.cookieNotifier = a[0]
		r.cookieUnsubscribe = r.agent.Page().ctx.cookies.Subscribe(func(row network.CookieSnapshot, deleted bool) {
			r.scheduler.Post(scheduler.DOM, 0, func(ctx context.Context) error {
				if r.cookieNotifier == nil {
					return nil
				}
				if !row.Visible(r.documentURL(), deleted, r.cookieContext()) {
					return nil
				}
				_, err := r.runtime.Call(ctx, r.cookieNotifier, nil, r.val(cookieStoreRow(row, deleted)), r.val(deleted))
				return err
			})
		})
		return nil, nil
	})
}
