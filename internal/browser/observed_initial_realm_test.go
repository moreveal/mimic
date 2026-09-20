//go:build (windows || linux) && amd64

package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// A main-response wait can expose the initial document to arbitrary debugger
// code. Bootstrap batching must preserve that document until commit, and must
// never reuse its modified JS graph for the replacement document.
func TestNavigationBootstrapPreservesObservedInitialRealm(t *testing.T) {
	serialBrowserTest(t)
	seed := bootstrapSnapshotPage(t)
	bootstrapSnapshotWarm(t, seed)
	entered, release := make(chan struct{}), make(chan struct{})
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		close(entered)
		select {
		case <-release:
		case <-req.Context().Done():
			return
		}
		fmt.Fprint(w, `<body><p id=new>replacement</p><script>globalThis.newDocument=true</script></body>`)
	}))
	defer fixture.Close()
	p, err := seed.ctx.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	initial := p.Top.Realm
	if err := p.StartNavigation(ctx, fixture.URL, p.ReserveNavigation()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("main request did not start")
	}
	value, err := p.EvaluateCommand(ctx, "", `(()=>{
  Object.prototype.oldDocumentMarker='initial';
  globalThis.oldGlobal={document,Array};
  document.body.textContent='observed initial document';
  Promise.resolve().then(()=>{globalThis.oldJob=true});
  return oldGlobal.document===document&&document.URL==='about:blank';
})()`)
	if err != nil || value != true {
		t.Fatalf("initial arbitrary JS: %v, %v", value, err)
	}
	value, err = p.EvaluateCommand(ctx, "", `oldJob===true&&({}).oldDocumentMarker==='initial'&&document.body.textContent==='observed initial document'`)
	if err != nil || value != true || p.Top.Realm != initial {
		t.Fatalf("initial realm changed before response: %v, %v", value, err)
	}
	close(release)
	for p.Top.Realm == initial || !p.LoadEventEnded() {
		if err := p.AdvanceTime(ctx, time.Millisecond); err != nil {
			t.Fatal(err)
		}
		if ctx.Err() != nil {
			t.Fatal("replacement did not finish")
		}
		time.Sleep(time.Millisecond)
	}
	value, err = p.EvaluateCommand(ctx, "", `newDocument===true&&typeof oldGlobal==='undefined'&&typeof oldJob==='undefined'&&({}).oldDocumentMarker===undefined&&document.getElementById('new').textContent==='replacement'`)
	if err != nil || value != true || p.Top.Realm.ID == initial.ID {
		t.Fatalf("replacement reused initial realm state: %v, %v", value, err)
	}
}

func TestNavigationCommitKeepsUnobservedInitialRuntimeDeferred(t *testing.T) {
	serialBrowserTest(t)
	p := bootstrapSnapshotPage(t)
	initial := p.Top.Realm
	deferred := initial.runtime.(*deferredRuntime)
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprint(w, `<body>committed<script>globalThis.answer=42</script></body>`)
	}))
	defer fixture.Close()
	committed := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.StartNavigation(ctx, fixture.URL, p.ReserveNavigation(), func(err error) { committed <- err }); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case err := <-committed:
			if err != nil {
				t.Fatal(err)
			}
			if p.Top.Realm == initial || initial.runtime != deferred || !deferred.closed {
				t.Fatal("commit unnecessarily materialized or retained the initial JS runtime")
			}
			value, err := p.EvaluateCommand(ctx, "", "answer")
			if err != nil || value != int64(42) && value != float64(42) {
				t.Fatalf("committed script: %v %v", value, err)
			}
			return
		default:
		}
		if err := p.AdvanceTime(ctx, time.Millisecond); err != nil {
			t.Fatal(err)
		}
		if ctx.Err() != nil {
			t.Fatal("navigation did not commit")
		}
		time.Sleep(time.Millisecond)
	}
}

