package browser

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// A neutral application must leave its loading state after either an empty
// or a populated JSON response. This tests delivery, promises and DOM updates.
func TestV8FetchJSONCompletesRenderedState(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"empty", `[]`, "Empty"},
		{"populated", `[{"label":"Ready"}]`, "Ready"},
		{"invalid", `{`, "Error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/data" {
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprint(w, tc.body)
					return
				}
				w.Header().Set("Content-Type", "text/html")
				fmt.Fprint(w, `<div id="result">Loading</div><script>
				window.finished=fetch('/data').then(r=>r.json()).then(rows=>{
				 document.getElementById('result').textContent=rows.length?rows[0].label:'Empty';
				}).catch(()=>{document.getElementById('result').textContent='Error'});
				</script>`)
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
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := p.Navigate(ctx, server.URL); err != nil {
				t.Fatal(err)
			}
			value, err := p.Evaluate(ctx, `finished.then(()=>document.getElementById('result').textContent)`)
			if err != nil {
				t.Fatal(err)
			}
			if value != tc.want {
				t.Fatalf("rendered state = %v, want %q", value, tc.want)
			}
		})
	}
}
