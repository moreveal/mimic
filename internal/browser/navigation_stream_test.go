package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	"github.com/moreveal/mimic/internal/engine"
	gojaengine "github.com/moreveal/mimic/internal/engine/goja"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestNavigationHeadScriptCannotSeeUnparsedBody(t *testing.T) {
	parallelBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<script>window.headBodyNull=document.body===null&&document.querySelector('body')===null</script><body><p id=tail>tail</p>`)
	}))
	defer server.Close()
	page := testPage(t)
	if err := page.Navigate(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := page.Evaluate(context.Background(), `headBodyNull===true&&document.body!==null&&document.getElementById('tail')!==null`)
	if err != nil || value != true {
		t.Fatalf("parser visibility: %v, %v", value, err)
	}
}

func TestNavigateReservedDoesNotDrainApplicationTimerQueue(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<script>window.timerRuns=0;setInterval(()=>timerRuns++,0)</script><main>ready</main>`)
	}))
	defer server.Close()

	page := testPage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := page.NavigateReserved(ctx, server.URL, page.ReserveNavigation()); err != nil {
		t.Fatal(err)
	}
	runs, ok := page.Top.Realm.runtime.Get("timerRuns").Export().(int64)
	if !ok || runs > 2 {
		t.Fatalf("navigation drained application timer turns: %v", runs)
	}
}

func TestNavigationDocumentWritesPreserveParserInsertion(t *testing.T) {
	serialBrowserTest(t)
	const html = `<!doctype html><p id=before>before</p><script>
	window.navEvents=[];window.oldD=document;window.sameOpen=document.open()===oldD;
	document.write('<p id=written>written</p><script>navEvents.push("nested");document.write("<i id=inner>inner</i>")<\/script>');
	navEvents.push('outer');document.close();window.readyInside=document.readyState;
	</script><p id=tail>tail</p><script>navEvents.push('tail-script')</script><script type="application/json">{"inert":true}</script>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if req.URL.Path == "/parent" {
			_, _ = fmt.Fprint(w, `<!doctype html><iframe id=child src="/document"></iframe>`)
			return
		}
		_, _ = fmt.Fprint(w, html)
	}))
	defer server.Close()
	for _, engineCase := range []struct {
		name    string
		factory engine.Factory
	}{{"goja", gojaengine.Factory{}}, {"v8", v8engine.Factory{}}} {
		for _, child := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/child=%v", engineCase.name, child), func(t *testing.T) {
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
				path := "/document"
				if child {
					path = "/parent"
				}
				if err := page.Navigate(ctx, server.URL+path); err != nil {
					t.Fatal(err)
				}
				read := `JSON.stringify({same:document===oldD&&sameOpen,readyInside,events:navEvents,ids:Array.from(document.querySelectorAll('p')).map(n=>n.id),inner:!!document.getElementById('inner'),ready:document.readyState})`
				source := `JSON.parse(` + read + `)`
				if child {
					source = `new Promise(resolve=>{const f=document.getElementById('child'),poll=()=>{if(f.contentDocument.readyState==='complete'&&f.contentWindow.eval('typeof navEvents')!=='undefined')resolve(JSON.parse(f.contentWindow.eval(` + fmt.Sprintf("%q", read) + `)));else setTimeout(poll,5)};poll()})`
				}
				value, err := page.Evaluate(ctx, source)
				if err != nil {
					t.Fatal(err)
				}
				result := value.(map[string]any)
				if result["same"] != true || result["readyInside"] != "loading" || fmt.Sprint(result["events"]) != "[nested outer tail-script]" || fmt.Sprint(result["ids"]) != "[before written tail]" || result["inner"] != true || result["ready"] != "complete" {
					t.Fatalf("navigation stream insertion/lifecycle: %#v", result)
				}
			})
		}
	}
}

func TestNavigationExternalScriptWritesPrecedeTail(t *testing.T) {
	serialBrowserTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/external.js" {
			time.Sleep(20 * time.Millisecond)
			w.Header().Set("Content-Type", "text/javascript")
			_, _ = fmt.Fprint(w, `window.tailWasAbsent=document.getElementById('tail')===null;window.scriptBefore=document.currentScript.id;document.write('<p id=written>written</p><script>window.nestedRan=true<\/script>');window.scriptAfter=document.currentScript.id`)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		if req.URL.Path == "/parent" {
			_, _ = fmt.Fprint(w, `<!doctype html><iframe id=child src="/document"></iframe>`)
			return
		}
		_, _ = fmt.Fprint(w, `<!doctype html><p id=before>before</p><script id=external src="/external.js"></script><p id=tail>tail</p>`)
	}))
	defer server.Close()
	for _, engineCase := range []struct {
		name    string
		factory engine.Factory
	}{{"goja", gojaengine.Factory{}}, {"v8", v8engine.Factory{}}} {
		for _, child := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/child=%v", engineCase.name, child), func(t *testing.T) {
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
				path := "/document"
				if child {
					path = "/parent"
				}
				if err := page.Navigate(ctx, server.URL+path); err != nil {
					t.Fatal(err)
				}
				read := `JSON.stringify({tailWasAbsent,nestedRan,scriptBefore,scriptAfter,ids:Array.from(document.querySelectorAll('p')).map(n=>n.id)})`
				source := `JSON.parse(` + read + `)`
				if child {
					source = `new Promise(resolve=>{const f=document.getElementById('child'),poll=()=>{if(f.contentDocument.readyState==='complete'&&f.contentWindow.eval('typeof nestedRan')!=='undefined')resolve(JSON.parse(f.contentWindow.eval(` + fmt.Sprintf("%q", read) + `)));else setTimeout(poll,5)};poll()})`
				}
				value, err := page.Evaluate(ctx, source)
				if err != nil {
					t.Fatal(err)
				}
				result := value.(map[string]any)
				if result["tailWasAbsent"] != true || result["nestedRan"] != true || result["scriptBefore"] != "external" || result["scriptAfter"] != "external" || fmt.Sprint(result["ids"]) != "[before written tail]" {
					t.Fatalf("external navigation parser insertion: %#v", result)
				}
			})
		}
	}
}
