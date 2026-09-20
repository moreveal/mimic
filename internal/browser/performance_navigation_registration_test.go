package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Frozen Chrome distinguishes observer registration before and after load.
// Combining an initial getEntries snapshot with future observations therefore
// legitimately yields two navigation observations in the early case.
func TestPerformanceNavigationSnapshotAndObserverRegistration(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<!doctype html><script>window.seen=performance.getEntriesByType('navigation').map(e=>e.entryType);new PerformanceObserver(list=>seen.push(...list.getEntries().map(e=>e.entryType))).observe({entryTypes:['navigation']});</script>`)
		}))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `new Promise(resolve=>setTimeout(()=>resolve(seen.length===2&&seen.every(v=>v==='navigation')),30))`, true)
		historyEval(t, p, `new Promise(resolve=>{const late=performance.getEntriesByType('navigation').map(e=>e.entryType);const observer=new PerformanceObserver(list=>late.push(...list.getEntries().map(e=>e.entryType)));observer.observe({entryTypes:['navigation']});setTimeout(()=>{observer.disconnect();resolve(late.length===1&&late[0]==='navigation')},30)})`, true)
	})
}
