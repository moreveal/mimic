package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"github.com/moreveal/mimic/internal/network"
)

func TestCanceledAndNonGETRequestsDoNotConsumePreload(t *testing.T) {
	parallelBrowserTest(t)
	target, _ := url.Parse("https://example.test/resource")
	request := network.Request{URL: target, Initiator: network.Fetch, Mode: "cors", Credentials: "same-origin"}
	pending := &resourcePreload{done: make(chan struct{})}
	r := &Realm{preloads: map[preloadKey]*resourcePreload{preloadRequestKey(request): pending}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.loadResource(ctx, request); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	post := request
	post.Method = http.MethodPost
	if r.consumePreload(post) != nil {
		t.Fatal("POST consumed GET preload")
	}
	if r.consumePreload(request) != pending {
		t.Fatal("canceled or non-GET request stole preload")
	}
}

func TestDocumentPreloadConsumersChrome152(t *testing.T) {
	serialBrowserTest(t)
	fixture, err := os.ReadFile("testdata/image_preload.js")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := os.ReadFile("testdata/resource_preload_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var reference struct {
		Cycles []struct {
			Options  map[string]any `json:"options"`
			Result   any            `json:"result"`
			Requests int            `json:"requests"`
		} `json:"cycles"`
	}
	if err := json.Unmarshal(evidence, &reference); err != nil {
		t.Fatal(err)
	}
	type observation struct {
		count                  int
		started, release       chan struct{}
		startOnce, releaseOnce sync.Once
	}
	var mu sync.Mutex
	states := map[string]*observation{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		name := req.URL.Query().Get("case")
		mu.Lock()
		state := states[name]
		if state == nil {
			state = &observation{started: make(chan struct{}), release: make(chan struct{})}
			states[name] = state
		}
		if req.URL.Path == "/ci" {
			state.count++
		}
		mu.Unlock()
		switch req.URL.Path {
		case "/ci":
			state.startOnce.Do(func() { close(state.started) })
			if req.URL.Query().Get("completed") != "true" {
				select {
				case <-state.release:
				case <-req.Context().Done():
					return
				}
			}
			if req.URL.Query().Get("fail") == "true" {
				w.WriteHeader(404)
			}
			if req.URL.Query().Get("invalid") == "true" {
				w.Header().Set("Content-Type", "image/svg+xml")
				fmt.Fprint(w, "not an image")
				return
			}
			switch req.URL.Query().Get("as") {
			case "script":
				w.Header().Set("Content-Type", "text/javascript")
				fmt.Fprint(w, "globalThis.preloadedScriptExecuted = true;")
			case "style":
				w.Header().Set("Content-Type", "text/css")
				fmt.Fprint(w, "body { color: rgb(1, 2, 3); }")
			case "fetch":
				w.Header().Set("Content-Type", "text/plain")
				fmt.Fprint(w, "preloaded fetch")
			default:
				w.Header().Set("Content-Type", "image/svg+xml")
				fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="2" height="1"/>`)
			}
		case "/started":
			select {
			case <-state.started:
			case <-req.Context().Done():
				return
			}
		case "/release":
			state.releaseOnce.Do(func() { close(state.release) })
		default:
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<body></body>")
		}
	}))
	defer server.Close()
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	for _, test := range reference.Cycles {
		t.Run(test.Options["name"].(string), func(t *testing.T) {
			p, err := c.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := p.Navigate(ctx, server.URL); err != nil {
				t.Fatal(err)
			}
			if _, err := p.Evaluate(ctx, string(fixture)); err != nil {
				t.Fatal(err)
			}
			options, _ := json.Marshal(test.Options)
			got, err := p.Evaluate(ctx, "runImagePreloadCase("+string(options)+")")
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var normalized any
			if err := json.Unmarshal(encoded, &normalized); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(normalized, test.Result) {
				t.Errorf("result %s, Chrome %v", encoded, test.Result)
			}
			mu.Lock()
			count := states[test.Options["name"].(string)].count
			mu.Unlock()
			if count != test.Requests {
				t.Errorf("physical requests=%d, Chrome=%d", count, test.Requests)
			}
		})
	}
}

func TestParserPreloadConsumers(t *testing.T) {
	serialBrowserTest(t)
	var mu sync.Mutex
	counts := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		mu.Lock()
		counts[req.URL.Path]++
		mu.Unlock()
		switch req.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<link rel="preload" as="script" href="/script"><link rel="preload" as="style" href="/style"><link rel="preload" as="image" href="/ci"><script src="/script"></script><link rel="stylesheet" href="/style"><img src="/ci">`)
		case "/script":
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `globalThis.executions=(globalThis.executions||0)+1`)
		case "/style":
			w.Header().Set("Content-Type", "text/css")
			fmt.Fprint(w, `body{color:red}`)
		case "/ci":
			w.Header().Set("Content-Type", "image/svg+xml")
			fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="2" height="1"/>`)
		default:
			w.WriteHeader(404)
		}
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `executions+':'+document.querySelector('img').naturalWidth`)
	if err != nil || value != "1:2" {
		t.Fatalf("parser consumers: %v %v", value, err)
	}
	mu.Lock()
	defer mu.Unlock()
	for _, path := range []string{"/script", "/style", "/ci"} {
		if counts[path] != 1 {
			t.Errorf("%s: %d physical requests", path, counts[path])
		}
	}
}

func TestPreloadCancellationBelongsToDocument(t *testing.T) {
	serialBrowserTest(t)
	for _, action := range []string{"replace-image", "close-page", "document-open"} {
		t.Run(action, func(t *testing.T) {
			started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				if req.URL.Path == "/" {
					fmt.Fprint(w, "<body></body>")
					return
				}
				if req.URL.Path == "/slow" {
					if requests.Add(1) == 1 {
						close(started)
					}
					select {
					case <-release:
					case <-req.Context().Done():
						select {
						case <-canceled:
						default:
							close(canceled)
						}
						return
					}
				}
				w.Header().Set("Content-Type", "image/svg+xml")
				fmt.Fprint(w, `<svg xmlns="http://www.w3.org/2000/svg" width="2" height="1"/>`)
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
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := p.Navigate(ctx, server.URL); err != nil {
				t.Fatal(err)
			}
			_, err = p.Evaluate(ctx, `globalThis.link=document.createElement('link');link.rel='preload';link.as='image';link.href='/slow';globalThis.linkDone=new Promise(resolve=>{link.onload=()=>resolve('load');link.onerror=()=>resolve('error')});document.head.appendChild(link);true`)
			if err != nil {
				t.Fatal(err)
			}
			select {
			case <-started:
			case <-ctx.Done():
				t.Fatal("preload did not start")
			}
			if action == "replace-image" {
				_, err = p.Evaluate(ctx, `globalThis.img=new Image();img.src='/slow';true`)
				if err != nil {
					t.Fatal(err)
				}
				// Wait until the image really subscribed, not just an attribute task.
				for {
					p.Top.Realm.preloadsMu.Lock()
					remaining := len(p.Top.Realm.preloads)
					p.Top.Realm.preloadsMu.Unlock()
					if remaining == 0 {
						break
					}
					if ctx.Err() != nil {
						t.Fatal("image did not consume preload")
					}
					if _, err := p.Evaluate(ctx, `true`); err != nil {
						t.Fatal(err)
					}
				}
				value, err := p.Evaluate(ctx, `(async()=>{const done=new Promise(resolve=>{img.onload=()=>resolve('load');img.onerror=()=>resolve('error')});img.src='/new';return await done})()`)
				if err != nil || value != "load" {
					t.Fatalf("replacement: %v %v", value, err)
				}
				select {
				case <-canceled:
					t.Fatal("image replacement canceled document preload")
				default:
				}
				close(release)
				value, err = p.Evaluate(ctx, `linkDone`)
				if err != nil || value != "load" {
					t.Fatalf("preload event: %v %v", value, err)
				}
			} else {
				if action == "close-page" {
					err = p.Close()
				} else {
					_, err = p.Evaluate(ctx, `document.open();document.write('<body>replacement</body>');document.close();true`)
				}
				if err != nil {
					t.Fatal(err)
				}
				select {
				case <-canceled:
				case <-ctx.Done():
					t.Fatal("document teardown retained preload")
				}
			}
			if requests.Load() != 1 {
				t.Fatalf("preload fetched %d times", requests.Load())
			}
		})
	}
}
