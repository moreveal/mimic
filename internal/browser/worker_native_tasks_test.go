package browser

import (
	"context"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestV8WorkerNativeWasmCompletesWithoutUnrelatedTasks(t *testing.T) {
	serialBrowserTest(t)
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	got, err := p.Evaluate(ctx, `new Promise((resolve,reject)=>{
 const source = "WebAssembly.instantiate(new Uint8Array([0,97,115,109,1,0,0,0,1,7,1,96,2,127,127,1,127,3,2,1,0,7,7,1,3,97,100,100,0,0,10,9,1,7,0,32,0,32,1,106,11])).then(m=>postMessage(m.instance.exports.add(19,23)))";
 const url=URL.createObjectURL(new Blob([source],{type:'text/javascript'}));
 const worker=new Worker(url);
 worker.onmessage=e=>{worker.terminate();URL.revokeObjectURL(url);resolve(e.data)};
 worker.onerror=e=>reject(e.message);
})`)
	t.Logf("worker async compilation and delivery: %s", time.Since(start))
	var workerStart time.Time
	for _, event := range p.Trace().Events() {
		if event.Name == "scriptStart" && event.Data["worker"] != nil {
			workerStart = event.Time
		}
		if event.Name == "workerMessageQueued" && event.Data["direction"] == "worker-to-parent" && !workerStart.IsZero() {
			t.Logf("worker script start to async result (excluding bootstrap): %s", event.Time.Sub(workerStart))
		}
	}
	if err != nil || got != float64(42) {
		t.Fatalf("native worker compilation: %v, %v", got, err)
	}
}
