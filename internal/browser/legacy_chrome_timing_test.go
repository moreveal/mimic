package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLegacyChromeCSIUsesDocumentClockAndLifecycle(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `<!doctype html><script>globalThis.earlyCSI=chrome.csi()</script><body>Timing</body>`)
		}))
		defer server.Close()
		ctx := context.Background()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		value, err := p.Evaluate(ctx, `(()=>{
			const c=chrome.csi(),timing=performance.timing;
			if(earlyCSI.onloadT!==0||!Number.isInteger(c.startE)||!Number.isInteger(c.onloadT)||c.onloadT<c.startE||Math.abs(c.onloadT-timing.domContentLoadedEventEnd)>1)return 'lifecycle';
			const original=performance.now;
			try{performance.now=()=>-1234;const next=chrome.csi();if(next.pageT<0||next.pageT<c.pageT)return 'author override'}finally{performance.now=original}
			let fine=false;
			for(let i=0;i<50;i++){const n=chrome.csi().pageT;fine ||= Math.abs(n*10-Math.round(n*10))>0.001}
			return fine ? 'ok' : 'clamped';
		})()`)
		if err != nil || value != "ok" {
			t.Fatalf("legacy CSI observation: %v (%v)", value, err)
		}
	})
}
