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

// Chrome 152.0.7977.82 (headful, fresh profile): navigation names and time
// origins survive replaceState; each frame reports its own response bytes.
func TestFrameNavigationTimingIsDocumentScoped(t *testing.T) {
	for _, engineCase := range []struct {
		name    string
		factory engine.Factory
	}{{"goja", gojaengine.Factory{}}, {"v8", v8engine.Factory{}}} {
		t.Run(engineCase.name, func(t *testing.T) {
			testFrameNavigationTimingIsDocumentScoped(t, engineCase.factory)
		})
	}
}

func testFrameNavigationTimingIsDocumentScoped(t *testing.T, factory engine.Factory) {
	const parentBody = `<!doctype html><title>Parent timing with deliberately distinct response length</title>`
	const childBody = `<!doctype html><title>Child timing</title>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if req.URL.Path == "/child" {
			_, _ = fmt.Fprint(w, childBody)
		} else {
			_, _ = fmt.Fprint(w, parentBody)
		}
	}))
	defer server.Close()
	browser, err := New(factory, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	p, err := browser.NewContext().NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL+"/parent"); err != nil {
		t.Fatal(err)
	}
	// Ensure the child starts measurably later than the parent document.
	if err := p.Top.Realm.AdvanceBy(ctx, 30*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `new Promise(resolve=>{
		const read="(()=>{const e=performance.getEntriesByType('navigation')[0];return JSON.stringify({name:e.name,origin:performance.timeOrigin,now:performance.now(),bytes:e.decodedBodySize,encoded:e.encodedBodySize,duration:e.duration,responseEnd:e.responseEnd})})()";
		const parentBefore=JSON.parse(eval(read)),frame=document.createElement('iframe');
		frame.src='/child';frame.onload=()=>{
			if(!frame.contentDocument.URL.endsWith('/child'))return;
			const childBefore=JSON.parse(frame.contentWindow.eval(read));
			history.replaceState(null,'','/parent-changed');
			frame.contentWindow.eval("history.replaceState(null,'','/child-changed')");
			setTimeout(()=>resolve({parentBefore,childBefore,parentAfter:JSON.parse(eval(read)),childAfter:JSON.parse(frame.contentWindow.eval(read))}),20);
		};document.body.appendChild(frame);
	})`)
	if err != nil {
		t.Fatal(err)
	}
	result := value.(map[string]any)
	for _, item := range []struct {
		prefix, path string
		bytes        int
	}{{"parent", "/parent", len(parentBody)}, {"child", "/child", len(childBody)}} {
		before := result[item.prefix+"Before"].(map[string]any)
		after := result[item.prefix+"After"].(map[string]any)
		for _, timing := range []map[string]any{before, after} {
			if timing["name"] != server.URL+item.path || numberValue(timing["bytes"]) != float64(item.bytes) || numberValue(timing["encoded"]) != float64(item.bytes) {
				t.Fatalf("%s navigation uses another document/history response: %#v", item.prefix, timing)
			}
		}
		if before["origin"] != after["origin"] || before["duration"] != after["duration"] || before["responseEnd"] != after["responseEnd"] {
			t.Fatalf("%s committed navigation timing mutated: before=%#v after=%#v", item.prefix, before, after)
		}
	}
	parent := result["parentBefore"].(map[string]any)
	child := result["childBefore"].(map[string]any)
	if numberValue(child["origin"]) <= numberValue(parent["origin"]) || numberValue(child["now"]) < 0 {
		t.Fatalf("child document did not get its navigation time origin: %#v", result)
	}
}

func TestChildNavigationObserverWaitsForChildLoad(t *testing.T) {
	for _, engineCase := range []struct {
		name    string
		factory engine.Factory
	}{{"goja", gojaengine.Factory{}}, {"v8", v8engine.Factory{}}} {
		t.Run(engineCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				switch req.URL.Path {
				case "/child":
					w.Header().Set("Content-Type", "text/html")
					_, _ = fmt.Fprint(w, `<!doctype html><script>
					window.observed=[];window.loadEnded=false;
					new PerformanceObserver(list=>{for(const e of list.getEntriesByType('navigation'))observed.push({ready:document.readyState,duration:e.duration,loadEnded})}).observe({entryTypes:['navigation']});
					addEventListener('load',()=>{window.durationInLoad=performance.getEntriesByType('navigation')[0].duration;window.loadEnded=true});
					fetch('/resource');
					</script><script src="/slow.js"></script>`)
				case "/slow.js":
					time.Sleep(100 * time.Millisecond)
					w.Header().Set("Content-Type", "text/javascript")
					_, _ = fmt.Fprint(w, `window.slowScriptLoaded=true`)
				default:
					_, _ = fmt.Fprint(w, `<!doctype html><title>Observer parent</title>`)
				}
			}))
			defer server.Close()
			browser, err := New(engineCase.factory, chrome152.New())
			if err != nil {
				t.Fatal(err)
			}
			p, err := browser.NewContext().NewPage()
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := p.Navigate(ctx, server.URL+"/parent"); err != nil {
				t.Fatal(err)
			}
			value, err := p.Evaluate(ctx, `new Promise(resolve=>{const f=document.createElement('iframe');f.src='/child';f.onload=()=>{if(!f.contentDocument.URL.endsWith('/child'))return;setTimeout(()=>resolve(JSON.parse(f.contentWindow.eval("JSON.stringify({observed,durationInLoad,slowScriptLoaded})"))),30)};document.body.appendChild(f)})`)
			if err != nil {
				t.Fatal(err)
			}
			result := value.(map[string]any)
			observed := result["observed"].([]any)
			if len(observed) != 1 || numberValue(result["durationInLoad"]) != 0 || result["slowScriptLoaded"] != true {
				t.Fatalf("child navigation finalization: %#v", result)
			}
			entry := observed[0].(map[string]any)
			if entry["ready"] != "complete" || entry["loadEnded"] != true || numberValue(entry["duration"]) < 0 {
				t.Fatalf("child observer ran before its own load ended: %#v", result)
			}
		})
	}
}
