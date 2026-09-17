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
)

func TestRetainedHistoryMatchesFrozenChrome(t *testing.T) {
	source, err := os.ReadFile("testdata/history_retained_realm_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/history_retained_realm_chrome152.json")
	if err != nil {
		t.Fatal(err)
	}
	var oracle struct {
		Result struct {
			Result struct{ Value map[string]any }
		}
	}
	if err = json.Unmarshal(data, &oracle); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "<!doctype html><body>") }))
	defer server.Close()
	historyTestPages(t, func(t *testing.T, p *Page) {
		if err := p.Navigate(context.Background(), server.URL); err != nil {
			t.Fatal(err)
		}
		value, err := p.Evaluate(context.Background(), "(async()=>JSON.stringify(await "+string(source)+"))()")
		if err != nil {
			t.Fatal(err)
		}
		var actual map[string]any
		if err = json.Unmarshal([]byte(value.(string)), &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, oracle.Result.Result.Value) {
			t.Fatalf("retained History: got %s; want %#v", value, oracle.Result.Result.Value)
		}
	})
}