// Opt-in attribution of normal bootstrap stages: no host/CPU profiler, no
// altered snapshot policy, and timing covers teardown as well as construction.
func TestBootstrapInitializationStages(t *testing.T) {
	serialBrowserTest(t)
	if os.Getenv("MIMIC_BOOTSTRAP_STAGE_DIAGNOSTIC") != "1" {
		t.Skip("set MIMIC_BOOTSTRAP_STAGE_DIAGNOSTIC=1 for bootstrap attribution")
	}
	seed := bootstrapSnapshotPage(t)
	workload := os.Getenv("MIMIC_BOOTSTRAP_STAGE_WORKLOAD")
	var workloadSource []byte
	if workload != "" {
		if workload != "static" && workload != "dom" {
			t.Fatal("stage workload must be static or dom")
		}
		var err error
		workloadSource, err = os.ReadFile("../../benchmark/fixtures/workload.js")
		if err != nil {
			t.Fatal(err)
		}
	}
	for wave := 0; wave < 6; wave++ {
		for _, secure := range []bool{false, true} {
			p, err := seed.ctx.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			r := p.Top.Realm
			// These two fixtures exercise the actual initial-document and secure
			// navigation profiles without adding network variance to attribution.
			if secure {
				u, _ := url.Parse(fmt.Sprintf("https://bootstrap-%d.test/", wave))
				p.current, r.url, r.origin = u, u, originOf(u.String())
				p.documentSecurity.secureContext = true
			}
			start := time.Now()
			plan := r.bootstrapSource()
			sourceTime := time.Since(start)
			cache := &p.ctx.browser.bootstrapSnapshots
			cache.mu.Lock()
			cached, snapshotBytes := false, 0
			if entry := cache.entries[plan.key]; entry != nil && entry.snapshot != nil {
				cached, snapshotBytes = true, entry.snapshot.SizeBytes()
			}
			cache.mu.Unlock()
			deferred := r.runtime.(*deferredRuntime)
			start = time.Now()
			r.runtime, err = r.newRuntime()
			runtimeTime := time.Since(start)
			if err != nil {
				t.Fatal(err)
			}
			r.runtime.SetTimeSource(deferred.now)
			r.runtime.SetGlobalAccessObserver(deferred.observer)
			start = time.Now()
			err = r.install()
			installTime := time.Since(start)
			if err != nil {
				t.Fatal(err)
			}
			r.registerPermissions()
			start = time.Now()
			value, err := p.EvaluateCommand(context.Background(), "", "document.defaultView===window")
			observeTime := time.Since(start)
			if err != nil || value != true {
				t.Fatalf("installed realm: %v, %v", value, err)
			}
			var workloadTime time.Duration
			if secure && workload != "" {
				start = time.Now()
				result, err := p.EvaluateCommand(context.Background(), "", fmt.Sprintf("document.body.setAttribute('data-workload',%q);document.body.innerHTML='<div id=root>baseline</div>';\n%s\n__benchRun().then(()=>__bench)", workload, workloadSource))
				workloadTime = time.Since(start)
				if err != nil {
					t.Fatal(err)
				}
				outcome, ok := result.(map[string]any)
				if !ok || outcome["done"] != true || outcome["error"] != nil {
					t.Fatalf("frozen workload failed: %v", result)
				}
			}
			start = time.Now()
			err = p.Close()
			closeTime := time.Since(start)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("wave=%d secure=%v key=%x cached=%v snapshot_bytes=%d source_bytes=%d source=%s runtime=%s install=%s observe=%s workload=%s first_use=%s close=%s", wave, secure, plan.key[:8], cached, snapshotBytes, len(plan.source), sourceTime, runtimeTime, installTime, observeTime, workload, workloadTime, closeTime)
		}
		if err := seed.ctx.browser.bootstrapSnapshots.wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

// This paired diagnostic compares the same restored graph and engine build.
// The only variable is whether the existing synchronous installation crosses
// the actor boundary once or separately for each engine operation.
func BenchmarkBootstrapBindingsOwner(b *testing.B) {
	for _, batched := range []bool{false, true} {
		b.Run(fmt.Sprintf("batched=%v", batched), func(b *testing.B) {
			browser, err := New(v8engine.Factory{}, chrome152.New())
			if err != nil {
				b.Fatal(err)
			}
			c := browser.NewContext()
			defer c.Close()
			for range 2 {
				p, err := c.NewPage()
				if err != nil {
					b.Fatal(err)
				}
				if _, err := p.EvaluateCommand(context.Background(), "", "true"); err != nil {
					b.Fatal(err)
				}
				p.Close()
			}
			if err := c.browser.bootstrapSnapshots.wait(context.Background()); err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for range b.N {
				b.StopTimer()
				p, err := c.NewPage()
				if err != nil {
					b.Fatal(err)
				}
				r := p.Top.Realm
				deferred := r.runtime.(*deferredRuntime)
				r.runtime, err = r.newRuntime()
				if err != nil {
					b.Fatal(err)
				}
				r.runtime.SetTimeSource(deferred.now)
				r.runtime.SetGlobalAccessObserver(deferred.observer)
				if !r.bootstrapRestored {
					b.Fatal("benchmark requires an ordinary warmed bootstrap snapshot")
				}
				b.StartTimer()
				if batched {
					err = r.installBindings()
				} else {
					err = r.installBindingsOnOwner()
				}
				b.StopTimer()
				if err != nil {
					b.Fatal(err)
				}
				if value, err := p.EvaluateCommand(context.Background(), "", "document.defaultView===window&&Array.isArray([])"); err != nil || value != true {
					b.Fatalf("invalid installed realm: %v, %v", value, err)
				}
				p.Close()
			}
		})
	}
}
