package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWorkerMessageTraceSnapshot(t *testing.T) {
	parallelBrowserTest(t)
	data := map[string]any{"nested": map[string]any{"value": "original"}}
	event := workerMessageTrace(1, "parent-to-worker", data)
	data["nested"].(map[string]any)["value"] = "changed"
	b, err := json.Marshal(event)
	if err != nil || !strings.Contains(string(b), "original") || strings.Contains(string(b), "changed") {
		t.Fatalf("message not frozen: %s %v", b, err)
	}
	invalid := workerMessageTrace(1, "parent-to-worker", math.NaN())
	if invalid["dataCaptureError"] == nil {
		t.Fatal("non-JSON export must report an explicit capture error")
	}
	if _, err := json.Marshal(invalid); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureWorkerMessagesAndQueryResults(t *testing.T) {
	serialBrowserTest(t)
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `<!doctype html><body><div id="capture-node" data-state="before"><span></span></div></body>`)
		}))
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		// A long Unicode source-like message exercises JSON escaping and makes
		// accidental preview truncation visible after the trace's wire encoding.
		payload := strings.Repeat("// capture λ\n", 10000)
		quoted, _ := json.Marshal(payload)
		workerSource := `onmessage=e=>postMessage({echo:e.data,trusted:e.isTrusted,origin:e.origin,sourceNull:e.source===null})`
		source, _ := json.Marshal(workerSource)
		_, err := p.Evaluate(ctx, `(async()=>{
			const node=document.querySelector('#capture-node');
			document.querySelector('#capture-absent');
			document.querySelectorAll('#capture-node');
			document.querySelectorAll('#capture-absent');
			node.setAttribute('data-state','after');
			const url=URL.createObjectURL(new Blob([`+string(source)+`]));
			const worker=new Worker(url);
			try {return await new Promise((resolve,reject)=>{
				worker.onmessage=e=>resolve(e.data.echo.length);
				worker.onerror=e=>reject(new Error(e.message));
				worker.postMessage(`+string(quoted)+`);
			});} finally {worker.terminate();URL.revokeObjectURL(url)}
		})()`)
		if err != nil {
			t.Fatal(err)
		}
		wire, err := json.Marshal(p.Trace().Events())
		if err != nil {
			t.Fatal(err)
		}
		var events []struct {
			Name string         `json:"name"`
			Data map[string]any `json:"data"`
		}
		if err := json.Unmarshal(wire, &events); err != nil {
			t.Fatal(err)
		}
		var sent, received, script, found, missing, all, empty bool
		for _, event := range events {
			d := event.Data
			switch event.Name {
			case "workerMessageQueued":
				if d["direction"] == "parent-to-worker" {
					sent = sent || d["data"] == payload
				} else if result, ok := d["data"].(map[string]any); ok {
					received = result["echo"] == payload && result["trusted"] == true && result["origin"] == "" && result["sourceNull"] == true
				}
			case "scriptStart":
				script = script || d["source"] == workerSource
			case "Document.querySelector", "Element.querySelector":
				if d["selector"] == "#capture-node" {
					if result, ok := d["result"].(map[string]any); ok {
						attrs, _ := result["attributes"].(map[string]any)
						found = found || attrs["data-state"] == "before"
					}
				}
				if d["selector"] == "#capture-absent" {
					value, present := d["result"]
					missing = present && value == nil
				}
			case "Document.querySelectorAll", "Element.querySelectorAll":
				ids, ok := d["resultNodeIds"].([]any)
				if d["selector"] == "#capture-node" {
					all = ok && len(ids) == 1
				}
				if d["selector"] == "#capture-absent" {
					empty = ok && len(ids) == 0
				}
			}
		}
		if !sent || !received || !script || !found || !missing || !all || !empty {
			t.Fatalf("capture incomplete: sent=%v received=%v script=%v found=%v missing=%v all=%v empty=%v", sent, received, script, found, missing, all, empty)
		}
	})
}
