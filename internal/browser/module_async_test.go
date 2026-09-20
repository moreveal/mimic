package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func newAsyncModulePage(t *testing.T) *Page {
	t.Helper()
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	t.Cleanup(func() { c.Close() })
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStaticModuleGraphNetworkWaitLeavesPageResponsive(t *testing.T) {
	serialBrowserTest(t)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	var childRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		switch req.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<script>globalThis.events=[];document.addEventListener('DOMContentLoaded',()=>events.push('dcl'))</script><script type="module" src="/entry.js"></script><p id="parsed">ready</p>`)
		case "/entry.js":
			fmt.Fprint(w, `import {answer} from './child.js';export function get(){return 42};events.push('module:'+answer);globalThis.entryRuns=(globalThis.entryRuns||0)+1`)
		case "/child.js":
			if childRequests.Add(1) == 1 {
				close(started)
			}
			select {
			case <-release:
			case <-req.Context().Done():
				return
			}
			fmt.Fprint(w, `import {get} from './entry.js';export const answer=get()`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer func() { once.Do(func() { close(release) }); server.Close() }()
	p := newAsyncModulePage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	navigated := make(chan error, 1)
	go func() { navigated <- p.Navigate(ctx, server.URL) }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("dependency fetch did not start")
	}
	probeCtx, probeCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	value, err := p.Evaluate(probeCtx, `document.querySelector('#parsed').textContent+':'+events.length`)
	probeCancel()
	if err != nil || value != "ready:0" {
		t.Fatalf("Page blocked behind module dependency: %v %v", value, err)
	}
	once.Do(func() { close(release) })
	if err := <-navigated; err != nil {
		t.Fatal(err)
	}
	value, err = p.Evaluate(ctx, `events.join(',')+':'+entryRuns`)
	if err != nil || value != "module:42,dcl:1" {
		t.Fatalf("cyclic graph/lifecycle: %v %v", value, err)
	}
	if childRequests.Load() != 1 {
		t.Fatalf("dependency fetched %d times", childRequests.Load())
	}
}

func TestDynamicModuleGraphWaitsForTopLevelAwaitAndEvaluatesOnce(t *testing.T) {
	serialBrowserTest(t)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(release) })
	var dependencies atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		switch req.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<script type="module">globalThis.startImport=()=>import('/dynamic.js')</script>`)
		case "/dynamic.js":
			fmt.Fprint(w, `import {answer as value} from './slow.js';globalThis.dynamicRuns=(globalThis.dynamicRuns||0)+1;await new Promise(r=>globalThis.finishDynamic=r);export const answer=value`)
		case "/slow.js":
			if dependencies.Add(1) == 1 {
				close(started)
			}
			select {
			case <-release:
			case <-req.Context().Done():
				return
			}
			fmt.Fprint(w, `export const answer=42`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer func() { once.Do(func() { close(release) }); server.Close() }()
	p := newAsyncModulePage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	_, err := p.Evaluate(ctx, `globalThis.importSettled=false;globalThis.imports=Promise.all([startImport(),startImport()]).then(([a,b])=>{importSettled=true;return [a===b,a.answer,dynamicRuns].join(',')});'started'`)
	if err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case <-started:
			goto dependencyStarted
		default:
		}
		if err := p.AdvanceTime(ctx, time.Millisecond); err != nil {
			t.Fatal(err)
		}
		if ctx.Err() != nil {
			t.Fatal("dynamic dependency did not start")
		}
		time.Sleep(time.Millisecond)
	}
dependencyStarted:
	probeCtx, probeCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	value, err := p.Evaluate(probeCtx, `6*7`)
	probeCancel()
	if err != nil || value != float64(42) {
		t.Fatalf("Page blocked during dynamic import: %v %v", value, err)
	}
	once.Do(func() { close(release) })
	value, err = p.Evaluate(ctx, `new Promise(resolve=>{const check=()=>typeof finishDynamic==='function'?resolve(importSettled):setTimeout(check,0);check()})`)
	if err != nil || value != false {
		t.Fatalf("dynamic import settled before top-level await: %v %v", value, err)
	}
	value, err = p.Evaluate(ctx, `finishDynamic();imports`)
	if err != nil || value != "true,42,1" {
		t.Fatalf("dynamic namespace/evaluation identity: %v %v", value, err)
	}
	if dependencies.Load() != 1 {
		t.Fatalf("shared graph fetched %d times", dependencies.Load())
	}
}

