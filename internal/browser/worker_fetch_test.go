package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

const startFetchWorker = `new Promise((resolve,reject)=>{globalThis.fetchWorker=new Worker('/start.js');fetchWorker.onmessage=e=>resolve(e.data);fetchWorker.onerror=e=>reject(new Error(e.message));fetchWorker.postMessage('start')})`

func TestBlobWorkerFetchHasOpaqueBaseAndInheritedSourceOrigin(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/echo" {
				_ = json.NewEncoder(w).Encode(map[string]string{"referer": r.Referer(), "site": r.Header.Get("Sec-Fetch-Site")})
				return
			}
			if r.URL.Path == "/relative" || r.URL.Path == "/root" {
				t.Error("blob worker inherited a document base URL")
			}
			fmt.Fprint(w, "<!doctype html><body>fixture</body>")
		}))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL+"/page"); err != nil {
			t.Fatal(err)
		}
		source := `onmessage=async e=>{try{let invalid=0;for(const value of ['relative','/root']){try{await fetch(value)}catch(error){if(error instanceof TypeError)invalid++}}const response=await fetch(e.data);const result=await response.json();postMessage(invalid===2&&result.referer===''&&result.site==='same-origin')}catch(error){postMessage(String(error))}}`
		script := `new Promise((resolve,reject)=>{const url=URL.createObjectURL(new Blob([` + strconv.Quote(source) + `]));const worker=new Worker(url);worker.onerror=e=>reject(new Error(e.message));worker.onmessage=e=>{worker.terminate();URL.revokeObjectURL(url);resolve(e.data)};worker.postMessage(` + strconv.Quote(server.URL+"/echo") + `)})`
		historyEval(t, p, script, true)
	})
}

func TestWorkerFetchUsesFinalScriptBaseAndSharedTransport(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/start.js":
				http.Redirect(w, r, "/dir/worker.js", http.StatusFound)
			case "/dir/worker.js":
				w.Header().Set("Content-Type", "text/javascript")
				fmt.Fprint(w, `onmessage=async()=>{try{const request=new Request('echo',{method:'POST',headers:{'X-Worker':'yes'},body:new URLSearchParams({a:'hello world'})});const response=await fetch(request);const copy=response.clone();let immutable=false;try{response.headers.set('x-test','bad')}catch(e){immutable=e instanceof TypeError}const payload=await response.json();postMessage(JSON.stringify({payload,location:location.href,url:response.url,type:response.type,bodyUsed:response.bodyUsed,immutable,clone:await copy.json()}))}catch(e){postMessage(JSON.stringify({error:String(e)}))}}`)
			case "/dir/echo":
				body, _ := io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"path": r.URL.Path, "method": r.Method, "body": string(body), "type": r.Header.Get("Content-Type"), "header": r.Header.Get("X-Worker"), "cookie": r.Header.Get("Cookie"), "referer": r.Referer()})
			default:
				w.Header().Set("Set-Cookie", "shared=1; Path=/")
				fmt.Fprint(w, "<!doctype html><body>fixture</body>")
			}
		}))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL+"/page"); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, startFetchWorker)
		if err != nil {
			t.Fatal(err)
		}
		var got struct {
			Payload                    map[string]string
			Location, URL, Type, Error string
			BodyUsed, Immutable        bool
			Clone                      map[string]string
		}
		if err := json.Unmarshal([]byte(value.(string)), &got); err != nil {
			t.Fatal(err)
		}
		if got.Error != "" || got.Location != server.URL+"/dir/worker.js" || got.URL != server.URL+"/dir/echo" || !got.BodyUsed || !got.Immutable || got.Type != "basic" {
			t.Fatalf("worker response: %#v", got)
		}
		for name, expected := range map[string]string{"path": "/dir/echo", "method": "POST", "body": "a=hello+world", "type": "application/x-www-form-urlencoded;charset=UTF-8", "header": "yes", "cookie": "shared=1", "referer": server.URL + "/dir/worker.js"} {
			if got.Payload[name] != expected || got.Clone[name] != expected {
				t.Fatalf("%s: payload=%q clone=%q expected=%q", name, got.Payload[name], got.Clone[name], expected)
			}
		}
	})
}

func TestWorkerFetchAbortIsolatedAndPreservesReason(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		started, canceled := make(chan struct{}), make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/start.js":
				w.Header().Set("Content-Type", "text/javascript")
				fmt.Fprint(w, `const active=new AbortController(),other=new AbortController(),reason={custom:true};let otherEvents=0;other.signal.addEventListener('abort',()=>otherEvents++);onmessage=async e=>{if(e.data==='abort'){active.abort(reason);return}let pre=false;const prior=new AbortController();prior.abort(reason);try{await fetch('/unused',{signal:prior.signal})}catch(e){pre=e===reason}fetch('/pending',{signal:active.signal}).then(()=>postMessage(false),async e=>{const response=await fetch('/ok',{signal:other.signal});postMessage(pre&&e===reason&&otherEvents===0&&!other.signal.aborted&&await response.text()==='ok')});postMessage('started')}`)
			case "/pending":
				close(started)
				<-r.Context().Done()
				close(canceled)
			case "/ok":
				fmt.Fprint(w, "ok")
			case "/unused":
				t.Error("pre-aborted request reached transport")
			default:
				fmt.Fprint(w, "<!doctype html><body>fixture</body>")
			}
		}))
		defer server.Close()
		defer p.Close()
		if err := p.Navigate(context.Background(), server.URL+"/page"); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, startFetchWorker, "started")
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("worker fetch never started")
		}
		historyEval(t, p, `new Promise(resolve=>{fetchWorker.onmessage=e=>resolve(e.data);fetchWorker.postMessage('abort')})`, true)
		select {
		case <-canceled:
		case <-time.After(time.Second):
			t.Fatal("worker abort did not cancel transport")
		}
	})
}

func TestWorkerFetchTeardownCancelsTransport(t *testing.T) {
	for _, mode := range []string{"terminate", "page-close", "self-close"} {
		t.Run(mode, func(t *testing.T) {
			historyTestPages(t, func(t *testing.T, p *Page) {
				started, canceled := make(chan struct{}), make(chan struct{})
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/start.js":
						w.Header().Set("Content-Type", "text/javascript")
						fmt.Fprint(w, `onmessage=e=>{if(e.data==='close'){close();return}fetch('/pending');postMessage('started')}`)
					case "/pending":
						close(started)
						<-r.Context().Done()
						close(canceled)
					default:
						fmt.Fprint(w, "<!doctype html><body>fixture</body>")
					}
				}))
				defer server.Close()
				defer p.Close()
				if err := p.Navigate(context.Background(), server.URL+"/page"); err != nil {
					t.Fatal(err)
				}
				historyEval(t, p, startFetchWorker, "started")
				select {
				case <-started:
				case <-time.After(2 * time.Second):
					t.Fatal("worker fetch never started")
				}
				if mode == "terminate" {
					historyEval(t, p, `fetchWorker.terminate();true`, true)
				} else if mode == "self-close" {
					historyEval(t, p, `fetchWorker.postMessage('close');true`, true)
				} else if err := p.Close(); err != nil {
					t.Fatal(err)
				}
				select {
				case <-canceled:
				case <-time.After(time.Second):
					t.Fatal("teardown did not cancel transport")
				}
			})
		})
	}
}
