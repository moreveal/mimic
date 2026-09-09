package browser

import (
	"context"
	"encoding/json"
	"fmt"
	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// Independent current-document fixtures compared against pinned Chrome 152.
func TestTemplateChrome152(t *testing.T) {
	read := func(name string) map[string]json.RawMessage {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", "compatibility", name))
		if err != nil {
			t.Fatal(err)
		}
		result := map[string]json.RawMessage{}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	probes := read("probes-templates.json")
	expected := read("probes-templates.expected.json")
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<!doctype html><title>Template semantics</title><body><template id="parsed-template"><span id="hidden-child">data</span><script>globalThis.__templateParserRan=(globalThis.__templateParserRan||0)+1</script></template>`)
	}))
	defer server.Close()
	c := b.NewContext()
	defer c.Close()
	for name, raw := range probes {
		t.Run(name, func(t *testing.T) {
			p, err := c.NewPage()
			if err != nil {
				t.Fatal(err)
			}
			var source string
			if err := json.Unmarshal(raw, &source); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := p.Navigate(ctx, server.URL); err != nil {
				t.Fatal(err)
			}
			value, err := p.Evaluate(ctx, "(async()=>JSON.stringify(await ("+source+")()))()")
			if err != nil {
				t.Fatal(err)
			}
			var want any
			if err := json.Unmarshal(expected[name], &want); err != nil {
				t.Fatal(err)
			}
			text, ok := value.(string)
			if !ok {
				t.Fatalf("unexpected result %T", value)
			}
			var got any
			if err := json.Unmarshal([]byte(text), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Chrome152=%v; Mimic=%v", want, got)
			}

		})
	}
}
