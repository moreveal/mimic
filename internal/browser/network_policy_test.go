package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Measured with Chrome 152.0.7977.82: Page overrides include child frames and
// workers, without changing another Page in the same browser context. Windows
// receive trusted connectivity events; workers observe the live navigator flag.
func TestPageNetworkPolicyIncludesFramesWorldsAndWorkers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		switch r.URL.Path {
		case "/echo":
			fmt.Fprint(w, r.Header.Get("X-Page"))
		case "/worker.js":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `onmessage=async()=>{let wire;try{wire=await fetch('/echo').then(r=>r.text())}catch{wire='offline'}postMessage(navigator.onLine+':'+wire)}`)
		default:
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<!doctype html><body>network policy</body>")
		}
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, a *Page) {
		b, err := a.ctx.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		defer b.Close()
		a.NetworkPolicy().SetExtraHeaders(http.Header{"X-Page": {"one"}})
		b.NetworkPolicy().SetExtraHeaders(http.Header{"X-Page": {"two"}})
		for _, page := range []*Page{a, b} {
			if err := page.Navigate(context.Background(), server.URL); err != nil {
				t.Fatal(err)
			}
		}
		historyEval(t, a, `fetch('/echo').then(r=>r.text())`, "one")
		historyEval(t, b, `fetch('/echo').then(r=>r.text())`, "two")
		historyEval(t, a, `globalThis.connectivity=[];for(const type of ['offline','online'])addEventListener(type,e=>connectivity.push([e.type,e.isTrusted,e.bubbles,e.cancelable,e.target===window].join(':')));globalThis.child=document.createElement('iframe');child.src='/frame';(async()=>{await new Promise(resolve=>{child.onload=resolve;document.body.append(child)});return await child.contentWindow.fetch('/echo').then(r=>r.text())})()`, "one")
		historyEval(t, a, `globalThis.worker=new Worker('/worker.js');new Promise((resolve,reject)=>{worker.onmessage=e=>resolve(e.data);worker.onerror=e=>reject(Error(e.message));worker.postMessage(null)})`, "true:one")
		d := NewDebugger(a)
		defer d.Close()
		world, err := a.IsolatedWorld(context.Background(), a.Top.ID, "network-policy")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.Evaluate(context.Background(), a.Top.ID, world, `globalThis.events=[];addEventListener('offline',e=>events.push(e.type+':'+e.isTrusted))`, DebuggerOptions{}); err != nil {
			t.Fatal(err)
		}
		a.SetNetworkOffline(true)
		if err := a.AdvanceTime(context.Background(), 0); err != nil {
			t.Fatal(err)
		}
		historyEval(t, a, `[navigator.onLine,child.contentWindow.navigator.onLine].join(',')+'|'+connectivity.join(',')`, "false,false|offline:true:false:false:true")
		historyEval(t, a, `new Promise(resolve=>{worker.onmessage=e=>resolve(e.data);worker.postMessage(null)})`, "false:offline")
		historyEval(t, a, `Promise.all([fetch('data:text/plain,data').then(r=>r.text()),fetch(URL.createObjectURL(new Blob(['blob']))).then(r=>r.text())]).then(v=>v.join(','))`, "data,blob")
		historyEval(t, b, `navigator.onLine&&fetch('/echo').then(r=>r.text())`, "two")
		result, err := d.Evaluate(context.Background(), a.Top.ID, world, `navigator.onLine+':'+events.join(',')`, DebuggerOptions{})
		if err != nil || result["result"].(map[string]any)["value"] != "false:offline:true" {
			t.Fatalf("isolated connectivity: %#v %v", result, err)
		}
		a.SetNetworkOffline(true)
		a.SetNetworkOffline(false)
		if err := a.AdvanceTime(context.Background(), 0); err != nil {
			t.Fatal(err)
		}
		historyEval(t, a, `connectivity.join(',')`, "offline:true:false:false:true,online:true:false:false:true")
		historyEval(t, a, `new Promise(resolve=>{worker.onmessage=e=>resolve(e.data);worker.postMessage(null)})`, "true:one")
		historyEval(t, a, `worker.terminate();typeof __mimicNetworkStateEvent`, "undefined")
	})
}
