package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchResponseBodyAbortUsesIndependentDOMException(t *testing.T) {
	serialBrowserTest(t)
	const probe = `async function probe(){for(const reason of [undefined,{custom:true},'custom']){
 const controller=new AbortController();const response=await fetch('/body',{signal:controller.signal});controller.abort(reason);
 try{await response.text();return 'body resolved'}catch(error){
 if(!(error instanceof DOMException)||error.name!=='AbortError'||error.message!=='The user aborted a request.'||error.code!==20||error===controller.signal.reason||error===reason||response.status!==200)return 'wrong body abort: '+String(error);
 }
 }return true}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/worker.js":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, probe+`;onmessage=()=>probe().then(postMessage,error=>postMessage(String(error)))`)
		case "/body":
			fmt.Fprint(w, "buffered response")
		default:
			fmt.Fprint(w, "<!doctype html><body>fixture")
		}
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, page *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := page.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		value, err := page.Evaluate(ctx, probe+`;probe()`)
		if err != nil || value != true {
			t.Fatalf("window body abort: %v, %v", value, err)
		}
		value, err = page.Evaluate(ctx, `new Promise((resolve,reject)=>{const worker=new Worker('/worker.js');worker.onmessage=e=>{worker.terminate();resolve(e.data)};worker.onerror=e=>reject(new Error(e.message));worker.postMessage('start')})`)
		if err != nil || value != true {
			t.Fatalf("worker body abort: %v, %v", value, err)
		}
	})
}
