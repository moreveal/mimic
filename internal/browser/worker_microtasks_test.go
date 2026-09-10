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

func TestWorkerMicrotaskChromeRelations(t *testing.T) {
	source, err := os.ReadFile("../../compatibility/corpus/worker-microtasks.js")
	if err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<!doctype html><body></body>") }))
		defer server.Close()
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		value, err := p.Evaluate(ctx, "("+string(source)+").then(value=>JSON.stringify(value))")
		if err != nil {
			events := p.Trace().Events()
			for _, event := range events[max(0, len(events)-22):] {
				t.Logf("%+v", event)
			}
			t.Fatal(err)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(value.(string)), &result); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result["order"], []any{"sync", "microtask", "promise", "after-error", "intrinsic", "nested"}) {
			t.Fatalf("%s", value)
		}
		if result["thisGlobal"] != false || result["thisUndefined"] != true || result["thenRead"] != float64(0) || result["poisonedPromise"] != "accepted" {
			t.Fatalf("%s", value)
		}
		errors, ok := result["errors"].([]any)
		if !ok || len(errors) != 1 {
			t.Fatalf("%s", value)
		}
		event := errors[0].(map[string]any)
		for _, key := range []string{"identity", "trusted", "cancelable", "prevented"} {
			if event[key] != true {
				t.Fatalf("%s: %s", key, value)
			}
		}
		if event["message"] != "Uncaught Error: microtask sentinel" {
			t.Fatalf("%s", value)
		}
		if t.Name() == "TestWorkerMicrotaskChromeRelations/v8" {
			for _, key := range []string{"filename", "line", "column"} {
				if event[key] != true {
					t.Fatalf("native %s: %s", key, value)
				}
			}
		}
		invalid := result["invalid"].([]any)
		if len(invalid) != 9 || !reflect.DeepEqual(invalid[7], []any{"TypeError", "Illegal invocation"}) {
			t.Fatalf("%s", value)
		}
	})
}
