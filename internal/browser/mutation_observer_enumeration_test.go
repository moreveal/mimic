package browser

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestMutationObserverOperationsCanBeInstrumentedByEnumeration(t *testing.T) {
	serialBrowserTest(t)
	source, err := os.ReadFile("testdata/mutation_observer_enumeration_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		got, err := p.Evaluate(ctx, string(source))
		if err != nil || got != `{"methods":["disconnect","observe","takeRecords"],"flags":[["disconnect",true,true,true],["observe",true,true,true],["takeRecords",true,true,true]],"attribute":"data-value"}` {
			t.Fatalf("instrumented observer enumeration/delivery: %v, %v", got, err)
		}
	})
}
