package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDynamicClassicScriptsRespectAsyncFalse(t *testing.T) {
	serialBrowserTest(t)
	second := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		switch r.URL.Path {
		case "/dependency.js":
			select {
			case <-second:
			case <-r.Context().Done():
				return
			}
			time.Sleep(75 * time.Millisecond)
			fmt.Fprint(w, `globalThis.Dependency=function(){this.value='ready'};order.push('dependency');queueMicrotask(()=>order.push('microtask'));`)
		case "/consumer.js":
			close(second)
			fmt.Fprint(w, `order.push(new Dependency().value);`)
		case "/failure.js":
			w.WriteHeader(404)
		default:
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<body></body>`)
		}
	}))
	defer server.Close()
	p := testPage(t)
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	result, err := p.Evaluate(ctx, `new Promise(resolve=>{
  globalThis.order=[];
  for(const src of ['/dependency.js','/failure.js','/consumer.js']){
   const s=document.createElement('script');s.async=false;s.src=src;
   s.onload=()=>{order.push('load:'+src);if(src==='/consumer.js')resolve(order.join('|'))};
   s.onerror=()=>order.push('error:'+src);
   document.head.appendChild(s);
  }
 })`)
	want := "dependency|microtask|load:/dependency.js|error:/failure.js|ready|load:/consumer.js"
	if err != nil || result != want {
		t.Fatalf("ordered execution: %v, %v; want %s", result, err, want)
	}
}

func TestDynamicDefaultAsyncDoesNotWaitForEarlierScript(t *testing.T) {
	serialBrowserTest(t)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow.js" {
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
		}
		w.Header().Set("Content-Type", "text/javascript")
		fmt.Fprint(w, `globalThis.arrivals.push(document.currentScript.id);`)
	}))
	defer server.Close()
	p := testPage(t)
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := p.Evaluate(ctx, fmt.Sprintf(`globalThis.arrivals=[];for(const name of ['slow','fast']){const s=document.createElement('script');s.id=name;s.src=%q+'/'+name+'.js';document.head.appendChild(s)}`, server.URL))
	if err != nil {
		close(release)
		t.Fatal(err)
	}
	var got any
	for ctx.Err() == nil {
		got, err = p.Evaluate(ctx, `arrivals.join('|')`)
		if got == "fast" || err != nil {
			break
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	if err != nil || got != "fast" {
		t.Fatalf("default async serialized: %v %v", got, err)
	}
}

func TestScriptAsyncCanonicalState(t *testing.T) {
	parallelBrowserTest(t)
	p := validationPage(t)
	defer p.Close()
	ctx := context.Background()
	value, err := p.Evaluate(ctx, `(()=>{const s=document.createElement('script');s.id='async-test';const fresh=s.async;s.setAttribute('async','');s.removeAttribute('async');const removed=s.async;s.async=false;const clone=s.cloneNode().async;const div=document.createElement('div');div.innerHTML='<script><\/script>';document.body.append(s);return fresh&&!removed&&!s.async&&clone&&!div.firstChild.async})()`)
	if err != nil || value != true {
		t.Fatalf("async state: %v %v", value, err)
	}
	world, err := p.IsolatedWorld(ctx, p.Top.ID, "scripts")
	if err != nil {
		t.Fatal(err)
	}
	debugger := NewDebugger(p)
	defer debugger.Close()
	got, err := debugger.Evaluate(ctx, p.Top.ID, world, `document.getElementById('async-test').async`, DebuggerOptions{ReturnByValue: true})
	if err != nil || got["result"].(map[string]any)["value"] != false {
		t.Fatalf("isolated script state: %v %v", got, err)
	}
}

func TestOrderedScriptsShareDocumentQueueAcrossWorlds(t *testing.T) {
	serialBrowserTest(t)
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		if r.URL.Path == "/first.js" {
			select {
			case <-ready:
			case <-r.Context().Done():
				return
			}
			time.Sleep(50 * time.Millisecond)
			fmt.Fprint(w, `globalThis.documentOrder=['first'];`)
		} else {
			close(ready)
			fmt.Fprint(w, `globalThis.documentOrder=(globalThis.documentOrder||[]).concat('second');`)
		}
	}))
	defer server.Close()
	p := validationPage(t)
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := p.Evaluate(ctx, fmt.Sprintf(`const first=document.createElement('script');first.async=false;first.src=%q+'/first.js';document.head.appendChild(first);`, server.URL))
	if err != nil {
		t.Fatal(err)
	}
	world, err := p.IsolatedWorld(ctx, p.Top.ID, "ordered-scripts")
	if err != nil {
		t.Fatal(err)
	}
	debugger := NewDebugger(p)
	defer debugger.Close()
	_, err = debugger.Evaluate(ctx, p.Top.ID, world, fmt.Sprintf(`const second=document.createElement('script');second.async=false;second.src=%q+'/second.js';document.head.appendChild(second);`, server.URL), DebuggerOptions{})
	if err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `new Promise(resolve=>{const poll=()=>{if(globalThis.documentOrder?.includes('second'))resolve(documentOrder.join('|'));else setTimeout(poll,1)};poll()})`)
	if err != nil || value != "first|second" {
		t.Fatalf("document-owned script order: %v %v", value, err)
	}
}
