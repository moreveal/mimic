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

func TestFrameTasksDoNotStarveParentMessages(t *testing.T) {
	testFrameTaskInterleaving(t, false)
}

func TestFrameTasksInterleaveDuringPageAdvanceTime(t *testing.T) {
	testFrameTaskInterleaving(t, true)
}

func testFrameTaskInterleaving(t *testing.T, pump bool) {
	source, err := os.ReadFile("../../compatibility/corpus/frame-task-interleaving.js")
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
		var value any
		if pump {
			_, err = p.Evaluate(ctx, "window.frameTaskResult=null;(async()=>{await new Promise(r=>setTimeout(r,100));return await ("+string(source)+");})().then(v=>window.frameTaskResult=v);'scheduled'")
			if err == nil {
				err = p.AdvanceTime(ctx, 200*time.Millisecond)
			}
			if err == nil {
				value, err = p.Evaluate(ctx, "JSON.stringify(window.frameTaskResult)")
			}
		} else {
			value, err = p.Evaluate(ctx, "("+string(source)+").then(v=>JSON.stringify(v))")
		}
		if err != nil {
			t.Fatal(err)
		}
		var rows []struct{ Sent, Observed int }
		if err := json.Unmarshal([]byte(value.(string)), &rows); err != nil {
			t.Fatal(err)
		}
		if len(rows) != 5 {
			t.Fatalf("missing messages: %s", value)
		}
		for i, row := range rows {
			if row.Sent != i+1 || row.Observed != i+1 {
				t.Fatalf("child tasks overtook parent delivery: %s", value)
			}
		}
	})
}
