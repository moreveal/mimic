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
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestFrameDocumentBridgePreservesReceiversAndIdentity(t *testing.T) {
	parallelBrowserTest(t)
	for _, engineCase := range []struct {
		name    string
		factory engine.Factory
	}{{"goja", gojaengine.Factory{}}, {"v8", v8engine.Factory{}}} {
		t.Run(engineCase.name, func(t *testing.T) {
			browser, err := New(engineCase.factory, chrome152.New())
			if err != nil {
				t.Fatal(err)
			}
			page, err := browser.NewContext().NewPage()
			if err != nil {
				t.Fatal(err)
			}
			defer page.Close()
			value, err := page.Evaluate(context.Background(), `(()=>{
				const f=document.createElement('iframe');document.body.appendChild(f);
				const d=f.contentDocument,n=d.createElement('section');n.id='child-only';n.textContent='child text';
				const appended=d.body.appendChild(n),found=d.querySelector('#child-only');d.title='Child title';
				const object=f.contentWindow.eval('({value:17,method(){return this.value}})'),method=object.method;
				return{sameDocument:d===f.contentWindow.document&&d===f.contentWindow.eval('document'),sameBody:d.body===d.body,
					sameNode:appended===n&&found===n&&d.querySelectorAll('section').item(0)===n,
					owner:n.ownerDocument===d&&d.documentElement.ownerDocument===d,window:d.defaultView===f.contentWindow,
					child:found.textContent,title:d.title,parentTitle:document.title,parentNode:document.querySelector('#child-only')===null,
					receiver:object.method(),callReceiver:method.call(object)};
			})()`)
			if err != nil {
				t.Fatal(err)
			}
			result := value.(map[string]any)
			for _, key := range []string{"sameDocument", "sameBody", "sameNode", "owner", "window", "parentNode"} {
				if result[key] != true {
					t.Fatalf("%s: %#v", key, result)
				}
			}
			if result["child"] != "child text" || result["title"] != "Child title" || result["parentTitle"] != "" || numberValue(result["receiver"]) != 17 || numberValue(result["callReceiver"]) != 17 {
				t.Fatalf("cross-frame document/receiver dispatch: %#v", result)
			}
		})
	}
}

func TestFrameDocumentBridgeRejectsCrossOriginRead(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<!doctype html><title>Cross-origin child</title>`)
	}))
	defer server.Close()
	page := testPage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := page.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	childURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	value, err := page.Evaluate(ctx, `new Promise(resolve=>{const f=document.createElement('iframe');f.src=`+fmt.Sprintf("%q", childURL)+`;f.onload=()=>{let read=false,write=false;try{void f.contentWindow.document.body}catch(error){read=String(error).includes('SecurityError')}try{f.contentWindow.remoteValue={}}catch(error){write=String(error).includes('SecurityError')}resolve(read&&write)};document.body.appendChild(f)})`)
	if err != nil || value != true {
		t.Fatalf("cross-origin document access: %v %v", value, err)
	}
}

func TestFrameCrossOriginPostMessageRemainsAvailable(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<!doctype html><script>addEventListener('message',e=>{if(e.data==='ping')e.source.postMessage('pong',e.origin)})</script>`)
	}))
	defer server.Close()
	page := testPage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := page.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	childURL := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	value, err := page.Evaluate(ctx, `new Promise(resolve=>{
		const f=document.createElement('iframe');f.src=`+fmt.Sprintf("%q", childURL)+`;
		addEventListener('message',e=>{if(e.data==='pong')resolve(e.source===f.contentWindow&&e.origin===`+fmt.Sprintf("%q", childURL)+`)});
		f.onload=()=>{try{f.contentWindow.postMessage('ping',`+fmt.Sprintf("%q", childURL)+`)}catch(e){resolve(String(e))}};
		document.body.appendChild(f);
	})`)
	if err != nil || value != true {
		t.Fatalf("cross-origin postMessage: %v %v", value, err)
	}
}

func TestFrameDocumentEntryFollowsSynchronousCalls(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!doctype html><title>Entry document</title>`)
	}))
	defer server.Close()
	for _, engineCase := range []struct {
		name    string
		factory engine.Factory
	}{{"goja", gojaengine.Factory{}}, {"v8", v8engine.Factory{}}} {
		t.Run(engineCase.name, func(t *testing.T) {
			browser, err := New(engineCase.factory, chrome152.New())
			if err != nil {
				t.Fatal(err)
			}
			page, err := browser.NewContext().NewPage()
			if err != nil {
				t.Fatal(err)
			}
			defer page.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := page.Navigate(ctx, server.URL+"/dir/parent#fragment"); err != nil {
				t.Fatal(err)
			}
			value, err := page.Evaluate(ctx, `(async()=>{
				const make=path=>new Promise(resolve=>{const f=document.createElement('iframe');f.src=path;f.onload=()=>{f.onload=null;resolve(f)};document.body.appendChild(f)});
				const first=await make('/other/eval'),byEval=first.contentWindow.eval('document.open();document.close();document.URL');
				const second=await make('/other/call'),fn=second.contentWindow.eval('(function(){document.open();document.close();return document.URL})'),byCall=fn();
				const third=await make('/other/timer#fragment'),later=new Promise(resolve=>addEventListener('message',event=>resolve(event.data),{once:true}));
				third.contentWindow.eval("setTimeout(()=>{document.open();document.close();parent.postMessage(document.URL,'*')},0)");
				return{byEval,byCall,byTimer:await later};
			})()`)
			if err != nil {
				t.Fatal(err)
			}
			result := value.(map[string]any)
			if result["byEval"] != server.URL+"/dir/parent" || result["byCall"] != server.URL+"/dir/parent" || result["byTimer"] != server.URL+"/other/timer#fragment" {
				t.Fatalf("entry document escaped synchronous call boundary: %#v", result)
			}
		})
	}
}
