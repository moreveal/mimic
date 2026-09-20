package browser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestFetchCanonicalBytesCloneAndCancellation(t *testing.T) {
	serialBrowserTest(t)
	canceled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/echo":
			body, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(body)
		case "/slow":
			<-r.Context().Done()
			select {
			case canceled <- struct{}{}:
			default:
			}
		default:
			fmt.Fprint(w, "<!doctype html><title>fetch fixture</title>")
		}
	}))
	defer server.Close()
	browser, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	contextState := browser.NewContext()
	defer contextState.Close()
	page, err := contextState.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = page.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := page.Evaluate(ctx, `(async()=>{
 const r=await fetch('/echo',{method:'POST',body:new Uint8Array([0,128,255,65])}),copy=r.clone();
 const before=[r.bodyUsed,copy.bodyUsed];const a=Array.from(await r.bytes()),b=Array.from(new Uint8Array(await copy.arrayBuffer()));
 let repeated=false;try{await r.text()}catch(e){repeated=e instanceof TypeError}
 const controller=new AbortController();controller.abort('preabort');let aborted=false;try{await fetch('/echo',{signal:controller.signal})}catch(e){aborted=e==='preabort'}
 return JSON.stringify({before,a,b,repeated,aborted,type:r.type});})()`)
	expected := `{"before":[false,false],"a":[0,128,255,65],"b":[0,128,255,65],"repeated":true,"aborted":true,"type":"basic"}`
	if err != nil || value != expected {
		t.Fatalf("binary/clone: %v %v", value, err)
	}
	value, err = page.Evaluate(ctx, `(async()=>{const c=new AbortController(),pending=fetch('/slow',{signal:c.signal});setTimeout(()=>c.abort('cancel-request'),50);try{await pending;return false}catch(e){return e==='cancel-request'}})()`)
	if err != nil || value != true {
		t.Fatalf("abort result: %v %v", value, err)
	}
	select {
	case <-canceled:
	case <-ctx.Done():
		t.Fatal("AbortSignal did not cancel canonical HTTP transport")
	}
}

func TestFetchBodyDisturbanceAndRequestTransfer(t *testing.T) {
	parallelBrowserTest(t)
	browser, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	contextState := browser.NewContext()
	defer contextState.Close()
	page, err := contextState.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	value, err := page.Evaluate(context.Background(), `(async()=>{
 const a=new Request('https://example.test/',{method:'POST',body:'abc'}),b=new Request(a);
 if(!a.bodyUsed||!a.body.locked||b.body===a.body||await b.text()!=='abc')return false;
 const response=new Response('x'),reader=response.body.getReader();await reader.read();reader.releaseLock();
 let rejected=false;try{response.clone()}catch(e){rejected=e instanceof TypeError}
 const empty=new Response();await empty.text();
 const c=new AbortController();let count=0;c.signal.onabort=()=>count++;c.abort();
 return response.bodyUsed&&rejected&&!empty.bodyUsed&&count===1;
 })()`)
	if err != nil || value != true {
		t.Fatalf("body semantic contract: %v %v", value, err)
	}
}

func TestFetchNetworkFailureIsTypeErrorAndBodyErrorIdentity(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := p.Evaluate(ctx, fmt.Sprintf(`(async()=>{
let network=false,body=false;try{await fetch(%q)}catch(e){network=e instanceof TypeError}
const reason={body:'sentinel'},stream=new ReadableStream({start(c){c.error(reason)}});
try{await fetch(%q,{method:'POST',body:stream,duplex:'half'})}catch(e){body=e===reason}
return network&&body;})()`, server.URL, server.URL))
	if err != nil || got != true {
		t.Fatalf("fetch rejection semantics: %v %v", got, err)
	}
}
