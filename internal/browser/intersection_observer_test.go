package browser

import (
	"context"
	_ "embed"
	"encoding/json"
	"testing"
	"time"

	chrome152 "github.com/moreveal/mimic/chrome/152"
	v8engine "github.com/moreveal/mimic/internal/engine/v8"
)

// The original frozen fixture is retained. This independent successor corrects
// its static-position assertion using the frozen geometry receipt; see
// docs/compatibility/geometry-integration-20260912.md.
//
//go:embed testdata/intersection_observer_regressions_v2.js
var intersectionObserverRegressions string

func TestIntersectionObserverInitialAndChangedObservations(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	got, err := p.Evaluate(ctx, intersectionObserverRegressions)
	if err != nil || got != "ok" {
		t.Fatalf("intersection regressions: %v, %v", got, err)
	}
}

func TestUnchangedIntersectionSampleAvoidsGeometryWork(t *testing.T) {
	t.Setenv("MIMIC_PROFILE_HOSTS", "1")
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := p.Evaluate(ctx, `document.body.innerHTML='<div style="width:40px;height:20px"></div>';window.entries=[];new IntersectionObserver(e=>entries.push(...e)).observe(document.body.firstElementChild)`); err != nil {
		t.Fatal(err)
	}
	advance := func() {
		t.Helper()
		if err := p.AdvanceTime(ctx, 20*time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	count := func() uint64 {
		t.Helper()
		data, err := json.Marshal(p.LiveDiagnostics())
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Costs map[string]struct{ Count uint64 }
		}
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		// Canonical JS geometry replaced host:rect. Eligibility is queried only
		// when sampling boxes, so retain the same no-work/invalidation invariant.
		return result.Costs["host:computedStyleAvailable"].Count
	}
	advance()
	before := count()
	if before == 0 {
		t.Fatal("initial observation did not sample geometry")
	}
	for i := 0; i < 10; i++ {
		advance()
	}
	if after := count(); after != before {
		t.Fatalf("unchanged sample repeated geometry: %d -> %d", before, after)
	}
	if err := p.SetViewport(800, 600); err != nil {
		t.Fatal(err)
	}
	advance()
	if count() <= before {
		t.Fatal("viewport change did not invalidate observation")
	}
}
