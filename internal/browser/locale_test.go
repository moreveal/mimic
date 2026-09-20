package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestProfileDateLocale(t *testing.T) {
	parallelBrowserTest(t)
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c, err := b.NewContextWithProfile([]byte(fmt.Sprintf(`{"schemaVersion":1,"baseProfile":%q,"locale":{"languages":["en-US","en"],"timezone":"America/New_York","intlLocale":"en-US"}}`, b.env.ProfileID)))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	for _, expr := range []string{
		`new Date('2026-01-01T12:00:00Z').getHours()===7`,
		`new Date('2026-07-01T12:00:00Z').getTimezoneOffset()===240`,
		`new Date(2026,2,8,2,30).toISOString()==='2026-03-08T07:30:00.000Z'`,
		`new Date(2026,10,1,1,30).toISOString()==='2026-11-01T05:30:00.000Z'`,
		`new Date('2026-03-08T02:30:00').toISOString()==='2026-03-08T07:30:00.000Z'`,
		`new Date('2026-01-01').toISOString()==='2026-01-01T00:00:00.000Z'`,
		`(()=>{const d=new Date('2026-03-08T06:30:00Z');d.setHours(2);return d.toISOString()==='2026-03-08T07:30:00.000Z'})()`,
		`new Date('2026-01-01T12:00:00Z').toString()==='Thu Jan 01 2026 07:00:00 GMT-0500 (Eastern Standard Time)'`,
		`new Date('2026-01-01T12:00:00Z').toLocaleString()==='1/1/2026, 7:00:00 AM'`,
		`Date.parse('2026-01-01T12:00:00')===1767286800000`,
		`(()=>{const d=new Date(123);d.valueOf=()=>456;return +new Date(d)===123})()`,
		`(()=>{class D extends Date{};return new D(2026,0,1) instanceof D})()`,
		`Intl.DateTimeFormat().resolvedOptions().timeZone==='America/New_York'`,
		`Temporal.Now.timeZoneId()==='America/New_York'&&Temporal.Now.zonedDateTimeISO().timeZoneId==='America/New_York'`,
		`Temporal.Instant.from('2026-01-01T12:00Z').toLocaleString()==='1/1/2026, 7:00:00 AM'`,
		`new Intl.NumberFormat().constructor===Intl.NumberFormat`,
	} {
		historyEval(t, p, expr, true)
	}
}

func TestProfileLocaleInheritanceAndConcurrentIsolation(t *testing.T) {
	parallelBrowserTest(t)
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []struct {
		zone, locale string
		offset       int
	}{{"America/New_York", "en-US", 300}, {"Asia/Tokyo", "ja-JP", -540}, {"Europe/Paris", "fr-FR", -60}} {
		t.Run(profile.zone, func(t *testing.T) {
			t.Parallel()
			c, err := b.NewContextWithProfile([]byte(fmt.Sprintf(`{"schemaVersion":1,"baseProfile":%q,"locale":{"timezone":%q,"intlLocale":%q}}`, b.env.ProfileID, profile.zone, profile.locale)))
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			p, err := c.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			check := fmt.Sprintf(`(()=>{for(let i=0;i<30;i++){if(new Date('2026-01-01T12:00Z').getTimezoneOffset()!==%d||new Intl.NumberFormat([]).resolvedOptions().locale!==%q)return false}return true})()`, profile.offset, profile.locale)
			historyEval(t, p, check, true)
			bootstrapSnapshotWarm(t, p)
			next, err := c.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			historyEval(t, next, check, true)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/worker.js" {
					w.Header().Set("Content-Type", "text/javascript")
					fmt.Fprintf(w, "postMessage(%s)", check)
				} else {
					fmt.Fprint(w, "<!doctype html><body>locale</body>")
				}
			}))
			defer server.Close()
			if err := next.Navigate(context.Background(), server.URL); err != nil {
				t.Fatal(err)
			}
			historyEval(t, next, check, true)
			historyEval(t, next, fmt.Sprintf(`(()=>{const f=document.createElement('iframe');document.body.appendChild(f);return f.contentWindow.eval(%q)})()`, check), true)
			historyEval(t, next, `new Promise((resolve,reject)=>{const w=new Worker('/worker.js');w.onmessage=e=>{w.terminate();resolve(e.data)};w.onerror=reject})`, true)
		})
	}
}
