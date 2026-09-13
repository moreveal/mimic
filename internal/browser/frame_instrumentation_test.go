package browser

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestFrameImportDoesNotInvokePublicReflection(t *testing.T) {
	source, err := os.ReadFile("testdata/frame_import_reflection_oracle.js")
	if err != nil {
		t.Fatal(err)
	}
	historyTestPages(t, func(t *testing.T, p *Page) {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		got, err := p.Evaluate(ctx, string(source))
		if err != nil || got != `{"answer":43,"privateReads":0}` {
			t.Fatalf("public reflection entered by private frame import: %v, %v", got, err)
		}
	})
}
