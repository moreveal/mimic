package browser

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// Expected values were independently captured from pinned Chrome 152. These
// single-realm cases isolate reaction semantics from unsupported iframe/XML APIs
// exercised by the unmodified WPT suite.
func TestMutationReactionChrome152(t *testing.T) {
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
	probes := read("probes-mutations-reactions.json")
	expected := read("probes-mutations-reactions.expected.json")
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
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
			value, err := p.Evaluate(ctx, "(async()=>JSON.stringify(await ("+source+")()))()")
			if err != nil {
				t.Fatal(err)
			}
			var want any
			if err := json.Unmarshal(expected[name], &want); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			if value != string(encoded) {
				t.Fatalf("Chrome152=%s; Mimic=%v", encoded, value)
			}
		})
	}
}
