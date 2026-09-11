package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// These are causal clock/ownership relations, not exact wall-clock durations.
// They hold in frozen Chrome for both fresh and cached resource retrievals.
func TestResourceTimingUsesInitiatingRealmClock(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/resource" {
				time.Sleep(25 * time.Millisecond)
				w.Header().Set("Cache-Control", "max-age=600")
			} else {
				w.Header().Set("Cache-Control", "no-store")
			}
			fmt.Fprint(w, "<!doctype html><body></body>")
		}))
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		for cycle := 0; cycle < 2; cycle++ {
			if err := p.Navigate(ctx, server.URL); err != nil {
				t.Fatal(err)
			}
			if err := p.AdvanceTime(ctx, 10*time.Second); err != nil {
				t.Fatal(err)
			}
			historyEval(t, p, `(async()=>{
const before=performance.now();await(await fetch('/resource')).text();
const now=performance.now(),entries=performance.getEntriesByType('resource').filter(e=>e.name.endsWith('/resource'));
if(entries.length!==1)throw new Error('resource count: '+entries.length);
const e=entries[0],nav=performance.getEntriesByType('navigation')[0];
if(!(e.startTime>=before-1&&e.startTime<=e.responseEnd&&e.responseEnd<=now+1))throw new Error(JSON.stringify({before,now,entry:e.toJSON()}));
return e.navigationId===nav.navigationId&&nav.toJSON().navigationId===nav.navigationId&&e.secureConnectionStart===0;
})()`, true)
			if cycle == 1 {
				historyEval(t, p, `(()=>{const e=performance.getEntriesByType('resource').find(e=>e.name.endsWith('/resource'));return e.deliveryType==='cache'&&e.nextHopProtocol===''&&e.transferSize===0&&e.domainLookupStart===e.fetchStart&&e.connectEnd===e.fetchStart})()`, true)
			}
		}
	})
}
