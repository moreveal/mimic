package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNestedFrameOwnerLoadDrainsMicrotasks(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		child := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `<!doctype html><body><script>
(async()=>{
 const f=document.createElement('iframe'),log=[];
 f.srcdoc='<!doctype html><body>nested</body>';
 const ready=new Promise(resolve=>f.onload=()=>{log.push('load');queueMicrotask(()=>log.push('microtask'));resolve()});
 document.body.append(f);await ready;log.push('await');parent.postMessage(log.join(','),'*');
})()
</script>`)
		}))
		defer child.Close()
		parent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<!doctype html><body></body>") }))
		defer parent.Close()
		// Allow instrumented Goja bootstrap for all three realms; the tested
		// ordering still must complete without any unrelated timer in the page.
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, parent.URL); err != nil {
			t.Fatal(err)
		}
		value, err := p.Evaluate(ctx, fmt.Sprintf(`new Promise(resolve=>{const f=document.createElement('iframe');f.src=%q;addEventListener('message',e=>{if(e.source===f.contentWindow)resolve(e.data)});document.body.append(f)})`, child.URL))
		if err != nil {
			t.Fatal(err)
		}
		if value != "load,microtask,await" {
			t.Fatalf("nested owner continuation: %v", value)
		}
	})
}