func TestDynamicModuleFailuresAreSharedAndPreserveRejections(t *testing.T) {
	serialBrowserTest(t)
	var failedRequests, thrownRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		switch req.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<script type="module">globalThis.startImport=name=>import(name)</script>`)
		case "/missing.js":
			failedRequests.Add(1)
			w.WriteHeader(404)
		case "/throw.js":
			thrownRequests.Add(1)
			fmt.Fprint(w, `globalThis.throwRuns=(globalThis.throwRuns||0)+1;await Promise.resolve();throw new RangeError('async module sentinel')`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	p := newAsyncModulePage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `(async()=>{const failure=p=>p.then(()=> 'accepted',e=>e.name+':'+e.message);const missing=await Promise.all([startImport('/missing.js'),startImport('/missing.js')].map(failure));const rejected=await Promise.all([startImport('/throw.js'),startImport('/throw.js')].map(failure));return [missing.every(x=>x.startsWith('TypeError:')),rejected.join('|'),throwRuns].join(';')})()`)
	if err != nil || value != "true;RangeError:async module sentinel|RangeError:async module sentinel;1" {
		t.Fatalf("module failure identity: %v %v", value, err)
	}
	if failedRequests.Load() != 1 || thrownRequests.Load() != 1 {
		t.Fatalf("shared failures: missing=%d thrown=%d", failedRequests.Load(), thrownRequests.Load())
	}
}

func TestDynamicModuleGraphCanceledWithDocument(t *testing.T) {
	serialBrowserTest(t)
	started, canceled := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/slow.js" {
			close(started)
			<-req.Context().Done()
			close(canceled)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		if req.URL.Path == "/clean" {
			fmt.Fprint(w, `<body>replacement</body>`)
			return
		}
		fmt.Fprint(w, `<script type="module">globalThis.startImport=()=>import('/slow.js')</script>`)
	}))
	defer server.Close()
	p := newAsyncModulePage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Evaluate(ctx, `startImport().then(()=>globalThis.staleModule=true);'started'`); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case <-started:
			goto dependencyStarted
		default:
		}
		if err := p.AdvanceTime(ctx, time.Millisecond); err != nil {
			t.Fatal(err)
		}
		if ctx.Err() != nil {
			t.Fatal("import did not start")
		}
		time.Sleep(time.Millisecond)
	}
dependencyStarted:
	if err := p.Navigate(ctx, server.URL+"/clean"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-canceled:
	case <-ctx.Done():
		t.Fatal("retired module request was not canceled")
	}
	value, err := p.Evaluate(ctx, `document.body.textContent+':'+typeof staleModule`)
	if err != nil || value != "replacement:undefined" {
		t.Fatalf("stale module changed replacement: %v %v", value, err)
	}
}

func TestDynamicImportRemainsInItsParentOrChildRealm(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/shared.js" {
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `globalThis.moduleRuns=(globalThis.moduleRuns||0)+1;export const tag=globalThis.realmTag`)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		if req.URL.Path == "/child" {
			fmt.Fprint(w, `<script>globalThis.realmTag='child';globalThis.startImport=()=>import('/shared.js')</script>`)
			return
		}
		fmt.Fprint(w, `<script>globalThis.realmTag='parent';globalThis.startImport=()=>import('/shared.js')</script><iframe src="/child"></iframe>`)
	}))
	defer server.Close()
	p := newAsyncModulePage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `(async()=>{const parent=await startImport(),child=await frames[0].startImport(),again=await startImport();return [parent.tag,child.tag,parent===again,moduleRuns,frames[0].moduleRuns].join(',')})()`)
	if err != nil || value != "parent,child,true,1,1" {
		for _, event := range p.trace.Events() {
			if event.Kind == "exception" || event.Kind == "error" {
				t.Log(event.Name, event.Data)
			}
		}
		t.Log("module fetches", p.Top.Realm.moduleFetches)
		t.Fatalf("dynamic import crossed realm: %v %v", value, err)
	}
}

func TestDynamicModuleEvaluationUsesCurrentTaskCancellation(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		switch req.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<script>globalThis.startImport=()=>import('/loop.js')</script>`)
		case "/loop.js":
			fmt.Fprint(w, `import './dependency.js';globalThis.loopStarted=true;for(;;){}`)
		case "/dependency.js":
			fmt.Fprint(w, `export const ready=true`)
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	p := newAsyncModulePage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	limited, stop := context.WithTimeout(ctx, 100*time.Millisecond)
	_, err := p.Evaluate(limited, `startImport()`)
	stop()
	if err == nil {
		t.Fatal("unbounded module execution ignored cancellation")
	}
	value, err := p.Evaluate(ctx, `[loopStarted,6*7].join(',')`)
	if err != nil || value != "true,42" {
		for _, event := range p.trace.Events() {
			if event.Kind == "exception" || event.Kind == "error" {
				t.Log(event.Name, event.Data)
			}
		}
		t.Log("module fetches", p.Top.Realm.moduleFetches)
		t.Fatalf("Page did not recover after canceled module task: %v %v", value, err)
	}
}
