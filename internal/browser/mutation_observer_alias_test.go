package browser

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMutationObserverLegacyAliasUsesCanonicalConstructor(t *testing.T) {
	source, err := os.ReadFile("testdata/mutation_observer_alias_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		got, err := p.Evaluate(ctx, string(source))
		if err != nil || got != `{"same":true,"brand":true,"records":["legacy:data-value","canonical:data-value"]}` {
			t.Fatalf("legacy observer identity/delivery: %v, %v", got, err)
		}
	})
}
