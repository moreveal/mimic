package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/moreveal/mimic/internal/network"
)

func TestPartitionCookiesFrameAndWorkerTransport(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		var own string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/echo":
				fmt.Fprint(w, r.Header.Get("Cookie"))
			case "/middle":
				fmt.Fprintf(w, `<!doctype html><body><script>onmessage=e=>parent.postMessage(e.data,'*');const f=document.createElement('iframe');f.src=%s;document.body.append(f)</script>`, strconv.Quote(own+"/inner"))
			case "/inner":
				fmt.Fprint(w, `<!doctype html><body><script>(async()=>{
const before=document.cookie;document.cookie='id=nested; Secure; SameSite=None; Partitioned; Path=/';
const documentValue=document.cookie,wire=await(await fetch('/echo')).text();
const worker=new Worker('/worker.js');
const workerValue=await new Promise((resolve,reject)=>{worker.onmessage=e=>resolve(e.data);worker.onerror=e=>reject(new Error(e.message));worker.postMessage(null)});
worker.terminate();parent.postMessage({before,documentValue,wire,workerValue},'*');
})().catch(e=>parent.postMessage({error:String(e)},'*'))</script>`)
			case "/worker.js":
				w.Header().Set("Content-Type", "text/javascript")
				// Verify the script load and its subsequent fetch use the same key.
				fmt.Fprintf(w, `const scriptCookie=%s;onmessage=async()=>postMessage({script:scriptCookie,fetch:await(await fetch('/echo')).text()})`, strconv.Quote(r.Header.Get("Cookie")))
			default:
				fmt.Fprint(w, "<!doctype html><body></body>")
			}
		}))
		defer server.Close()
		own = server.URL
		if err := p.Navigate(context.Background(), own); err != nil {
			t.Fatal(err)
		}
		p.Cookies().SetFromResponse(p.Top.Realm.documentURL(), http.Header{"Set-Cookie": {"secret=top; Secure; HttpOnly; SameSite=None; Partitioned; Path=/"}}, network.CookieContext{TopLevelSite: "http://127.0.0.1"})
		historyEval(t, p, `document.cookie='id=top; Secure; SameSite=None; Partitioned; Path=/';document.cookie='shared=1; Secure; SameSite=None; Path=/';true`, true)
		middle := strings.Replace(own, "127.0.0.1", "localhost", 1) + "/middle"
		script := `new Promise(resolve=>{const f=document.createElement('iframe');const listener=e=>{if(e.source!==f.contentWindow)return;removeEventListener('message',listener);const v=e.data;f.remove();resolve(v.before==='shared=1'&&v.documentValue==='shared=1; id=nested'&&v.wire==='shared=1; id=nested'&&v.workerValue.script===v.wire&&v.workerValue.fetch===v.wire&&document.cookie==='id=top; shared=1')};addEventListener('message',listener);f.src=` + strconv.Quote(middle) + `;document.body.append(f)})`
		historyEval(t, p, script, true)
	})
}
