package browser

import (
	"context"
	_ "embed"
	"testing"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

//go:embed testdata/snapshot_dom_regressions.js
var snapshotDOMRegressions string

func TestSnapshotDOMCachesObserveMutations(t *testing.T) {
	parallelBrowserTest(t)
	b, err := New(v8engine.Factory{}, chrome152.New())
	if err != nil {
		t.Fatal(err)
	}
	c := b.NewContext()
	defer c.Close()
	p, err := c.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.Evaluate(context.Background(), snapshotDOMRegressions)
	if err != nil || got != "ok" {
		t.Fatalf("DOM/cache regressions: %v, %v", got, err)
	}
}
