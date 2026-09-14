package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDynamicModuleCachedSyntaxErrorKeepsOwner(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/invalid.js" {
			requests.Add(1)
			w.Header().Set("Content-Type", "text/javascript")
			fmt.Fprint(w, `export const value = ;`)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><script>globalThis.startImport=()=>import('/invalid.js')</script>`)
	}))
	defer server.Close()
	p := newAsyncModulePage(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err := p.Navigate(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	const source = `(async()=>{const order=[];const read=label=>startImport().catch(error=>{order.push(label);return error});const [first,concurrent]=await Promise.all([read('first'),read('concurrent')]);const cached=await read('cached');return JSON.stringify({names:[first.name,concurrent.name,cached.name],same:first===concurrent&&first===cached,order})})()`
	for iteration := 0; iteration < 4; iteration++ {
		before := persistentHandleCount(t, p.Top.Realm.runtime)
		value, err := p.Evaluate(ctx, source)
		if err != nil {
			t.Fatalf("cached module error iteration %d: %v %v", iteration, value, err)
		}
		var result struct {
			Names, Order []string
			Same         bool
		}
		encoded, ok := value.(string)
		if !ok || json.Unmarshal([]byte(encoded), &result) != nil || !result.Same || len(result.Names) != 3 || len(result.Order) != 3 {
			t.Fatalf("invalid cached module rejection result: %v", value)
		}
		for _, name := range result.Names {
			if name != "SyntaxError" {
				t.Fatalf("cached syntax error changed its name: %v", value)
			}
		}
		// Chrome 152 preserves the same SyntaxError across concurrent and
		// cached imports. The separately awaited third import follows both
		// concurrent rejections; their relative scheduling is not asserted here.
		pair := result.Order[0] + ":" + result.Order[1]
		if result.Order[2] != "cached" || pair != "first:concurrent" && pair != "concurrent:first" {
			t.Fatalf("cached import settled before the concurrent pair: %v", value)
		}
		t.Logf("iteration %d rejection order=%v", iteration, result.Order)
		if iteration > 0 && persistentHandleCount(t, p.Top.Realm.runtime) != before {
			t.Fatal("repeated cached imports retained completion temporaries")
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("cached invalid module was fetched %d times", requests.Load())
	}
}
