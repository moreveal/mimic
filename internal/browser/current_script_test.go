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

// Chrome 152 retains currentScript through the script cleanup checkpoint,
// including after a throw, but clears it before timers and script load events.
func TestClassicScriptCleanupCurrentScript(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		if req.URL.Path == "/dynamic.js" {
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `record('dynamic');Promise.resolve().then(()=>record('dynamic-then'));`)
			return
		}
		fmt.Fprint(w, `<!doctype html><body><script id="entry">
window.observations=[];window.record=label=>observations.push(label+':'+(document.currentScript?.id||'null'));
record('sync');Promise.resolve().then(()=>record('then'));
(async()=>{await 0;record('await');await 0;record('await-again')})();
setTimeout(()=>record('timer'),0);
</script><script id="throwing">Promise.resolve().then(()=>record('after-throw'));throw Error('intentional');</script>
<script id="following">record('following');</script></body>`)
	}))
	defer server.Close()
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	p, err := b.NewContext().NewPage()
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	value, err := p.Evaluate(ctx, `new Promise(resolve=>{const s=document.createElement('script');s.id='dynamic';s.src='/dynamic.js';s.onload=()=>{record('load');resolve(observations.join('|'))};document.head.appendChild(s)})`)
	if err != nil {
		t.Fatal(err)
	}
	observations := fmt.Sprint(value)
	for _, expected := range []string{"sync:entry", "then:entry", "await:entry", "await-again:entry", "after-throw:throwing", "following:following", "timer:null", "dynamic:dynamic", "dynamic-then:dynamic", "load:null"} {
		if !strings.Contains("|"+observations+"|", "|"+expected+"|") {
			t.Fatalf("missing %s in %s", expected, observations)
		}
	}
	if !strings.Contains(observations, "dynamic:dynamic|dynamic-then:dynamic|load:null") {
		t.Fatalf("script load overtook cleanup checkpoint: %s", observations)
	}
	value, err = p.Evaluate(ctx, `document.currentScript`)
	if err != nil || value != nil {
		t.Fatalf("script leaked into evaluation: value=%v err=%v", value, err)
	}
}

func TestDynamicInlineScriptHasNoResourceLoadEvent(t *testing.T) {
	p := testPage(t)
	value, err := p.Evaluate(context.Background(), `new Promise(resolve=>{const events=[];const script=document.createElement('script');script.text='globalThis.inlineProbeRan=true';script.onload=()=>events.push('load');script.onerror=()=>events.push('error');document.body.append(script);setTimeout(()=>resolve({ran:globalThis.inlineProbeRan,events:events.join(',')}),30)})`)
	if err != nil {
		t.Fatal(err)
	}
	result := value.(map[string]any)
	if result["ran"] != true || result["events"] != "" {
		t.Fatalf("inline completion: %#v", result)
	}
}
