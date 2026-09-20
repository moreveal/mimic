package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchBinaryTransportCloneAndIndependentReads(t *testing.T) {
	serialBrowserTest(t)
	const probe = `async function probe(){
 const response=await fetch('/binary'),copy=response.clone();
 const reader=response.body.getReader(),first=await reader.read();
 if(!(first.value instanceof Uint8Array)||first.value.byteLength===0)return 'chunk';
 for(let i=0;i<first.value.length;i++)if(first.value[i]!==i%256)return 'bytes';
 first.value[0]=79;
 const cloned=new Uint8Array(await copy.arrayBuffer());
 if(cloned[0]!==0||cloned[255]!==255||cloned.length!==1024)return 'clone alias';
 let total=first.value.byteLength;
 for(;;){const next=await reader.read();if(next.done)break;for(let i=0;i<next.value.length;i++)if(next.value[i]!==((total+i)%256))return 'remaining bytes';total+=next.value.length}
 if(total!==1024)return 'end';reader.releaseLock();
 const next=await fetch('/binary');const second=await next.bytes();
 if(second[0]!==0||first.value[0]!==79)return 'separate response alias';
 return true;
 }`
	body := make([]byte, 1024)
	for i := range body {
		body[i] = byte(i)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/binary":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Cache-Control", "max-age=60")
			_, _ = w.Write(body)
		case "/worker.js":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, probe+`;onmessage=()=>probe().then(postMessage,error=>postMessage(String(error)))`)
		default:
			fmt.Fprint(w, `<!doctype html><link rel="icon" href="data:,"><body>binary`)
		}
	}))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		value, err := p.Evaluate(ctx, probe+`;probe()`)
		if err != nil || value != true {
			t.Fatalf("Window binary transport: %v, %v", value, err)
		}
		value, err = p.Evaluate(ctx, `new Promise((resolve,reject)=>{const worker=new Worker('/worker.js');worker.onmessage=e=>{worker.terminate();resolve(e.data)};worker.onerror=e=>reject(Error(e.message));worker.postMessage('start')})`)
		if err != nil || value != true {
			t.Fatalf("Worker binary transport: %v, %v", value, err)
		}
	})
}
