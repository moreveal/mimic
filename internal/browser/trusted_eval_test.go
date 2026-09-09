package browser

import (
	"context"
	"os"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestTrustedScriptEvalPreservesNativeScopeAndBrand(t *testing.T) {
	source, err := os.ReadFile("testdata/trusted_eval.js")
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	// Two independent Pages must each resolve only their own branded values.
	for i := 0; i < 2; i++ {
		browserContext := b.NewContext()
		t.Cleanup(func() { _ = browserContext.Close() })
		p, err := browserContext.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		value, err := p.Evaluate(context.Background(), string(source))
		if err != nil {
			t.Fatal(err)
		}
		got := value.(map[string]any)
		for key, expected := range map[string]int{"direct": 8, "ordinary": 9, "localFunction": 19, "globalFunction": 23, "tamperedResult": 42, "intrinsicResult": 42} {
			if numberValue(got[key]) != float64(expected) {
				t.Errorf("%s = %v, want %d", key, got[key], expected)
			}
		}
		for _, key := range []string{"localStayedLocal", "object", "boxed", "html", "forged", "proxy", "number", "empty", "brand", "sameException", "syntaxError", "resolverHidden"} {
			if got[key] != true {
				t.Errorf("%s = %v, want true", key, got[key])
			}
		}
		if _, err := p.Evaluate(context.Background(), `setTimeout(()=>{const policy=trustedTypes.createPolicy('timer-eval',{createScript:s=>s});let local=30;globalThis.timerEval=eval(policy.createScript('local+12'))},0)`); err != nil {
			t.Fatal(err)
		}
		if err := p.Top.Realm.RunUntilIdle(context.Background()); err != nil {
			t.Fatal(err)
		}
		value, err = p.Evaluate(context.Background(), `timerEval`)
		if err != nil || numberValue(value) != 42 {
			t.Fatalf("timer eval = %v, error = %v", value, err)
		}
	}
}

func TestWorkerTrustedScriptEval(t *testing.T) {
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.NewContext().NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	value, err := p.Evaluate(ctx, `new Promise((resolve,reject)=>{
 const source="const p=trustedTypes.createPolicy('worker-eval',{createScript:s=>s});(0,eval)(p.createScript('function workerDeclared(){return 42}'));postMessage({answer:workerDeclared(),hidden:!Object.hasOwn(globalThis,'__mimicEvalSourceResolver')})";
 const url=URL.createObjectURL(new Blob([source],{type:'text/javascript'})),worker=new Worker(url);
 worker.onmessage=e=>{worker.terminate();URL.revokeObjectURL(url);resolve(e.data)};
 worker.onerror=e=>{worker.terminate();URL.revokeObjectURL(url);reject(new Error(e.message))};
})`)
	if err != nil {
		t.Fatal(err)
	}
	got := value.(map[string]any)
	if numberValue(got["answer"]) != 42 || got["hidden"] != true {
		t.Fatalf("worker eval = %#v", got)
	}
}
