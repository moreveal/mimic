package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

func TestModuleThrowIsTracedWithoutStoppingOtherScripts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad.js" {
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `throw new Error('module failure sentinel')`)
			return
		}
		fmt.Fprint(w, `<script type="module" src="/bad.js"></script><script type="module">globalThis.nextModule=true</script>`)
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
	if err := p.Navigate(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(context.Background(), `globalThis.nextModule===true`)
	if err != nil || value != true {
		t.Fatalf("next module: %v %v", value, err)
	}
	found := false
	for _, event := range p.trace.Events() {
		if string(event.Kind) == "exception" && strings.Contains(fmt.Sprint(event.Data), "module failure sentinel") {
			found = true
		}
	}
	if !found {
		t.Fatal("module rejection absent from exception trace")
	}
}

func TestModulePendingEvaluationDoesNotPumpMicrotasks(t *testing.T) {
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
	r := p.Top.Realm
	v, err := r.EvaluateModule(context.Background(), `globalThis.moduleJobRan=false;
Promise.resolve().then(()=>moduleJobRan=true);await new Promise(()=>{});`,
		"https://example.test/pending.js", func(string, string) (string, string, error) { return "", "", fmt.Errorf("unexpected import") })
	if err != nil {
		t.Fatal(err)
	}
	if _, settled, err := r.runtime.Await(v); err != nil || settled {
		t.Fatalf("pending: settled=%v err=%v", settled, err)
	}
	if r.runtime.Get("moduleJobRan").Export() != false {
		t.Fatal("module diagnostics pumped microtasks")
	}
	if err := r.runtime.MicrotaskCheckpoint(); err != nil {
		t.Fatal(err)
	}
	if r.runtime.Get("moduleJobRan").Export() != true {
		t.Fatal("explicit checkpoint did not run queued job")
	}
}
