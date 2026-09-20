package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestChildDocumentInheritsDomainAndNavigationReferrer(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<!doctype html><body></body>") }))
		defer server.Close()
		// This semantic probe boots several Goja realms under -race. The
		// watchdog bounds hangs; elapsed bootstrap time is not its assertion.
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, server.URL+"/parent?q=1"); err != nil {
			t.Fatal(err)
		}
		value, err := p.Evaluate(ctx, `(async()=>{
for(const policy of ['', 'no-referrer']){
 const f=document.createElement('iframe');f.referrerPolicy=policy;document.body.append(f);
 if(f.contentDocument.domain!==location.hostname||f.contentDocument.referrer!==location.href)throw new Error('initial inheritance');
 for(const kind of ['blank','srcdoc','network']){
  const ready=new Promise(resolve=>f.onload=resolve);
  if(kind==='blank')f.src='about:blank';else if(kind==='srcdoc')f.srcdoc='<!doctype html><body>inline</body>';else {f.removeAttribute('srcdoc');f.src='/child'}
  await ready;
  const expected=policy==='no-referrer'?'':kind==='network'?location.href:location.origin+'/';
  if(f.contentDocument.domain!==location.hostname||f.contentDocument.referrer!==expected)throw new Error(JSON.stringify({kind,domain:f.contentDocument.domain,referrer:f.contentDocument.referrer,expected}));
 }
 f.remove();
}
return document.referrer==='';
})()`)
		if err != nil || value != true {
			t.Fatalf("document inheritance: %v %v", value, err)
		}
	})
}

func TestCrossOriginIsolationHonorsDelegation(t *testing.T) {
	parallelBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		child := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
			w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
			if r.URL.Path == "/deny" {
				w.Header().Set("Permissions-Policy", "cross-origin-isolated=()")
			}
			fmt.Fprint(w, `<!doctype html><body><script>const blank=document.createElement('iframe');document.body.append(blank);parent.postMessage({isolated:crossOriginIsolated,blank:blank.contentWindow.crossOriginIsolated},'*')</script>`)
		}))
		defer child.Close()
		parent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
			w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
			fmt.Fprint(w, "<!doctype html><body></body>")
		}))
		defer parent.Close()
		if err := p.Navigate(context.Background(), parent.URL); err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct {
			path, allow string
			expected    bool
		}{{"/child", "", false}, {"/child", "cross-origin-isolated", true}, {"/deny", "cross-origin-isolated", false}} {
			historyEval(t, p, fmt.Sprintf(`(async()=>{const f=document.createElement('iframe');f.allow=%q;f.src=%q;const result=await new Promise(resolve=>{const listener=e=>{if(e.source!==f.contentWindow)return;removeEventListener('message',listener);resolve(e.data)};addEventListener('message',listener);document.body.append(f)});f.remove();return result.isolated===%t&&result.blank===%t})()`, c.allow, child.URL+c.path, c.expected, c.expected), true)
		}
	})
}
