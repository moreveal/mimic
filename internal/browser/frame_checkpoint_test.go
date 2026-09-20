package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestNativeFrameMicrotasksBeforeNextTask(t *testing.T) {
	serialBrowserTest(t)
	source, err := os.ReadFile("../../compatibility/corpus/frame-microtask-checkpoint.js")
	if err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, "<!doctype html><body></body>")
		}))
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		if _, native := p.Top.Realm.runtime.(interface {
			MicrotaskCheckpointContext(context.Context) error
		}); !native {
			t.Skip("Goja drains jobs when a nested RunScript returns; deferring cross-runtime jobs requires engine support")
		}
		value, err := p.Evaluate(ctx, "("+string(source)+").then(v=>JSON.stringify(v))")
		if err != nil {
			t.Fatal(err)
		}
		var results map[string][]string
		if err := json.Unmarshal([]byte(value.(string)), &results); err != nil {
			t.Fatal(err)
		}
		for _, mode := range []string{"eval", "getter", "call", "construct", "throw"} {
			if !reflect.DeepEqual(results[mode], []string{"parent-sync", "child-job", "parent-timer"}) {
				t.Fatalf("cross-realm %s checkpoint: %s", mode, value)
			}
		}
	})
}
