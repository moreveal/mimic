package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestWorkerTrustworthyScriptOrigins(t *testing.T) {
	for _, test := range []struct {
		url    string
		secure bool
	}{
		{"https://example.test/worker.js", true},
		{"http://localhost/worker.js", true},
		{"http://sub.localhost/worker.js", true},
		{"http://127.0.0.2/worker.js", true},
		{"http://[::1]/worker.js", true},
		{"blob:http://127.0.0.1:1234/id", true},
		{"blob:https://example.test/id", true},
		{"blob:http://example.test/id", false},
		{"http://example.test/worker.js", false},
		{"http://localhost.example.test/worker.js", false},
		{"blob:null/id", false},
	} {
		t.Run(test.url, func(t *testing.T) {
			u, err := url.Parse(test.url)
			if err != nil {
				t.Fatal(err)
			}
			w := DedicatedWorker{url: u}
			if actual := w.isSecureContext(); actual != test.secure {
				t.Fatalf("secure=%t want%t", actual, test.secure)
			}
		})
	}
}

func TestLoopbackWorkerAndDocumentSelectSecureExposure(t *testing.T) {
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("<!doctype html><body>")) }))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		historyEval(t, p, `new Promise((resolve,reject)=>{if(!isSecureContext)throw Error('document insecure');const url=URL.createObjectURL(new Blob(['onmessage=()=>postMessage(isSecureContext && OffscreenCanvas.length===2)'])),worker=new Worker(url);worker.onerror=e=>reject(Error(e.message));worker.onmessage=e=>{worker.terminate();URL.revokeObjectURL(url);resolve(e.data)};worker.postMessage(null)})`, true)
	})
}
