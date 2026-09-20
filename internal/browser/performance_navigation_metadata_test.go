package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Frozen Chrome 152 local probes: Location.reload/history.go(0) create a
// reload entry; resources identify the navigation of their initiating document.
func TestPerformanceNavigationReasonAndResourceIdentity(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			fmt.Fprint(w, "<!doctype html><body>local timing fixture</body>")
		}))
		defer server.Close()
		// Multiple full realm bootstraps under -race exceed a short generic
		// evaluation watchdog. Assertions concern navigation state, not latency.
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		check := func(script string, expected any) {
			t.Helper()
			value, err := p.Evaluate(ctx, script)
			if err != nil || value != expected {
				t.Fatalf("navigation metadata: value=%v expected=%v error=%v", value, expected, err)
			}
		}
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		check(`performance.getEntriesByType('navigation')[0].type`, "navigate")
		checkResources := `(async()=>{await(await fetch('/resource')).text();const nav=performance.getEntriesByType('navigation')[0];const resource=performance.getEntriesByType('resource').find(e=>e.name.endsWith('/resource'));const id=nav.navigationId;history.replaceState(null,'','#state');return id>0&&resource.navigationId===id&&resource.toJSON().navigationId===id&&performance.getEntriesByType('navigation')[0].navigationId===id})()`
		check(checkResources, true)
		for _, step := range []struct{ script, kind string }{
			{`location.reload()`, "reload"},
			{`history.go(0)`, "reload"},
			{`location.replace(location.origin+'/replacement')`, "navigate"},
		} {
			old := p.Top.Realm
			if _, err := old.Evaluate(ctx, step.script, "local:navigation-reason"); err != nil {
				t.Fatal(err)
			}
			for p.Top.Realm == old || !p.LoadEventEnded() {
				if err := p.AdvanceTime(ctx, time.Millisecond); err != nil {
					t.Fatal(err)
				}
				if err := ctx.Err(); err != nil {
					t.Fatal(err)
				}
				time.Sleep(time.Millisecond)
			}
			check(`performance.getEntriesByType('navigation')[0].type`, step.kind)
			check(checkResources, true)
		}
		check(`(async()=>{
const f=document.createElement('iframe');f.src='/child';await new Promise(resolve=>{f.onload=resolve;document.body.append(f)});
const w=f.contentWindow,entry=w.performance.getEntriesByType('navigation')[0];
await new Promise(resolve=>{f.onload=resolve;w.location.reload()});
const next=w.performance.getEntriesByType('navigation')[0];
const result=entry.type==='navigate'&&next.type==='reload'&&next.navigationId>0&&performance.getEntriesByType('navigation')[0].type==='navigate';
f.remove();return result;
})()`, true)
	})
}
