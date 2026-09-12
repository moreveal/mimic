package cdp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Chrome 152 oracle: stopping a document aborts its stylesheet/module and
// existing fetch/XHR, but not worker fetches or requests initiated afterwards.
func TestStopLoadingAbortsDocumentRequestsAndPreservesFutureLoads(t *testing.T) {
	for _, kind := range []string{"stylesheet", "module"} {
		t.Run(kind, func(t *testing.T) {
			entered, canceled := map[string]chan struct{}{}, map[string]chan struct{}{}
			for _, path := range []string{"/held", "/fetch", "/xhr", "/worker-fetch"} {
				entered[path], canceled[path] = make(chan struct{}, 1), make(chan struct{}, 1)
			}
			release := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			var heldCount atomic.Int32
			fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				path := r.URL.Path
				switch path {
				case "/":
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, `<script>globalThis.events=[];addEventListener('load',()=>events.push('window-load'));document.addEventListener('DOMContentLoaded',()=>events.push('dcl'))</script>`)
					if kind == "stylesheet" {
						fmt.Fprint(w, `<link rel=stylesheet href=/held onload="events.push('style-load')" onerror="events.push('style-error')"><script>events.push('parser-after-style')</script>`)
					} else {
						fmt.Fprint(w, `<script type=module src=/entry.js onload="events.push('module-load')"></script>`)
					}
					return
				case "/entry.js":
					w.Header().Set("Content-Type", "text/javascript")
					fmt.Fprint(w, `import '/held';events.push('module-eval')`)
					return
				case "/worker.js":
					w.Header().Set("Content-Type", "text/javascript")
					fmt.Fprint(w, `onmessage=()=>fetch('/worker-fetch').then(r=>r.text()).then(()=>postMessage('ok'),()=>postMessage('error'))`)
					return
				case "/fresh":
					fmt.Fprint(w, "fresh")
					return
				case "/fresh-module":
					w.Header().Set("Content-Type", "text/javascript")
					fmt.Fprint(w, `events.push('fresh-module')`)
					return
				}
				if entered[path] == nil {
					http.NotFound(w, r)
					return
				}
				if path == "/held" {
					heldCount.Add(1)
				}
				entered[path] <- struct{}{}
				select {
				case <-r.Context().Done():
					canceled[path] <- struct{}{}
					return
				case <-release:
				}
				if path == "/held" && kind == "stylesheet" {
					w.Header().Set("Content-Type", "text/css")
					fmt.Fprint(w, "body{color:red}")
				} else if path == "/held" {
					w.Header().Set("Content-Type", "text/javascript")
					fmt.Fprint(w, `events.push('dependency-eval')`)
				} else {
					fmt.Fprint(w, "done")
				}
			}))
			defer fixture.Close()
			defer unblock()
			s, addr := runningServer(t)
			c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/devtools/page/"+s.Page.ID, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			c.SetReadDeadline(time.Now().Add(8 * time.Second))
			c.WriteJSON(map[string]any{"id": 1, "method": "Page.navigate", "params": map[string]any{"url": fixture.URL}})
			readReply(t, c, 1)
			wait := func(ch <-chan struct{}, name string) {
				t.Helper()
				select {
				case <-ch:
				case <-time.After(2 * time.Second):
					t.Fatal(name)
				}
			}
			wait(entered["/held"], "parser resource did not start")
			id := 2
			evaluate := func(expression string) any {
				t.Helper()
				id++
				c.WriteJSON(map[string]any{"id": id, "method": "Runtime.evaluate", "params": map[string]any{"expression": expression, "returnByValue": true}})
				reply := readReply(t, c, float64(id))
				if reply["error"] != nil {
					t.Fatal(reply)
				}
				return reply["result"].(map[string]any)["result"].(map[string]any)["value"]
			}
			evaluate(`fetch('/fetch').then(r=>r.text()).then(()=>events.push('fetch-ok'),()=>events.push('fetch-error'));globalThis.x=new XMLHttpRequest;x.open('GET','/xhr');for(const type of ['load','error','abort'])x.addEventListener(type,()=>events.push('xhr-'+type));x.send();globalThis.w=new Worker('/worker.js');w.onmessage=e=>events.push('worker-'+e.data);w.postMessage(true);true`)
			for _, path := range []string{"/fetch", "/xhr", "/worker-fetch"} {
				wait(entered[path], path+" did not start")
			}
			id++
			c.WriteJSON(map[string]any{"id": id, "method": "Page.stopLoading"})
			readReply(t, c, float64(id))
			for _, path := range []string{"/held", "/fetch", "/xhr"} {
				wait(canceled[path], path+" survived stopLoading")
			}
			select {
			case <-canceled["/worker-fetch"]:
				t.Fatal("stopLoading canceled independent worker")
			default:
			}
			until := func(expression string) {
				t.Helper()
				deadline := time.Now().Add(2 * time.Second)
				for evaluate(expression) != true {
					if time.Now().After(deadline) {
						t.Fatal("missing observation", expression, evaluate(`JSON.stringify({events,ready:document.readyState})`))
					}
					time.Sleep(time.Millisecond)
				}
			}
			until(`document.readyState==='complete'&&events.includes('fetch-error')&&events.includes('xhr-abort')`)
			if kind == "stylesheet" {
				until(`events.includes('style-error')`)
			}
			unblock()
			evaluate(`fetch('/fresh').then(r=>r.text()).then(t=>events.push(t));import('/fresh-module');true`)
			if kind == "module" {
				evaluate(`import('/entry.js').then(()=>events.push('module-retry-ok'),()=>events.push('module-retry-error'));true`)
				until(`events.includes('module-retry-error')`)
			}
			until(`events.includes('worker-ok')&&events.includes('fresh')&&events.includes('fresh-module')`)
			if evaluate(`!events.some(x=>['fetch-ok','xhr-load','xhr-error','style-load','parser-after-style','dependency-eval','module-eval','module-load','module-retry-ok','dcl','window-load'].includes(x))`) != true {
				t.Fatal("stopped resource executed or emitted success", evaluate(`JSON.stringify(events)`))
			}
			if heldCount.Load() != 1 {
				t.Fatal("module failure was not cached", heldCount.Load())
			}
		})
	}
}
