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

func TestResourceTimingDoesNotLeakParentInflightFetch(t *testing.T) {
	serialBrowserTest(t)
	source, err := os.ReadFile("../../compatibility/corpus/resource-realm-isolation.js")
	if err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("scope") == "parent" {
				time.Sleep(700 * time.Millisecond)
			}
			fmt.Fprint(w, "<!doctype html><body></body>")
		}))
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := p.Navigate(ctx, server.URL); err != nil {
			t.Fatal(err)
		}
		value, err := p.Evaluate(ctx, "("+string(source)+").then(v=>JSON.stringify(v))")
		if err != nil {
			t.Fatal(err)
		}
		var result map[string][]string
		if err := json.Unmarshal([]byte(value.(string)), &result); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(result["parent"], []string{"parent"}) || len(result["child"]) != 0 {
			t.Fatalf("resource ownership: %s", value)
		}
	})
}
