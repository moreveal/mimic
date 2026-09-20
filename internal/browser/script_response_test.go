package browser

import (
	"context"
	"fmt"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUnsuccessfulScriptResponsesAreNotExecuted(t *testing.T) {
	serialBrowserTest(t)
	const rejectedCode = "globalThis.rejectedScriptRan=true;"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rejected.js" {
			w.Header().Set("Content-Type", "application/javascript")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, rejectedCode)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><body><script id="rejected" src="/rejected.js"></script><script>globalThis.parserContinued=true</script>`)
	}))
	defer server.Close()
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
	p.AddInitScript(`globalThis.scriptErrors=[];document.addEventListener('error',e=>{if(e.target instanceof HTMLScriptElement)scriptErrors.push([e.target.id,e.isTrusted])},true)`)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	result, err := p.Evaluate(ctx, `(async()=>{
 if(globalThis.rejectedScriptRan||!globalThis.parserContinued||JSON.stringify(scriptErrors)!=='[["rejected",true]]')return JSON.stringify({ran:globalThis.rejectedScriptRan,continued:globalThis.parserContinued,errors:scriptErrors});
 const type=await new Promise(resolve=>{const s=document.createElement('script');s.id='dynamic';s.onload=()=>resolve('load');s.onerror=()=>resolve('error');s.src='/rejected.js';document.body.appendChild(s)});
 if(type!=='error'||globalThis.rejectedScriptRan)return 'dynamic executed';
 const response=await fetch('/rejected.js');return response.status===403&&await response.text()==='globalThis.rejectedScriptRan=true;'?'ok':'fetch was changed';
 })()`)
	if err != nil || result != "ok" {
		t.Fatalf("script response: %v %v", result, err)
	}
}
