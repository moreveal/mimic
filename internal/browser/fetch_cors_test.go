package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestFetchCORSWindowAndWorker(t *testing.T) {
	serialBrowserTest(t)
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/allow" {
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		}
		if r.URL.Path == "/credentials" {
			w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		if r.URL.Path == "/wildcard" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		w.Header().Set("X-Hidden", "secret")
		fmt.Fprint(w, "remote body")
	}))
	defer remote.Close()
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "<!doctype html><title>CORS fixture</title>")
	}))
	defer local.Close()
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	bc := b.NewContext()
	defer bc.Close()
	p, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	opaque, opaqueErr := p.Evaluate(ctx, `(async()=>{const base=`+strconv.Quote(remote.URL)+`;try{await fetch(base+'/denied');return 'opaque origin accepted'}catch(e){if(!(e instanceof TypeError))return 'wrong error'}const r=await fetch(base+'/allow');return r.type==='cors'&&await r.text()==='remote body'})()`)
	if opaqueErr != nil || opaque != true {
		t.Fatalf("opaque origin=%#v err=%v", opaque, opaqueErr)
	}
	if err = p.Navigate(ctx, local.URL); err != nil {
		t.Fatal(err)
	}
	probe := `async function run(base){
const reject=async(path,options)=>{try{await fetch(base+path,options);return false}catch(e){return e instanceof TypeError}};
if(!await reject('/denied'))return 'missing ACAO';
if(!await reject('/wildcard',{credentials:'include'}))return 'credential wildcard';
if(!await reject('/allow',{credentials:'include'}))return 'missing ACAC';
if(!await reject('/allow',{mode:'same-origin'}))return 'same-origin';
for(const [path,options]of [['/allow',{}],['/credentials',{credentials:'include'}],['/wildcard',{credentials:'omit'}]]){const r=await fetch(base+path,options);if(r.type!=='cors'||r.status!==200||r.headers.has('x-hidden')||await r.text()!=='remote body')return 'allowed response'}
const opaque=await fetch(base+'/denied',{mode:'no-cors'});if(opaque.type!=='opaque'||opaque.url!==''||opaque.status!==0||opaque.ok||opaque.body!==null||[...opaque.headers].length||await opaque.text()!=='')return 'opaque';
return 'ok';}`
	expression := `(async()=>{` + probe + `;const base=` + strconv.Quote(remote.URL) + `;const main=await run(base);if(main!=='ok')return 'window: '+main;const source=` + strconv.Quote(probe) + `+';onmessage=async e=>postMessage(await run(e.data));';const url=URL.createObjectURL(new Blob([source],{type:'text/javascript'})),worker=new Worker(url);try{return await new Promise((resolve,reject)=>{worker.onmessage=e=>resolve(e.data);worker.onerror=reject;worker.postMessage(base)})}finally{worker.terminate();URL.revokeObjectURL(url)}})()`
	got, err := p.Evaluate(ctx, expression)
	if err != nil || got != "ok" {
		t.Fatalf("CORS=%#v err=%v", got, err)
	}
}
