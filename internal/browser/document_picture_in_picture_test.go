package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestDocumentPictureInPictureLifecycle(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		v, err := p.Evaluate(ctx, `documentPictureInPicture.requestWindow().then(()=>"unexpected",e=>e.name)`)
		if err != nil || v != "NotAllowedError" {
			t.Fatalf("activation: %v %v", v, err)
		}
		r := p.Top.Realm
		r.activationAt = r.scheduler.Now()
		v, err = p.Evaluate(ctx, `(async()=>{
 const d=documentPictureInPicture,events=[];d.onenter=e=>events.push(e.isTrusted&&e.window===d.window);
 globalThis.pipWindow=await d.requestWindow({width:420,height:260});const w=pipWindow;
 w.document.body.innerHTML='<p>hello</p>';
 const result=[w===d.window,w.window===w,w.top===w,w.parent===w,w.opener===window,w.frameElement===null,w.document.defaultView===w,w.Array!==Array,w.document instanceof w.Document,w.document.body.textContent==='hello',w.innerWidth===420,w.innerHeight===260,navigator.userActivation.isActive===false,navigator.userActivation.hasBeenActive,events.length===1&&events[0],window.length===0];
 const beforeHistory=history.length;w.history.pushState({a:1},'');w.history.pushState({a:2},'','about:blank#x');result.push(w.history.length===0,w.history.state.a===2,history.length===beforeHistory,w.location.href==='about:blank#x');
 const nested=await w.documentPictureInPicture.requestWindow().then(()=>false,e=>e.name==='NotAllowedError');result.push(nested);
 w.onpagehide=e=>{globalThis.pipHidden=e.isTrusted&&!e.persisted&&w.closed&&w.document.hidden&&d.window===w};
 w.close();result.push(w.closed,d.window===w);
 return result.every(Boolean);
 })()`)
		if err != nil || v != true {
			t.Fatalf("open/close: %v %v", v, err)
		}
		if err := p.AdvanceTime(ctx, time.Millisecond); err != nil {
			t.Fatal(err)
		}
		v, err = p.Evaluate(ctx, `pipHidden&&documentPictureInPicture.window===null&&pipWindow.closed&&pipWindow.top===null&&pipWindow.innerWidth===0`)
		if err != nil || v != true {
			t.Fatalf("retired: %v %v", v, err)
		}
	})
}

func TestDocumentPictureInPictureOwnerLifecycle(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		if _, err := p.Evaluate(ctx, `0`); err != nil {
			t.Fatal(err)
		}
		owner := p.Top.Realm
		owner.activationAt = owner.scheduler.Now()
		if _, err := p.Evaluate(ctx, `documentPictureInPicture.requestWindow({width:400,height:300}).then(w=>w.document.body.textContent='retained')`); err != nil {
			t.Fatal(err)
		}
		f := owner.pictureInPicture
		if f == nil {
			t.Fatal("no auxiliary context")
		}
		if err := p.Navigate(ctx, server.URL+"/next"); err != nil {
			t.Fatal(err)
		}
		if !f.Realm.inactive || owner.pictureInPicture != nil || p.frames[f.ID] != nil {
			t.Fatal("auxiliary survives owner deactivation")
		}
	})
}

func TestDocumentPictureInPictureChromeOracle(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		probe, err := os.ReadFile("testdata/document_pip_oracle.js")
		if err != nil {
			t.Fatal(err)
		}
		reference, err := os.ReadFile("testdata/document_pip_chrome152.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.Evaluate(ctx, `0`); err != nil {
			t.Fatal(err)
		}
		p.Top.Realm.activationAt = p.Top.Realm.scheduler.Now()
		v, err := p.Evaluate(ctx, string(probe))
		if err != nil {
			t.Fatal(err)
		}
		var got, want any
		if err := json.Unmarshal([]byte(v.(string)), &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(reference, &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %s\nwant %s", v, reference)
		}
	})
}

func TestDocumentPictureInPictureContextIsolationAndClose(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		q, err := p.ctx.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		defer q.Close()
		if err := q.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		for _, page := range []*Page{p, q} {
			if _, err := page.Evaluate(ctx, `0`); err != nil {
				t.Fatal(err)
			}
		}
		p.Top.Realm.activationAt = p.Top.Realm.scheduler.Now()
		if _, err := p.Evaluate(ctx, `documentPictureInPicture.requestWindow({width:400,height:300})`); err != nil {
			t.Fatal(err)
		}
		f := p.Top.Realm.pictureInPicture
		v, err := q.Evaluate(ctx, `documentPictureInPicture.window===null&&documentPictureInPicture.onenter===null`)
		if err != nil || v != true {
			t.Fatalf("snapshot isolation: %v %v", v, err)
		}
		if err := p.Close(); err != nil {
			t.Fatal(err)
		}
		if len(p.frames) != 0 || len(p.realmOwners) != 0 || !f.Realm.closed {
			t.Fatal("page close retained auxiliary runtime")
		}
	})
}
