package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// This opt-in ownership soak is separate from latency benchmarks. Natural and
// explicitly collected samples are recorded separately; no production GC policy
// is changed. The fixed DOM is reused so retained canonical nodes are not
// confused with temporary callback/value ownership.
func TestPageLifetimeSoak(t *testing.T) {
	dir := os.Getenv("MIMIC_LIFETIME_SOAK_DIR")
	if dir == "" {
		t.Skip("set MIMIC_LIFETIME_SOAK_DIR for the long-lived Page ownership soak")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, 64<<10)
	body[0], body[len(body)-1] = 17, 29
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/body" {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(body)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><p id="stable">stable</p>`)
	}))
	defer server.Close()
	browser, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	owner := browser.NewContext()
	defer owner.Close()
	p, err := owner.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	engine := p.Top.Realm.runtime
	diagnostic := engine.(interface{ Diagnostics() (any, error) })
	collector := engine.(interface{ ProfileCollect() error })
	started := time.Now()
	var samples []map[string]any
	defer func() {
		encoded, err := json.MarshalIndent(map[string]any{
			"iterations": 2048, "warmup": 256, "body_bytes": len(body),
			"policy":  "one Page; fixed DOM; 16 listeners, 16 canceled timers, one completed timer, one 64 KiB no-store Fetch and Promise evaluation per iteration; 40 ms pacing; full trace retained; explicit collection only at labeled checkpoints",
			"samples": samples,
		}, "", "  ")
		if err == nil {
			err = os.WriteFile(filepath.Join(dir, "raw.json"), encoded, 0644)
		}
		if err != nil {
			t.Error(err)
		}
	}()
	sample := func(phase string, iteration int, live bool) {
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		row := map[string]any{"phase": phase, "iteration": iteration, "elapsed_ms": float64(time.Since(started)) / 1e6,
			"go_heap": memory.HeapAlloc, "go_total_alloc": memory.TotalAlloc, "go_gc_cycles": memory.NumGC,
			"goroutines": runtime.NumGoroutine(), "body_storage": owner.network.BodyStorageStats()}
		if profileProcessMemory != nil {
			row["process"] = profileProcessMemory(t)
		}
		if p != nil {
			row["trace_events"] = len(p.Trace().Events())
		}
		if live {
			value, err := diagnostic.Diagnostics()
			if err != nil {
				t.Fatal(err)
			}
			row["v8"], row["timers"] = value, len(p.Top.Realm.timers)
		}
		samples = append(samples, row)
	}
	const source = `(async()=>{
		let count=0;const target=new EventTarget();
		for(let i=0;i<16;i++){
			const payload=new Uint8Array(4096);payload[0]=1;
			const callback=()=>count+=payload[0];
			target.addEventListener('probe',callback);target.dispatchEvent(new Event('probe'));
			target.removeEventListener('probe',callback);
			clearTimeout(setTimeout(()=>payload[0],60000));
		}
		await new Promise(resolve=>setTimeout(resolve,0));
		const bytes=new Uint8Array(await (await fetch('/body')).arrayBuffer());
		document.getElementById('stable').setAttribute('data-count',String(count));
		return await Promise.resolve(count===16&&bytes.length===65536&&bytes[0]===17&&bytes[65535]===29);
	})()`
	sample("ready", 0, true)
	baseline := -1
	for iteration := 1; iteration <= 2048; iteration++ {
		value, err := p.Evaluate(ctx, source)
		if err != nil || value != true {
			t.Fatalf("iteration %d: %v %v", iteration, value, err)
		}
		// Keep one Page alive across real timer deadlines as well as thousands
		// of operations. Pacing is a soak policy, never a latency measurement.
		time.Sleep(40 * time.Millisecond)
		if iteration%256 != 0 {
			continue
		}
		sample("natural", iteration, true)
		if baseline < 0 {
			baseline = persistentHandleCount(t, engine)
		}
		if handles := persistentHandleCount(t, engine); handles != baseline || len(p.Top.Realm.timers) != 0 {
			t.Fatalf("iteration %d: handles=%d baseline=%d timers=%d", iteration, handles, baseline, len(p.Top.Realm.timers))
		}
		if err := collector.ProfileCollect(); err != nil {
			t.Fatal(err)
		}
		debug.FreeOSMemory()
		sample("collected", iteration, true)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	sample("page_closed", 2048, false)
	if owner.network.BodyStorageStats().Bodies != 0 {
		t.Fatal("closed no-store Page retained response bodies")
	}
	owner.Close()
	if _, err := diagnostic.Diagnostics(); err == nil {
		t.Fatal("closed Page still exposes a live isolate")
	}
	// Embedders may inspect a closed Page's trace. Release those caller-owned
	// Go references before measuring full reclamation of its retained history.
	p, engine, diagnostic, collector = nil, nil, nil, nil
	debug.FreeOSMemory()
	sample("context_closed_collected", 2048, false)
}
