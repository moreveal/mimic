//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBootstrapSnapshotWindowHandlersRetainDispatchState(t *testing.T) {
	p := bootstrapSnapshotPage(t)
	bootstrapSnapshotWarm(t, p)
	restored, err := p.ctx.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	value := bootstrapSnapshotEvaluate(t, restored, `(()=>{
const log=[];addEventListener('message',()=>log.push('before'));
onmessage=()=>log.push('handler');addEventListener('message',()=>log.push('after'));
dispatchEvent(new MessageEvent('message'));
onmessage=null;dispatchEvent(new MessageEvent('message'));
onmessage=()=>log.push('again');dispatchEvent(new MessageEvent('message'));
return log.join(',');
})()`)
	if value != "before,handler,after,before,after,before,after,again" {
		t.Fatalf("restored handler dispatch: %v (restored=%t)", value, restored.Top.Realm.bootstrapRestored)
	}
}

func TestBootstrapSnapshotNestedWindowMessageHandler(t *testing.T) {
	var childURL string
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/middle":
			fmt.Fprintf(w, `<!doctype html><body><script>(async()=>{
const f=document.createElement('iframe');const answer=new Promise(resolve=>{onmessage=e=>{if(e.source===f.contentWindow)resolve(e.data)}});
f.src=%q;document.body.append(f);parent.postMessage(await answer,'*');
})()</script>`, childURL)
		case "/inner":
			fmt.Fprint(w, `<!doctype html><body><script>parent.postMessage('nested','*')</script>`)
		default:
			fmt.Fprint(w, `<!doctype html><body></body>`)
		}
	}))
	defer fixture.Close()
	childURL = fixture.URL + "/inner"
	p := bootstrapSnapshotPage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, fixture.URL); err != nil {
		t.Fatal(err)
	}
	bootstrapSnapshotWarm(t, p)
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer probeCancel()
	value, probeErr := p.Evaluate(probeCtx, fmt.Sprintf(`new Promise(resolve=>{
const f=document.createElement('iframe');addEventListener('message',e=>{if(e.source===f.contentWindow)resolve(e.data)});
f.src=%q;document.body.append(f);
})`, fixture.URL+"/middle"))
	if value != "nested" {
		t.Fatalf("nested message: %v %v", value, probeErr)
	}
	bootstrapSnapshotAssertRestored(t, p, 1)
}
