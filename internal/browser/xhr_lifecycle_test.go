package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func newXHRTestPage(t *testing.T, serverURL string) (*Page, context.Context) {
	t.Helper()
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	bc := b.NewContext()
	t.Cleanup(func() { _ = bc.Close() })
	p, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	if err := p.Navigate(ctx, serverURL); err != nil {
		t.Fatal(err)
	}
	return p, ctx
}

func TestXMLHttpRequestResponseTypesAndReuse(t *testing.T) {
	parallelBrowserTest(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"answer":42}`)
		case "/invalid":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{invalid`)
		case "/bytes":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte{0, 128, 255, 1})
		default:
			fmt.Fprint(w, "fixture")
		}
	}))
	defer srv.Close()
	p, ctx := newXHRTestPage(t, srv.URL)
	got, err := p.Evaluate(ctx, `(async()=>{
const load=(xhr,path,type)=>new Promise((resolve,reject)=>{xhr.open('GET',path);xhr.responseType=type;xhr.onload=()=>resolve(xhr.response);xhr.onerror=reject;xhr.send()});
const xhr=new XMLHttpRequest(),json=await load(xhr,'/json','json'),invalid=await load(xhr,'/invalid','json'),buffer=await load(xhr,'/bytes','arraybuffer'),blob=await load(xhr,'/bytes','blob');
let responseTextError=false;xhr.open('GET','/json');xhr.responseType='json';try{void xhr.responseText}catch(e){responseTextError=e.name==='InvalidStateError'}
return JSON.stringify({json,invalid,bytes:Array.from(new Uint8Array(buffer)),blob:[blob.type,blob.size],responseTextError})
})()`)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"json":{"answer":42},"invalid":null,"bytes":[0,128,255,1],"blob":["application/octet-stream",4],"responseTextError":true}`
	if got != want {
		t.Fatalf("XHR typed responses = %s, want %s", got, want)
	}
}

func TestXMLHttpRequestAbortCancelsTransportAndAllowsReuse(t *testing.T) {
	parallelBrowserTest(t)
	started := make(chan struct{}, 1)
	canceled := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			started <- struct{}{}
			<-r.Context().Done()
			canceled <- struct{}{}
			return
		}
		fmt.Fprint(w, "fresh")
	}))
	defer srv.Close()
	p, ctx := newXHRTestPage(t, srv.URL)
	type evaluation struct {
		value any
		err   error
	}
	result := make(chan evaluation, 1)
	go func() {
		value, err := p.Evaluate(ctx, `new Promise(resolve=>{
const xhr=new XMLHttpRequest(),events=[];
xhr.onreadystatechange=()=>events.push('rs:'+xhr.readyState);
xhr.onabort=()=>events.push('abort:'+xhr.readyState);
xhr.onload=()=>events.push('load:'+xhr.responseText);
xhr.onloadend=()=>{events.push('loadend:'+xhr.readyState);if(xhr.readyState===0){xhr.open('GET','/fresh');xhr.onloadend=()=>resolve({events,text:xhr.responseText,state:xhr.readyState,status:xhr.status});xhr.send()}};
xhr.open('GET','/slow');xhr.send();setTimeout(()=>xhr.abort(),10)
})`)
		result <- evaluation{value, err}
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("slow XHR never reached transport")
	}
	select {
	case <-canceled:
	case <-ctx.Done():
		t.Fatal("abort did not cancel the underlying request")
	}
	out := <-result
	if out.err != nil {
		t.Fatal(out.err)
	}
	value, ok := out.value.(map[string]any)
	if !ok || value["text"] != "fresh" || fmt.Sprint(value["state"]) != "4" || fmt.Sprint(value["status"]) != "200" {
		t.Fatalf("XHR reuse after abort = %#v", out.value)
	}
	events := fmt.Sprint(value["events"])
	if !strings.Contains(events, "abort:0") || strings.Contains(events, "load:") && !strings.Contains(events, "load:fresh") {
		t.Fatalf("XHR abort lifecycle = %s", events)
	}
}

func TestXMLHttpRequestAbortDuringHeadersStopsCompletionEvents(t *testing.T) {
	parallelBrowserTest(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "body")
	}))
	defer srv.Close()
	p, ctx := newXHRTestPage(t, srv.URL)
	got, err := p.Evaluate(ctx, `new Promise(resolve=>{
const xhr=new XMLHttpRequest(),events=[];
xhr.onreadystatechange=()=>{events.push('rs:'+xhr.readyState);if(xhr.readyState===2)xhr.abort()};
xhr.onabort=()=>events.push('abort:'+xhr.readyState);
xhr.onload=()=>events.push('load');
xhr.onloadend=()=>{events.push('loadend:'+xhr.readyState);resolve(events.join(','))};
xhr.open('GET','/body');xhr.send()
})`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "rs:1,rs:2,rs:4,abort:0,loadend:0" {
		t.Fatalf("abort from HEADERS_RECEIVED events = %q", got)
	}
}

func TestXMLHttpRequestTimeoutHasSingleTerminalLifecycle(t *testing.T) {
	parallelBrowserTest(t)
	canceled := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/timeout" {
			<-r.Context().Done()
			canceled <- struct{}{}
			return
		}
		fmt.Fprint(w, "fixture")
	}))
	defer srv.Close()
	p, ctx := newXHRTestPage(t, srv.URL)
	got, err := p.Evaluate(ctx, `new Promise(resolve=>{
const xhr=new XMLHttpRequest(),events=[];
xhr.onreadystatechange=()=>events.push('rs:'+xhr.readyState);
for(const name of ['load','error','abort','timeout'])xhr.addEventListener(name,()=>events.push(name));
xhr.onloadend=()=>resolve({events:events.join(','),state:xhr.readyState,status:xhr.status,response:xhr.response});
xhr.open('GET','/timeout');xhr.timeout=20;xhr.send()
})`)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-canceled:
	case <-ctx.Done():
		t.Fatal("timeout did not cancel the underlying request")
	}
	value, ok := got.(map[string]any)
	if !ok || value["events"] != "rs:1,rs:4,timeout" || fmt.Sprint(value["state"]) != "4" || fmt.Sprint(value["status"]) != "0" || value["response"] != "" {
		t.Fatalf("XHR timeout lifecycle = %#v", got)
	}
}
