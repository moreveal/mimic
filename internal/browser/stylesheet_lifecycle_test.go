package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParserStylesheetCompletionKeepsDialogOpen(t *testing.T) {
	parallelBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/style.css":
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, "dialog {color:rgb(1, 2, 3)}")
		case "/missing.css":
			http.NotFound(w, r)
		default:
			fmt.Fprint(w, `<!doctype html><html><head><script>globalThis.sheetEvents=[];globalThis.watchdogFailed=false</script>
<link id="one" rel="stylesheet" href="/style.css" onload="sheetEvents.push([this.id,'load',this.sheet!==null,event.isTrusted])">
<link id="two" rel="stylesheet" href="/style.css" onload="sheetEvents.push([this.id,'load',this.sheet!==null,event.isTrusted])">
<link id="bad" rel="stylesheet" href="/missing.css" onerror="sheetEvents.push([this.id,'error',this.sheet===null,event.isTrusted])">
</head><body><dialog id="dialog">Stay open</dialog><script>
document.getElementById('dialog').showModal();
setTimeout(()=>{if(sheetEvents.length!==3){watchdogFailed=true;document.getElementById('dialog').close()}},100);
</script></body></html>`)
		}
	}))
	defer server.Close()
	p := bootstrapSnapshotPage(t)
	if err := p.Navigate(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	historyEval(t, p, `new Promise(resolve=>setTimeout(()=>resolve(!watchdogFailed&&document.getElementById('dialog').open&&sheetEvents.length===3&&sheetEvents.every(e=>e[2]&&e[3])?true:JSON.stringify(sheetEvents)),200))`, true)
}
