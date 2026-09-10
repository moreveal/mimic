package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestResourceTimingJSONUsesInternalValues(t *testing.T) {
	source, err := os.ReadFile("../../compatibility/corpus/resource-json.js")
	if err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<!doctype html><body></body>") }))
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
		var result struct {
			Present  [][]any `json:"present"`
			Override string  `json:"override"`
			Brand    string  `json:"brand"`
		}
		if err := json.Unmarshal([]byte(value.(string)), &result); err != nil {
			t.Fatal(err)
		}
		if result.Override != "ignored" || result.Brand != "TypeError" || len(result.Present) != 6 {
			t.Fatalf("%s", value)
		}
		for _, field := range result.Present {
			if field[1] != true || field[2] != true {
				t.Fatalf("%s", value)
			}
		}
	})
}
